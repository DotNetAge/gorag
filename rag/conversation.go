package rag

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Message represents a message in a conversation
type Message struct {
	ID        string
	Role      string // "user" or "assistant"
	Content   string
	Timestamp time.Time
}

// Conversation represents a multi-turn conversation
type Conversation struct {
	ID        string
	Messages  []Message
	CreatedAt time.Time
	UpdatedAt time.Time
	mu        sync.RWMutex
}

// NewConversation creates a new conversation
func NewConversation() *Conversation {
	return &Conversation{
		ID:        uuid.New().String(),
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// AddMessage adds a message to the conversation
func (c *Conversation) AddMessage(role, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	message := Message{
		ID:        uuid.New().String(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	c.Messages = append(c.Messages, message)
	c.UpdatedAt = time.Now()
}

// GetRecentMessages gets the most recent messages
func (c *Conversation) GetRecentMessages(max int) []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.Messages) <= max {
		// Return a copy to avoid race conditions
		result := make([]Message, len(c.Messages))
		copy(result, c.Messages)
		return result
	}
	result := make([]Message, max)
	copy(result, c.Messages[len(c.Messages)-max:])
	return result
}

// GetContext returns the conversation context as a string
func (c *Conversation) GetContext(maxMessages int) string {
	messages := c.GetRecentMessages(maxMessages)
	var context strings.Builder

	for _, msg := range messages {
		if msg.Role == "user" {
			context.WriteString("User: " + msg.Content + "\n")
		} else {
			context.WriteString("Assistant: " + msg.Content + "\n")
		}
	}

	return strings.TrimSpace(context.String())
}

// ConversationManager manages multiple conversations
type ConversationManager struct {
	conversations map[string]*Conversation
	mu            sync.RWMutex
}

// NewConversationManager creates a new conversation manager
func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		conversations: make(map[string]*Conversation),
	}
}

// CreateConversation creates a new conversation
func (cm *ConversationManager) CreateConversation() *Conversation {
	conv := NewConversation()
	cm.mu.Lock()
	cm.conversations[conv.ID] = conv
	cm.mu.Unlock()
	return conv
}

// GetConversation gets a conversation by ID
func (cm *ConversationManager) GetConversation(id string) *Conversation {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.conversations[id]
}

// UpdateConversation updates a conversation
func (cm *ConversationManager) UpdateConversation(conv *Conversation) {
	cm.mu.Lock()
	cm.conversations[conv.ID] = conv
	cm.mu.Unlock()
}

// DeleteConversation deletes a conversation
func (cm *ConversationManager) DeleteConversation(id string) {
	cm.mu.Lock()
	delete(cm.conversations, id)
	cm.mu.Unlock()
}

// ListConversations lists all conversations
func (cm *ConversationManager) ListConversations() []*Conversation {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conversations := make([]*Conversation, 0, len(cm.conversations))
	for _, conv := range cm.conversations {
		conversations = append(conversations, conv)
	}
	return conversations
}

// ConversationOptions configures conversation behavior
type ConversationOptions struct {
	ConversationID string
	MaxHistory    int
	PromptTemplate string
}

// QueryWithConversation performs a RAG query with conversation history
func (e *Engine) QueryWithConversation(ctx context.Context, question string, opts ConversationOptions) (*Response, error) {
	// Get or create conversation
	var conversation *Conversation
	if opts.ConversationID != "" {
		if e.conversationManager != nil {
			conversation = e.conversationManager.GetConversation(opts.ConversationID)
		}
	}

	if conversation == nil {
		if e.conversationManager != nil {
			conversation = e.conversationManager.CreateConversation()
		} else {
			conversation = NewConversation()
		}
	}

	// Add user message to conversation
	conversation.AddMessage("user", question)

	// Get conversation context
	maxHistory := opts.MaxHistory
	if maxHistory <= 0 {
		maxHistory = 5 // Default: 5 most recent messages
	}
	conversationContext := conversation.GetContext(maxHistory)

	// Enhance query with conversation context
	enhancedQuestion := question
	if conversationContext != "" {
		enhancedQuestion = fmt.Sprintf("%s\n\nConversation history:\n%s", question, conversationContext)
	}

	// Perform RAG query
	queryOpts := QueryOptions{
		TopK:           5,
		PromptTemplate: opts.PromptTemplate,
		Stream:         false,
	}

	response, err := e.Query(ctx, enhancedQuestion, queryOpts)
	if err != nil {
		return nil, err
	}

	// Add assistant response to conversation
	if response != nil {
		conversation.AddMessage("assistant", response.Answer)
		if e.conversationManager != nil {
			e.conversationManager.UpdateConversation(conversation)
		}
	}

	return response, nil
}
