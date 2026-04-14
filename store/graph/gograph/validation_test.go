package gograph

import (
	"context"
	"fmt"
	"os"
	"testing"

	gograph "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ValidationIssue struct {
	ID          string
	Title       string
	Description string
	Severity    string
	ReproSteps  []string
}

func TestValidationReport(t *testing.T) {
	var issues []ValidationIssue

	t.Run("Issue1_NodeIDNotPreserved", func(t *testing.T) {
		tmpPath := "/tmp/gograph_validation_issue1"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		result, err := db.Exec(ctx, `CREATE (n:Person {id: "test123", name: "Alice"})`)
		require.NoError(t, err)
		assert.Equal(t, 1, result.AffectedNodes)

		rows, err := db.Query(ctx, `MATCH (n:Person) RETURN n`)
		require.NoError(t, err)
		defer rows.Close()

		var nodeFound bool
		for rows.Next() {
			var n interface{}
			err := rows.Scan(&n)
			require.NoError(t, err)
			nodeFound = true

			nodeMap, ok := n.(map[string]interface{})
			if ok {
				t.Logf("Node returned: %+v", nodeMap)
				props, hasProps := nodeMap["properties"]
				if hasProps {
					propsMap := props.(map[string]interface{})
					t.Logf("Properties: %+v", propsMap)
					if idProp, hasID := propsMap["id"]; hasID {
						t.Logf("ID property: %v", idProp)
					} else {
						issues = append(issues, ValidationIssue{
							ID:          "ISSUE-001",
							Title:       "Node ID property not stored",
							Description: "When creating a node with an 'id' property, the property is not being stored correctly",
							Severity:    "HIGH",
							ReproSteps: []string{
								"1. Create a node with CREATE (n:Person {id: \"test123\", name: \"Alice\"})",
								"2. Query the node with MATCH (n:Person) RETURN n",
								"3. Check if the 'id' property exists in the returned node properties",
							},
						})
					}
				}
			}
		}
		assert.True(t, nodeFound, "Node should be found")
	})

	t.Run("Issue2_PropertyRetrieval", func(t *testing.T) {
		tmpPath := "/tmp/gograph_validation_issue2"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		_, err = db.Exec(ctx, `CREATE (n:Test {name: "Bob", age: 30, active: true})`)
		require.NoError(t, err)

		rows, err := db.Query(ctx, `MATCH (n:Test) RETURN n.name, n.age, n.active`)
		require.NoError(t, err)
		defer rows.Close()

		if rows.Next() {
			var name, age, active interface{}
			err := rows.Scan(&name, &age, &active)
			t.Logf("Scanned values - name: %v, age: %v, active: %v", name, age, active)
			require.NoError(t, err)
		} else {
			issues = append(issues, ValidationIssue{
				ID:          "ISSUE-002",
				Title:       "Property return not working",
				Description: "MATCH query with property return (n.name, n.age) returns no rows",
				Severity:    "HIGH",
				ReproSteps: []string{
					"1. Create a node with properties",
					"2. Query with MATCH (n:Test) RETURN n.name, n.age",
					"3. Observe that no rows are returned",
				},
			})
		}
	})

	t.Run("Issue3_RelationshipCreation", func(t *testing.T) {
		tmpPath := "/tmp/gograph_validation_issue3"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		result, err := db.Exec(ctx, `CREATE (a:Person {id: "p1"}), (b:Person {id: "p2"}), (a)-[:KNOWS {since: 2020}]->(b)`)
		require.NoError(t, err)
		t.Logf("Created %d nodes, %d relationships", result.AffectedNodes, result.AffectedRels)

		rows, err := db.Query(ctx, `MATCH (a)-[r:KNOWS]->(b) RETURN a, r, b`)
		require.NoError(t, err)
		defer rows.Close()

		var relCount int
		for rows.Next() {
			relCount++
			var a, r, b interface{}
			err := rows.Scan(&a, &r, &b)
			if err != nil {
				t.Logf("Scan error: %v", err)
			} else {
				t.Logf("Relationship found: a=%+v, r=%+v, b=%+v", a, r, b)
			}
		}

		if relCount == 0 {
			issues = append(issues, ValidationIssue{
				ID:          "ISSUE-003",
				Title:       "Relationship not created or not queryable",
				Description: "Creating relationships in a single CREATE statement doesn't work or relationships can't be queried",
				Severity:    "HIGH",
				ReproSteps: []string{
					"1. Create nodes and relationship: CREATE (a:Person), (b:Person), (a)-[:KNOWS]->(b)",
					"2. Query: MATCH (a)-[r:KNOWS]->(b) RETURN a, r, b",
					"3. Observe that no relationships are returned",
				},
			})
		}
	})

	t.Run("Issue4_WhereClauseFiltering", func(t *testing.T) {
		tmpPath := "/tmp/gograph_validation_issue4"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		_, err = db.Exec(ctx, `CREATE (n:Person {name: "Alice", age: 30})`)
		require.NoError(t, err)
		_, err = db.Exec(ctx, `CREATE (n:Person {name: "Bob", age: 25})`)
		require.NoError(t, err)

		rows, err := db.Query(ctx, `MATCH (n:Person) WHERE n.age > 28 RETURN n`)
		require.NoError(t, err)
		defer rows.Close()

		var count int
		for rows.Next() {
			count++
			var n interface{}
			rows.Scan(&n)
			t.Logf("Found node: %+v", n)
		}

		t.Logf("WHERE clause returned %d nodes (expected 1)", count)
	})

	t.Run("Issue5_TransactionSupport", func(t *testing.T) {
		tmpPath := "/tmp/gograph_validation_issue5"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		_, err = tx.Exec(`CREATE (n:Person {name: "InTransaction"})`)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		rows, err := db.Query(ctx, `MATCH (n:Person) WHERE n.name = "InTransaction" RETURN n`)
		require.NoError(t, err)
		defer rows.Close()

		var found bool
		for rows.Next() {
			found = true
		}

		assert.True(t, found, "Node created in transaction should be persisted")
	})

	t.Run("Issue6_MERGE_Support", func(t *testing.T) {
		tmpPath := "/tmp/gograph_validation_issue6"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		result, err := db.Exec(ctx, `MERGE (n:Person {id: "unique1"})`)
		require.NoError(t, err)
		t.Logf("First MERGE: %d nodes affected", result.AffectedNodes)

		result, err = db.Exec(ctx, `MERGE (n:Person {id: "unique1"})`)
		require.NoError(t, err)
		t.Logf("Second MERGE: %d nodes affected", result.AffectedNodes)

		rows, err := db.Query(ctx, `MATCH (n:Person) RETURN n`)
		require.NoError(t, err)
		defer rows.Close()

		var count int
		for rows.Next() {
			count++
		}

		assert.Equal(t, 1, count, "MERGE should create only one node")
	})

	t.Run("Issue7_ParameterizedQueries", func(t *testing.T) {
		tmpPath := "/tmp/gograph_validation_issue7"
		defer os.RemoveAll(tmpPath)

		db, err := gograph.Open(tmpPath)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		params := map[string]interface{}{
			"name": "ParameterizedUser",
			"age":  35,
		}

		result, err := db.Exec(ctx, `CREATE (n:Person {name: $name, age: $age})`, params)
		require.NoError(t, err)
		t.Logf("Parameterized CREATE: %d nodes affected", result.AffectedNodes)

		rows, err := db.Query(ctx, `MATCH (n:Person) WHERE n.name = $name RETURN n`, params)
		require.NoError(t, err)
		defer rows.Close()

		var found bool
		for rows.Next() {
			found = true
		}

		assert.True(t, found, "Parameterized query should find created node")
	})

	t.Log("\n\n========== VALIDATION REPORT ==========")
	t.Logf("Total issues found: %d", len(issues))
	for _, issue := range issues {
		t.Logf("\n[%s] %s (Severity: %s)", issue.ID, issue.Title, issue.Severity)
		t.Logf("Description: %s", issue.Description)
		t.Log("Reproduction Steps:")
		for _, step := range issue.ReproSteps {
			t.Logf("  %s", step)
		}
	}
}

func TestGraphStoreValidation(t *testing.T) {
	tmpPath := "/tmp/gograph_store_validation"
	defer os.RemoveAll(tmpPath)

	store, err := NewGraphStore(tmpPath)
	require.NoError(t, err)
	defer store.Close(context.Background())

	ctx := context.Background()

	t.Run("NodeCRUD", func(t *testing.T) {
		nodes := []*core.Node{
			{
				ID:         "node-1",
				Type:       "Person",
				Properties: map[string]any{"name": "Alice", "age": 30},
			},
			{
				ID:         "node-2",
				Type:       "Person",
				Properties: map[string]any{"name": "Bob", "age": 25},
			},
		}

		err := store.UpsertNodes(ctx, nodes)
		require.NoError(t, err, "UpsertNodes should succeed")

		for _, node := range nodes {
			retrieved, err := store.GetNode(ctx, node.ID)
			require.NoError(t, err, "GetNode should not error")
			if retrieved == nil {
				t.Errorf("Node %s not found after upsert", node.ID)
			} else {
				t.Logf("Retrieved node %s: Type=%s, Props=%+v", retrieved.ID, retrieved.Type, retrieved.Properties)
			}
		}
	})

	t.Run("EdgeCRUD", func(t *testing.T) {
		edges := []*core.Edge{
			{
				ID:         "edge-1",
				Type:       "KNOWS",
				Source:     "node-1",
				Target:     "node-2",
				Properties: map[string]any{"since": 2020},
			},
		}

		err := store.UpsertEdges(ctx, edges)
		require.NoError(t, err, "UpsertEdges should succeed")

		neighbors, edges, err := store.GetNeighbors(ctx, "node-1", 1, 10)
		require.NoError(t, err, "GetNeighbors should not error")
		t.Logf("Found %d neighbors, %d edges", len(neighbors), len(edges))

		if len(neighbors) == 0 {
			t.Error("Expected to find neighbors but got none")
		}
	})

	t.Run("QueryFunctionality", func(t *testing.T) {
		results, err := store.Query(ctx, "MATCH (n:Person) RETURN n", nil)
		require.NoError(t, err, "Query should not error")
		t.Logf("Query returned %d results", len(results))

		for i, r := range results {
			t.Logf("Result %d: %+v", i, r)
		}
	})
}

func TestCypherSyntaxSupport(t *testing.T) {
	tmpPath := "/tmp/gograph_syntax_test"
	defer os.RemoveAll(tmpPath)

	db, err := gograph.Open(tmpPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	syntaxTests := []struct {
		name  string
		query string
		setup string
	}{
		{"Simple MATCH", "MATCH (n) RETURN n", "CREATE (n:Test)"},
		{"MATCH with label", "MATCH (n:Person) RETURN n", "CREATE (n:Person)"},
		{"MATCH with WHERE", "MATCH (n:Person) WHERE n.age > 25 RETURN n", "CREATE (n:Person {age: 30})"},
		{"CREATE simple", "CREATE (n:Test)", ""},
		{"CREATE with properties", "CREATE (n:Person {name: 'Alice', age: 30})", ""},
		{"CREATE with multiple nodes", "CREATE (a:Person), (b:Person)", ""},
		{"CREATE relationship", "CREATE (a:Person)-[:KNOWS]->(b:Person)", ""},
		{"SET clause", "MATCH (n:Person) SET n.updated = true", "CREATE (n:Person)"},
		{"DELETE clause", "MATCH (n:ToDelete) DELETE n", "CREATE (n:ToDelete)"},
		{"DETACH DELETE", "MATCH (n:ToDelete) DETACH DELETE n", "CREATE (n:ToDelete)"},
		{"MERGE clause", "MERGE (n:Person {id: 'unique'})", ""},
		{"RETURN with alias", "MATCH (n) RETURN n.name AS name", "CREATE (n:Person {name: 'Test'})"},
	}

	fmt.Println("\n========== CYPHER SYNTAX SUPPORT TEST ==========")
	for _, tt := range syntaxTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != "" {
				_, err := db.Exec(ctx, tt.setup)
				if err != nil {
					t.Logf("Setup failed: %v", err)
				}
			}

			result, err := db.Exec(ctx, tt.query)
			if err != nil {
				t.Logf("[FAIL] %s: %v", tt.name, err)
			} else {
				t.Logf("[PASS] %s (affected: nodes=%d, rels=%d)", tt.name, result.AffectedNodes, result.AffectedRels)
			}
		})
	}
}
