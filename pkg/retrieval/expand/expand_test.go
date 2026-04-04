package expand

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockDocStore struct {
	doc *core.Document
	err error
}

func (m *mockDocStore) SetDocument(ctx context.Context, doc *core.Document) error {
	return nil
}

func (m *mockDocStore) GetDocument(ctx context.Context, docID string) (*core.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.doc, nil
}

func (m *mockDocStore) DeleteDocument(ctx context.Context, docID string) error {
	return nil
}

func (m *mockDocStore) SetChunks(ctx context.Context, chunks []*core.Chunk) error {
	return nil
}

func (m *mockDocStore) GetChunk(ctx context.Context, chunkID string) (*core.Chunk, error) {
	return nil, nil
}

func (m *mockDocStore) GetChunksByDocID(ctx context.Context, docID string) ([]*core.Chunk, error) {
	return nil, nil
}

type mockCollectorForExpand struct{}

func (m *mockCollectorForExpand) RecordCount(name, value string, labels map[string]string) {}
func (m *mockCollectorForExpand) RecordDuration(name string, duration any, labels map[string]string) {
}
func (m *mockCollectorForExpand) RecordValue(name string, value float64, labels map[string]string) {}

func TestNewParentDoc(t *testing.T) {
	store := &mockDocStore{}
	expander := NewParentDoc(store)

	assert.NotNil(t, expander)
	assert.Equal(t, store, expander.docStore)
}

func TestParentDoc_Enhance_Success(t *testing.T) {
	store := &mockDocStore{
		doc: &core.Document{ID: "parent1", Content: "parent content"},
	}
	expander := NewParentDoc(store)

	chunks := []*core.Chunk{
		{ID: "c1", ParentID: "parent1", Content: "child content 1"},
		{ID: "c2", ParentID: "", Content: "standalone content"},
	}

	result, err := expander.Enhance(context.Background(), nil, chunks)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "parent content", result[0].Content)
	assert.Equal(t, "standalone content", result[1].Content)
}

func TestParentDoc_Enhance_EmptyChunks(t *testing.T) {
	expander := NewParentDoc(&mockDocStore{})

	result, err := expander.Enhance(context.Background(), nil, []*core.Chunk{})

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestParentDoc_Enhance_NilChunks(t *testing.T) {
	expander := NewParentDoc(&mockDocStore{})

	result, err := expander.Enhance(context.Background(), nil, nil)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestParentDoc_Enhance_NoParentID(t *testing.T) {
	store := &mockDocStore{}
	expander := NewParentDoc(store)

	chunks := []*core.Chunk{
		{ID: "c1", Content: "no parent content"},
	}

	result, err := expander.Enhance(context.Background(), nil, chunks)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "no parent content", result[0].Content)
}

func TestParentDoc_Enhance_ParentNotFound(t *testing.T) {
	store := &mockDocStore{err: errors.New("not found")}
	expander := NewParentDoc(store)

	chunks := []*core.Chunk{
		{ID: "c1", ParentID: "missing", Content: "child content"},
	}

	result, err := expander.Enhance(context.Background(), nil, chunks)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "child content", result[0].Content)
}

func TestParentDoc_Enhance_DuplicateParent(t *testing.T) {
	store := &mockDocStore{
		doc: &core.Document{ID: "parent1", Content: "parent content"},
	}
	expander := NewParentDoc(store)

	chunks := []*core.Chunk{
		{ID: "c1", ParentID: "parent1", Content: "child 1"},
		{ID: "c2", ParentID: "parent1", Content: "child 2"},
	}

	result, err := expander.Enhance(context.Background(), nil, chunks)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "parent content", result[0].Content)
}

func TestMergeMetadata(t *testing.T) {
	parent := map[string]any{"key1": "parent_value", "key2": "parent_value2"}
	child := map[string]any{"key2": "child_value", "key3": "child_value"}

	result := mergeMetadata(child, parent)

	assert.Equal(t, "parent_value", result["key1"])
	assert.Equal(t, "child_value", result["key2"])
	assert.Equal(t, "child_value", result["key3"])
}

func TestMergeMetadata_ChildOverrides(t *testing.T) {
	parent := map[string]any{"key": "parent"}
	child := map[string]any{"key": "child"}

	result := mergeMetadata(child, parent)

	assert.Equal(t, "child", result["key"])
}

func TestMergeMetadata_EmptyMaps(t *testing.T) {
	result := mergeMetadata(map[string]any{}, map[string]any{})

	assert.Empty(t, result)
}

func TestParentDoc_Enhance_PreservesChunkID(t *testing.T) {
	store := &mockDocStore{
		doc: &core.Document{ID: "parent1", Content: "parent content"},
	}
	expander := NewParentDoc(store)

	chunks := []*core.Chunk{
		{ID: "original_chunk_id", ParentID: "parent1", Content: "child"},
	}

	result, err := expander.Enhance(context.Background(), nil, chunks)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "original_chunk_id", result[0].ID)
}

func TestParentDoc_Enhance_MetadataMerge(t *testing.T) {
	store := &mockDocStore{
		doc: &core.Document{
			ID:       "parent1",
			Content:  "parent content",
			Metadata: map[string]any{"parent_key": "parent_meta"},
		},
	}
	expander := NewParentDoc(store)

	chunks := []*core.Chunk{
		{ID: "c1", ParentID: "parent1", Content: "child", Metadata: map[string]any{"child_key": "child_meta"}},
	}

	result, err := expander.Enhance(context.Background(), nil, chunks)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "parent_meta", result[0].Metadata["parent_key"])
	assert.Equal(t, "child_meta", result[0].Metadata["child_key"])
}
