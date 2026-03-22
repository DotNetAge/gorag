package hyde

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockGeneratorForHyde struct {
	hypotheticalDoc string
	err             error
}

func (m *mockGeneratorForHyde) Generate(ctx context.Context, query *core.Query, chunks []*core.Chunk) (*core.Result, error) {
	return nil, nil
}

func (m *mockGeneratorForHyde) GenerateHypotheticalDocument(ctx context.Context, query *core.Query) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.hypotheticalDoc, nil
}

type mockLoggerForHyde struct{}

func (m *mockLoggerForHyde) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForHyde) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForHyde) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForHyde) Error(msg string, err error, fields ...map[string]any) {}

func TestGenerate_Name(t *testing.T) {
	step := Generate(nil, nil)
	assert.Equal(t, "HyDE-Generate", step.Name())
}

func TestGenerate_Execute_Success(t *testing.T) {
	gen := &mockGeneratorForHyde{hypotheticalDoc: "Hypothetical document content"}
	step := Generate(gen, &mockLoggerForHyde{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.Equal(t, "Hypothetical document content", state.Agentic.HypotheticalDocument)
	assert.True(t, state.Agentic.HydeApplied)
}

func TestGenerate_Execute_NilQuery(t *testing.T) {
	gen := &mockGeneratorForHyde{hypotheticalDoc: "doc"}
	step := Generate(gen, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerate_Execute_EmptyQuery(t *testing.T) {
	gen := &mockGeneratorForHyde{hypotheticalDoc: "doc"}
	step := Generate(gen, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "", nil),
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerate_Execute_GeneratorError(t *testing.T) {
	gen := &mockGeneratorForHyde{err: errors.New("generation failed")}
	step := Generate(gen, &mockLoggerForHyde{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate doc")
}

func TestGenerate_Execute_WithNilLogger(t *testing.T) {
	gen := &mockGeneratorForHyde{hypotheticalDoc: "doc"}
	step := Generate(gen, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
}

func TestGenerate_Execute_CreatesAgenticContext(t *testing.T) {
	gen := &mockGeneratorForHyde{hypotheticalDoc: "doc"}
	step := Generate(gen, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: nil,
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.Equal(t, "doc", state.Agentic.HypotheticalDocument)
}
