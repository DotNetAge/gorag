package graphstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultGraphStore(t *testing.T) {
	// Skip if no Neo4j server available
	t.Skip("Skipping test - requires Neo4j server")

	store, err := DefaultGraphStore("", "", "")
	if err != nil {
		t.Logf("Neo4j not available: %v", err)
		return
	}
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewNeo4JStore(t *testing.T) {
	// Skip if no Neo4j server available
	t.Skip("Skipping test - requires Neo4j server")

	store, err := NewNeo4JStore("bolt://localhost:7687", "neo4j", "password")
	if err != nil {
		t.Logf("Neo4j not available: %v", err)
		return
	}
	assert.NoError(t, err)
	assert.NotNil(t, store)
}
