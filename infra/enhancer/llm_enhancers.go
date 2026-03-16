// Package enhancer provides query and document enhancement utilities for RAG systems.
// It includes components for query rewriting, hypothetical document generation,
// and step-back prompting to improve retrieval and generation quality.
package enhancer

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
	"github.com/google/uuid"
)

// Note: To remain independent and strictly rely on interfaces, we declare
// the LLM contract we expect. If gochat provides an interface, we can use it.
// type SimpleLLMClient interface {
// 	Generate(ctx context.Context, prompt string) (string, error)
// }

// ensure interface implementations
var _ retrieval.QueryRewriter = (*QueryRewriter)(nil)
var _ retrieval.HyDEGenerator = (*HyDEGenerator)(nil)
var _ retrieval.StepBackGenerator = (*StepBackGenerator)(nil)
var _ retrieval.CRAGEvaluator = (*CRAGEvaluator)(nil)

// QueryRewriter uses an LLM to rewrite and expand the user's query.
// It improves query clarity and specificity for better vector database search.
type QueryRewriter struct {
	// llm is the chat client used for query rewriting
	llm chat.Client
}

// NewQueryRewriter creates a new query rewriter.
//
// Parameters:
// - llm: The chat client to use for rewriting
//
// Returns:
// - A new QueryRewriter instance
func NewQueryRewriter(llm chat.Client) *QueryRewriter {
	return &QueryRewriter{llm: llm}
}

// Rewrite rewrites and expands the user's query to improve search quality.
//
// Parameters:
// - ctx: The context for the operation
// - query: The original query
//
// Returns:
// - The rewritten query
// - An error if rewriting fails
func (r *QueryRewriter) Rewrite(ctx context.Context, query *entity.Query) (*entity.Query, error) {
	prompt := fmt.Sprintf(`You are an AI assistant helping to rewrite a search query.
Please rewrite the following query to make it clearer, more specific, and better suited for a vector database search.
Remove conversational filler words. Resolve pronouns if context permits.
Only return the rewritten query, nothing else.

Original query: "%s"`, query.Text)

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}
	response, err := r.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	rewrittenText := strings.TrimSpace(response.Content)
	if rewrittenText == "" {
		rewrittenText = query.Text // fallback
	}

	newQuery := entity.NewQuery(uuid.New().String(), rewrittenText, nil)

	return newQuery, nil
}

// HyDEGenerator generates hypothetical answers to improve dense retrieval.
// It uses an LLM to create hypothetical documents that are then embedded
// alongside the original query to improve search results.
type HyDEGenerator struct {
	// llm is the chat client used for generating hypothetical documents
	llm chat.Client
}

// NewHyDEGenerator creates a new hypothetical document generator.
//
// Parameters:
// - llm: The chat client to use for generation
//
// Returns:
// - A new HyDEGenerator instance
func NewHyDEGenerator(llm chat.Client) *HyDEGenerator {
	return &HyDEGenerator{llm: llm}
}

// GenerateHypotheticalDocument generates a hypothetical document based on the query.
//
// Parameters:
// - ctx: The context for the operation
// - query: The query to generate a hypothetical document for
//
// Returns:
// - The generated hypothetical document
// - An error if generation fails
func (h *HyDEGenerator) GenerateHypotheticalDocument(ctx context.Context, query *entity.Query) (*entity.Document, error) {
	prompt := fmt.Sprintf(`Please write a paragraph answering the following question.
Write it as if you are a domain expert. Even if you don't know the exact answer, make an educated guess using relevant terminology and keywords.
Do not include conversational filler like "Here is an answer".

Question: "%s"`, query.Text)

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}
	response, err := h.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	doc := entity.NewDocument(
		uuid.New().String(),
		strings.TrimSpace(response.Content),
		"hyde_generator",
		"text/plain",
		map[string]any{"generated_for": query.ID},
	)

	return doc, nil
}

// StepBackGenerator abstracts queries into higher-level background questions.
// It helps retrieve broader context by generating more general questions
// that capture the underlying principles behind the original query.
type StepBackGenerator struct {
	// llm is the chat client used for generating step-back queries
	llm chat.Client
}

// NewStepBackGenerator creates a new step-back query generator.
//
// Parameters:
// - llm: The chat client to use for generation
//
// Returns:
// - A new StepBackGenerator instance
func NewStepBackGenerator(llm chat.Client) *StepBackGenerator {
	return &StepBackGenerator{llm: llm}
}

// GenerateStepBackQuery generates a step-back query that captures the underlying principles
// behind the original query.
//
// Parameters:
// - ctx: The context for the operation
// - query: The original query
//
// Returns:
// - The generated step-back query
// - An error if generation fails
func (s *StepBackGenerator) GenerateStepBackQuery(ctx context.Context, query *entity.Query) (*entity.Query, error) {
	prompt := fmt.Sprintf(`You are an expert at abstraction.
The user is asking a very specific question. To answer it correctly, we first need to retrieve broader background information.
Please write a "Step-back" question that asks for the underlying principles, concepts, or historical background related to the original question.
Only return the Step-back question, nothing else.

Original question: "%s"`, query.Text)

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}
	response, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	newQuery := entity.NewQuery(uuid.New().String(), strings.TrimSpace(response.Content), nil)

	return newQuery, nil
}

// CRAGEvaluator evaluates retrieval quality and decides corrective actions.
// It assesses whether retrieved documents are relevant and triggers web search if needed.
type CRAGEvaluator struct {
	// llm is the chat client used for evaluation
	llm chat.Client
}

// NewCRAGEvaluator creates a new CRAG evaluator.
//
// Parameters:
// - llm: The chat client to use for evaluation
//
// Returns:
// - A new CRAGEvaluator instance
func NewCRAGEvaluator(llm chat.Client) *CRAGEvaluator {
	return &CRAGEvaluator{llm: llm}
}

// Evaluate evaluates the quality of retrieved chunks and returns assessment.
//
// Parameters:
// - ctx: The context for the operation
// - query: The query used for retrieval
// - chunks: The retrieved chunks to evaluate
//
// Returns:
// - The evaluation result with relevance score and action recommendation
// - An error if evaluation fails
func (c *CRAGEvaluator) Evaluate(ctx context.Context, query *entity.Query, chunks []*entity.Chunk) (*retrieval.CRAGEvaluation, error) {
	// Build context from chunks
	var contextBuilder strings.Builder
	for i, chunk := range chunks {
		contextBuilder.WriteString(fmt.Sprintf("[Chunk %d]\n%s\n\n", i+1, chunk.Content))
	}
	contextStr := contextBuilder.String()

	prompt := fmt.Sprintf(`You are evaluating the quality of retrieved documents for answering a question.

Question: "%s"

Retrieved Documents:
%s

Rate the relevance and quality of these documents on a scale of 0-10:
- 0-3: Completely irrelevant or low quality
- 4-6: Partially relevant but missing key information  
- 7-8: Mostly relevant and helpful
- 9-10: Highly relevant and comprehensive

Also decide if additional web search is needed.

Return your response in this format:
Relevance Score: [0-10]
Need Web Search: [Yes/No]
Reason: [Brief explanation]`, query.Text, contextStr)

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}
	response, err := c.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Parse response
	content := response.Content
	score := parseScore(content)
	label := determineCRAGLabel(score)

	evaluation := &retrieval.CRAGEvaluation{
		Relevance: float32(score),
		Label:     label,
		Reason:    content,
	}

	return evaluation, nil
}

// determineCRAGLabel determines the CRAG label based on relevance score.
func determineCRAGLabel(score float64) retrieval.CRAGLabel {
	if score >= 7.0 {
		return retrieval.CRAGRelevant
	} else if score >= 4.0 {
		return retrieval.CRAGAmbiguous
	}
	return retrieval.CRAGIrrelevant
}

// parseScore extracts the relevance score from the LLM response.
func parseScore(content string) float64 {
	// Simple parsing: look for "Relevance Score: X"
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "relevance score") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				var score float64
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &score)
				return score
			}
		}
	}
	return 5.0 // default
}
