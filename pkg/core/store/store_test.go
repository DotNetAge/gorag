package store

import (
	"context"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockChatMemory struct {
	messages map[string][]core.Message
}

func (m *mockChatMemory) AddUserMessage(ctx context.Context, sessionID string, msg string) error {
	if m.messages == nil {
		m.messages = make(map[string][]core.Message)
	}
	m.messages[sessionID] = append(m.messages[sessionID], core.NewUserMessage(msg))
	return nil
}

func NewAIMessage(text string) core.Message {
	return core.NewTextMessage(core.RoleAssistant, text)
}

func (m *mockChatMemory) AddAIMessage(ctx context.Context, sessionID string, msg string) error {
	if m.messages == nil {
		m.messages = make(map[string][]core.Message)
	}
	m.messages[sessionID] = append(m.messages[sessionID], NewAIMessage(msg))
	return nil
}

func (m *mockChatMemory) GetMessages(ctx context.Context, sessionID string) ([]core.Message, error) {
	if m.messages == nil {
		return []core.Message{}, nil
	}
	return m.messages[sessionID], nil
}

func (m *mockChatMemory) Clear(ctx context.Context, sessionID string) error {
	delete(m.messages, sessionID)
	return nil
}

func TestIndexStruct_DefaultValues(t *testing.T) {
	idx := &IndexStruct{}
	assert.Empty(t, idx.IndexID)
	assert.Empty(t, idx.Type)
	assert.Nil(t, idx.Nodes)
	assert.Empty(t, idx.Summary)
	assert.Nil(t, idx.Config)
}

func TestIndexStruct_WithValues(t *testing.T) {
	idx := &IndexStruct{
		IndexID: "idx-001",
		Type:    "vector",
		Nodes:   []string{"node1", "node2"},
		Summary: "Test index",
		Config: map[string]any{
			"dimension": 768,
		},
	}

	assert.Equal(t, "idx-001", idx.IndexID)
	assert.Equal(t, "vector", idx.Type)
	assert.Len(t, idx.Nodes, 2)
	assert.Equal(t, "Test index", idx.Summary)
	assert.Equal(t, 768, idx.Config["dimension"])
}

func TestMockChatMemory_AddAndGetMessages(t *testing.T) {
	mem := &mockChatMemory{}

	ctx := context.Background()
	sessionID := "session-1"

	err := mem.AddUserMessage(ctx, sessionID, "Hello")
	assert.NoError(t, err)

	err = mem.AddAIMessage(ctx, sessionID, "Hi there!")
	assert.NoError(t, err)

	messages, err := mem.GetMessages(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "Hello", messages[0].TextContent())
	assert.Equal(t, "Hi there!", messages[1].TextContent())
}

func TestMockChatMemory_GetMessages_EmptySession(t *testing.T) {
	mem := &mockChatMemory{}
	ctx := context.Background()

	messages, err := mem.GetMessages(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, messages)
}

func TestMockChatMemory_Clear(t *testing.T) {
	mem := &mockChatMemory{}
	ctx := context.Background()
	sessionID := "session-1"

	mem.AddUserMessage(ctx, sessionID, "Hello")
	mem.AddAIMessage(ctx, sessionID, "Hi")

	messages, _ := mem.GetMessages(ctx, sessionID)
	assert.Len(t, messages, 2)

	err := mem.Clear(ctx, sessionID)
	assert.NoError(t, err)

	messages, _ = mem.GetMessages(ctx, sessionID)
	assert.Empty(t, messages)
}

func TestMockChatMemory_Clear_NonExistentSession(t *testing.T) {
	mem := &mockChatMemory{}
	ctx := context.Background()

	err := mem.Clear(ctx, "nonexistent")
	assert.NoError(t, err)
}

func TestMockChatMemory_MultipleSessions(t *testing.T) {
	mem := &mockChatMemory{}
	ctx := context.Background()

	mem.AddUserMessage(ctx, "session-1", "Message 1")
	mem.AddUserMessage(ctx, "session-2", "Message 2")

	messages1, _ := mem.GetMessages(ctx, "session-1")
	messages2, _ := mem.GetMessages(ctx, "session-2")

	assert.Len(t, messages1, 1)
	assert.Len(t, messages2, 1)
	assert.Equal(t, "Message 1", messages1[0].TextContent())
	assert.Equal(t, "Message 2", messages2[0].TextContent())
}
