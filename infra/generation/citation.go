package generation

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// SimpleLLMClient represents the required LLM functions.
type SimpleLLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// CitationGenerator handles the "Citation & Grounding" spec.
// It injects citation markers into the context and forces the LLM to use them.
type CitationGenerator struct {
	llm SimpleLLMClient
}

func NewCitationGenerator(llm SimpleLLMClient) *CitationGenerator {
	return &CitationGenerator{llm: llm}
}

// GenerateWithCitations formats the retrieved chunks with markers [doc_1], [doc_2]
// and instructs the LLM to strictly cite its claims.
func (g *CitationGenerator) GenerateWithCitations(ctx context.Context, query string, chunks []*entity.Chunk) (string, error) {
	var contextBuilder strings.Builder
	
	// Inject Citation Markers
	for i, chunk := range chunks {
		marker := fmt.Sprintf("[doc_%d]", i+1)
		contextBuilder.WriteString(fmt.Sprintf("%s\n%s\n\n", marker, chunk.Content))
	}

	prompt := fmt.Sprintf(`You are a professional assistant. Please answer the user's question based STRICTLY on the provided documents.
You MUST cite your sources using the exact document markers provided (e.g., [doc_1], [doc_2]).
If a claim cannot be supported by the documents, do not make it. If the documents don't contain the answer, say "I don't have enough information."

[Documents]
%s

[Question]
%s

Answer:`, contextBuilder.String(), query)

	return g.llm.Generate(ctx, prompt)
}
