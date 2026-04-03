package core

import (
	"context"
)

// ToolInput schema (often defined via JSON schema in LLM calls)
type ToolInput map[string]any

// ToolResult represents the output of a tool invocation
type ToolResult struct {
	Result   string
	IsError  bool
	Metadata map[string]any
}

// Tool is the basic abstraction for Agentic tasks (like Search, Calculator, WebScraper).
type Tool interface {
	// Name must be unique within an agent
	Name() string

	// Description provides semantic understanding to the LLM
	Description() string

	// InputSchema returns JSON schema describing the required arguments
	InputSchema() map[string]any

	// Call executes the tool logic
	Call(ctx context.Context, input ToolInput) (*ToolResult, error)
}
