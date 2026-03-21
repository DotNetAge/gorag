package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNode(t *testing.T) {
	t.Run("basic node", func(t *testing.T) {
		node := Node{
			ID:   "node-1",
			Type: "Person",
			Properties: map[string]any{
				"name": "Alice",
				"age":  30,
			},
		}

		assert.Equal(t, "node-1", node.ID)
		assert.Equal(t, "Person", node.Type)
		assert.Len(t, node.Properties, 2)
		assert.Equal(t, "Alice", node.Properties["name"])
		assert.Equal(t, 30, node.Properties["age"])
	})

	t.Run("empty properties", func(t *testing.T) {
		node := Node{
			ID:         "node-2",
			Type:       "Concept",
			Properties: map[string]any{},
		}

		assert.Empty(t, node.Properties)
	})

	t.Run("nil properties", func(t *testing.T) {
		node := Node{
			ID:         "node-3",
			Type:       "Entity",
			Properties: nil,
		}

		assert.Nil(t, node.Properties)
	})
}

func TestEdge(t *testing.T) {
	t.Run("basic edge", func(t *testing.T) {
		edge := Edge{
			ID:     "edge-1",
			Type:   "KNOWS",
			Source: "node-1",
			Target: "node-2",
			Properties: map[string]any{
				"since": "2020",
				"strength": 0.9,
			},
		}

		assert.Equal(t, "edge-1", edge.ID)
		assert.Equal(t, "KNOWS", edge.Type)
		assert.Equal(t, "node-1", edge.Source)
		assert.Equal(t, "node-2", edge.Target)
		assert.Len(t, edge.Properties, 2)
	})

	t.Run("empty properties", func(t *testing.T) {
		edge := Edge{
			ID:         "edge-2",
			Type:       "RELATED_TO",
			Source:     "node-3",
			Target:     "node-4",
			Properties: map[string]any{},
		}

		assert.Empty(t, edge.Properties)
	})

	t.Run("nil properties", func(t *testing.T) {
		edge := Edge{
			ID:         "edge-3",
			Type:       "CONNECTED",
			Source:     "node-5",
			Target:     "node-6",
			Properties: nil,
		}

		assert.Nil(t, edge.Properties)
	})
}
