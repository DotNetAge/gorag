package agentic

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockLLMJudge is a mock implementation of evaluation.LLMJudge
type MockLLMJudge struct {
	evaluateFaithfulnessFn func(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (float32, string, error)
}

func (m *MockLLMJudge) EvaluateFaithfulness(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (float32, string, error) {
	if m.evaluateFaithfulnessFn != nil {
		return m.evaluateFaithfulnessFn(ctx, query, chunks, answer)
	}
	return 0.9, "Good answer", nil
}

func (m *MockLLMJudge) EvaluateAnswerRelevance(ctx context.Context, query string, answer string) (float32, string, error) {
	return 0.9, "Good answer", nil
}

func (m *MockLLMJudge) EvaluateContextPrecision(ctx context.Context, query string, chunks []*entity.Chunk) (float32, string, error) {
	return 0.9, "Good context", nil
}

func TestSelfRAGStep_New(t *testing.T) {
	// Create a mock LLM judge
	mockJudge := &MockLLMJudge{}

	// Test with custom threshold
	step := NewSelfRAGStep(mockJudge, true, 0.85)
	assert.NotNil(t, step)
	assert.True(t, step.strictMode)
	assert.Equal(t, float32(0.85), step.scoreThreshold)

	// Test with negative threshold (should default to 0.8)
	step = NewSelfRAGStep(mockJudge, false, -0.1)
	assert.NotNil(t, step)
	assert.False(t, step.strictMode)
	assert.Equal(t, float32(0.8), step.scoreThreshold)

	// Test with zero threshold (should default to 0.8)
	step = NewSelfRAGStep(mockJudge, true, 0)
	assert.NotNil(t, step)
	assert.True(t, step.strictMode)
	assert.Equal(t, float32(0.8), step.scoreThreshold)
}

func TestSelfRAGStep_Name(t *testing.T) {
	// Create a mock LLM judge
	mockJudge := &MockLLMJudge{}

	// Create a SelfRAGStep
	step := NewSelfRAGStep(mockJudge, true, 0.8)

	// Test Name method
	name := step.Name()
	assert.Equal(t, "SelfRAGStep", name)
}

func TestSelfRAGStep_Execute_MissingData(t *testing.T) {
	// Create a mock LLM judge
	mockJudge := &MockLLMJudge{}

	// Create a SelfRAGStep
	step := NewSelfRAGStep(mockJudge, true, 0.8)

	// Test with missing query
	state1 := &entity.PipelineState{
		Answer: "Paris is the capital of France.",
		RetrievedChunks: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Paris is the capital of France."}},
		},
	}
	ctx := context.Background()
	err := step.Execute(ctx, state1)
	assert.NoError(t, err)

	// Test with missing answer
	state2 := &entity.PipelineState{
		Query: &entity.Query{Text: "What is the capital of France?"},
		RetrievedChunks: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Paris is the capital of France."}},
		},
	}
	err = step.Execute(ctx, state2)
	assert.NoError(t, err)

	// Test with missing retrieved chunks
	state3 := &entity.PipelineState{
		Query:  &entity.Query{Text: "What is the capital of France?"},
		Answer: "Paris is the capital of France.",
	}
	err = step.Execute(ctx, state3)
	assert.NoError(t, err)

	// Test with empty retrieved chunks
	state4 := &entity.PipelineState{
		Query:           &entity.Query{Text: "What is the capital of France?"},
		Answer:          "Paris is the capital of France.",
		RetrievedChunks: [][]*entity.Chunk{},
	}
	err = step.Execute(ctx, state4)
	assert.NoError(t, err)

	// Test with empty flattened chunks
	state5 := &entity.PipelineState{
		Query:           &entity.Query{Text: "What is the capital of France?"},
		Answer:          "Paris is the capital of France.",
		RetrievedChunks: [][]*entity.Chunk{{}},
	}
	err = step.Execute(ctx, state5)
	assert.NoError(t, err)
}

func TestSelfRAGStep_Execute_EvaluationPass(t *testing.T) {
	// Create a mock LLM judge that returns a high score
	mockJudge := &MockLLMJudge{
		evaluateFaithfulnessFn: func(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (float32, string, error) {
			assert.Equal(t, "What is the capital of France?", query)
			assert.Len(t, chunks, 1)
			assert.Equal(t, "Paris is the capital of France.", answer)
			return 0.9, "The answer is supported by the context.", nil
		},
	}

	// Create a SelfRAGStep
	step := NewSelfRAGStep(mockJudge, true, 0.8)

	// Create a pipeline state with all required data
	state := &entity.PipelineState{
		Query:  &entity.Query{Text: "What is the capital of France?"},
		Answer: "Paris is the capital of France.",
		RetrievedChunks: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Paris is the capital of France."}},
		},
	}

	// Test Execute method
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)

	// Check that evaluation metrics were set
	assert.Equal(t, float32(0.9), state.SelfRagScore)
	assert.Equal(t, "The answer is supported by the context.", state.SelfRagReason)
	assert.Equal(t, "Paris is the capital of France.", state.Answer)
}

func TestSelfRAGStep_Execute_EvaluationFail_StrictMode(t *testing.T) {
	// Create a mock LLM judge that returns a low score
	mockJudge := &MockLLMJudge{
		evaluateFaithfulnessFn: func(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (float32, string, error) {
			return 0.7, "The answer is not fully supported by the context.", nil
		},
	}

	// Create a SelfRAGStep in strict mode
	step := NewSelfRAGStep(mockJudge, true, 0.8)

	// Create a pipeline state with all required data
	state := &entity.PipelineState{
		Query:  &entity.Query{Text: "What is the capital of France?"},
		Answer: "Paris is the capital of France.",
		RetrievedChunks: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Paris is the capital of France."}},
		},
	}

	// Test Execute method in strict mode
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SelfRAG validation failed")
	assert.Contains(t, err.Error(), "0.700000 < 0.800000")
}

func TestSelfRAGStep_Execute_EvaluationFail_NonStrictMode(t *testing.T) {
	// Create a mock LLM judge that returns a low score
	mockJudge := &MockLLMJudge{
		evaluateFaithfulnessFn: func(ctx context.Context, query string, chunks []*entity.Chunk, answer string) (float32, string, error) {
			return 0.7, "The answer is not fully supported by the context.", nil
		},
	}

	// Create a SelfRAGStep in non-strict mode
	step := NewSelfRAGStep(mockJudge, false, 0.8)

	// Create a pipeline state with all required data
	state := &entity.PipelineState{
		Query:  &entity.Query{Text: "What is the capital of France?"},
		Answer: "Paris is the capital of France.",
		RetrievedChunks: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Paris is the capital of France."}},
		},
	}

	// Test Execute method in non-strict mode
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)

	// Check that evaluation metrics were set
	assert.Equal(t, float32(0.7), state.SelfRagScore)
	assert.Equal(t, "The answer is not fully supported by the context.", state.SelfRagReason)

	// Check that a warning was appended to the answer
	assert.Contains(t, state.Answer, "[Warning: System detected potential hallucinations in this answer.]")
}
