package testkit

import (
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// NewTestQuery creates a new query for testing.
func NewTestQuery(text string, metadata map[string]interface{}) *entity.Query {
	return &entity.Query{
		Text:     text,
		Metadata: metadata,
	}
}

// NewTestPipelineState creates a new pipeline state for testing.
func NewTestPipelineState() *entity.PipelineState {
	return &entity.PipelineState{
		Query:           nil,
		RetrievedChunks: make([][]*entity.Chunk, 0),
		Answer:          "",
		Agentic:         entity.NewAgenticMetadata(),
	}
}

// NewTestChunk creates a new chunk for testing.
func NewTestChunk(content string, metadata map[string]interface{}) *entity.Chunk {
	return &entity.Chunk{
		ID:       "test-chunk-" + content[:5],
		Content:  content,
		Metadata: metadata,
	}
}

// NewTestDocument creates a new document for testing.
func NewTestDocument(id, content, docType string) *entity.Document {
	return &entity.Document{
		ID:      id,
		Content: content,
		Metadata: map[string]interface{}{
			"type": docType,
		},
	}
}

// CreateTestChunks creates multiple test chunks.
func CreateTestChunks(contents ...string) []*entity.Chunk {
	chunks := make([]*entity.Chunk, len(contents))
	for i, content := range contents {
		chunks[i] = NewTestChunk(content, map[string]interface{}{"source": "test"})
	}
	return chunks
}

// WithQuery sets the query in the pipeline state.
func WithQuery(state *entity.PipelineState, query *entity.Query) *entity.PipelineState {
	state.Query = query
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	if query != nil && query.Text != "" {
		state.Agentic.OriginalQueryText = query.Text
	}
	return state
}

// WithRetrievedChunks sets the retrieved chunks in the pipeline state.
func WithRetrievedChunks(state *entity.PipelineState, chunks ...[]*entity.Chunk) *entity.PipelineState {
	state.RetrievedChunks = chunks
	return state
}

// WithAnswer sets the answer in the pipeline state.
func WithAnswer(state *entity.PipelineState, answer string) *entity.PipelineState {
	state.Answer = answer
	return state
}
