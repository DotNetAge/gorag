package html

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

// TestStreamingParser_Parse tests the streaming parser with large HTML content
func TestStreamingParser_Parse(t *testing.T) {
	// Create a large HTML content
	var largeContent bytes.Buffer
	largeContent.WriteString("<!DOCTYPE html><html><body>")
	for i := 0; i < 10000; i++ {
		largeContent.WriteString(fmt.Sprintf("<p>This is paragraph %d for streaming HTML parser.</p>", i))
	}
	largeContent.WriteString("</body></html>")

	// Create streaming parser
	parser := NewStreamingParser()

	// Test parsing
	ctx := context.Background()
	chunks, err := parser.Parse(ctx, &largeContent)
	if err != nil {
		t.Fatalf("Failed to parse large HTML content: %v", err)
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

	t.Logf("Successfully parsed large HTML content into %d chunks", len(chunks))
}

// TestStreamingParser_Parse_SmallContent tests the streaming parser with small HTML content
func TestStreamingParser_Parse_SmallContent(t *testing.T) {
	// Create small HTML content
	smallContent := bytes.NewBufferString("<!DOCTYPE html><html><body><p>This is a small test content.</p></body></html>")

	// Create streaming parser
	parser := NewStreamingParser()

	// Test parsing
	ctx := context.Background()
	chunks, err := parser.Parse(ctx, smallContent)
	if err != nil {
		t.Fatalf("Failed to parse small HTML content: %v", err)
	}

	// Verify chunks were created
	if len(chunks) == 0 {
		t.Fatalf("No chunks created")
	}

	t.Logf("Successfully parsed small HTML content into %d chunks", len(chunks))
}
