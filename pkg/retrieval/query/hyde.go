package query

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
)

// ensure interface implementation
var _ core.HyDEGenerator = (*HyDE)(nil)

// HyDE generates hypothetical answers to improve search results.
type HyDE struct {
	llm chat.Client
}

// NewHyDE creates a new HyDE generator.
func NewHyDE(llm chat.Client) *HyDE {
	return &HyDE{llm: llm}
}

// Generate implements core.HyDEGenerator.
func (h *HyDE) Generate(ctx context.Context, query *core.Query) (string, error) {
	prompt := fmt.Sprintf(`Please write a paragraph answering the following question.
Write it as if you are a domain expert. Even if you don't know the exact answer, make an educated guess using relevant terminology and keywords.
Do not include conversational filler like "Here is an answer".

Question: "%s"`, query.Text)

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	response, err := h.llm.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response.Content), nil
}

// GenerateHypotheticalDocument implements core.HyDEGenerator compatibility.
func (h *HyDE) GenerateHypotheticalDocument(ctx context.Context, query *core.Query) (string, error) {
	return h.Generate(ctx, query)
}
