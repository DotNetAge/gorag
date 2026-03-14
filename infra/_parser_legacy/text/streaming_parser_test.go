package text

import (
	"bytes"
	"context"
	"testing"
)

// TestStreamingParser_Parse tests the streaming parser with large content
func TestStreamingParser_Parse(t *testing.T) {
	// Create a large content (simulating a 100MB file)
	var largeContent bytes.Buffer
	for i := 0; i < 100000; i++ {
		largeContent.WriteString("This is a test line for streaming parser. ")
	}

	// Create streaming parser
	parser := NewStreamingParser()

	// Test parsing
	ctx := context.Background()
	chunks, err := parser.Parse(ctx, &largeContent)
	if err != nil {
		t.Fatalf("Failed to parse large content: %v", err)
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

	t.Logf("Successfully parsed large content into %d chunks", len(chunks))
}

// TestStreamingParser_Parse_SmallContent tests the streaming parser with small content
func TestStreamingParser_Parse_SmallContent(t *testing.T) {
	// Create small content
	smallContent := bytes.NewBufferString("This is a small test content.")

	// Create streaming parser
	parser := NewStreamingParser()

	// Test parsing
	ctx := context.Background()
	chunks, err := parser.Parse(ctx, smallContent)
	if err != nil {
		t.Fatalf("Failed to parse small content: %v", err)
	}

	// Verify chunks were created
	if len(chunks) == 0 {
		t.Fatalf("No chunks created")
	}

	t.Logf("Successfully parsed small content into %d chunks", len(chunks))
}
