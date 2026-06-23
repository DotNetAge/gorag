package core

import (
	"testing"
)

func TestNodeLabels(t *testing.T) {
	n := &Node{
		ID:     "test-1",
		Labels: []string{"Person", "Employee"},
		Name:   "Alice",
		Properties: map[string]any{
			"age": 30,
			"role": "engineer",
		},
	}
	if len(n.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(n.Labels))
	}
	if n.Labels[0] != "Person" {
		t.Errorf("expected first label to be Person, got %s", n.Labels[0])
	}
}

func TestEdge(t *testing.T) {
	e := &Edge{
		ID:       "edge-1",
		Type:     "WORKS_FOR",
		Source:   "node-1",
		Target:   "node-2",
		Properties: map[string]any{
			"since": 2020,
		},
	}
	if e.Type != "WORKS_FOR" {
		t.Errorf("expected WORKS_FOR, got %s", e.Type)
	}
	if e.Source != "node-1" {
		t.Errorf("expected source node-1, got %s", e.Source)
	}
	if e.Target != "node-2" {
		t.Errorf("expected target node-2, got %s", e.Target)
	}
}

func TestEdgeSourceChunkIDs(t *testing.T) {
	e := &Edge{
		ID:             "edge-1",
		Type:           "KNOWS",
		Source:         "person-1",
		Target:         "person-2",
		Predicate:      "knows",
		SourceChunkIDs: []string{"chunk-1"},
	}
	if len(e.SourceChunkIDs) != 1 {
		t.Errorf("expected 1 chunk ID, got %d", len(e.SourceChunkIDs))
	}
	if e.SourceChunkIDs[0] != "chunk-1" {
		t.Errorf("expected chunk-1, got %s", e.SourceChunkIDs[0])
	}
}
