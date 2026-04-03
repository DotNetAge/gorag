package query

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
)

// StepBack abstracts queries into higher-level background questions.
type StepBack struct {
	llm chat.Client
}

// NewStepBack creates a new StepBack generator.
func NewStepBack(llm chat.Client) *StepBack {
	return &StepBack{llm: llm}
}

// GenerateStepBackQuery generates a step-back query.
func (s *StepBack) GenerateStepBackQuery(ctx context.Context, query *core.Query) (*core.Query, error) {
	if query == nil || query.Text == "" {
		return nil, fmt.Errorf("query is nil or empty")
	}

	prompt := fmt.Sprintf(`You are an expert at core.
The user is asking a very specific question. To answer it correctly, we first need to retrieve broader background information.
Please write a "Step-back" question that asks for the underlying principles, concepts, or historical background related to the original question.
Only return the Step-back question, nothing else.

Original question: "%s"`, query.Text)

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	response, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	newQuery := core.NewQuery(uuid.New().String(), strings.TrimSpace(response.Content), nil)
	return newQuery, nil
}
