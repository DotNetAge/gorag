package json

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// TestStreamingParser_Parse tests the streaming parser with large JSON content
func TestStreamingParser_Parse(t *testing.T) {
	// Create a large JSON content
	largeData := make(map[string]interface{})
	items := make([]map[string]interface{}, 10000)
	for i := 0; i < 10000; i++ {
		items[i] = map[string]interface{}{
			"id":   i,
			"name": fmt.Sprintf("Item %d", i),
			"value": fmt.Sprintf("This is value for item %d", i),
		}
	}
	largeData["items"] = items

	// Convert to JSON
	largeContent, err := json.Marshal(largeData)
	if err != nil {
		t.Fatalf("Failed to create large JSON content: %v", err)
	}

	// Create streaming parser
	parser := NewStreamingParser()

	// Test parsing
	ctx := context.Background()
	chunks, err := parser.Parse(ctx, bytes.NewReader(largeContent))
	if err != nil {
		t.Fatalf("Failed to parse large JSON content: %v", err)
	}

	// Verify chunks were created
	if len(chunks) == 0 {
		t.Fatalf("No chunks created")
	}

	// Verify chunk content
	for i, chunk := range chunks {
		if chunk.Content == "" {
			t.Errorf("Chunk %d is empty", i)
		}
	}

	t.Logf("Successfully parsed large JSON content into %d chunks", len(chunks))
}

// TestStreamingParser_Parse_SmallContent tests the streaming parser with small JSON content
func TestStreamingParser_Parse_SmallContent(t *testing.T) {
	// Create small JSON content
	smallContent := bytes.NewBufferString(`{"name": "test", "value": "This is a small test content"}`)

	// Create streaming parser
	parser := NewStreamingParser()

	// Test parsing
	ctx := context.Background()
	chunks, err := parser.Parse(ctx, smallContent)
	if err != nil {
		t.Fatalf("Failed to parse small JSON content: %v", err)
	}

	// Verify chunks were created
	if len(chunks) == 0 {
		t.Fatalf("No chunks created")
	}

	t.Logf("Successfully parsed small JSON content into %d chunks", len(chunks))
}
