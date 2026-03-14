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
var _ retrieval.QueryRewriter = (*QueryRewriterImpl)(nil)
var _ retrieval.HyDEGenerator = (*HyDEGeneratorImpl)(nil)
var _ retrieval.StepBackGenerator = (*StepBackGeneratorImpl)(nil)

// QueryRewriterImpl uses an LLM to rewrite and expand the user's query.
type QueryRewriterImpl struct {
	llm chat.Client
}

func NewQueryRewriter(llm chat.Client) *QueryRewriterImpl {
	return &QueryRewriterImpl{llm: llm}
}

func (r *QueryRewriterImpl) Rewrite(ctx context.Context, query *entity.Query) (*entity.Query, error) {
	prompt := fmt.Sprintf(`You are an AI assistant helping to rewrite a search query.
Please rewrite the following query to make it clearer, more specific, and better suited for a vector database search.
Remove conversational filler words. Resolve pronouns if context permits.
Only return the rewritten query, nothing else.

Original query: "%s"`, query.Text)

	rewritten, err := r.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	rewrittenText := strings.TrimSpace(rewritten)
	if rewrittenText == "" {
		rewrittenText = query.Text // fallback
	}

	newQuery := entity.NewQuery(uuid.New().String(), rewrittenText, query.Metadata)
	newQuery.Metadata["original_query"] = query.Text
	newQuery.Metadata["is_rewritten"] = true

	return newQuery, nil
}

// HyDEGeneratorImpl generates hypothetical answers to improve dense retrieval.
type HyDEGeneratorImpl struct {
	llm chat.Client
}

func NewHyDEGenerator(llm chat.Client) *HyDEGeneratorImpl {
	return &HyDEGeneratorImpl{llm: llm}
}

func (h *HyDEGeneratorImpl) GenerateHypotheticalDocument(ctx context.Context, query *entity.Query) (*entity.Document, error) {
	prompt := fmt.Sprintf(`Please write a paragraph answering the following question.
Write it as if you are a domain expert. Even if you don't know the exact answer, make an educated guess using relevant terminology and keywords.
Do not include conversational filler like "Here is an answer".

Question: "%s"`, query.Text)

	hypotheticalText, err := h.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	doc := entity.NewDocument(
		uuid.New().String(),
		strings.TrimSpace(hypotheticalText),
		"hyde_generator",
		"text/plain",
		map[string]any{"generated_for": query.ID},
	)

	return doc, nil
}

// StepBackGeneratorImpl abstracts queries into higher-level background questions.
type StepBackGeneratorImpl struct {
	llm chat.Client
}

func NewStepBackGenerator(llm chat.Client) *StepBackGeneratorImpl {
	return &StepBackGeneratorImpl{llm: llm}
}

func (s *StepBackGeneratorImpl) GenerateStepBackQuery(ctx context.Context, query *entity.Query) (*entity.Query, error) {
	prompt := fmt.Sprintf(`You are an expert at abstraction.
The user is asking a very specific question. To answer it correctly, we first need to retrieve broader background information.
Please write a "Step-back" question that asks for the underlying principles, concepts, or historical background related to the original question.
Only return the Step-back question, nothing else.

Original question: "%s"`, query.Text)

	stepBackText, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	newQuery := entity.NewQuery(uuid.New().String(), strings.TrimSpace(stepBackText), query.Metadata)
	newQuery.Metadata["step_back_for"] = query.Text

	return newQuery, nil
}
