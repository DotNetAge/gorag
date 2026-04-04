// Package graph provides GraphRAG retrieval implementation following Microsoft GraphRAG architecture.
// It supports three search modes: Local, Global, and Hybrid.
package graph

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"text/template"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/retrieval/query"
	"github.com/DotNetAge/gorag/pkg/steps/enrich"
	graphstep "github.com/DotNetAge/gorag/pkg/steps/graph"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
	"github.com/DotNetAge/gorag/pkg/store/vector/govector"
)

const defaultGraphRAGPrompt = `You are a helpful and professional AI assistant.
Please answer the user's question based on the provided reference documents and knowledge graph context.
If the information do not contain the answer, say "I don't know based on the provided context."

[Reference Documents]
{{.Chunks}}

[Knowledge Graph Context]
{{.Graph}}

[User Question]
{{.Query}}

Answer:`

type graphRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
	tracer   observability.Tracer
}

// DefaultGraphRetriever creates a GraphRAG retriever with default settings.
// It automatically selects search mode based on available components.
func DefaultGraphRetriever(opts ...Option) (core.Retriever, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// 1. Fallback to default vector store if none provided
	if options.vectorStore == nil {
		workDir := "./data"
		vecName := "gorag_vectors.db"
		if options.name != "" {
			vecName = fmt.Sprintf("gorag_vectors_%s.db", options.name)
		}
		vecPath := filepath.Join(workDir, vecName)
		dimension := 1536
		if options.embedder != nil {
			dimension = options.embedder.Dimension()
		}

		colName := "gorag"
		if options.name != "" {
			colName = options.name
		}

		options.vectorStore, _ = govector.NewStore(
			govector.WithDBPath(vecPath),
			govector.WithDimension(dimension),
			govector.WithCollection(colName),
		)
	}

	// 2. GraphStore is required
	if options.graphStore == nil {
		return nil, fmt.Errorf("GraphStore is required for GraphRAG retriever")
	}

	// 3. Auto-select search mode if not specified
	if options.searchMode == "" {
		options.searchMode = core.SearchModeLocal
	}

	return NewRetriever(
		options.vectorStore,
		options.graphStore,
		options.embedder,
		options.llm,
		opts...,
	), nil
}

// NewRetriever creates a new GraphRAG retriever with configurable search mode.
// Following Microsoft GraphRAG architecture:
// - Local: Entity-centric graph traversal (best for specific questions)
// - Global: Community-based search (best for thematic questions)
// - Hybrid: Combines local, global, and vector search
func NewRetriever(
	vectorStore core.VectorStore,
	graphStore core.GraphStore,
	embedder embedding.Provider,
	llm chat.Client,
	opts ...Option,
) core.Retriever {
	options := defaultOptions()
	options.embedder = embedder
	options.llm = llm
	options.vectorStore = vectorStore
	options.graphStore = graphStore
	for _, opt := range opts {
		opt(options)
	}

	if options.logger == nil {
		options.logger = logging.DefaultNoopLogger()
	}

	if options.tracer == nil {
		options.tracer = observability.DefaultNoopTracer()
	}

	p := pipeline.New[*core.RetrievalContext]()

	// ===== Step 1: Entity Extraction =====
	// All modes need entity extraction from query
	extractor := resolveEntityExtractor(options)
	if extractor != nil {
		p.AddStep(&entityExtractionStep{
			extractor:   extractor,
			logger:      options.logger,
			failOnError: false,
		})
	}

	// ===== Step 2: Graph Search (Mode-specific) =====
	switch options.searchMode {
	case core.SearchModeGlobal:
		// Global search: Community-based retrieval
		p.AddStep(graphstep.NewGlobalSearch(graphStore, embedder,
			graphstep.WithGlobalTopK(options.topK),
		))

	case core.SearchModeHybrid:
		// Hybrid search: Local + Global + Vector
		p.AddStep(graphstep.NewHybridSearch(graphStore, vectorStore, embedder,
			graphstep.WithHybridTopK(options.topK),
			graphstep.WithHybridDepth(options.depth),
		))

	default: // SearchModeLocal
		// Local search: Graph traversal from entities
		p.AddStep(graphstep.NewLocalSearch(graphStore,
			graphstep.WithDepth(options.depth),
			graphstep.WithLimit(options.limit),
		))

		// Add vector search for hybrid-like behavior
		if options.embedder != nil {
			p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
				TopK: options.topK,
			}))
		}
	}

	// ===== Step 3: Custom Steps =====
	for _, step := range options.customSteps {
		p.AddStep(step)
	}

	// ===== Step 4: Enrichment =====
	// Retrieve full chunk content from DocStore
	if options.docStore != nil {
		p.AddStep(enrich.EnrichWithDocStore(options.docStore, options.logger))
	}

	// ===== Step 5: Generation =====
	if options.llm != nil {
		p.AddStep(&graphGenerationStep{
			llm:            options.llm,
			promptTemplate: options.promptTemplate,
			logger:         options.logger,
		})
	}

	return &graphRetriever{
		pipeline: p,
		tracer:   options.tracer,
	}
}

// Retrieve executes the GraphRAG pipeline for each query.
func (r *graphRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		retrievalCtx := core.NewRetrievalContext(ctx, q)
		retrievalCtx.Tracer = r.tracer

		// Start root span
		retrievalCtx.Ctx, retrievalCtx.Span = r.tracer.StartSpan(retrievalCtx.Ctx, "GraphRAG.Retrieve")
		retrievalCtx.Span.SetTag("query", q)

		if err := r.pipeline.Execute(retrievalCtx.Ctx, retrievalCtx); err != nil {
			retrievalCtx.Span.LogEvent("error", map[string]any{"error": err.Error()})
			retrievalCtx.Span.End()
			return nil, err
		}

		retrievalCtx.Span.End()

		// Collect all chunks
		var allChunks []*core.Chunk
		for _, group := range retrievalCtx.RetrievedChunks {
			allChunks = append(allChunks, group...)
		}

		res := &core.RetrievalResult{
			Query:  q,
			Chunks: allChunks,
			Answer: retrievalCtx.Answer.Answer,
		}
		results = append(results, res)
	}

	return results, nil
}

// ===== Entity Extraction Step =====

type entityExtractionStep struct {
	extractor   core.EntityExtractor
	logger      logging.Logger
	failOnError bool
}

func (s *entityExtractionStep) Name() string {
	return "EntityExtraction"
}

func (s *entityExtractionStep) Execute(ctx context.Context, rctx *core.RetrievalContext) error {
	_, span := rctx.Tracer.StartSpan(ctx, "GraphRAG.ExtractEntities")
	defer span.End()

	res, err := s.extractor.Extract(ctx, rctx.Query)
	if err != nil {
		s.logger.Error("failed to extract entities", err)
		span.LogEvent("error", map[string]any{"error": err.Error()})
		if s.failOnError {
			return err
		}
		return nil
	}

	span.LogEvent("entities_extracted", map[string]any{"count": len(res.Entities), "entities": res.Entities})
	rctx.ExtractedEntities = res.Entities
	rctx.Custom["extracted_entities"] = res.Entities
	return nil
}

// ===== Generation Step =====

type graphGenerationStep struct {
	llm            chat.Client
	promptTemplate string
	logger         logging.Logger
}

func (s *graphGenerationStep) Name() string {
	return "GraphGeneration"
}

func (s *graphGenerationStep) Execute(ctx context.Context, rctx *core.RetrievalContext) error {
	// Build chunks context
	var chunksText string
	for i, group := range rctx.RetrievedChunks {
		for _, chunk := range group {
			chunksText += fmt.Sprintf("--- Document %d ---\n%s\n\n", i+1, chunk.Content)
		}
	}

	graphCtx := rctx.GraphContext

	data := struct {
		Chunks string
		Graph  string
		Query  string
	}{
		Chunks: chunksText,
		Graph:  graphCtx,
		Query:  rctx.Query.Text,
	}

	tmpl, err := template.New("graph_rag").Parse(s.promptTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute prompt template: %w", err)
	}

	messages := []chat.Message{
		chat.NewSystemMessage("You are a helpful AI assistant specialized in answering questions based on provided context."),
		chat.NewUserMessage(buf.String()),
	}

	resp, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	rctx.Answer = &core.Result{
		Answer: resp.Content,
	}
	return nil
}

// ===== Entity Extractor Resolution =====

func resolveEntityExtractor(opts *Options) core.EntityExtractor {
	// 1. Use custom extractor if provided
	if opts.entityExtractor != nil {
		return opts.entityExtractor
	}

	// 2. Resolve based on strategy
	switch opts.extractionStrategy {
	case ExtractionStrategyLLM:
		if opts.llm != nil {
			return query.NewEntityExtractor(opts.llm)
		}
	case ExtractionStrategyVector:
		if opts.embedder != nil && opts.graphStore != nil {
			return query.NewVectorExtractor(opts.graphStore, opts.embedder)
		}
	case ExtractionStrategyKeyword:
		return query.NewKeywordExtractor()
	case ExtractionStrategyAuto:
		// Auto-select: LLM > Vector > Keyword
		if opts.llm != nil {
			return query.NewEntityExtractor(opts.llm)
		}
		if opts.embedder != nil && opts.graphStore != nil {
			return query.NewVectorExtractor(opts.graphStore, opts.embedder)
		}
		return query.NewKeywordExtractor()
	}

	return nil
}
