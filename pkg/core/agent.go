package core

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
)

// AgentStep tracks intermediate thoughts, tool calls, and observations for ReAct/Agentic RAG.
type AgentStep struct {
	Thought     string
	Action      string
	ActionInput ToolInput
	Observation string
}

// AgentResponse is the final multi-hop output from an Agent.
type AgentResponse struct {
	Response string
	Steps    []AgentStep
}

// Agent core interface (mimicking LangChain's AgentExecutor / LlamaIndex AgentRunner).
type Agent interface {
	// Name defines the persona/agent logic
	Name() string

	// AddTool gives the agent additional capabilities
	AddTool(tool Tool)

	// Chat executes the ReAct loop dynamically planning and resolving tools until a final answer.
	Chat(ctx context.Context, query string, history []chat.Message) (*AgentResponse, error)

	// Memory returns the agent's contextual memory store (if applicable).
	Memory() ChatMemory
}
