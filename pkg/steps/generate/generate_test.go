package stepgen

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockGenerator struct {
	result *core.Result
	err    error
}

func (m *mockGenerator) Generate(ctx context.Context, query *core.Query, chunks []*core.Chunk) (*core.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockGenerator) GenerateHypotheticalDocument(ctx context.Context, query *core.Query) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.result.Answer, nil
}

type mockLoggerForGen struct{}

func (m *mockLoggerForGen) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForGen) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForGen) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForGen) Error(msg string, err error, fields ...map[string]any) {}

func TestGenerate_Name(t *testing.T) {
	gen := &mockGenerator{result: &core.Result{Answer: "test"}}
	step := Generate(gen, nil, nil)
	assert.Equal(t, "Generator", step.Name())
}

func TestGenerate_Execute_Success(t *testing.T) {
	gen := &mockGenerator{result: &core.Result{Answer: "Generated answer"}}
	step := Generate(gen, &mockLoggerForGen{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1", Content: "chunk1"}, {ID: "c2", Content: "chunk2"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Answer)
	assert.Equal(t, "Generated answer", state.Answer.Answer)
}

func TestGenerate_Execute_NilQuery(t *testing.T) {
	gen := &mockGenerator{result: &core.Result{Answer: "test"}}
	step := Generate(gen, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerate_Execute_EmptyQuery(t *testing.T) {
	gen := &mockGenerator{result: &core.Result{Answer: "test"}}
	step := Generate(gen, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "", nil),
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerate_Execute_GeneratorError(t *testing.T) {
	gen := &mockGenerator{err: errors.New("generation failed")}
	step := Generate(gen, &mockLoggerForGen{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1", Content: "chunk1"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Generate failed")
}

func TestGenerate_Execute_EmptyChunks(t *testing.T) {
	gen := &mockGenerator{result: &core.Result{Answer: "Generated answer"}}
	step := Generate(gen, &mockLoggerForGen{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:           core.NewQuery("1", "test query", nil),
		RetrievedChunks: [][]*core.Chunk{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Answer)
}

func TestGenerate_Execute_FlattensChunks(t *testing.T) {
	gen := &mockGenerator{result: &core.Result{Answer: "Generated"}}
	step := Generate(gen, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1", Content: "chunk1"}},
			{{ID: "c2", Content: "chunk2"}},
			{{ID: "c3", Content: "chunk3"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Answer)
}

func TestGenerate_WithNilLogger(t *testing.T) {
	gen := &mockGenerator{result: &core.Result{Answer: "test"}}
	step := Generate(gen, nil, nil)

	assert.NotNil(t, step)
	assert.Equal(t, "Generator", step.Name())
}
