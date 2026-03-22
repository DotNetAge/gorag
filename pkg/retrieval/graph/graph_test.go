package graph

import (
	"context"
	"errors"
	"testing"

	gchat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMForGraph struct {
	response *gchat.Response
	err      error
}

func (m *mockLLMForGraph) Chat(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMForGraph) ChatStream(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Stream, error) {
	return nil, nil
}

func TestNewGraphExtractor(t *testing.T) {
	llm := &mockLLMForGraph{response: &gchat.Response{Content: "{}"}}
	extractor := NewGraphExtractor(llm)

	assert.NotNil(t, extractor)
	assert.Equal(t, llm, extractor.llm)
}

func TestDefaultGraphExtractor(t *testing.T) {
	llm := &mockLLMForGraph{response: &gchat.Response{Content: "{}"}}
	extractor := DefaultGraphExtractor(llm)

	assert.NotNil(t, extractor)
	assert.Equal(t, llm, extractor.llm)
}

func TestExtract_Success(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{
			Content: `{"nodes":[{"id":"Alice","type":"PERSON","properties":{"age":30}}],"edges":[{"source":"Alice","target":"Bob","type":"KNOWS","properties":{}}]}`,
		},
	}
	extractor := NewGraphExtractor(llm)

	chunks := []*core.Chunk{{ID: "chunk1", Content: "Alice knows Bob"}}

	nodes, edges, err := extractor.Extract(context.Background(), chunks[0])

	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Len(t, edges, 1)
	assert.Equal(t, "Alice", nodes[0].ID)
	assert.Equal(t, "PERSON", nodes[0].Type)
	assert.Equal(t, "Alice", edges[0].Source)
	assert.Equal(t, "Bob", edges[0].Target)
	assert.Equal(t, "KNOWS", edges[0].Type)
}

func TestExtract_EmptyChunk(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{
			Content: `{"nodes":[],"edges":[]}`,
		},
	}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "chunk1", Content: ""}
	nodes, edges, err := extractor.Extract(context.Background(), chunk)

	assert.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Empty(t, edges)
}

func TestExtract_LLMError(t *testing.T) {
	llm := &mockLLMForGraph{err: errors.New("llm error")}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "chunk1", Content: "test content"}
	nodes, edges, err := extractor.Extract(context.Background(), chunk)

	assert.Error(t, err)
	assert.Nil(t, nodes)
	assert.Nil(t, edges)
}

func TestExtract_InvalidJSON(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{Content: "not valid json"},
	}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "chunk1", Content: "test content"}
	nodes, edges, err := extractor.Extract(context.Background(), chunk)

	assert.Error(t, err)
	assert.Nil(t, nodes)
	assert.Nil(t, edges)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestExtract_WithMarkdown(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{
			Content: "```json\n{\"nodes\":[{\"id\":\"Test\",\"type\":\"CONCEPT\"}],\"edges\":[]}\n```",
		},
	}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "chunk1", Content: "Test concept"}
	nodes, _, err := extractor.Extract(context.Background(), chunk)

	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "Test", nodes[0].ID)
}

func TestExtract_ChunkMetadataPropagation(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{
			Content: `{"nodes":[{"id":"Entity1","type":"TEST"}],"edges":[]}`,
		},
	}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "my-chunk-id", Content: "test"}
	nodes, _, err := extractor.Extract(context.Background(), chunk)

	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "my-chunk-id", nodes[0].Properties["source_chunk_id"])
}

func TestExtractResult_Structure(t *testing.T) {
	result := extractResult{
		Nodes: []core.Node{
			{ID: "node1", Type: "PERSON"},
			{ID: "node2", Type: "ORG"},
		},
		Edges: []core.Edge{
			{Source: "node1", Target: "node2", Type: "WORKS_AT"},
		},
	}

	assert.Len(t, result.Nodes, 2)
	assert.Len(t, result.Edges, 1)
	assert.Equal(t, "node1", result.Nodes[0].ID)
	assert.Equal(t, "WORKS_AT", result.Edges[0].Type)
}

func TestExtract_MultipleNodes(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{
			Content: `{"nodes":[{"id":"A","type":"PERSON"},{"id":"B","type":"PERSON"},{"id":"C","type":"ORG"}],"edges":[{"source":"A","target":"B","type":"KNOWS"},{"source":"A","target":"C","type":"WORKS_AT"}]}`,
		},
	}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "chunk1", Content: "A knows B and works at C"}
	nodes, edges, err := extractor.Extract(context.Background(), chunk)

	assert.NoError(t, err)
	assert.Len(t, nodes, 3)
	assert.Len(t, edges, 2)
}

func TestExtract_EdgeMetadataPropagation(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{
			Content: `{"nodes":[{"id":"Source","type":"A"}],"edges":[{"source":"Source","target":"Target","type":"RELATES_TO"}]}`,
		},
	}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "edge-chunk", Content: "test"}
	_, edges, err := extractor.Extract(context.Background(), chunk)

	assert.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.Equal(t, "edge-chunk", edges[0].Properties["source_chunk_id"])
}

func TestExtract_NilPropertiesInitialized(t *testing.T) {
	llm := &mockLLMForGraph{
		response: &gchat.Response{
			Content: `{"nodes":[{"id":"Test","type":"CONCEPT"}],"edges":[]}`,
		},
	}
	extractor := NewGraphExtractor(llm)

	chunk := &core.Chunk{ID: "chunk1", Content: "test"}
	nodes, _, err := extractor.Extract(context.Background(), chunk)

	assert.NoError(t, err)
	assert.NotNil(t, nodes[0].Properties)
	assert.Equal(t, "chunk1", nodes[0].Properties["source_chunk_id"])
}
