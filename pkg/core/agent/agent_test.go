package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTool struct {
	name        string
	description string
	inputSchema map[string]any
	result      string
	isError     bool
	metadata    map[string]any
}

func (m *mockTool) Name() string { return m.name }

func (m *mockTool) Description() string { return m.description }

func (m *mockTool) InputSchema() map[string]any { return m.inputSchema }

func (m *mockTool) Call(ctx context.Context, input ToolInput) (*ToolResult, error) {
	return &ToolResult{
		Result:   m.result,
		IsError:  m.isError,
		Metadata: m.metadata,
	}, nil
}

func TestToolInput_MapBehavior(t *testing.T) {
	input := ToolInput{
		"query": "test query",
		"limit": 10,
	}

	assert.Equal(t, "test query", input["query"])
	assert.Equal(t, 10, input["limit"])
}

func TestToolResult_Structure(t *testing.T) {
	result := &ToolResult{
		Result:   "test result",
		IsError:  false,
		Metadata: map[string]any{"key": "value"},
	}

	assert.Equal(t, "test result", result.Result)
	assert.False(t, result.IsError)
	assert.Equal(t, "value", result.Metadata["key"])
}

func TestToolResult_WithError(t *testing.T) {
	result := &ToolResult{
		Result:  "error result",
		IsError: true,
	}

	assert.Equal(t, "error result", result.Result)
	assert.True(t, result.IsError)
}

func TestMockTool_Name(t *testing.T) {
	tool := &mockTool{
		name:        "search",
		description: "Search for documents",
		inputSchema: map[string]any{"type": "object"},
	}

	assert.Equal(t, "search", tool.Name())
}

func TestMockTool_Description(t *testing.T) {
	tool := &mockTool{
		name:        "search",
		description: "Search for documents",
	}

	assert.Equal(t, "Search for documents", tool.Description())
}

func TestMockTool_InputSchema(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
	}
	tool := &mockTool{inputSchema: schema}

	assert.Equal(t, schema, tool.InputSchema())
}

func TestMockTool_Call(t *testing.T) {
	tool := &mockTool{
		name:   "calculator",
		result: "42",
		metadata: map[string]any{
			"operation": "add",
		},
	}

	input := ToolInput{"a": 1, "b": 2}
	result, err := tool.Call(context.Background(), input)

	assert.NoError(t, err)
	assert.Equal(t, "42", result.Result)
	assert.False(t, result.IsError)
	assert.Equal(t, "add", result.Metadata["operation"])
}

func TestMockTool_CallWithError(t *testing.T) {
	tool := &mockTool{
		name:    "failing_tool",
		result:  "failed",
		isError: true,
	}

	input := ToolInput{"query": "test"}
	result, err := tool.Call(context.Background(), input)

	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Result)
	assert.True(t, result.IsError)
}

func TestAgentStep_Structure(t *testing.T) {
	step := AgentStep{
		Thought:     "I need to search for this",
		Action:      "search",
		ActionInput: ToolInput{"query": "test"},
		Observation: "Found 5 results",
	}

	assert.Equal(t, "I need to search for this", step.Thought)
	assert.Equal(t, "search", step.Action)
	assert.Equal(t, "test", step.ActionInput["query"])
	assert.Equal(t, "Found 5 results", step.Observation)
}

func TestAgentResponse_Structure(t *testing.T) {
	steps := []AgentStep{
		{
			Thought:     "Step 1 thought",
			Action:      "action1",
			ActionInput: ToolInput{"key": "value1"},
			Observation: "Observation 1",
		},
		{
			Thought:     "Step 2 thought",
			Action:      "action2",
			ActionInput: ToolInput{"key": "value2"},
			Observation: "Observation 2",
		},
	}

	response := &AgentResponse{
		Response: "Final answer",
		Steps:    steps,
	}

	assert.Equal(t, "Final answer", response.Response)
	assert.Len(t, response.Steps, 2)
	assert.Equal(t, "Step 1 thought", response.Steps[0].Thought)
}

func TestAgentResponse_EmptySteps(t *testing.T) {
	response := &AgentResponse{
		Response: "Direct answer",
		Steps:    []AgentStep{},
	}

	assert.Equal(t, "Direct answer", response.Response)
	assert.Empty(t, response.Steps)
}

func TestToolInput_Empty(t *testing.T) {
	input := ToolInput{}
	assert.Empty(t, input)
	input["key"] = "value"
	assert.Equal(t, "value", input["key"])
}

func TestToolInput_MultipleKeys(t *testing.T) {
	input := ToolInput{
		"string_key": "string_value",
		"int_key":    42,
		"float_key":  3.14,
		"bool_key":   true,
		"map_key":    map[string]any{"nested": "value"},
	}

	assert.Equal(t, "string_value", input["string_key"])
	assert.Equal(t, 42, input["int_key"])
	assert.Equal(t, 3.14, input["float_key"])
	assert.Equal(t, true, input["bool_key"])
	assert.Equal(t, "value", input["map_key"].(map[string]any)["nested"])
}
