package gograph

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	gograph "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gograph/pkg/graph"
	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type IssueReport struct {
	ID          string
	Title       string
	Description string
	Severity    string
	Category    string
	ReproSteps  []string
	Expected    string
	Actual      string
}

func TestComprehensiveValidation(t *testing.T) {
	var issues []IssueReport

	t.Run("Issue1_NodeIDMismatch", func(t *testing.T) {
		tmpPath := "/tmp/validation_issue1"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		result, err := db.Exec(ctx, `CREATE (n:Person {id: "user123", name: "Alice"})`)
		require.NoError(t, err)
		require.Equal(t, 1, result.AffectedNodes)

		rows, err := db.Query(ctx, `MATCH (n:Person) RETURN n`)
		require.NoError(t, err)
		defer rows.Close()

		if rows.Next() {
			var n interface{}
			err := rows.Scan(&n)
			require.NoError(t, err)

			switch node := n.(type) {
			case *graph.Node:
				t.Logf("Node structure (as *graph.Node): ID=%s, Labels=%v, Props=%+v", node.ID, node.Labels, node.Properties)
				if userPropID, exists := node.Properties["id"]; exists {
					t.Logf("User 'id' property: %+v", userPropID)
				} else {
					issues = append(issues, IssueReport{
						ID:          "ISSUE-001",
						Title:       "User-defined 'id' property not preserved",
						Description: "When creating a node with an 'id' property, it conflicts with gograph's internal ID system",
						Severity:    "HIGH",
						Category:    "Data Integrity",
						ReproSteps: []string{
							"1. Execute: CREATE (n:Person {id: \"user123\", name: \"Alice\"})",
							"2. Query: MATCH (n:Person) RETURN n",
							"3. Check properties map for 'id' key",
						},
						Expected: "Properties should contain 'id' with value 'user123'",
						Actual:   "'id' property is missing or overwritten",
					})
				}
			case map[string]interface{}:
				t.Logf("Node structure: %+v", node)
				internalID, _ := node["id"].(string)
				props, _ := node["properties"].(map[string]interface{})

				t.Logf("Internal ID: %s", internalID)
				t.Logf("Properties: %+v", props)

				if userPropID, exists := props["id"]; exists {
					t.Logf("User 'id' property: %v", userPropID)
				} else {
					issues = append(issues, IssueReport{
						ID:          "ISSUE-001",
						Title:       "User-defined 'id' property not preserved",
						Description: "When creating a node with an 'id' property, it conflicts with gograph's internal ID system",
						Severity:    "HIGH",
						Category:    "Data Integrity",
						ReproSteps: []string{
							"1. Execute: CREATE (n:Person {id: \"user123\", name: \"Alice\"})",
							"2. Query: MATCH (n:Person) RETURN n",
							"3. Check properties map for 'id' key",
						},
						Expected: "Properties should contain 'id' with value 'user123'",
						Actual:   "'id' property is missing or overwritten",
					})
				}
			default:
				t.Logf("Node returned as type: %T", n)
			}
		}
	})

	t.Run("Issue2_GetNodeImplementation", func(t *testing.T) {
		tmpPath := "/tmp/validation_issue2"
		defer os.RemoveAll(tmpPath)

		store, err := NewGraphStore(tmpPath)
		require.NoError(t, err)
		defer store.Close(context.Background())

		ctx := context.Background()

		node := &core.Node{
			ID:         "test-node-id",
			Type:       "Person",
			Properties: map[string]any{"name": "Test", "age": 25},
		}

		err = store.UpsertNodes(ctx, []*core.Node{node})
		require.NoError(t, err)

		results, err := store.Query(ctx, "MATCH (n:Person) RETURN n", nil)
		require.NoError(t, err)

		t.Logf("Query results: %+v", results)

		if len(results) > 0 {
			if n, ok := results[0]["n"].(map[string]interface{}); ok {
				t.Logf("Node from query: %+v", n)
				if props, ok := n["properties"].(map[string]interface{}); ok {
					t.Logf("Properties from query: %+v", props)
					for k, v := range props {
						t.Logf("  Property %s = %v (type: %T)", k, v, v)
					}
				}
			}
		}

		retrieved, err := store.GetNode(ctx, "test-node-id")
		require.NoError(t, err)

		if retrieved == nil {
			issues = append(issues, IssueReport{
				ID:          "ISSUE-002",
				Title:       "GetNode returns nil for existing node",
				Description: "The GetNode method cannot find nodes created via UpsertNodes",
				Severity:    "HIGH",
				Category:    "API Implementation",
				ReproSteps: []string{
					"1. Create GraphStore instance",
					"2. Call UpsertNodes with a node having ID 'test-node-id'",
					"3. Call GetNode with the same ID",
					"4. Observe that nil is returned",
				},
				Expected: "GetNode should return the node with matching ID",
				Actual:   "GetNode returns nil",
			})
		} else {
			t.Logf("Retrieved node: %+v", retrieved)
		}
	})

	t.Run("Issue3_PropertyNameCaseSensitivity", func(t *testing.T) {
		tmpPath := "/tmp/validation_issue3"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		_, err = db.Exec(ctx, `CREATE (n:Test {ID: "upper", id: "lower", Name: "Test"})`)
		require.NoError(t, err)

		rows, err := db.Query(ctx, `MATCH (n:Test) RETURN n`)
		require.NoError(t, err)
		defer rows.Close()

		if rows.Next() {
			var n interface{}
			rows.Scan(&n)

			switch node := n.(type) {
			case *graph.Node:
				t.Logf("Properties with case variations: %+v", node.Properties)
				for _, key := range []string{"ID", "id", "Name", "name"} {
					if v, exists := node.Properties[key]; exists {
						t.Logf("Property '%s' exists: %+v", key, v)
					}
				}
			case map[string]interface{}:
				props, _ := node["properties"].(map[string]interface{})
				t.Logf("Properties with case variations: %+v", props)
				for _, key := range []string{"ID", "id", "Name", "name"} {
					if v, exists := props[key]; exists {
						t.Logf("Property '%s' exists with value: %v", key, v)
					}
				}
			default:
				t.Logf("Node returned as type: %T", n)
			}
		}
	})

	t.Run("Issue4_EdgeRetrieval", func(t *testing.T) {
		tmpPath := "/tmp/validation_issue4"
		defer os.RemoveAll(tmpPath)

		store, err := NewGraphStore(tmpPath)
		require.NoError(t, err)
		defer store.Close(context.Background())

		ctx := context.Background()

		nodes := []*core.Node{
			{ID: "src", Type: "Person", Properties: map[string]any{"name": "Source"}},
			{ID: "dst", Type: "Person", Properties: map[string]any{"name": "Dest"}},
		}
		err = store.UpsertNodes(ctx, nodes)
		require.NoError(t, err)

		edge := &core.Edge{
			ID:         "edge1",
			Type:       "KNOWS",
			Source:     "src",
			Target:     "dst",
			Properties: map[string]any{"since": 2020},
		}
		err = store.UpsertEdges(ctx, []*core.Edge{edge})
		require.NoError(t, err)

		results, err := store.Query(ctx, "MATCH (a)-[r:KNOWS]->(b) RETURN a, r, b", nil)
		require.NoError(t, err)

		t.Logf("Edge query results: %+v", results)

		neighbors, edges, err := store.GetNeighbors(ctx, "src", 1, 10)
		require.NoError(t, err)

		t.Logf("GetNeighbors: %d neighbors, %d edges", len(neighbors), len(edges))

		if len(edges) == 0 {
			issues = append(issues, IssueReport{
				ID:          "ISSUE-004",
				Title:       "GetNeighbors returns no edges",
				Description: "After creating nodes and edges, GetNeighbors returns empty edge list",
				Severity:    "HIGH",
				Category:    "API Implementation",
				ReproSteps: []string{
					"1. Create two nodes with UpsertNodes",
					"2. Create an edge between them with UpsertEdges",
					"3. Call GetNeighbors with source node ID",
					"4. Observe that edges slice is empty",
				},
				Expected: "GetNeighbors should return the created edge",
				Actual:   "GetNeighbors returns empty edges slice",
			})
		}
	})

	t.Run("Issue5_UpsertNodesImplementation", func(t *testing.T) {
		tmpPath := "/tmp/validation_issue5"
		defer os.RemoveAll(tmpPath)

		store, err := NewGraphStore(tmpPath)
		require.NoError(t, err)
		defer store.Close(context.Background())

		ctx := context.Background()

		node := &core.Node{
			ID:   "test",
			Type: "Person",
			Properties: map[string]any{
				"name":    "Alice",
				"age":     30,
				"active":  true,
				"score":   95.5,
				"tags":    []string{"a", "b"},
				"created": time.Now(),
			},
		}

		err = store.UpsertNodes(ctx, []*core.Node{node})
		require.NoError(t, err)

		results, err := store.Query(ctx, "MATCH (n:Person) RETURN n", nil)
		require.NoError(t, err)

		if len(results) > 0 {
			n := results[0]["n"].(map[string]interface{})
			props := n["properties"].(map[string]interface{})

			t.Logf("Stored properties: %+v", props)

			for k, v := range node.Properties {
				if stored, exists := props[k]; exists {
					t.Logf("Property '%s': stored=%v (%T), original=%v (%T)", k, stored, stored, v, v)
				} else {
					t.Logf("Property '%s' is missing", k)
				}
			}
		}
	})

	t.Log("\n\n========== COMPREHENSIVE VALIDATION REPORT ==========")
	t.Logf("Total issues found: %d\n", len(issues))
	for _, issue := range issues {
		t.Logf("\n[%s] %s", issue.ID, issue.Title)
		t.Logf("Severity: %s | Category: %s", issue.Severity, issue.Category)
		t.Logf("Description: %s", issue.Description)
		t.Log("Reproduction Steps:")
		for i, step := range issue.ReproSteps {
			t.Logf("  %d. %s", i+1, step)
		}
		t.Logf("Expected: %s", issue.Expected)
		t.Logf("Actual: %s", issue.Actual)
	}
}

func TestCypherFeatureSupport(t *testing.T) {
	tmpPath := "/tmp/cypher_feature_test"
	defer os.RemoveAll(tmpPath)

	db, err := gograph.Open(tmpPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	features := []struct {
		name        string
		query       string
		setup       string
		shouldError bool
		description string
	}{
		{
			name:        "CREATE with single node",
			query:       "CREATE (n:Person {name: 'Alice'})",
			shouldError: false,
			description: "Basic node creation with properties",
		},
		{
			name:        "CREATE with multiple nodes",
			query:       "CREATE (a:Person), (b:Person)",
			shouldError: false,
			description: "Creating multiple nodes in single statement",
		},
		{
			name:        "CREATE with relationship",
			query:       "CREATE (a:Person)-[:KNOWS]->(b:Person)",
			shouldError: false,
			description: "Creating nodes with relationship",
		},
		{
			name:        "MATCH all nodes",
			query:       "MATCH (n) RETURN n",
			setup:       "CREATE (n:Test)",
			shouldError: false,
			description: "Basic pattern matching",
		},
		{
			name:        "MATCH with label",
			query:       "MATCH (n:Person) RETURN n",
			setup:       "CREATE (n:Person)",
			shouldError: false,
			description: "Label-based filtering",
		},
		{
			name:        "MATCH with WHERE",
			query:       "MATCH (n:Person) WHERE n.age > 25 RETURN n",
			setup:       "CREATE (n:Person {age: 30})",
			shouldError: false,
			description: "WHERE clause filtering",
		},
		{
			name:        "MATCH with property equality",
			query:       "MATCH (n:Person {name: 'Alice'}) RETURN n",
			setup:       "CREATE (n:Person {name: 'Alice'})",
			shouldError: false,
			description: "Property-based matching",
		},
		{
			name:        "MATCH relationship pattern",
			query:       "MATCH (a)-[r]->(b) RETURN a, r, b",
			setup:       "CREATE (a)-[:REL]->(b)",
			shouldError: false,
			description: "Relationship pattern matching",
		},
		{
			name:        "SET property",
			query:       "MATCH (n:Person) SET n.updated = true",
			setup:       "CREATE (n:Person)",
			shouldError: false,
			description: "Property modification",
		},
		{
			name:        "DELETE node",
			query:       "MATCH (n:ToDelete) DELETE n",
			setup:       "CREATE (n:ToDelete)",
			shouldError: false,
			description: "Node deletion",
		},
		{
			name:        "DETACH DELETE",
			query:       "MATCH (n:ToDelete) DETACH DELETE n",
			setup:       "CREATE (n:ToDelete)-[:REL]->(m)",
			shouldError: false,
			description: "Cascade deletion",
		},
		{
			name:        "MERGE create",
			query:       "MERGE (n:Person {id: 'unique'})",
			shouldError: false,
			description: "MERGE for creation",
		},
		{
			name:        "MERGE match",
			query:       "MERGE (n:Person {id: 'unique'})",
			setup:       "CREATE (n:Person {id: 'unique'})",
			shouldError: false,
			description: "MERGE for matching existing",
		},
		{
			name:        "RETURN with alias",
			query:       "MATCH (n) RETURN n.name AS name",
			setup:       "CREATE (n:Person {name: 'Test'})",
			shouldError: false,
			description: "Result aliasing",
		},
		{
			name:        "Parameterized query",
			query:       "CREATE (n:Person {name: $name})",
			shouldError: false,
			description: "Parameterized queries",
		},
		{
			name:        "ORDER BY",
			query:       "MATCH (n:Person) RETURN n ORDER BY n.name",
			setup:       "CREATE (a:Person {name: 'B'}), (b:Person {name: 'A'})",
			shouldError: false,
			description: "Result ordering",
		},
		{
			name:        "LIMIT",
			query:       "MATCH (n) RETURN n LIMIT 1",
			setup:       "CREATE (a:Test), (b:Test)",
			shouldError: false,
			description: "Result limiting",
		},
		{
			name:        "SKIP",
			query:       "MATCH (n) RETURN n SKIP 1",
			setup:       "CREATE (a:Test), (b:Test)",
			shouldError: false,
			description: "Result pagination",
		},
		{
			name:        "WITH clause",
			query:       "MATCH (n) WITH n RETURN n",
			setup:       "CREATE (n:Test)",
			shouldError: false,
			description: "WITH clause for pipelining",
		},
		{
			name:        "REMOVE property",
			query:       "MATCH (n:Person) REMOVE n.temp",
			setup:       "CREATE (n:Person {temp: 'value'})",
			shouldError: false,
			description: "Property removal",
		},
	}

	fmt.Println("\n========== CYPHER FEATURE SUPPORT TEST ==========")
	fmt.Printf("%-35s | %-10s | %s\n", "Feature", "Status", "Description")
	fmt.Println(string(make([]byte, 80)))

	supported := 0
	partial := 0
	unsupported := 0

	for _, tt := range features {
		if tt.setup != "" {
			_, err := db.Exec(ctx, tt.setup)
			if err != nil {
				t.Logf("Setup failed for %s: %v", tt.name, err)
			}
		}

		var params map[string]interface{}
		if tt.name == "Parameterized query" {
			params = map[string]interface{}{"name": "ParamUser"}
		}

		result, err := db.Exec(ctx, tt.query, params)

		status := "✅ PASS"
		if err != nil {
			if tt.shouldError {
				status = "✅ PASS (expected error)"
			} else {
				status = "❌ FAIL"
				unsupported++
			}
			t.Logf("%s error: %v", tt.name, err)
		} else {
			supported++
		}

		fmt.Printf("%-35s | %-10s | %s\n", tt.name, status, tt.description)
		_ = result
	}

	fmt.Println("\n========== SUMMARY ==========")
	fmt.Printf("Supported: %d | Unsupported: %d | Partial: %d\n", supported, unsupported, partial)
}

func TestGraphStoreAPICompliance(t *testing.T) {
	tmpPath := "/tmp/api_compliance_test"
	defer os.RemoveAll(tmpPath)

	store, err := NewGraphStore(tmpPath)
	require.NoError(t, err)
	defer store.Close(context.Background())

	ctx := context.Background()

	t.Run("UpsertNodes", func(t *testing.T) {
		nodes := []*core.Node{
			{ID: "n1", Type: "Person", Properties: map[string]any{"name": "Alice"}},
			{ID: "n2", Type: "Person", Properties: map[string]any{"name": "Bob"}},
		}

		err := store.UpsertNodes(ctx, nodes)
		assert.NoError(t, err, "UpsertNodes should not error")

		results, err := store.Query(ctx, "MATCH (n:Person) RETURN n", nil)
		assert.NoError(t, err)
		assert.Len(t, results, 2, "Should have 2 Person nodes")
	})

	t.Run("UpsertEdges", func(t *testing.T) {
		edges := []*core.Edge{
			{ID: "e1", Type: "KNOWS", Source: "n1", Target: "n2"},
		}

		err := store.UpsertEdges(ctx, edges)
		assert.NoError(t, err, "UpsertEdges should not error")
	})

	t.Run("GetNode", func(t *testing.T) {
		node, err := store.GetNode(ctx, "n1")
		assert.NoError(t, err)
		if node != nil {
			assert.Equal(t, "n1", node.ID)
			assert.Equal(t, "Person", node.Type)
		} else {
			t.Log("WARNING: GetNode returned nil - this is a known issue")
		}
	})

	t.Run("GetNeighbors", func(t *testing.T) {
		nodes, edges, err := store.GetNeighbors(ctx, "n1", 1, 10)
		assert.NoError(t, err)
		t.Logf("GetNeighbors returned %d nodes, %d edges", len(nodes), len(edges))
	})

	t.Run("Query", func(t *testing.T) {
		results, err := store.Query(ctx, "MATCH (n:Person) WHERE n.name = $name RETURN n", map[string]any{"name": "Alice"})
		assert.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("GetCommunitySummaries", func(t *testing.T) {
		summaries, err := store.GetCommunitySummaries(ctx, 1)
		assert.NoError(t, err)
		t.Logf("Community summaries: %+v", summaries)
	})
}

func TestPropertyTypeHandling(t *testing.T) {
	tmpPath := "/tmp/property_type_test"
	defer os.RemoveAll(tmpPath)

	db, err := gograph.Open(tmpPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	propertyTests := []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{"string", "hello", "hello"},
		{"int", 42, 42},
		{"int64", int64(100), int64(100)},
		{"float64", 3.14, 3.14},
		{"bool_true", true, true},
		{"bool_false", false, false},
	}

	for _, tt := range propertyTests {
		t.Run(tt.name, func(t *testing.T) {
			query := fmt.Sprintf("CREATE (n:Test {value: %v})", tt.value)
			if _, ok := tt.value.(string); ok {
				query = fmt.Sprintf("CREATE (n:Test {value: '%v'})", tt.value)
			}

			_, err := db.Exec(ctx, query)
			require.NoError(t, err)

			rows, err := db.Query(ctx, "MATCH (n:Test) RETURN n.value")
			require.NoError(t, err)
			defer rows.Close()

			if rows.Next() {
				var val interface{}
				rows.Scan(&val)
				t.Logf("Stored %s: %v (type: %T)", tt.name, val, val)
			}
		})
	}
}

func TestScanMethodVariations(t *testing.T) {
	tmpPath := "/tmp/scan_test"
	defer os.RemoveAll(tmpPath)

	db, err := gograph.Open(tmpPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	_, err = db.Exec(ctx, "CREATE (n:Person {name: 'Alice', age: 30})")
	require.NoError(t, err)

	t.Run("ScanIntoInterface", func(t *testing.T) {
		rows, err := db.Query(ctx, "MATCH (n:Person) RETURN n")
		require.NoError(t, err)
		defer rows.Close()

		if rows.Next() {
			var n interface{}
			err := rows.Scan(&n)
			require.NoError(t, err)
			t.Logf("Scanned into interface: %+v (%T)", n, n)
		}
	})

	t.Run("ScanIntoMap", func(t *testing.T) {
		rows, err := db.Query(ctx, "MATCH (n:Person) RETURN n")
		require.NoError(t, err)
		defer rows.Close()

		if rows.Next() {
			cols := rows.Columns()
			t.Logf("Columns: %v", cols)

			vals := make([]interface{}, len(cols))
			for i := range vals {
				vals[i] = new(interface{})
			}

			err := rows.Scan(vals...)
			require.NoError(t, err)

			for i, col := range cols {
				v := *(vals[i].(*interface{}))
				t.Logf("Column %s: %+v (%T)", col, v, v)
			}
		}
	})

	t.Run("ScanNodePointer", func(t *testing.T) {
		rows, err := db.Query(ctx, "MATCH (n:Person) RETURN n")
		require.NoError(t, err)
		defer rows.Close()

		if rows.Next() {
			var n interface{}
			rows.Scan(&n)

			if nodePtr, ok := n.(*graph.Node); ok {
				t.Logf("Scanned as *graph.Node: ID=%s, Labels=%v, Props=%+v",
					nodePtr.ID, nodePtr.Labels, nodePtr.Properties)
			} else {
				t.Logf("Not scanned as *graph.Node, got %T", n)
			}
		}
	})
}
