package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*toolExecutor)(nil)

// toolExecutor is a thin adapter that uses gochat's built-in tool calling capability.
type toolExecutor struct {
	llm    core.Client
	logger logging.Logger
}

// NewToolExecutor creates a new tool executor step with logger.
func NewToolExecutor(llm core.Client, logger logging.Logger) *toolExecutor {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &toolExecutor{
		llm:    llm,
		logger: logger,
	}
}

// Name returns the step name
func (s *toolExecutor) Name() string {
	return "ToolExecutor"
}

// Execute executes tools using gochat's native tool calling mechanism.
// This is a thin adapter (<30 lines) that delegates to gochat client.
func (s *toolExecutor) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("toolExecutor: query required")
	}

	s.logger.Debug("checking if tool execution needed", map[string]interface{}{
		"step":  "ToolExecutor",
		"query": state.Query.Text,
	})

	// Check if fallback is needed (from CRAGEvaluatorStep)
	if !NeedsFallback(state) {
		s.logger.Debug("fallback not needed, skipping tool execution", map[string]interface{}{
			"step": "ToolExecutor",
		})
		return nil // Skip if no fallback needed
	}

	s.logger.Info("executing tools", map[string]interface{}{
		"step":  "ToolExecutor",
		"query": state.Query.Text,
	})

	// Use gochat's native tool calling with Options
	tools := getToolSchemas()
	var opts []core.Option
	if len(tools) > 0 {
		opts = append(opts, core.WithTools(tools...))
	}

	result, err := s.llm.Chat(ctx, []core.Message{
		core.NewUserMessage(state.Query.Text),
	}, opts...)

	if err != nil {
		s.logger.Error("tool chat failed", err, map[string]interface{}{
			"step":  "ToolExecutor",
			"query": state.Query.Text,
		})
		return fmt.Errorf("toolExecutor: Chat failed: %w", err)
	}

	// Update state using AgenticMetadata (thin adapter 職责)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	if len(result.ToolCalls) > 0 {
		state.Agentic.ToolExecuted = true
		s.logger.Info("tools executed successfully", map[string]interface{}{
			"step":       "ToolExecutor",
			"tool_count": len(result.ToolCalls),
			"query":      state.Query.Text,
		})
	}

	return nil
}

// getToolSchemas returns tool schemas for LLM tool calling.
// TODO: This should come from infra/service or configuration.
func getToolSchemas() []core.Tool {
	return []core.Tool{} // Placeholder - implement based on use case
}
