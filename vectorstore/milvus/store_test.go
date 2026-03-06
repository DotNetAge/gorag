package milvus

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMilvusClient is a mock Milvus client for testing
type mockMilvusClient struct {
	hasCollectionCalled bool
	hasCollectionResult bool
	createCollectionCalled bool
	createIndexCalled bool
	loadCollectionCalled bool
	insertCalled bool
	searchCalled bool
	deleteCalled bool
	closeCalled bool
}

func (m *mockMilvusClient) HasCollection(ctx context.Context, collection string) (bool, error) {
	m.hasCollectionCalled = true
	return m.hasCollectionResult, nil
}

func (m *mockMilvusClient) CreateCollection(ctx context.Context, schema interface{}, shardsNum int) error {
	m.createCollectionCalled = true
	return nil
}

func (m *mockMilvusClient) CreateIndex(ctx context.Context, collection string, field string, index interface{}, async bool) error {
	m.createIndexCalled = true
	return nil
}

func (m *mockMilvusClient) LoadCollection(ctx context.Context, collection string, async bool) error {
	m.loadCollectionCalled = true
	return nil
}

func (m *mockMilvusClient) Insert(ctx context.Context, collection string, partitionName string, columns ...interface{}) (interface{}, error) {
	m.insertCalled = true
	return nil, nil
}

func (m *mockMilvusClient) Search(ctx context.Context, collection string, partitionNames []string, expr string, outputFields []string, vectors []interface{}, vectorField string, metricType interface{}, topK int, params interface{}) (interface{}, error) {
	m.searchCalled = true
	return nil, nil
}

func (m *mockMilvusClient) Delete(ctx context.Context, collection string, partitionName string, expr string) error {
	m.deleteCalled = true
	return nil
}

func (m *mockMilvusClient) Close() error {
	m.closeCalled = true
	return nil
}

func TestNewStore(t *testing.T) {
	// This test would typically require a real Milvus connection
	// For now, we'll skip it and focus on unit tests for other methods
	t.Skip("Skipping Milvus connection test - requires running Milvus server")
}

func TestStore_Add(t *testing.T) {
	// Create a mock store (we'll use a real Store struct but with mock dependencies)
	store := &Store{
		dimension: 1536,
	}

	// Test with empty chunks
	err := store.Add(context.Background(), []vectorstore.Chunk{}, [][]float32{})
	require.NoError(t, err)

	// Test with mismatched lengths
	err = store.Add(context.Background(), []vectorstore.Chunk{{ID: "1", Content: "test"}}, [][]float32{})
	require.NoError(t, err)
}

func TestStore_Search(t *testing.T) {
	// Test with default options - this will fail because client is nil
	// We'll skip this test since it requires a real Milvus client
	t.Skip("Skipping Milvus search test - requires real client")
}

func TestStore_Delete(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test with empty IDs
	err := store.Delete(context.Background(), []string{})
	require.NoError(t, err)
}

func TestStore_Close(t *testing.T) {
	// Create a mock store
	store := &Store{}

	// Test close
	err := store.Close()
	require.NoError(t, err)
}

func TestIntIDsToString(t *testing.T) {
	// Test with empty slice
	result := intIDsToString([]int64{})
	assert.Equal(t, "", result)

	// Test with single ID
	result = intIDsToString([]int64{1})
	assert.Equal(t, "1", result)

	// Test with multiple IDs
	result = intIDsToString([]int64{1, 2, 3})
	assert.Equal(t, "1,2,3", result)
}

func TestOptions(t *testing.T) {
	// Test WithCollection option
	store := &Store{collection: "default"}
	WithCollection("test")(store)
	assert.Equal(t, "test", store.collection)

	// Test WithDimension option
	store = &Store{dimension: 1536}
	WithDimension(768)(store)
	assert.Equal(t, 768, store.dimension)
}
