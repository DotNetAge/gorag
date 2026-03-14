package indexer

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockParser is a mock implementation of dataprep.Parser
type MockParser struct {
	parseStreamFn func(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *entity.Document, error)
}

func (m *MockParser) ParseStream(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	if m.parseStreamFn != nil {
		return m.parseStreamFn(ctx, reader, metadata)
	}
	// Return an empty channel
	ch := make(chan *entity.Document)
	close(ch)
	return ch, nil
}

func (m *MockParser) GetSupportedTypes() []string {
	return []string{"txt"}
}

// MockSemanticChunker is a mock implementation of dataprep.SemanticChunker
type MockSemanticChunker struct {
	hierarchicalChunkFn func(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, []*entity.Chunk, error)
	chunkFn             func(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, error)
	contextualChunkFn   func(ctx context.Context, doc *entity.Document, docSummary string) ([]*entity.Chunk, error)
}

func (m *MockSemanticChunker) HierarchicalChunk(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, []*entity.Chunk, error) {
	if m.hierarchicalChunkFn != nil {
		return m.hierarchicalChunkFn(ctx, doc)
	}
	return nil, nil, nil
}

func (m *MockSemanticChunker) Chunk(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, error) {
	if m.chunkFn != nil {
		return m.chunkFn(ctx, doc)
	}
	return nil, nil
}

func (m *MockSemanticChunker) ContextualChunk(ctx context.Context, doc *entity.Document, docSummary string) ([]*entity.Chunk, error) {
	if m.contextualChunkFn != nil {
		return m.contextualChunkFn(ctx, doc, docSummary)
	}
	return nil, nil
}

// MockEmbeddingProvider is a mock implementation of embedding.Provider
type MockEmbeddingProvider struct {
	embedFn     func(ctx context.Context, texts []string) ([][]float32, error)
	dimensionFn func() int
}

func (m *MockEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, texts)
	}
	// Return empty embeddings
	return make([][]float32, len(texts)), nil
}

func (m *MockEmbeddingProvider) Dimension() int {
	if m.dimensionFn != nil {
		return m.dimensionFn()
	}
	return 1536 // Default dimension
}

// MockVectorStore is a mock implementation of abstraction.VectorStore
type MockVectorStore struct {
	addFn func(ctx context.Context, vector *entity.Vector) error
}

func (m *MockVectorStore) Add(ctx context.Context, vector *entity.Vector) error {
	if m.addFn != nil {
		return m.addFn(ctx, vector)
	}
	return nil
}

func (m *MockVectorStore) AddBatch(ctx context.Context, vectors []*entity.Vector) error {
	return nil
}

func (m *MockVectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]any) ([]*entity.Vector, []float32, error) {
	return nil, nil, nil
}

func (m *MockVectorStore) SearchByText(ctx context.Context, query string, topK int, filter map[string]any) ([]*entity.Vector, error) {
	return nil, nil
}

func (m *MockVectorStore) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockVectorStore) DeleteBatch(ctx context.Context, ids []string) error {
	return nil
}

func (m *MockVectorStore) Get(ctx context.Context, id string) (*entity.Vector, error) {
	return nil, nil
}

func (m *MockVectorStore) GetBatch(ctx context.Context, ids []string) ([]*entity.Vector, error) {
	return nil, nil
}

func (m *MockVectorStore) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockVectorStore) Close(ctx context.Context) error {
	return nil
}

// MockGraphStore is a mock implementation of abstraction.GraphStore
type MockGraphStore struct {
	upsertNodesFn func(ctx context.Context, nodes []*abstraction.Node) error
	upsertEdgesFn func(ctx context.Context, edges []*abstraction.Edge) error
}

func (m *MockGraphStore) UpsertNodes(ctx context.Context, nodes []*abstraction.Node) error {
	if m.upsertNodesFn != nil {
		return m.upsertNodesFn(ctx, nodes)
	}
	return nil
}

func (m *MockGraphStore) UpsertEdges(ctx context.Context, edges []*abstraction.Edge) error {
	if m.upsertEdgesFn != nil {
		return m.upsertEdgesFn(ctx, edges)
	}
	return nil
}

func (m *MockGraphStore) SearchNodes(ctx context.Context, query string, topK int) ([]*abstraction.Node, error) {
	return nil, nil
}

func (m *MockGraphStore) SearchEdges(ctx context.Context, source string, target string, topK int) ([]*abstraction.Edge, error) {
	return nil, nil
}

func (m *MockGraphStore) GetNode(ctx context.Context, id string) (*abstraction.Node, error) {
	return nil, nil
}

func (m *MockGraphStore) GetEdge(ctx context.Context, id string) (*abstraction.Edge, error) {
	return nil, nil
}

func (m *MockGraphStore) CreateNode(ctx context.Context, node *abstraction.Node) error {
	return nil
}

func (m *MockGraphStore) CreateEdge(ctx context.Context, edge *abstraction.Edge) error {
	return nil
}

func (m *MockGraphStore) DeleteNode(ctx context.Context, id string) error {
	return nil
}

func (m *MockGraphStore) DeleteEdge(ctx context.Context, id string) error {
	return nil
}

func (m *MockGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	return nil, nil
}

func (m *MockGraphStore) GetNeighbors(ctx context.Context, nodeID string, limit int) ([]*abstraction.Node, error) {
	return nil, nil
}

func (m *MockGraphStore) GetCommunitySummaries(ctx context.Context, limit int) ([]map[string]any, error) {
	return nil, nil
}

func (m *MockGraphStore) Close(ctx context.Context) error {
	return nil
}

// MockGraphExtractor is a mock implementation of dataprep.GraphExtractor
type MockGraphExtractor struct {
	extractFn func(ctx context.Context, chunk *entity.Chunk) ([]abstraction.Node, []abstraction.Edge, error)
}

func (m *MockGraphExtractor) Extract(ctx context.Context, chunk *entity.Chunk) ([]abstraction.Node, []abstraction.Edge, error) {
	if m.extractFn != nil {
		return m.extractFn(ctx, chunk)
	}
	return nil, nil, nil
}

func TestConcurrentIndexer_New(t *testing.T) {
	// Create mock dependencies
	mockParser := &MockParser{}
	mockChunker := &MockSemanticChunker{}
	mockEmbedder := &MockEmbeddingProvider{}
	mockVectorStore := &MockVectorStore{}
	mockGraphStore := &MockGraphStore{}
	mockExtractor := &MockGraphExtractor{}

	// Test with custom options
	opts := IndexerOptions{
		ParseWorkers:  2,
		EmbedWorkers:  2,
		UpsertWorkers: 5,
	}

	// Create a ConcurrentIndexer
	indexer := NewConcurrentIndexer(
		mockParser,
		mockChunker,
		mockEmbedder,
		mockVectorStore,
		mockGraphStore,
		mockExtractor,
		opts,
	)

	// Check that the indexer is created correctly
	assert.NotNil(t, indexer)
	assert.Equal(t, 2, indexer.parseWorkers)
	assert.Equal(t, 2, indexer.embedWorkers)
	assert.Equal(t, 5, indexer.upsertWorkers)
	assert.NotNil(t, indexer.parser)
	assert.NotNil(t, indexer.chunker)
	assert.NotNil(t, indexer.embedder)
	assert.NotNil(t, indexer.vectorStore)
	assert.NotNil(t, indexer.graphStore)
	assert.NotNil(t, indexer.extractor)
}

func TestConcurrentIndexer_New_DefaultOptions(t *testing.T) {
	// Create mock dependencies
	mockParser := &MockParser{}
	mockChunker := &MockSemanticChunker{}
	mockEmbedder := &MockEmbeddingProvider{}
	mockVectorStore := &MockVectorStore{}

	// Test with default options (negative values)
	opts := IndexerOptions{
		ParseWorkers:  -1,
		EmbedWorkers:  -1,
		UpsertWorkers: -1,
	}

	// Create a ConcurrentIndexer
	indexer := NewConcurrentIndexer(
		mockParser,
		mockChunker,
		mockEmbedder,
		mockVectorStore,
		nil, // No graph store
		nil, // No extractor
		opts,
	)

	// Check that default values are used
	assert.NotNil(t, indexer)
	assert.Equal(t, 5, indexer.parseWorkers)   // Default parse workers
	assert.Equal(t, 4, indexer.embedWorkers)   // Default embed workers
	assert.Equal(t, 10, indexer.upsertWorkers) // Default upsert workers
	assert.Nil(t, indexer.graphStore)
	assert.Nil(t, indexer.extractor)
}

func TestConcurrentIndexer_IndexFile(t *testing.T) {
	// Create a temporary test file
	tempFile, err := os.CreateTemp("", "test-*.txt")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write some content to the file
	_, err = tempFile.WriteString("Test content")
	assert.NoError(t, err)
	tempFile.Close()

	// Create mock dependencies
	mockParser := &MockParser{
		parseStreamFn: func(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
			ch := make(chan *entity.Document, 1)
			doc := &entity.Document{
				ID:       "doc1",
				Content:  "Test content",
				Metadata: metadata,
			}
			ch <- doc
			close(ch)
			return ch, nil
		},
	}

	mockChunker := &MockSemanticChunker{
		hierarchicalChunkFn: func(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, []*entity.Chunk, error) {
			// Return some chunks
			parent := &entity.Chunk{
				ID:       "parent1",
				Content:  "Test content",
				Metadata: doc.Metadata,
			}
			child := &entity.Chunk{
				ID:       "child1",
				Content:  "Test content",
				Metadata: doc.Metadata,
			}
			return []*entity.Chunk{parent}, []*entity.Chunk{child}, nil
		},
	}

	mockEmbedder := &MockEmbeddingProvider{
		embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			// Return some embeddings
			embeddings := make([][]float32, len(texts))
			for i := range texts {
				embeddings[i] = []float32{0.1, 0.2, 0.3}
			}
			return embeddings, nil
		},
	}

	// Track if Add was called
	addCalled := false
	mockVectorStore := &MockVectorStore{
		addFn: func(ctx context.Context, vector *entity.Vector) error {
			addCalled = true
			assert.NotNil(t, vector)
			assert.Equal(t, "child1", vector.ID)
			assert.Equal(t, []float32{0.1, 0.2, 0.3}, vector.Values)
			return nil
		},
	}

	// Create a ConcurrentIndexer
	indexer := NewConcurrentIndexer(
		mockParser,
		mockChunker,
		mockEmbedder,
		mockVectorStore,
		nil,
		nil,
		IndexerOptions{},
	)

	// Test IndexFile
	ctx := context.Background()
	err = indexer.IndexFile(ctx, tempFile.Name())
	assert.NoError(t, err)

	// Check that Add was called
	assert.True(t, addCalled)
}

func TestConcurrentIndexer_IndexDirectory(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "test-dir-")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test file in the directory
	testFile := filepath.Join(tempDir, "test1.txt")
	err = os.WriteFile(testFile, []byte("Test content 1"), 0644)
	assert.NoError(t, err)

	// Create another test file in the directory
	testFile2 := filepath.Join(tempDir, "test2.txt")
	err = os.WriteFile(testFile2, []byte("Test content 2"), 0644)
	assert.NoError(t, err)

	// Create mock dependencies
	fileCount := 0
	mockParser := &MockParser{
		parseStreamFn: func(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
			fileCount++
			ch := make(chan *entity.Document, 1)
			doc := &entity.Document{
				ID:       "doc" + string(rune(fileCount)),
				Content:  "Test content",
				Metadata: metadata,
			}
			ch <- doc
			close(ch)
			return ch, nil
		},
	}

	mockChunker := &MockSemanticChunker{
		hierarchicalChunkFn: func(ctx context.Context, doc *entity.Document) ([]*entity.Chunk, []*entity.Chunk, error) {
			// Return some chunks
			child := &entity.Chunk{
				ID:       "child" + doc.ID,
				Content:  doc.Content,
				Metadata: doc.Metadata,
			}
			return nil, []*entity.Chunk{child}, nil
		},
	}

	mockEmbedder := &MockEmbeddingProvider{
		embedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			// Return some embeddings
			embeddings := make([][]float32, len(texts))
			for i := range texts {
				embeddings[i] = []float32{0.1, 0.2, 0.3}
			}
			return embeddings, nil
		},
	}

	// Track if Add was called
	addCount := 0
	mockVectorStore := &MockVectorStore{
		addFn: func(ctx context.Context, vector *entity.Vector) error {
			addCount++
			assert.NotNil(t, vector)
			return nil
		},
	}

	// Create a ConcurrentIndexer
	indexer := NewConcurrentIndexer(
		mockParser,
		mockChunker,
		mockEmbedder,
		mockVectorStore,
		nil,
		nil,
		IndexerOptions{
			ParseWorkers:  2,
			EmbedWorkers:  2,
			UpsertWorkers: 2,
		},
	)

	// Test IndexDirectory with recursive=true
	ctx := context.Background()
	err = indexer.IndexDirectory(ctx, tempDir, true)
	assert.NoError(t, err)

	// Check that both files were processed
	assert.Equal(t, 2, fileCount)
	assert.Equal(t, 2, addCount)
}
