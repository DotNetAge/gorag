package graph

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/indexing/vectorstore/govector"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/retrieval/query"
	"github.com/DotNetAge/gorag/pkg/steps/enrich"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
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

// DefaultGraphRetriever creates a Knowledge-Graph enabled retriever.
// It implements hybrid search combining vector similarity and graph neighbor traversal.
func DefaultGraphRetriever(opts ...Option) (core.Retriever, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// 1. Fallback to default vector store if none provided
	if options.vectorStore == nil {
		workDir := "./data"
		vecPath := filepath.Join(workDir, "gorag_vectors.db")
		options.vectorStore, _ = govector.NewStore(
			govector.WithDBPath(vecPath),
			govector.WithDimension(1536),
		)
	}

	// 2. Initialize the retriever using the expanded Options
	return NewRetriever(options.vectorStore, options.graphStore, options.embedder, options.llm, opts...), nil
}

// NewRetriever creates a new GraphRAG retriever.
func NewRetriever(
	vectorStore core.VectorStore,
	graphStore store.GraphStore,
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

	// 1. Entity Extraction
	if options.llm != nil {
		p.AddStep(&entityExtractionStep{
			extractor: query.NewEntityExtractor(options.llm),
			logger:    options.logger,
		})
	}

	// 2. Graph Search
	p.AddStep(&graphSearchStep{
		store:  graphStore,
		depth:  options.depth,
		limit:  options.limit,
		logger: options.logger,
	})

	// 2.5 Custom Steps (e.g. Cypher reasoning)
	for _, step := range options.customSteps {
		p.AddStep(step)
	}

	// 3. Vector Search (for hybrid retrieval)
	if options.embedder != nil {
		p.AddStep(vector.Search(vectorStore, options.embedder, vector.SearchOptions{
			TopK: options.topK,
		}))
	}

	// 3.5 DocStore Enrichment (PDR)
	if options.docStore != nil {
		p.AddStep(enrich.EnrichWithDocStore(options.docStore, options.logger))
	}

	// 4. Generation
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
func (r *graphRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		retrievalCtx := core.NewRetrievalContext(ctx, q)
		retrievalCtx.Tracer = r.tracer

		// Start root span for retrieval
		retrievalCtx.Ctx, retrievalCtx.Span = r.tracer.StartSpan(retrievalCtx.Ctx, "GraphRAG.Retrieve")
		retrievalCtx.Span.SetTag("query", q)

		if err := r.pipeline.Execute(retrievalCtx.Ctx, retrievalCtx); err != nil {
			retrievalCtx.Span.LogEvent("error", map[string]any{"error": err.Error()})
			retrievalCtx.Span.End()
			return nil, err
		}

		retrievalCtx.Span.End()

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

// entityExtractionStep extracts entities from query and stores them in context.
type entityExtractionStep struct {
	extractor core.EntityExtractor
	logger    logging.Logger
}

func (s *entityExtractionStep) Name() string {
	return "EntityExtraction"
}

func (s *entityExtractionStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	_, span := context.Tracer.StartSpan(ctx, "GraphRAG.ExtractEntities")
	defer span.End()

	res, err := s.extractor.Extract(ctx, context.Query)
	if err != nil {
		s.logger.Error("failed to extract entities", err)
		span.LogEvent("error", map[string]any{"error": err.Error()})
		return nil // Non-fatal
	}

	span.LogEvent("entities_extracted", map[string]any{"count": len(res.Entities), "entities": res.Entities})
	context.Custom["extracted_entities"] = res.Entities
	return nil
}

// graphSearchStep searches the knowledge graph using extracted entities.
type graphSearchStep struct {
	store  store.GraphStore
	depth  int
	limit  int
	logger logging.Logger
}

func (s *graphSearchStep) Name() string {
	return "GraphSearch"
}

func (s *graphSearchStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if s.store == nil {
		s.logger.Warn("GraphStore is nil, skipping graph search step", nil)
		return nil
	}

	_, span := context.Tracer.StartSpan(ctx, "GraphSearch")
	defer span.End()

	entities, ok := context.Custom["extracted_entities"].([]string)
	if !ok || len(entities) == 0 {
		span.LogEvent("no_entities_found", nil)
		return nil
	}

	span.SetTag("entities_count", len(entities))

	var graphContext strings.Builder
	for _, entity := range entities {
		nodes, edges, err := s.store.GetNeighbors(ctx, entity, s.depth, s.limit)
		if err != nil {
			s.logger.Warn("failed to get neighbors for entity", map[string]any{
				"entity": entity,
				"error":  err,
			})
			span.LogEvent("error_fetching_neighbors", map[string]any{"entity": entity, "error": err.Error()})
			continue
		}

		if len(nodes) > 0 {
			span.LogEvent("subgraph_found", map[string]any{"entity": entity, "nodes": len(nodes), "edges": len(edges)})
			graphContext.WriteString(fmt.Sprintf("Entity: %s\n", entity))
			for _, node := range nodes {
				if node.ID != entity {
					graphContext.WriteString(fmt.Sprintf("- Node: %s (Type: %s)\n", node.ID, node.Type))
				}
			}
			for _, edge := range edges {
				graphContext.WriteString(fmt.Sprintf("- Relationship: %s --(%s)--> %s\n", edge.Source, edge.Type, edge.Target))
			}
			graphContext.WriteString("\n")
		}
	}

	context.Custom["graph_context"] = graphContext.String()

	return nil
}

// Custom step for GraphRAG generation to handle both chunks and graph context
type graphGenerationStep struct {
	llm            chat.Client
	promptTemplate string
	logger         logging.Logger
}

func (s *graphGenerationStep) Name() string {
	return "GraphGeneration"
}

func (s *graphGenerationStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	// Build chunks context
	var chunksBuilder strings.Builder
	i := 1
	for _, group := range context.RetrievedChunks {
		for _, chunk := range group {
			chunksBuilder.WriteString(fmt.Sprintf("--- Document %d --\n%s\n\n", i, chunk.Content))
			i++
		}
	}

	graphCtx, _ := context.Custom["graph_context"].(string)

	data := struct {
		Chunks string
		Graph  string
		Query  string
	}{
		Chunks: chunksBuilder.String(),
		Graph:  graphCtx,
		Query:  context.Query.Text,
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
		chat.NewUserMessage(buf.String()),
	}

	resp, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("LLM chat failed: %w", err)
	}

	context.Answer = &core.Result{
		Answer: resp.Content,
	}

	return nil
}

// Options for GraphRAG retriever
type Options struct {
	topK           int
	depth          int
	limit          int
	promptTemplate string
	docStore       store.DocStore
	logger         logging.Logger
	tracer         observability.Tracer
	embedder       embedding.Provider
	llm            chat.Client
	vectorStore    core.VectorStore
	graphStore     store.GraphStore
	customSteps    []pipeline.Step[*core.RetrievalContext]
}

func WithVectorStore(s core.VectorStore) Option {
	return func(o *Options) { o.vectorStore = s }
}

func WithGraphStore(s store.GraphStore) Option {
	return func(o *Options) { o.graphStore = s }
}
func defaultOptions() *Options {
	return &Options{
		topK:           5,
		depth:          1,
		limit:          10,
		promptTemplate: defaultGraphRAGPrompt,
		tracer:         observability.DefaultNoopTracer(),
		customSteps:    make([]pipeline.Step[*core.RetrievalContext], 0),
	}
}

type Option func(*Options)

func WithEmbedder(e embedding.Provider) Option {
	return func(o *Options) {
		o.embedder = e
	}
}

func WithLLM(l chat.Client) Option {
	return func(o *Options) {
		o.llm = l
	}
}

func WithTopK(k int) Option {
	return func(o *Options) {
		o.topK = k
	}
}

func WithDepth(d int) Option {
	return func(o *Options) {
		o.depth = d
	}
}

func WithLimit(l int) Option {
	return func(o *Options) {
		o.limit = l
	}
}

func WithDocStore(s store.DocStore) Option {
	return func(o *Options) {
		o.docStore = s
	}
}

func WithPromptTemplate(t string) Option {
	return func(o *Options) {
		if t != "" {
			o.promptTemplate = t
		}
	}
}

func WithLogger(l logging.Logger) Option {
	return func(o *Options) {
		o.logger = l
	}
}

func WithTracer(t observability.Tracer) Option {
	return func(o *Options) {
		o.tracer = t
	}
}

func WithCustomStep(step pipeline.Step[*core.RetrievalContext]) Option {
	return func(o *Options) {
		if step != nil {
			o.customSteps = append(o.customSteps, step)
		}
	}
}
