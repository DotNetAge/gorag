package pattern

import (
	"context"
	"os"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeRAG_BasicFlow(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "gorag_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	// Create NativeRAG with minimal config
	rag, err := NativeRAG("test-app",
		WithBoltDoc(tmpDir+"/docs.bolt"),
		WithGoVector("test", tmpDir+"/vectors.db", 128),
	)
	require.NoError(t, err)
	require.NotNil(t, rag)
	
	// Test Indexer interface
	idx := rag.Indexer()
	assert.NotNil(t, idx)
	assert.NotNil(t, idx.VectorStore())
	assert.NotNil(t, idx.DocStore())
	
	// Test Retriever interface
	ret := rag.Retriever()
	assert.NotNil(t, ret)
	
	// Test Repository interface
	repo := rag.Repository()
	assert.NotNil(t, repo)
}

func TestNativeRAG_IndexText(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	rag, err := NativeRAG("test-index",
		WithBoltDoc(tmpDir+"/docs.bolt"),
		WithGoVector("test", tmpDir+"/vectors.db", 128),
	)
	require.NoError(t, err)
	
	// Test IndexText
	err = rag.IndexText(ctx, "This is a test document about GoRAG framework.", map[string]any{
		"source": "test",
		"author": "tester",
	})
	assert.NoError(t, err)
	
	// Test IndexTexts
	texts := []string{
		"Document 1: Introduction to RAG.",
		"Document 2: Advanced RAG techniques.",
		"Document 3: Production deployment.",
	}
	err = rag.IndexTexts(ctx, texts, map[string]any{
		"batch": "test",
	})
	assert.NoError(t, err)
}

func TestNativeRAG_Retrieve(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	rag, err := NativeRAG("test-retrieve",
		WithBoltDoc(tmpDir+"/docs.bolt"),
		WithGoVector("test", tmpDir+"/vectors.db", 128),
	)
	require.NoError(t, err)
	
	// Index some documents
	docs := []string{
		"GoRAG is a high-performance RAG framework for Go.",
		"It supports multiple retrieval strategies including vector search and graph-based retrieval.",
		"The framework is designed for production use with built-in observability.",
	}
	err = rag.IndexTexts(ctx, docs)
	require.NoError(t, err)
	
	// Test retrieval
	results, err := rag.Retrieve(ctx, []string{"What is GoRAG?"}, 3)
	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "What is GoRAG?", results[0].Query)
}

func TestNativeRAG_WithEnhancements(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	// Create NativeRAG with query rewrite and fusion
	rag, err := NativeRAG("test-enhanced",
		WithBoltDoc(tmpDir+"/docs.bolt"),
		WithGoVector("test", tmpDir+"/vectors.db", 128),
		WithQueryRewrite(),
		WithFusion(3),
	)
	require.NoError(t, err)
	require.NotNil(t, rag)
	
	// Index documents
	err = rag.IndexText(ctx, "Test document for enhancement features.")
	require.NoError(t, err)
	
	// Retrieve with enhancements
	results, err := rag.Retrieve(ctx, []string{"test query"}, 5)
	assert.NoError(t, err)
	assert.NotNil(t, results)
}

func TestNativeRAG_Delete(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	rag, err := NativeRAG("test-delete",
		WithBoltDoc(tmpDir+"/docs.bolt"),
		WithGoVector("test", tmpDir+"/vectors.db", 128),
	)
	require.NoError(t, err)
	
	// Index a document with known ID
	doc := core.NewDocument("test-doc-1", "Test document for deletion.", "text", "text/plain", nil)
	err = rag.Indexer().IndexDocuments(ctx, doc)
	require.NoError(t, err)
	
	// Test delete
	err = rag.Delete(ctx, "test-doc-1")
	assert.NoError(t, err)
}

func TestNativeRAG_Repository(t *testing.T) {
	ctx := context.Background()
	
	tmpDir, err := os.MkdirTemp("", "gorag_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	rag, err := NativeRAG("test-repo",
		WithBoltDoc(tmpDir+"/docs.bolt"),
		WithGoVector("test", tmpDir+"/vectors.db", 128),
	)
	require.NoError(t, err)
	
	repo := rag.Repository()
	require.NotNil(t, repo)
	
	// Test Create
	entity := &testEntity{id: "test-entity-1", name: "Test Entity"}
	content := "Test entity content for repository"
	err = repo.Create(ctx, "test_collection", entity, content)
	assert.NoError(t, err)
	
	// Test Delete
	err = repo.Delete(ctx, "test_collection", "test-entity-1")
	assert.NoError(t, err)
}

// testEntity is a simple entity for testing
type testEntity struct {
	id   string
	name string
}

func (e *testEntity) GetID() string {
	return e.id
}
