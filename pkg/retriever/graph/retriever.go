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
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/retrieval/query"
	"github.com/DotNetAge/gorag/pkg/steps/enrich"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
	"github.com/DotNetAge/gorag/pkg/store/vector/govector"
	"golang.org/x/sync/errgroup"
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

	// 1.5 Fallback to default graph store if none provided
	if options.graphStore == nil {
		return nil, fmt.Errorf("GraphStore is required for GraphRAG retriever")
	}

	// 2. Initialize the retriever using the expanded Options
	return NewRetriever(options.vectorStore, options.graphStore, options.embedder, options.llm, opts...), nil
}

// NewRetriever creates a new GraphRAG retriever.
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
	extractor   core.EntityExtractor
	logger      logging.Logger
	failOnError bool
}

type EntityExtractionOption func(*entityExtractionStep)

func WithFailOnError(fail bool) EntityExtractionOption {
	return func(s *entityExtractionStep) {
		s.failOnError = fail
	}
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
		if s.failOnError {
			return err
		}
		return nil
	}

	span.LogEvent("entities_extracted", map[string]any{"count": len(res.Entities), "entities": res.Entities})
	context.Custom["extracted_entities"] = res.Entities
	return nil
}

// graphSearchStep searches the knowledge graph using extracted entities.
type graphSearchStep struct {
	store  core.GraphStore
	depth  int
	limit  int
	logger logging.Logger
}

func (s *graphSearchStep) Name() string {
	return "GraphSearch"
}

func (s *graphSearchStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if s.store == nil {
		if s.logger != nil {
			s.logger.Warn("GraphStore is nil, skipping graph search step", nil)
		}
		return nil
	}

	tracer := context.Tracer
	if tracer == nil {
		tracer = observability.DefaultNoopTracer()
	}

	_, span := tracer.StartSpan(ctx, "GraphSearch")
	defer span.End()

	entities, ok := context.Custom["extracted_entities"].([]string)
	if !ok || len(entities) == 0 {
		span.LogEvent("no_entities_found", nil)
		return nil
	}

	// Industrial Gate: Limit number of entities to process to prevent resource exhaustion
	const maxEntities = 10
	if len(entities) > maxEntities {
		if s.logger != nil {
			s.logger.Debug("too many entities extracted, limiting to top", map[string]any{"total": len(entities), "limit": maxEntities})
		}
		entities = entities[:maxEntities]
	}

	span.SetTag("entities_count", len(entities))

	// Concurrent Fetching
	g, gctx := errgroup.WithContext(ctx)
	results := make(chan string, len(entities))

	for _, entity := range entities {
		e := entity // capture
		g.Go(func() error {
			nodes, edges, err := s.store.GetNeighbors(gctx, e, s.depth, s.limit)
			if err != nil {
				if s.logger != nil {
					s.logger.Warn("failed to get neighbors for entity", map[string]any{
						"entity": e,
						"error":  err,
					})
				}
				return nil // Non-fatal for individual entity
			}

			if len(nodes) > 0 {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Entity: %s\n", e))
				for _, node := range nodes {
					if node.ID != e {
						sb.WriteString(fmt.Sprintf("- Node: %s (Type: %s)\n", node.ID, node.Type))
					}
				}
				for _, edge := range edges {
					sb.WriteString(fmt.Sprintf("- Relationship: %s --(%s)--> %s\n", edge.Source, edge.Type, edge.Target))
				}
				results <- sb.String()
			}
			return nil
		})
	}

	// Close results chan when all workers done
	go func() {
		_ = g.Wait()
		close(results)
	}()

	var graphContext strings.Builder
	for res := range results {
		graphContext.WriteString(res)
		graphContext.WriteString("\n")
	}

	if err := g.Wait(); err != nil {
		return err
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
	name           string
	topK           int
	depth          int
	limit          int
	promptTemplate string
	docStore       core.DocStore
	logger         logging.Logger
	tracer         observability.Tracer
	embedder       embedding.Provider
	llm            chat.Client
	vectorStore    core.VectorStore
	graphStore     core.GraphStore
	customSteps    []pipeline.Step[*core.RetrievalContext]
}

func WithName(name string) Option {
	return func(o *Options) { o.name = name }
}

func WithVectorStore(s core.VectorStore) Option {
	return func(o *Options) { o.vectorStore = s }
}

func WithGraphStore(s core.GraphStore) Option {
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

func WithDocStore(s core.DocStore) Option {
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
