// Package semantic provides semantic chunking utilities for RAG systems.
// It implements advanced chunking strategies based on semantic similarity
// and sentence boundaries to create more meaningful chunks for retrieval.
package semantic

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ dataprep.SemanticChunker = (*SemanticChunker)(nil)

// SemanticChunker implements semantic chunking based on sentence boundaries
// and semantic similarity, using gochat's embedding.Provider.
// It creates chunks that are semantically coherent and optimized for retrieval.
type SemanticChunker struct {
	// embedder is the embedding provider used to calculate semantic similarity
	embedder embedding.Provider
	// maxChunkSize is the maximum size of a chunk in characters
	maxChunkSize int
	// minChunkSize is the minimum size of a chunk in characters
	minChunkSize int
	// similarityThreshold is the threshold for semantic similarity between sentences
	similarityThreshold float32
}

// NewSemanticChunker creates a new semantic chunker with the given parameters.
//
// Parameters:
// - embedder: The embedding provider to use for semantic similarity calculations
// - minSize: The minimum chunk size in characters
// - maxSize: The maximum chunk size in characters
// - threshold: The semantic similarity threshold for chunk boundaries
//
// Returns:
// - A new SemanticChunker instance
func NewSemanticChunker(embedder embedding.Provider, minSize, maxSize int, threshold float32) *SemanticChunker {
	if minSize <= 0 {
		minSize = 100
	}
	if maxSize <= 0 {
		maxSize = 1000
	}
	if threshold <= 0 {
		threshold = 0.85
	}
	return &SemanticChunker{
		embedder:            embedder,
		maxChunkSize:        maxSize,
		minChunkSize:        minSize,
		similarityThreshold: threshold,
	}
}

// Chunk splits a single document into a slice of chunks based on semantic similarity of sentences.
// This is the standard chunking implementation that creates semantically coherent chunks.
//
// Parameters:
// - ctx: The context for the operation
// - doc: The document to split
//
// Returns:
// - A slice of chunks
// - An error if chunking fails
func (c *SemanticChunker) Chunk(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, error) {
	// 1. Split text into sentences
	sentences := c.splitIntoSentences(doc.Content)
	if len(sentences) == 0 {
		return nil, nil
	}

	// 2. Get embeddings for all sentences
	embeddings, err := c.getSentenceEmbeddings(ctx, sentences)
	if err != nil {
		return nil, fmt.Errorf("failed to get sentence embeddings: %w", err)
	}

	// 3. Group sentences into chunks based on semantic similarity
	var chunks []*entity.Chunk
	var currentChunk strings.Builder
	var currentSize int
	var startIndex int

	// Add the first sentence to the current chunk
	currentChunk.WriteString(sentences[0])
	currentSize += len([]rune(sentences[0]))

	for i := 1; i < len(sentences); i++ {
		// Calculate similarity between the current sentence and the previous one
		sim := cosineSimilarity(embeddings[i-1], embeddings[i])

		sentenceSize := len([]rune(sentences[i]))

		// Decide whether to split or append
		// If similarity is low (topic change) AND current chunk is big enough
		// OR current chunk is too big
		shouldSplit := (sim < c.similarityThreshold && currentSize >= c.minChunkSize) ||
			(currentSize+sentenceSize > c.maxChunkSize)

		if shouldSplit && currentSize > 0 {
			// Save the current chunk
			chunks = append(chunks, entity.NewChunk(
				uuid.New().String(),
				doc.ID,
				currentChunk.String(),
				startIndex,
				startIndex+currentSize,
				c.inheritMetadata(doc.Metadata),
			))

			// Start a new chunk
			currentChunk.Reset()
			startIndex += currentSize
			currentSize = 0
		}

		// Append sentence to the current chunk
		if currentChunk.Len() > 0 && !strings.HasSuffix(currentChunk.String(), " ") {
			currentChunk.WriteString(" ")
			currentSize++
		}
		currentChunk.WriteString(sentences[i])
		currentSize += sentenceSize
	}

	// Add the last chunk if not empty
	if currentSize > 0 {
		chunks = append(chunks, entity.NewChunk(
			uuid.New().String(),
			doc.ID,
			currentChunk.String(),
			startIndex,
			startIndex+currentSize,
			c.inheritMetadata(doc.Metadata),
		))
	}

	return chunks, nil
}

// HierarchicalChunk creates Parent-Child relationships for fine-grained retrieval
// but broad context augmentation. This pattern allows for retrieving specific
// chunks while maintaining access to broader context.
//
// Parameters:
// - ctx: The context for the operation
// - doc: The document to split
//
// Returns:
// - A slice of parent chunks (larger, more context-rich)
// - A slice of child chunks (smaller, more specific)
// - An error if chunking fails
func (c *SemanticChunker) HierarchicalChunk(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, []*entity.Chunk, error) {
	// First, do a rough split for Parent Chunks (e.g., using a larger max size or standard semantic logic)
	// For demonstration, we just use the standard Chunk method as the "Parent" 
	// and then sub-divide them further for "Children".
	parents, err := c.Chunk(ctx, doc)
	if err != nil {
		return nil, nil, err
	}

	var allChildren []*entity.Chunk

	for _, parent := range parents {
		parent.Level = 1 // Mark as Parent

		// Temporarily reduce the max chunk size for child chunks
		originalMax := c.maxChunkSize
		c.maxChunkSize = originalMax / 4 // Force smaller chunks
		
		// Create a temporary document from the parent chunk to reuse the Chunk logic
		tempDoc := &entity.Document{
			ID:       parent.ID, // Inherit parent ID conceptually
			Content:  parent.Content,
			Metadata: parent.Metadata,
		}
		
		children, err := c.Chunk(ctx, tempDoc)
		
		// Restore max size
		c.maxChunkSize = originalMax
		
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate child chunks for parent %s: %w", parent.ID, err)
		}

		for _, child := range children {
			child.ParentID = parent.ID
			child.Level = 2 // Mark as Child
			child.DocumentID = doc.ID
			allChildren = append(allChildren, child)
		}
	}

	return parents, allChildren, nil
}

// ContextualChunk injects a document-level summary into each child chunk's content
// to preserve global context (Anthropic's Contextual Retrieval pattern).
// This helps maintain context awareness during retrieval and generation.
//
// Parameters:
// - ctx: The context for the operation
// - doc: The document to split
// - docSummary: A summary of the document to inject into each chunk
//
// Returns:
// - A slice of chunks with injected context
// - An error if chunking fails
func (c *SemanticChunker) ContextualChunk(ctx context.Context, doc *entity.Document, docSummary string) ([]*entity.Chunk, error) {
	chunks, err := c.Chunk(ctx, doc)
	if err != nil {
		return nil, err
	}

	for _, chunk := range chunks {
		// Prepend the summary context to the chunk content.
		// This ensures that when the chunk is Embedded later, the dense vector
		// carries the global context of the document.
		chunk.Content = fmt.Sprintf("Document Context: %s\n\nChunk Content: %s", docSummary, chunk.Content)
		chunk.Metadata["is_contextual"] = true
	}

	return chunks, nil
}

// --- Helper Functions ---

// inheritMetadata creates a copy of the document metadata to attach to chunks.
//
// Parameters:
// - docMeta: The document metadata to copy
//
// Returns:
// - A copy of the metadata
func (c *SemanticChunker) inheritMetadata(docMeta map[string]any) map[string]any {
	meta := make(map[string]any)
	for k, v := range docMeta {
		meta[k] = v
	}
	return meta
}

// getSentenceEmbeddings calculates embeddings for a slice of sentences.
//
// Parameters:
// - ctx: The context for the operation
// - sentences: The sentences to embed
//
// Returns:
// - A slice of embeddings
// - An error if embedding fails
func (c *SemanticChunker) getSentenceEmbeddings(ctx context.Context, sentences []string) ([][]float32, error) {
	// Create a batch processor using gochat's embedding system
	batchProcessor := embedding.NewBatchProcessor(c.embedder, embedding.BatchOptions{
		MaxBatchSize:  32,
		MaxConcurrent: 4,
	})

	embeddings, err := batchProcessor.Process(ctx, sentences)
	if err != nil {
		return nil, err
	}

	// We need to convert the float64 embeddings from gochat to float32 for our domain entity
	var result [][]float32
	for _, e := range embeddings {
		var f32Emb []float32
		for _, val := range e {
			f32Emb = append(f32Emb, float32(val))
		}
		result = append(result, f32Emb)
	}

	return result, nil
}

// splitIntoSentences splits a text into individual sentences based on punctuation and whitespace.
//
// Parameters:
// - text: The text to split
//
// Returns:
// - A slice of sentences
func (c *SemanticChunker) splitIntoSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		current.WriteRune(r)

		// Simple sentence boundary detection
		if r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？' || r == '\n' {
			// Check if it's the end of a word/sentence
			if i+1 == len(runes) || unicode.IsSpace(runes[i+1]) {
				str := strings.TrimSpace(current.String())
				if len(str) > 0 {
					sentences = append(sentences, str)
				}
				current.Reset()
			}
		}
	}

	str := strings.TrimSpace(current.String())
	if len(str) > 0 {
		sentences = append(sentences, str)
	}

	return sentences
}

// cosineSimilarity calculates the cosine similarity between two vectors.
//
// Parameters:
// - a: The first vector
// - b: The second vector
//
// Returns:
// - The cosine similarity score (0-1)
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
