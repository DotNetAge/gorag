package core

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
)

// ChatMemory defines conversation history buffers crucial for multi-turn / Agentic RAG tasks.
// Matches LangChain's Memory concept (BufferMemory, WindowMemory, SummaryMemory).
type ChatMemory interface {
	// AddUserMessage appends a user query to memory
	AddUserMessage(ctx context.Context, sessionID string, msg string) error

	// AddAIMessage appends agent's response
	AddAIMessage(ctx context.Context, sessionID string, msg string) error

	// GetMessages returns conversational history tailored for context limits
	GetMessages(ctx context.Context, sessionID string) ([]chat.Message, error)

	// Clear flushes history
	Clear(ctx context.Context, sessionID string) error
}
