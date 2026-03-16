package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// AgenticSearchService orchestrates the complete Agentic RAG search workflow.
//
// This is a thin orchestration layer that coordinates multiple domain interfaces
// to complete the full Agentic RAG use case.
//
// Business logic lives in infra/* implementations, not here.
type AgenticSearchService struct {
	intentClassifier IntentClassifier
	decomposer       QueryDecomposer
	cragEvaluator    CRAGEvaluator
	retriever        Retriever
	generator        Generator
	ragEvaluator     RAGEvaluator
}

// NewAgenticSearchService creates a new Agentic search service.
func NewAgenticSearchService(
	ic IntentClassifier,
	d QueryDecomposer,
	ce CRAGEvaluator,
	r Retriever,
	g Generator,
	re RAGEvaluator,
) *AgenticSearchService {
	return &AgenticSearchService{
		intentClassifier: ic,
		decomposer:       d,
		cragEvaluator:    ce,
		retriever:        r,
		generator:        g,
		ragEvaluator:     re,
	}
}

// Search executes the complete Agentic RAG workflow.
//
// Workflow:
// 1. Intent Classification → Determine if RAG is needed
// 2. Query Decomposition → Break into sub-queries (if complex)
// 3. Parallel Retrieval → Retrieve chunks for all sub-queries
// 4. CRAG Evaluation → Assess retrieval quality
// 5. Answer Generation → Generate response from chunks
// 6. RAG Evaluation → Assess answer quality
//
// Parameters:
// - ctx: The context for cancellation and timeouts
// - req: The search request containing query and configuration
//
// Returns:
// - *AgenticSearchResponse: Complete response with metadata
// - error: Any error encountered during the workflow
func (s *AgenticSearchService) Search(ctx context.Context, req *SearchRequest) (*AgenticSearchResponse, error) {
	if req.Query == nil || req.Query.Text == "" {
		return nil, fmt.Errorf("query is required")
	}

	resp := &AgenticSearchResponse{}

	// Step 1: Intent Classification
	intent, err := s.intentClassifier.Classify(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("intent classification failed: %w", err)
	}
	resp.Intent = intent.Intent

	// Handle different intents
	switch intent.Intent {
	case retrieval.IntentChat:
		return s.handleChat(ctx, req)
	case retrieval.IntentFactCheck:
		return s.handleFactCheck(ctx, req)
	case retrieval.IntentDomainSpecific:
		// Continue with full RAG workflow
	default:
		// Default to RAG
	}

	// Step 2: Query Decomposition (for complex queries)
	var subQueries []string
	if intent.Confidence > 0.8 { // High confidence complex query
		decomp, err := s.decomposer.Decompose(ctx, req.Query)
		if err != nil {
			return nil, fmt.Errorf("decomposition failed: %w", err)
		}

		if decomp.IsComplex && len(decomp.SubQueries) > 0 {
			subQueries = decomp.SubQueries
			resp.SubQueries = subQueries
		}
	}

	// Use original query if no decomposition
	if len(subQueries) == 0 {
		subQueries = []string{req.Query.Text}
	}

	// Step 3: Parallel Retrieval
	allResults, err := s.retriever.Retrieve(ctx, subQueries, req.TopK)
	if err != nil {
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	// Flatten chunks from all retrieval results
	var flatChunks []*entity.Chunk
	for _, res := range allResults {
		flatChunks = append(flatChunks, res.Chunks...)
	}

	// Step 4: CRAG Evaluation
	cragEval, err := s.cragEvaluator.Evaluate(ctx, req.Query, flatChunks)
	if err != nil {
		return nil, fmt.Errorf("CRAG evaluation failed: %w", err)
	}
	resp.CRAGEvaluation = cragEval

	// Handle CRAG results
	switch cragEval.Label {
	case retrieval.CRAGIrrelevant:
		// Consider fallback to web search or return low-confidence answer
		resp.Score = 0.2
		resp.Answer = "I'm not confident about this answer based on the available information."
		return resp, nil
	case retrieval.CRAGAmbiguous:
		// Continue but note lower confidence
		resp.Score = 0.5
	case retrieval.CRAGRelevant:
		resp.Score = 0.9
	}

	// Step 5: Answer Generation
	genResult, err := s.generator.Generate(ctx, req.Query, flatChunks)
	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}
	resp.Answer = genResult.Answer

	// Extract source documents
	resp.SourceDocuments = extractSourceDocuments(flatChunks)

	// Step 6: RAG Evaluation (optional, can be async)
	contextStr := buildContextString(flatChunks)
	ragEval, err := s.ragEvaluator.Evaluate(ctx, req.Query.Text, genResult.Answer, contextStr)
	if err != nil {
		// non-fatal: evaluation failure should not block the response
		_ = err
	} else {
		resp.RAGEvaluation = ragEval
		if ragEval.Passed {
			resp.Score = ragEval.OverallScore
		}
	}

	return resp, nil
}

// handleChat handles simple chat queries without RAG.
func (s *AgenticSearchService) handleChat(ctx context.Context, req *SearchRequest) (*AgenticSearchResponse, error) {
	// Direct generation without retrieval
	genResult, err := s.generator.Generate(ctx, req.Query, nil)
	if err != nil {
		return nil, err
	}

	return &AgenticSearchResponse{
		SearchResponse: SearchResponse{
			Answer: genResult.Answer,
			Score:  0.95, // High confidence for chat
			Intent: retrieval.IntentChat,
		},
	}, nil
}

// handleFactCheck handles fact-checking queries using RAG retrieval and generation.
func (s *AgenticSearchService) handleFactCheck(ctx context.Context, req *SearchRequest) (*AgenticSearchResponse, error) {
	allChunks, err := s.retriever.Retrieve(ctx, []string{req.Query.Text}, req.TopK)
	if err != nil {
		return nil, fmt.Errorf("fact check retrieval failed: %w", err)
	}

	var flatChunks []*entity.Chunk
	for _, result := range allChunks {
		flatChunks = append(flatChunks, result.Chunks...)
	}

	genResult, err := s.generator.Generate(ctx, req.Query, flatChunks)
	if err != nil {
		return nil, fmt.Errorf("fact check generation failed: %w", err)
	}

	return &AgenticSearchResponse{
		SearchResponse: SearchResponse{
			Answer:          genResult.Answer,
			Score:           0.7,
			Intent:          retrieval.IntentFactCheck,
			SourceDocuments: extractSourceDocuments(flatChunks),
		},
	}, nil
}

// extractSourceDocuments extracts unique document IDs from chunks.
func extractSourceDocuments(chunks []*entity.Chunk) []string {
	seen := make(map[string]bool)
	var docs []string

	for _, chunk := range chunks {
		if chunk.DocumentID != "" && !seen[chunk.DocumentID] {
			seen[chunk.DocumentID] = true
			docs = append(docs, chunk.DocumentID)
		}
	}

	return docs
}

// buildContextString builds a context string from chunks for RAG evaluation.
func buildContextString(chunks []*entity.Chunk) string {
	var builder strings.Builder

	for i, chunk := range chunks {
		builder.WriteString(fmt.Sprintf("[Context %d]: %s\n", i+1, chunk.Content))
	}

	return builder.String()
}
