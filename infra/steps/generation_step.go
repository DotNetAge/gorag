package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step = (*GenerationStep)(nil)

// GenerationStep takes the final query and retrieved chunks, builds a prompt, and generates the answer.
type GenerationStep struct {
	llm abstraction.LLMClient
}

// NewGenerationStep creates a standard generation step.
func NewGenerationStep(llm abstraction.LLMClient) *GenerationStep {
	return &GenerationStep{llm: llm}
}

func (s *GenerationStep) Execute(ctx context.Context, state *pipeline.State) error {
	query, ok := state.Get("query").(*entity.Query)
	if !ok {
		return fmt.Errorf("GenerationStep: 'query' (*entity.Query) not found in state")
	}

	var contextBuilder strings.Builder
	if chunks, ok := state.Get("retrieved_chunks").([]*entity.Chunk); ok {
		for i, chunk := range chunks {
			contextBuilder.WriteString(fmt.Sprintf("--- Document %d ---\n%s\n\n", i+1, chunk.Content))
		}
	}

	prompt := fmt.Sprintf(`You are a helpful and professional AI assistant.
Please answer the user's question based on the provided reference documents.
If the documents do not contain the answer, say "I don't know based on the provided context."

[Reference Documents]
%s

[User Question]
%s

Answer:`, contextBuilder.String(), query.Text)

	// Update state with the final prompt being used
	state.Set("generation_prompt", prompt)

	response, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("GenerationStep failed: %w", err)
	}

	state.Set("answer", response)

	return nil
}
