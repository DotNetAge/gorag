package csv

import (
	"context"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
)

func TestParser_BasicCSV(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)

	content := `name,age,city
Alice,25,New York
Bob,30,Los Angeles
Charlie,35,Chicago
`

	ctx := context.Background()
	var chunkCount int
	var firstChunk *core.Chunk

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		chunkCount++
		if firstChunk == nil {
			firstChunk = &chunk
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if chunkCount == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify metadata
	if firstChunk.Metadata["streaming"] != "true" {
		t.Errorf("Chunk should be marked as streaming")
	}
	if firstChunk.Metadata["type"] != "csv" {
		t.Errorf("Chunk has wrong type: %s", firstChunk.Metadata["type"])
	}
}

func TestParser_TSV(t *testing.T) {
	parser := NewParser()
	parser.SetSeparator('\t')

	content := "name\tage\tcity\nAlice\t25\tNew York\n"

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}
}

func TestParser_AutoDetectSeparator(t *testing.T) {
	parser := NewParser()
	parser.SetDetectSep(true)

	// CSV with commas
	content := "a,b,c\n1,2,3\n4,5,6\n"

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected chunks from CSV")
	}
}

func TestParser_QuotedFields(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)

	content := `name,description,value
"Product A","High-quality, durable item",100
"Product B","Special offer: 50% off!",80
`

	ctx := context.Background()
	var foundContent bool

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if strings.Contains(chunk.Content, "High-quality") {
			foundContent = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if !foundContent {
		t.Error("Expected to find quoted field content")
	}
}

func TestParser_LargeFile(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)

	// Create a large CSV file (10000 rows)
	var builder strings.Builder
	builder.WriteString("id,name,value\n")
	for i := 0; i < 10000; i++ {
		builder.WriteString(string(rune(i)))
		builder.WriteString(",Item ")
		builder.WriteString(string(rune(i)))
		builder.WriteString(",")
		builder.WriteString(string(rune(i % 1000)))
		builder.WriteString("\n")
	}
	content := builder.String()

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse large file: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks from large file")
	}

	t.Logf("Parsed %d chunks from large CSV file", len(chunks))

	// Verify memory efficiency - chunks should be reasonable size
	for i, chunk := range chunks {
		if len(chunk.Content) > 2000 {
			t.Errorf("Chunk %d too large: %d bytes", i, len(chunk.Content))
		}
	}
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50)

	// Create large content
	var builder strings.Builder
	for i := 0; i < 10000; i++ {
		builder.WriteString(string(rune(i)))
		builder.WriteString(",value")
		builder.WriteString(string(rune(i)))
		builder.WriteString("\n")
	}
	content := builder.String()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := parser.Parse(ctx, strings.NewReader(content))
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestParser_EmptyContent(t *testing.T) {
	parser := NewParser()

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(""))
	if err != nil {
		t.Fatalf("Failed to parse empty content: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got: %d", len(chunks))
	}
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()

	content := "a,b,c\n1,2,3\n"

	ctx := context.Background()
	expectedErr := "callback error"

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		return &testError{msg: expectedErr}
	})

	if err == nil || !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected callback error, got: %v", err)
	}
}

func TestParser_ChunkSizing(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50)
	parser.SetChunkOverlap(10)

	content := strings.Repeat("a,b,c,d,e\n", 100)

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks")
	}

	// Verify chunk sizes respect configuration
	for i, chunk := range chunks {
		if len(chunk.Content) > 100 { // chunkSize + overlap tolerance
			t.Errorf("Chunk %d exceeds size limit: %d bytes", i, len(chunk.Content))
		}
	}
}

func TestParser_MetadataCompleteness(t *testing.T) {
	parser := NewParser()

	content := "name,value\ntest,123\n"

	ctx := context.Background()
	var firstChunk *core.Chunk

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if firstChunk == nil {
			firstChunk = &chunk
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if firstChunk == nil {
		t.Fatal("Expected at least one chunk")
	}

	requiredFields := []string{"type", "position", "streaming"}
	for _, field := range requiredFields {
		if _, ok := firstChunk.Metadata[field]; !ok {
			t.Errorf("Missing required metadata field: %s", field)
		}
	}
}

func TestParser_SemicolonSeparator(t *testing.T) {
	parser := NewParser()
	parser.SetSeparator(';')

	content := "name;age;city\nAlice;25;Paris\nBob;30;London\n"

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks from semicolon-separated file")
	}
}

func TestParser_EscapedQuotes(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)

	content := `name,quote
John,"He said ""Hello"""
Jane,"It's a ""great"" day"
`

	ctx := context.Background()
	var foundEscaped bool

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if strings.Contains(chunk.Content, `Hello`) {
			foundEscaped = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if !foundEscaped {
		t.Error("Expected to find escaped quotes")
	}
}

// Helper type
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
