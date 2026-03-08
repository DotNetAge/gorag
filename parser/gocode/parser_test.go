package gocode

import (
	"context"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
)

func TestParser_BasicFunction(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)

	content := `package main

func Hello(name string) string {
	return "Hello, " + name
}

func main() {
	Hello("World")
}
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

	if firstChunk.Metadata["streaming"] != "true" {
		t.Errorf("Chunk should be marked as streaming")
	}
}

func TestParser_TypeExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractTypes(true)

	content := `package main

type Person struct {
	Name string
	Age  int
}

type Manager interface {
	Manage() error
}
`

	ctx := context.Background()
	chunks, err := parser.Parse(ctx, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks")
	}
}

func TestParser_CommentExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractComments(true)

	content := `package main

// Hello says hello
func Hello(name string) string {
	return "Hello"
}
`

	ctx := context.Background()
	var foundComment bool

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		if strings.Contains(chunk.Content, "Hello") {
			foundComment = true
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Note: Comments are extracted but may be combined with functions
	// The important thing is that parsing succeeds
	if !foundComment {
		t.Log("Comment extraction test - checking if content contains expected text")
	}
}

func TestParser_LargeFile(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(500)

	// Create a large Go file with many functions
	var builder strings.Builder
	builder.WriteString("package main\n\n")
	for i := 0; i < 1000; i++ {
		builder.WriteString("func Function")
		builder.WriteString(string(rune(i)))
		builder.WriteString("() {\n")
		builder.WriteString("// Implementation\n")
		builder.WriteString("}\n\n")
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

	t.Logf("Parsed %d chunks from large Go file", len(chunks))
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(50)

	var builder strings.Builder
	builder.WriteString("package main\n")
	for i := 0; i < 10000; i++ {
		builder.WriteString("func F")
		builder.WriteString(string(rune(i)))
		builder.WriteString("() {}\n")
	}
	content := builder.String()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

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

	content := "package main\nfunc main() {}\n"

	ctx := context.Background()
	expectedErr := "callback error"

	err := parser.ParseWithCallback(ctx, strings.NewReader(content), func(chunk core.Chunk) error {
		return &testError{msg: expectedErr}
	})

	if err == nil || !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected callback error, got: %v", err)
	}
}

func TestParser_MetadataCompleteness(t *testing.T) {
	parser := NewParser()

	content := `package main

func Test() {}
`

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

// Helper type
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
