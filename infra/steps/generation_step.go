package steps

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*GenerationStep)(nil)

// GenerationStep takes the final query and retrieved chunks, builds a prompt, and generates the answer.
type GenerationStep struct {
	llm chat.Client
}

// NewGenerationStep creates a standard generation step.
func NewGenerationStep(llm chat.Client) *GenerationStep {
	return &GenerationStep{llm: llm}
}

func (s *GenerationStep) Name() string {
	return "GenerationStep"
}

func (s *GenerationStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("GenerationStep: 'query' not found in state")
	}

	var contextBuilder strings.Builder
	for i, chunks := range state.RetrievedChunks {
		for j, chunk := range chunks {
			contextBuilder.WriteString(fmt.Sprintf("--- Document %d-%d --\n%s\n\n", i+1, j+1, chunk.Content))
		}
	}

	prompt := fmt.Sprintf(`You are a helpful and professional AI assistant.
Please answer the user's question based on the provided reference documents.
If the documents do not contain the answer, say "I don't know based on the provided context."

[Reference Documents]
%s

[User Question]
%s

Answer:`, contextBuilder.String(), state.Query.Text)

	// Update state with the final prompt being used
	state.GenerationPrompt = prompt

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}
	response, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("GenerationStep failed: %w", err)
	}

	state.Answer = response.Content

	return nil
}
