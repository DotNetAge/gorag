package service

// Package service provides use case orchestration services for goRAG.
//
// These services coordinate multiple domain interfaces to complete
// full business use cases (Search, Chat, Indexing, etc.).
//
// Design Principles:
// 1. Orchestrate use cases by calling interfaces from pkg/usecase/*
// 2. Accept DTOs as input/output
// 3. Stateless and thread-safe
// 4. Business logic lives in infra/* implementations

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// IntentClassifier defines the interface for intent classification.
type IntentClassifier interface {
	Classify(ctx context.Context, query *entity.Query) (*retrieval.IntentResult, error)
}

// QueryDecomposer defines the interface for query decomposition.
type QueryDecomposer interface {
	Decompose(ctx context.Context, query *entity.Query) (*retrieval.DecompositionResult, error)
}

// CRAGEvaluator defines the interface for retrieval quality evaluation.
type CRAGEvaluator interface {
	Evaluate(ctx context.Context, query *entity.Query, chunks []*entity.Chunk) (*retrieval.CRAGEvaluation, error)
}

// Retriever defines the interface for document retrieval.
// The signature aligns with retrieval.Retriever and infra/service.retriever.
type Retriever interface {
	Retrieve(ctx context.Context, queries []string, topK int) ([]*retrieval.RetrievalResult, error)
}

// Generator defines the interface for answer generation.
// The return type matches retrieval.Generator.Generate which returns *retrieval.GenerationResult.
type Generator interface {
	Generate(ctx context.Context, query *entity.Query, chunks []*entity.Chunk) (*retrieval.GenerationResult, error)
}

// RAGEvaluator defines the interface for answer quality evaluation.
type RAGEvaluator interface {
	Evaluate(ctx context.Context, query, answer, context string) (*retrieval.RAGEScores, error)
}
