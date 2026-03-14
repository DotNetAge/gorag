//go:build neo4j
// +build neo4j

package graphstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jGraphStore implements GraphStore using Neo4J
type Neo4jGraphStore struct {
	driver neo4j.DriverWithContext
}

// NewNeo4jGraphStore creates a new Neo4jGraphStore
func NewNeo4jGraphStore() *Neo4jGraphStore {
	return &Neo4jGraphStore{}
}

// Initialize initializes the Neo4J connection
func (n *Neo4jGraphStore) Initialize(options map[string]interface{}) error {
	// Extract connection details from options
	uri, ok := options["uri"].(string)
	if !ok {
		return fmt.Errorf("uri is required")
	}

	username, ok := options["username"].(string)
	if !ok {
		return fmt.Errorf("username is required")
	}

	password, ok := options["password"].(string)
	if !ok {
		return fmt.Errorf("password is required")
	}

	// Create Neo4J driver
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	// Verify connection
	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("failed to verify connectivity: %w", err)
	}

	n.driver = driver
	return nil
}

// CreateNode creates a new node in Neo4J
func (n *Neo4jGraphStore) CreateNode(ctx context.Context, node *abstraction.Node) error {
	query := `
		CREATE (n:` + node.Type + ` {id: $id`

	// Add properties
	for key := range node.Properties {
		query += `, ` + key + `: $` + key
	}

	query += `})`

	// Prepare parameters
	params := map[string]interface{}{
		"id": node.ID,
	}

	for key, val := range node.Properties {
		params[key] = val
	}

	// Execute query
	_, err := n.executeQuery(ctx, query, params)
	return err
}

// CreateEdge creates a new edge in Neo4J
func (n *Neo4jGraphStore) CreateEdge(ctx context.Context, edge *abstraction.Edge) error {
	query := `
		MATCH (s {id: $source})
		MATCH (t {id: $target})
		CREATE (s)-[r:` + edge.Type + ` {id: $id`

	// Add properties
	for key := range edge.Properties {
		query += `, ` + key + `: $` + key
	}

	query += `}]->(t)`

	// Prepare parameters
	params := map[string]interface{}{
		"id":     edge.ID,
		"source": edge.Source,
		"target": edge.Target,
	}

	for key, val := range edge.Properties {
		params[key] = val
	}

	// Execute query
	_, err := n.executeQuery(ctx, query, params)
	return err
}

// GetNode retrieves a node by its ID
func (n *Neo4jGraphStore) GetNode(ctx context.Context, id string) (*abstraction.Node, error) {
	query := `
		MATCH (n {id: $id})
		RETURN n
	`

	params := map[string]interface{}{
		"id": id,
	}

	result, err := n.executeQuery(ctx, query, params)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	nodeData := result[0]["n"].(neo4j.Node)
	return n.mapToNode(nodeData)
}

// GetEdge retrieves an edge by its ID
func (n *Neo4jGraphStore) GetEdge(ctx context.Context, id string) (*abstraction.Edge, error) {
	query := `
		MATCH ()-[r {id: $id}]->()
		RETURN r, startNode(r) as source, endNode(r) as target
	`

	params := map[string]interface{}{
		"id": id,
	}

	result, err := n.executeQuery(ctx, query, params)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	edgeData := result[0]["r"].(neo4j.Relationship)
	sourceNode := result[0]["source"].(neo4j.Node)
	targetNode := result[0]["target"].(neo4j.Node)

	return n.mapToEdge(edgeData, sourceNode, targetNode)
}

// DeleteNode deletes a node from Neo4J
func (n *Neo4jGraphStore) DeleteNode(ctx context.Context, id string) error {
	query := `
		MATCH (n {id: $id})
		DETACH DELETE n
	`

	params := map[string]interface{}{
		"id": id,
	}

	_, err := n.executeQuery(ctx, query, params)
	return err
}

// DeleteEdge deletes an edge from Neo4J
func (n *Neo4jGraphStore) DeleteEdge(ctx context.Context, id string) error {
	query := `
		MATCH ()-[r {id: $id}]->()
		DELETE r
	`

	params := map[string]interface{}{
		"id": id,
	}

	_, err := n.executeQuery(ctx, query, params)
	return err
}

// Query executes a graph query
func (n *Neo4jGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	return n.executeQuery(ctx, query, params)
}

// GetNeighbors retrieves the neighbors of a node
func (n *Neo4jGraphStore) GetNeighbors(ctx context.Context, nodeID string, limit int) ([]*abstraction.Node, error) {
	query := `
		MATCH (n {id: $id})--(m)
		RETURN m
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	params := map[string]interface{}{
		"id": nodeID,
	}

	result, err := n.executeQuery(ctx, query, params)
	if err != nil {
		return nil, err
	}

	nodes := make([]*abstraction.Node, 0, len(result))
	for _, row := range result {
		nodeData := row["m"].(neo4j.Node)
		node, err := n.mapToNode(nodeData)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetCommunitySummaries retrieves community summaries
func (n *Neo4jGraphStore) GetCommunitySummaries(ctx context.Context, limit int) ([]map[string]any, error) {
	// This is a placeholder implementation
	// In a real implementation, you would use Neo4J's community detection algorithms
	query := `
		MATCH (n)
		RETURN n.type as community, count(*) as size
		GROUP BY n.type
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	result, err := n.executeQuery(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// UpsertNodes batch updates or inserts nodes
func (n *Neo4jGraphStore) UpsertNodes(ctx context.Context, nodes []*abstraction.Node) error {
	// Neo4J doesn't support true upsert for multiple nodes in a single query
	// We'll process them one by one
	for _, node := range nodes {
		query := `
			MERGE (n {id: $id})
			SET n.type = $type
		`

		// Add properties
		for key := range node.Properties {
			query += fmt.Sprintf("\t\t\tSET n.%s = $%s\n", key, key)
		}

		// Prepare parameters
		params := map[string]interface{}{
			"id":   node.ID,
			"type": node.Type,
		}

		for key, val := range node.Properties {
			params[key] = val
		}

		// Execute query
		_, err := n.executeQuery(ctx, query, params)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpsertEdges batch updates or inserts edges
func (n *Neo4jGraphStore) UpsertEdges(ctx context.Context, edges []*abstraction.Edge) error {
	// Neo4J doesn't support true upsert for multiple edges in a single query
	// We'll process them one by one
	for _, edge := range edges {
		query := `
			MATCH (s {id: $source})
			MATCH (t {id: $target})
			MERGE (s)-[r:{{type}} {id: $id}]->(t)
		`

		// Replace {{type}} with actual edge type
		query = strings.ReplaceAll(query, "{{type}}", edge.Type)

		// Add properties
		for key := range edge.Properties {
			query += fmt.Sprintf("\t\t\tSET r.%s = $%s\n", key, key)
		}

		// Prepare parameters
		params := map[string]interface{}{
			"id":     edge.ID,
			"source": edge.Source,
			"target": edge.Target,
		}

		for key, val := range edge.Properties {
			params[key] = val
		}

		// Execute query
		_, err := n.executeQuery(ctx, query, params)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close closes the Neo4J driver
func (n *Neo4jGraphStore) Close(ctx context.Context) error {
	if n.driver != nil {
		return n.driver.Close(ctx)
	}
	return nil
}

// executeQuery executes a Neo4J query and returns the results
func (n *Neo4jGraphStore) executeQuery(ctx context.Context, query string, params map[string]interface{}) ([]map[string]any, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	var results []map[string]any
	for result.Next(ctx) {
		row := make(map[string]any)
		record := result.Record()
		for i, key := range record.Keys {
			row[key] = record.Values[i]
		}
		results = append(results, row)
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("query execution error: %w", err)
	}

	return results, nil
}

// mapToNode maps a Neo4J node to an abstraction.Node
func (n *Neo4jGraphStore) mapToNode(node neo4j.Node) (*abstraction.Node, error) {
	// Extract node type
	labels := node.Labels
	var nodeType string
	if len(labels) > 0 {
		nodeType = labels[0]
	} else {
		nodeType = "Node"
	}

	// Extract properties
	properties := make(map[string]any)
	for key, val := range node.Props {
		if key != "id" {
			properties[key] = val
		}
	}

	return &abstraction.Node{
		ID:         node.Props["id"].(string),
		Type:       nodeType,
		Properties: properties,
	}, nil
}

// mapToEdge maps a Neo4J relationship to an abstraction.Edge
func (n *Neo4jGraphStore) mapToEdge(edge neo4j.Relationship, source, target neo4j.Node) (*abstraction.Edge, error) {
	// Extract properties
	properties := make(map[string]any)
	for key, val := range edge.Props {
		if key != "id" {
			properties[key] = val
		}
	}

	return &abstraction.Edge{
		ID:         edge.Props["id"].(string),
		Type:       string(edge.Type),
		Source:     source.Props["id"].(string),
		Target:     target.Props["id"].(string),
		Properties: properties,
	}, nil
}
