package text

import (
	"context"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/embedding/ollama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemanticParser_Parse(t *testing.T) {
	// Create embedder
	embedder, err := ollama.New(ollama.Config{
		Model: "qllama/bge-small-zh-v1.5:latest",
	})
	require.NoError(t, err)

	// Create semantic parser
	parser := NewSemanticParser(embedder)

	// Test text
	testText := `
Go is an open source programming language designed for simplicity and efficiency. It is statically typed and compiled, with garbage collection and concurrency support.

Python is a high-level programming language known for its readability and simplicity. It has a large ecosystem of libraries and frameworks for various applications.

JavaScript is a scripting language that is primarily used for web development. It is now also used for server-side development with Node.js.
`

	// Parse text
	chunks, err := parser.Parse(context.Background(), strings.NewReader(testText))
	require.NoError(t, err)

	// Verify chunks
	assert.Greater(t, len(chunks), 0)
	for i, chunk := range chunks {
		t.Logf("Chunk %d: %s", i+1, chunk.Content)
		assert.NotEmpty(t, chunk.Content)
		assert.NotEmpty(t, chunk.ID)
		assert.Equal(t, "text", chunk.Metadata["type"])
		assert.Equal(t, "semantic", chunk.Metadata["method"])
	}
}

func TestSemanticParser_SplitIntoSentences(t *testing.T) {
	// Create parser (embedder can be nil for sentence splitting test)
	parser := NewSemanticParser(nil)

	testText := "Hello world! How are you? I'm fine."
	sentences := parser.splitIntoSentences(testText)

	// The test text has 3 sentences
	assert.GreaterOrEqual(t, len(sentences), 3)
	assert.Contains(t, sentences, "Hello world!")
	assert.Contains(t, sentences, "How are you?")
	assert.Contains(t, sentences, "I'm fine.")
	t.Logf("Found sentences: %v", sentences)
}

func TestSemanticParser_CosineSimilarity(t *testing.T) {
	// Create parser
	parser := NewSemanticParser(nil)

	// Test identical vectors
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0, 3.0}
	similarity := parser.cosineSimilarity(a, b)
	assert.InDelta(t, 1.0, similarity, 0.001)

	// Test orthogonal vectors
	a = []float32{1.0, 0.0, 0.0}
	b = []float32{0.0, 1.0, 0.0}
	similarity = parser.cosineSimilarity(a, b)
	assert.InDelta(t, 0.0, similarity, 0.001)
}
