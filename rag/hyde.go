package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/llm"
)

// HyDE (Hypothetical Document Embeddings) enhances query embeddings by generating a hypothetical document
// https://arxiv.org/abs/2212.10496
type HyDE struct {
	llm        llm.Client
	promptTemplate string
}

// NewHyDE creates a new HyDE instance
func NewHyDE(llm llm.Client) *HyDE {
	return &HyDE{
		llm:        llm,
		promptTemplate: defaultHyDEPrompt,
	}
}

// WithPromptTemplate sets a custom prompt template for HyDE
func (h *HyDE) WithPromptTemplate(template string) *HyDE {
	h.promptTemplate = template
	return h
}

// EnhanceQuery enhances the query using HyDE
func (h *HyDE) EnhanceQuery(ctx context.Context, query string) (string, error) {
	if h == nil {
		return query, nil // Fallback to original query
	}
	// Generate hypothetical document
	hypotheticalDoc, err := h.generateHypotheticalDocument(ctx, query)
	if err != nil {
		return query, err // Fallback to original query
	}

	// Combine query with hypothetical document
	enhancedQuery := fmt.Sprintf("%s\n\nHypothetical document:\n%s", query, hypotheticalDoc)

	return enhancedQuery, nil
}

// generateHypotheticalDocument generates a hypothetical document for the query
func (h *HyDE) generateHypotheticalDocument(ctx context.Context, query string) (string, error) {
	// Build prompt
	prompt := strings.Replace(h.promptTemplate, "{query}", query, 1)

	// Get response from LLM
	response, err := h.llm.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

const defaultHyDEPrompt = `
You are given a question. Please write a comprehensive answer to this question as if you were an expert on the topic. The answer should be detailed and cover all aspects of the question.

Question: {query}

Answer:
`
