package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// needsFallback checks if the context needs fallback to external search (CRAG irrelevant).
func needsFallback(state *entity.PipelineState) bool {
	if state.Agentic != nil {
		return state.Agentic.CRAGEvaluation == "irrelevant"
	}
	return false
}

// toolExecutor is a thin adapter that uses gochat's built-in tool calling capability.
type toolExecutor struct {
	llm    core.Client
	tools  []core.Tool
	logger logging.Logger
}

// ToolExecutor creates a tool executor step with injected tools and logger.
//
// Parameters:
//   - llm: LLM client for tool execution
//   - tools: tool schemas to pass to the LLM (pass nil or empty to skip tool-calling mode)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//
// Example:
//
//	p.AddStep(agentic.ToolExecutor(llm, tools, logger))
func ToolExecutor(llm core.Client, tools []core.Tool, logger logging.Logger) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &toolExecutor{
		llm:    llm,
		tools:  tools,
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

	// Check if fallback is needed (from CRAGEvaluator)
	if !needsFallback(state) {
		s.logger.Debug("fallback not needed, skipping tool execution", map[string]interface{}{
			"step": "ToolExecutor",
		})
		return nil // Skip if no fallback needed
	}

	s.logger.Info("executing tools", map[string]interface{}{
		"step":  "ToolExecutor",
		"query": state.Query.Text,
	})

	// Use gochat's native tool calling with injected tools
	var opts []core.Option
	if len(s.tools) > 0 {
		opts = append(opts, core.WithTools(s.tools...))
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

	// Update state using AgenticMetadata (thin adapter 职责)
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
