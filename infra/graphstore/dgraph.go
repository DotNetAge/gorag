// +build dgraph

package graphstore

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/dgraph-io/dgo/v2"
	"github.com/dgraph-io/dgo/v2/protos/api"
	"google.golang.org/grpc"
)

// DgraphStore implements GraphStore using Dgraph
type DgraphStore struct {
	client *dgo.Dgraph
}

// NewDgraphStore creates a new DgraphStore
func NewDgraphStore() *DgraphStore {
	return &DgraphStore{}
}

// Initialize initializes the Dgraph connection
func (d *DgraphStore) Initialize(options map[string]interface{}) error {
	// Extract connection details from options
	address, ok := options["address"].(string)
	if !ok {
		return fmt.Errorf("address is required")
	}

	// Connect to Dgraph
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to dgraph: %w", err)
	}

	// Create Dgraph client
	d.client = dgo.NewDgraphClient(api.NewDgraphClient(conn))

	// Create schema
	if err := d.createSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// createSchema creates the necessary schema for Dgraph
func (d *DgraphStore) createSchema() error {
	ctx := context.Background()

	schema := `
		node: string @index(exact) .
		type: string @index(exact) .
		edge: string @index(exact) .
		source: string @index(exact) .
		target: string @index(exact) .
	`

	req := &api.Operation{
		Schema: schema,
	}

	if err := d.client.Alter(ctx, req); err != nil {
		return err
	}

	return nil
}

// CreateNode creates a new node in Dgraph
func (d *DgraphStore) CreateNode(ctx context.Context, node *abstraction.Node) error {
	// Create a mutation
	mutation := fmt.Sprintf(`
		{
			set {
				_:node <node> "%s" .
				_:node <type> "%s" .
		`, node.ID, node.Type)

	// Add properties
	for key, value := range node.Properties {
		mutation += fmt.Sprintf(`				_:node <%s> "%v" .
		`, key, value)
	}

	mutation += `
			}
		}
	`

	// Execute the mutation
	resp, err := d.client.NewTxn().Mutate(ctx, &api.Mutation{
		SetNquads: []byte(mutation),
	})
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	// Check if the mutation was successful
	if len(resp.Uids) == 0 {
		return fmt.Errorf("failed to create node: no uids returned")
	}

	return nil
}

// CreateEdge creates a new edge in Dgraph
func (d *DgraphStore) CreateEdge(ctx context.Context, edge *abstraction.Edge) error {
	// First find the source and target nodes
	sourceUID, err := d.findNodeUID(ctx, edge.Source)
	if err != nil {
		return err
	}

	targetUID, err := d.findNodeUID(ctx, edge.Target)
	if err != nil {
		return err
	}

	// Create a mutation
	mutation := fmt.Sprintf(`
		{
			set {
				%s <%s> %s .
				%s <%s> "%s" .
		`, sourceUID, edge.Type, targetUID, sourceUID, "edge", edge.ID)

	// Add properties
	for key, value := range edge.Properties {
		mutation += fmt.Sprintf(`				%s <%s> "%v" .
		`, sourceUID, key, value)
	}

	mutation += `
			}
		}
	`

	// Execute the mutation
	resp, err := d.client.NewTxn().Mutate(ctx, &api.Mutation{
		SetNquads: []byte(mutation),
	})
	if err != nil {
		return fmt.Errorf("failed to create edge: %w", err)
	}

	// Check if the mutation was successful
	if len(resp.Uids) == 0 {
		return fmt.Errorf("failed to create edge: no uids returned")
	}

	return nil
}

// GetNode retrieves a node by its ID
func (d *DgraphStore) GetNode(ctx context.Context, id string) (*abstraction.Node, error) {
	// Create a query
	query := fmt.Sprintf(`
		{
			node(func: eq(node, "%s")) {
				uid
				node
				type
			}
		}
	`, id)

	// Execute the query
	var resp struct {
		Node []struct {
			UID   string            `json:"uid"`
			Node  string            `json:"node"`
			Type  string            `json:"type"`
			Extra map[string]string `json:"extra"`
		} `json:"node"`
	}

	txn := d.client.NewTxn()
	if err := txn.Query(ctx, query, &resp); err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if len(resp.Node) == 0 {
		return nil, nil
	}

	nodeData := resp.Node[0]

	// Extract properties
	properties := make(map[string]any)
	for key, value := range nodeData.Extra {
		if key != "uid" && key != "node" && key != "type" {
			properties[key] = value
		}
	}

	return &abstraction.Node{
		ID:         nodeData.Node,
		Type:       nodeData.Type,
		Properties: properties,
	}, nil
}

// GetEdge retrieves an edge by its ID
func (d *DgraphStore) GetEdge(ctx context.Context, id string) (*abstraction.Edge, error) {
	// Create a query
	query := fmt.Sprintf(`
		{
			edge(func: eq(edge, "%s")) {
				uid
				edge
				source
				target
			}
		}
	`, id)

	// Execute the query
	var resp struct {
		Edge []struct {
			UID    string            `json:"uid"`
			Edge   string            `json:"edge"`
			Source string            `json:"source"`
			Target string            `json:"target"`
			Extra  map[string]string `json:"extra"`
		} `json:"edge"`
	}

	txn := d.client.NewTxn()
	if err := txn.Query(ctx, query, &resp); err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}

	if len(resp.Edge) == 0 {
		return nil, nil
	}

	edgeData := resp.Edge[0]

	// Extract properties
	properties := make(map[string]any)
	for key, value := range edgeData.Extra {
		if key != "uid" && key != "edge" && key != "source" && key != "target" {
			properties[key] = value
		}
	}

	return &abstraction.Edge{
		ID:         edgeData.Edge,
		Type:       "", // Dgraph doesn't store edge types as properties
		Source:     edgeData.Source,
		Target:     edgeData.Target,
		Properties: properties,
	}, nil
}

// DeleteNode deletes a node from Dgraph
func (d *DgraphStore) DeleteNode(ctx context.Context, id string) error {
	// Find the node UID
	uid, err := d.findNodeUID(ctx, id)
	if err != nil {
		return err
	}

	// Create a mutation
	mutation := fmt.Sprintf(`
		{
			delete {
				%s * * .
				* * %s .
			}
		}
	`, uid, uid)

	// Execute the mutation
	resp, err := d.client.NewTxn().Mutate(ctx, &api.Mutation{
		DelNquads: []byte(mutation),
	})
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	// Check if the mutation was successful
	if len(resp.Uids) == 0 {
		return fmt.Errorf("failed to delete node: no uids returned")
	}

	return nil
}

// DeleteEdge deletes an edge from Dgraph
func (d *DgraphStore) DeleteEdge(ctx context.Context, id string) error {
	// Create a query to find the edge
	query := fmt.Sprintf(`
		{
			edge(func: eq(edge, "%s")) {
				uid
				source
				target
			}
		}
	`, id)

	// Execute the query
	var resp struct {
		Edge []struct {
			UID    string `json:"uid"`
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"edge"`
	}

	txn := d.client.NewTxn()
	if err := txn.Query(ctx, query, &resp); err != nil {
		return fmt.Errorf("failed to find edge: %w", err)
	}

	if len(resp.Edge) == 0 {
		return nil
	}

	edgeData := resp.Edge[0]

	// Create a mutation
	mutation := fmt.Sprintf(`
		{
			delete {
				%s * %s .
			}
		}
	`, edgeData.Source, edgeData.Target)

	// Execute the mutation
	_, err := d.client.NewTxn().Mutate(ctx, &api.Mutation{
		DelNquads: []byte(mutation),
	})
	if err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}

	return nil
}

// Query executes a graph query
func (d *DgraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	// Execute the query
	var resp map[string]interface{}

	txn := d.client.NewTxn()
	if err := txn.Query(ctx, query, &resp); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Convert the response to the expected format
	var results []map[string]any
	for _, value := range resp {
		if items, ok := value.([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					results = append(results, itemMap)
				}
			}
		}
	}

	return results, nil
}

// GetNeighbors retrieves the neighbors of a node
func (d *DgraphStore) GetNeighbors(ctx context.Context, nodeID string, limit int) ([]*abstraction.Node, error) {
	// Find the node UID
	uid, err := d.findNodeUID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	// Create a query
	query := fmt.Sprintf(`
		{
			neighbors(func: uid(%s)) {
				~* {
					uid
					node
					type
				}
			}
		}
	`, uid)

	// Execute the query
	var resp struct {
		Neighbors []struct {
			Inverse []struct {
				UID   string            `json:"uid"`
				Node  string            `json:"node"`
				Type  string            `json:"type"`
				Extra map[string]string `json:"extra"`
			} `json:"~*"`
		} `json:"neighbors"`
	}

	txn := d.client.NewTxn()
	if err := txn.Query(ctx, query, &resp); err != nil {
		return nil, fmt.Errorf("failed to get neighbors: %w", err)
	}

	if len(resp.Neighbors) == 0 || len(resp.Neighbors[0].Inverse) == 0 {
		return []*abstraction.Node{}, nil
	}

	// Extract neighbors
	var nodes []*abstraction.Node
	for _, neighbor := range resp.Neighbors[0].Inverse {
		// Extract properties
		properties := make(map[string]any)
		for key, value := range neighbor.Extra {
			if key != "uid" && key != "node" && key != "type" {
				properties[key] = value
			}
		}

		nodes = append(nodes, &abstraction.Node{
			ID:         neighbor.Node,
			Type:       neighbor.Type,
			Properties: properties,
		})

		if limit > 0 && len(nodes) >= limit {
			break
		}
	}

	return nodes, nil
}

// GetCommunitySummaries retrieves community summaries
func (d *DgraphStore) GetCommunitySummaries(ctx context.Context, limit int) ([]map[string]any, error) {
	// This is a placeholder implementation
	// In a real implementation, you would use Dgraph's community detection algorithms
	return nil, fmt.Errorf("GetCommunitySummaries not implemented for Dgraph")
}

// UpsertNodes batch updates or inserts nodes
func (d *DgraphStore) UpsertNodes(ctx context.Context, nodes []*abstraction.Node) error {
	for _, node := range nodes {
		// Check if node exists
		existingNode, err := d.GetNode(ctx, node.ID)
		if err != nil {
			return err
		}

		if existingNode == nil {
			// Create new node
			if err := d.CreateNode(ctx, node); err != nil {
				return err
			}
		} else {
			// Update existing node
			uid, err := d.findNodeUID(ctx, node.ID)
			if err != nil {
				return err
			}

			// Create a mutation
			mutation := fmt.Sprintf(`
				{
					set {
						%s <type> "%s" .
				`, uid, node.Type)

			// Add properties
			for key, value := range node.Properties {
				mutation += fmt.Sprintf(`					%s <%s> "%v" .
				`, uid, key, value)
			}

			mutation += `
					}
				}
			`

			// Execute the mutation
			_, err = d.client.NewTxn().Mutate(ctx, &api.Mutation{
				SetNquads: []byte(mutation),
			})
			if err != nil {
				return fmt.Errorf("failed to update node: %w", err)
			}
		}
	}

	return nil
}

// UpsertEdges batch updates or inserts edges
func (d *DgraphStore) UpsertEdges(ctx context.Context, edges []*abstraction.Edge) error {
	for _, edge := range edges {
		// Check if edge exists
		existingEdge, err := d.GetEdge(ctx, edge.ID)
		if err != nil {
			return err
		}

		if existingEdge == nil {
			// Create new edge
			if err := d.CreateEdge(ctx, edge); err != nil {
				return err
			}
		} else {
			// Update existing edge
			// For Dgraph, we need to delete and re-create the edge
			if err := d.DeleteEdge(ctx, edge.ID); err != nil {
				return err
			}
			if err := d.CreateEdge(ctx, edge); err != nil {
				return err
			}
		}
	}

	return nil
}

// Close closes the Dgraph connection
func (d *DgraphStore) Close(ctx context.Context) error {
	// Dgraph client doesn't have a Close method
	// The gRPC connection will be closed when the client is garbage collected
	return nil
}

// findNodeUID finds the UID of a node by its ID
func (d *DgraphStore) findNodeUID(ctx context.Context, id string) (string, error) {
	// Create a query
	query := fmt.Sprintf(`
		{
			node(func: eq(node, "%s")) {
				uid
			}
		}
	`, id)

	// Execute the query
	var resp struct {
		Node []struct {
			UID string `json:"uid"`
		} `json:"node"`
	}

	txn := d.client.NewTxn()
	if err := txn.Query(ctx, query, &resp); err != nil {
		return "", fmt.Errorf("failed to find node UID: %w", err)
	}

	if len(resp.Node) == 0 {
		return "", fmt.Errorf("node not found: %s", id)
	}

	return resp.Node[0].UID, nil
}
