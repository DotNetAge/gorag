package vectorstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultVectorStore(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "vectorstore-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "govector")
	store, err := DefaultVectorStore(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	assert.NotNil(t, store)
}

func TestNewQdrantStore(t *testing.T) {
	// Skip if no Qdrant server available
	t.Skip("Skipping test - requires Qdrant server")

	store, err := NewQdrantStore("localhost:6334", "", "gorag")
	if err != nil {
		t.Logf("Qdrant not available: %v", err)
		return
	}
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewMilvusStore(t *testing.T) {
	// Skip if no Milvus server available
	t.Skip("Skipping test - requires Milvus server")

	store, err := NewMilvusStore("localhost:19530", "user", "pass", "gorag")
	if err != nil {
		t.Logf("Milvus not available: %v", err)
		return
	}
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewPineconeStore(t *testing.T) {
	// Skip if no Pinecone API key available
	t.Skip("Skipping test - requires Pinecone API key")

	store, err := NewPineconeStore("api-key", "gcp-starter", "gorag")
	if err != nil {
		t.Logf("Pinecone not available: %v", err)
		return
	}
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewWeaviateStore(t *testing.T) {
	// Skip if no Weaviate server available
	t.Skip("Skipping test - requires Weaviate server")

	store, err := NewWeaviateStore("localhost:8080", "http", "api-key", "GoRAG")
	if err != nil {
		t.Logf("Weaviate not available: %v", err)
		return
	}
	assert.NoError(t, err)
	assert.NotNil(t, store)
}
