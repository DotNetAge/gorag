package neo4j

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type neo4jGraphStore struct {
	driver neo4j.DriverWithContext
	dbName string
}

// NewGraphStore creates a new Neo4j based graph store.
func NewGraphStore(uri, username, password, dbName string) (store.GraphStore, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	if dbName == "" {
		dbName = "neo4j"
	}

	return &neo4jGraphStore{
		driver: driver,
		dbName: dbName,
	}, nil
}

func (s *neo4jGraphStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.dbName})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, node := range nodes {
			// Using MERGE for idempotency. 
			// We use a generic 'Entity' label plus the specific node type as a label.
			query := fmt.Sprintf("MERGE (n:Entity:%s {id: $id}) SET n += $props", node.Type)
			_, err := tx.Run(ctx, query, map[string]any{
				"id":    node.ID,
				"props": node.Properties,
			})
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (s *neo4jGraphStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.dbName})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, edge := range edges {
			// In Cypher, relationship types cannot be parameterized directly in the MERGE clause easily 
			// without string concatenation or using APOC. 
			// We'll use string formatting for the type since it's controlled.
			query := fmt.Sprintf(`
				MATCH (s:Entity {id: $sourceID})
				MATCH (t:Entity {id: $targetID})
				MERGE (s)-[r:%s {id: $edgeID}]->(t)
				SET r += $props
			`, edge.Type)
			
			_, err := tx.Run(ctx, query, map[string]any{
				"sourceID": edge.Source,
				"targetID": edge.Target,
				"edgeID":   edge.ID,
				"props":    edge.Properties,
			})
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (s *neo4jGraphStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.dbName})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, "MATCH (n:Entity {id: $id}) RETURN n", map[string]any{"id": id})
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			record := res.Record()
			node, _ := record.Get("n")
			return node, nil
		}
		return nil, nil
	})

	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	neoNode := result.(neo4j.Node)
	
	// Extract types from labels (excluding 'Entity')
	nodeType := "Unknown"
	for _, label := range neoNode.Labels {
		if label != "Entity" {
			nodeType = label
			break
		}
	}

	return &core.Node{
		ID:         neoNode.Props["id"].(string),
		Type:       nodeType,
		Properties: neoNode.Props,
	}, nil
}

func (s *neo4jGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.dbName})
	defer session.Close(ctx)

	query := fmt.Sprintf(`
		MATCH (s:Entity {id: $id})
		MATCH (s)-[r*1..%d]-(n:Entity)
		RETURN DISTINCT n, r
		LIMIT $limit
	`, depth)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, map[string]any{
			"id":    nodeID,
			"limit": limit,
		})
		if err != nil {
			return nil, err
		}

		nodeMap := make(map[string]*core.Node)
		edgeMap := make(map[string]*core.Edge)

		for res.Next(ctx) {
			record := res.Record()
			
			// Handle Node
			if nVal, ok := record.Get("n"); ok && nVal != nil {
				n := nVal.(neo4j.Node)
				id := n.Props["id"].(string)
				if _, exists := nodeMap[id]; !exists {
					nodeType := "Unknown"
					for _, label := range n.Labels {
						if label != "Entity" {
							nodeType = label
							break
						}
					}
					nodeMap[id] = &core.Node{
						ID:         id,
						Type:       nodeType,
						Properties: n.Props,
					}
				}
			}

			// Handle Relationship(s) - r can be a list due to *1..depth
			if rVal, ok := record.Get("r"); ok && rVal != nil {
				// Cypher path returns r as a list of relationships
				rels, isList := rVal.([]any)
				if !isList {
					rels = []any{rVal}
				}

				for _, relAny := range rels {
					r := relAny.(neo4j.Relationship)
					edgeID, _ := r.Props["id"].(string)
					if edgeID == "" {
						edgeID = fmt.Sprintf("%d", r.ElementId)
					}
					
					if _, exists := edgeMap[edgeID]; !exists {
						// We need to resolve source and target node IDs
						// This might require additional MATCH if element IDs are not enough, 
						// but in our schema source/target are Entity nodes with 'id' prop.
						edgeMap[edgeID] = &core.Edge{
							ID:         edgeID,
							Type:       r.Type,
							// Note: Simplified. Actual source/target resolution might need ElementId mapping
							Properties: r.Props,
						}
					}
				}
			}
		}
		
		nodes := make([]*core.Node, 0, len(nodeMap))
		for _, n := range nodeMap {
			nodes = append(nodes, n)
		}
		edges := make([]*core.Edge, 0, len(edgeMap))
		for _, e := range edgeMap {
			edges = append(edges, e)
		}

		return [2]any{nodes, edges}, nil
	})

	if err != nil {
		return nil, nil, err
	}

	resArr := result.([2]any)
	return resArr[0].([]*core.Node), resArr[1].([]*core.Edge), nil
}

func (s *neo4jGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.dbName})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var records []map[string]any
		for res.Next(ctx) {
			records = append(records, res.Record().AsMap())
		}
		return records, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]map[string]any), nil
}

func (s *neo4jGraphStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	// Custom implementation for Microsoft GraphRAG pattern in Neo4j
	query := "MATCH (c:Community {level: $level}) RETURN c.id as id, c.summary as summary, c.nodes as nodes"
	return s.Query(ctx, query, map[string]any{"level": level})
}

func (s *neo4jGraphStore) Close(ctx context.Context) error {
	return s.driver.Close(ctx)
}
