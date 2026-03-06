package rag

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/rag/retrieval"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/google/uuid"
)

// Engine represents the RAG engine
type Engine struct {
	parser    parser.Parser
	embedder  embedding.Provider
	store     vectorstore.Store
	llm       llm.Client
	retriever *retrieval.HybridRetriever
	reranker  *retrieval.Reranker
	cache     Cache
	router    Router
	metrics   observability.Metrics
	logger    observability.Logger
	tracer    observability.Tracer
}

// Option configures the Engine
type Option func(*Engine)

// WithParser sets the document parser
func WithParser(p parser.Parser) Option {
	return func(e *Engine) {
		e.parser = p
	}
}

// WithVectorStore sets the vector store
func WithVectorStore(s vectorstore.Store) Option {
	return func(e *Engine) {
		e.store = s
	}
}

// WithEmbedder sets the embedding provider
func WithEmbedder(e embedding.Provider) Option {
	return func(engine *Engine) {
		engine.embedder = e
	}
}

// WithLLM sets the LLM client
func WithLLM(l llm.Client) Option {
	return func(e *Engine) {
		e.llm = l
	}
}

// WithRetriever sets the hybrid retriever
func WithRetriever(r *retrieval.HybridRetriever) Option {
	return func(e *Engine) {
		e.retriever = r
	}
}

// WithReranker sets the reranker
func WithReranker(r *retrieval.Reranker) Option {
	return func(e *Engine) {
		e.reranker = r
	}
}

// WithCache sets the query cache
func WithCache(c Cache) Option {
	return func(e *Engine) {
		e.cache = c
	}
}

// WithRouter sets the query router
func WithRouter(r Router) Option {
	return func(e *Engine) {
		e.router = r
	}
}

// WithMetrics sets the metrics collector
func WithMetrics(m observability.Metrics) Option {
	return func(e *Engine) {
		e.metrics = m
	}
}

// WithLogger sets the logger
func WithLogger(l observability.Logger) Option {
	return func(e *Engine) {
		e.logger = l
	}
}

// WithTracer sets the tracer
func WithTracer(t observability.Tracer) Option {
	return func(e *Engine) {
		e.tracer = t
	}
}

// New creates a new RAG engine
func New(opts ...Option) (*Engine, error) {
	engine := &Engine{}
	for _, opt := range opts {
		opt(engine)
	}

	if engine.parser == nil {
		return nil, fmt.Errorf("parser is required")
	}
	if engine.embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	if engine.store == nil {
		return nil, fmt.Errorf("vector store is required")
	}
	if engine.llm == nil {
		return nil, fmt.Errorf("LLM client is required")
	}

	return engine, nil
}

// QueryOptions configures query behavior
type QueryOptions struct {
	TopK           int
	PromptTemplate string
	Stream         bool
}

// StreamResponse represents a streaming RAG query response
type StreamResponse struct {
	Chunk   string
	Sources []vectorstore.Result
	Done    bool
	Error   error
}

// Response represents the RAG query response
type Response struct {
	Answer  string
	Sources []vectorstore.Result
}

// Source represents a document source
type Source struct {
	Type    string
	Path    string
	Content string
	Reader  interface{}
}

// Index adds documents to the RAG engine
func (e *Engine) Index(ctx context.Context, source Source) error {
	startTime := time.Now()
	status := "success"

	// Start span
	if e.tracer != nil {
		var span observability.Span
		ctx, span = e.tracer.StartSpan(ctx, "RAGIndex")
		defer span.End()
		span.SetAttribute("source_type", source.Type)
		span.SetAttribute("has_content", source.Content != "")
		span.SetAttribute("has_path", source.Path != "")
	}

	if source.Type == "" {
		err := fmt.Errorf("type is required")
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Invalid index input", err, nil)
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return err
	}

	if source.Content == "" && source.Path == "" {
		err := fmt.Errorf("content or path is required")
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Invalid index input", err, nil)
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return err
	}

	var reader io.Reader
	if source.Content != "" {
		reader = strings.NewReader(source.Content)
	} else if source.Reader != nil {
		if r, ok := source.Reader.(io.Reader); ok {
			reader = r
		} else {
			err := fmt.Errorf("invalid reader type")
			if e.metrics != nil {
				e.metrics.RecordErrorCount(ctx, "invalid_input")
			}
			if e.logger != nil {
				e.logger.Error(ctx, "Invalid reader type", err, nil)
			}
			if e.tracer != nil {
				if span, ok := e.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return err
		}
	}

	if reader == nil {
		err := fmt.Errorf("no content to index")
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "No content to index", err, nil)
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return err
	}

	parseStartTime := time.Now()
	chunks, err := e.parser.Parse(ctx, reader)
	if err != nil {
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "parsing")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Failed to parse document", err, map[string]interface{}{"source_type": source.Type})
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return fmt.Errorf("failed to parse document: %w", err)
	}
	if e.logger != nil {
		e.logger.Debug(ctx, "Document parsed", map[string]interface{}{
			"duration": time.Since(parseStartTime).Seconds(),
			"chunks_count": len(chunks),
			"source_type": source.Type,
		})
	}

	if len(chunks) == 0 {
		if e.logger != nil {
			e.logger.Info(ctx, "No chunks to index", map[string]interface{}{"source_type": source.Type})
		}
		return nil
	}

	vsChunks := make([]vectorstore.Chunk, len(chunks))
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		vsChunks[i] = vectorstore.Chunk{
			ID:         chunk.ID,
			Content:    chunk.Content,
			Metadata:   chunk.Metadata,
			MediaType:  chunk.MediaType,
			MediaData:  chunk.MediaData,
		}
		texts[i] = chunk.Content
	}

	embedStartTime := time.Now()
	embeddings, err := e.embedder.Embed(ctx, texts)
	if err != nil {
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "embedding")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Failed to embed chunks", err, map[string]interface{}{"chunks_count": len(chunks)})
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return fmt.Errorf("failed to embed chunks: %w", err)
	}
	if e.logger != nil {
		e.logger.Debug(ctx, "Chunks embedded", map[string]interface{}{
			"duration": time.Since(embedStartTime).Seconds(),
			"chunks_count": len(chunks),
			"embeddings_count": len(embeddings),
		})
	}

	storeStartTime := time.Now()
	err = e.store.Add(ctx, vsChunks, embeddings)
	if err != nil {
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "storage")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Failed to store chunks", err, map[string]interface{}{"chunks_count": len(chunks)})
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return fmt.Errorf("failed to store chunks: %w", err)
	}
	if e.logger != nil {
		e.logger.Debug(ctx, "Chunks stored", map[string]interface{}{
			"duration": time.Since(storeStartTime).Seconds(),
			"chunks_count": len(chunks),
		})
	}

	// Record metrics
	if e.metrics != nil {
		duration := time.Since(startTime)
		e.metrics.RecordIndexLatency(ctx, duration)
		e.metrics.RecordIndexCount(ctx, status)
	}

	// Log index
	if e.logger != nil {
		e.logger.Info(ctx, "Index completed", map[string]interface{}{
			"duration": time.Since(startTime).Seconds(),
			"chunks_count": len(chunks),
			"source_type": source.Type,
		})
	}

	return nil
}

// BatchIndex adds multiple documents to the RAG engine in batch
func (e *Engine) BatchIndex(ctx context.Context, sources []Source) error {
	if len(sources) == 0 {
		return nil
	}

	// Process each source in batch
	for _, source := range sources {
		if err := e.Index(ctx, source); err != nil {
			return fmt.Errorf("failed to index source: %w", err)
		}
	}

	return nil
}

// BatchQuery performs multiple RAG queries in batch
func (e *Engine) BatchQuery(ctx context.Context, questions []string, opts QueryOptions) ([]*Response, error) {
	if len(questions) == 0 {
		return []*Response{}, nil
	}

	responses := make([]*Response, len(questions))

	// Process each question in batch
	for i, question := range questions {
		response, err := e.Query(ctx, question, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to query: %w", err)
		}
		responses[i] = response
	}

	return responses, nil
}

// AsyncIndex adds documents to the RAG engine asynchronously
func (e *Engine) AsyncIndex(ctx context.Context, source Source) error {
	go func() {
		if err := e.Index(ctx, source); err != nil {
			// Log error (in a real implementation, you would use a logger)
			fmt.Printf("Error indexing document: %v\n", err)
		}
	}()

	return nil
}

// AsyncBatchIndex adds multiple documents to the RAG engine asynchronously
func (e *Engine) AsyncBatchIndex(ctx context.Context, sources []Source) error {
	go func() {
		if err := e.BatchIndex(ctx, sources); err != nil {
			// Log error (in a real implementation, you would use a logger)
			fmt.Printf("Error batch indexing documents: %v\n", err)
		}
	}()

	return nil
}

// Query performs a RAG query
func (e *Engine) Query(ctx context.Context, question string, opts QueryOptions) (*Response, error) {
	startTime := time.Now()
	status := "success"

	// Start span
	if e.tracer != nil {
		var span observability.Span
		ctx, span = e.tracer.StartSpan(ctx, "RAGQuery")
		defer span.End()
		span.SetAttribute("question", question)
		span.SetAttribute("stream", opts.Stream)
		span.SetAttribute("topK", opts.TopK)
	}

	if opts.Stream {
		// For streaming, use QueryStream
		ch, err := e.QueryStream(ctx, question, opts)
		if err != nil {
			if e.metrics != nil {
				e.metrics.RecordErrorCount(ctx, "streaming_query")
			}
			if e.logger != nil {
				e.logger.Error(ctx, "Failed to start streaming query", err, map[string]interface{}{"question": question})
			}
			if e.tracer != nil {
				if span, ok := e.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return nil, err
		}

		// Collect all chunks
		var answer strings.Builder
		var sources []vectorstore.Result
		var finalErr error

		for resp := range ch {
			if resp.Error != nil {
				finalErr = resp.Error
				break
			}
			answer.WriteString(resp.Chunk)
			if len(resp.Sources) > 0 {
				sources = resp.Sources
			}
		}

		if finalErr != nil {
			if e.metrics != nil {
				e.metrics.RecordErrorCount(ctx, "streaming_query")
			}
			if e.logger != nil {
				e.logger.Error(ctx, "Failed to collect streaming response", finalErr, map[string]interface{}{"question": question})
			}
			if e.tracer != nil {
				if span, ok := e.tracer.Extract(ctx); ok {
					span.SetError(finalErr)
				}
			}
			status = "error"
			return nil, finalErr
		}

		response := &Response{
			Answer:  answer.String(),
			Sources: sources,
		}

		// Cache the response if cache is available
		if e.cache != nil {
			cacheKey := question + opts.PromptTemplate
			e.cache.Set(ctx, cacheKey, response, 1*time.Hour)
		}

		// Record metrics
		if e.metrics != nil {
			duration := time.Since(startTime)
			e.metrics.RecordQueryLatency(ctx, duration)
			e.metrics.RecordQueryCount(ctx, status)
		}

		// Log query
		if e.logger != nil {
			e.logger.Info(ctx, "Streaming query completed", map[string]interface{}{
				"question": question,
				"duration": time.Since(startTime).Seconds(),
				"answer_length": len(response.Answer),
				"sources_count": len(response.Sources),
			})
		}

		return response, nil
	}

	// Non-streaming path
	if question == "" {
		err := fmt.Errorf("question is required")
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Invalid query input", err, nil)
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, err
	}

	// Check cache if available
	if e.cache != nil {
		cacheKey := question + opts.PromptTemplate
		if cachedResponse, found := e.cache.Get(ctx, cacheKey); found {
			// Record metrics
			if e.metrics != nil {
				duration := time.Since(startTime)
				e.metrics.RecordQueryLatency(ctx, duration)
				e.metrics.RecordQueryCount(ctx, status)
			}

			// Log query
			if e.logger != nil {
				e.logger.Info(ctx, "Query completed from cache", map[string]interface{}{
					"question": question,
					"duration": time.Since(startTime).Seconds(),
					"answer_length": len(cachedResponse.Answer),
					"sources_count": len(cachedResponse.Sources),
				})
			}

			return cachedResponse, nil
		}
	}

	// Determine routing for the query
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Use router if available
	var routeResult RouteResult
	var err error
	if e.router != nil {
		routeResult, err = e.router.Route(ctx, question)
		if err != nil {
			// Fall back to default routing if router fails
			routeResult = RouteResult{
				Type: "hybrid",
				Params: map[string]interface{}{
					"topK": topK,
				},
			}
			if e.logger != nil {
				e.logger.Warn(ctx, "Router failed, falling back to default routing", map[string]interface{}{"error": err.Error()})
			}
		}
	} else {
		// Default routing
		routeResult = RouteResult{
			Type: "hybrid",
			Params: map[string]interface{}{
				"topK": topK,
			},
		}
	}

	// Override topK if specified in route params
	if routeTopK, ok := routeResult.Params["topK"].(int); ok {
		topK = routeTopK
	}

	queryEmbeddings, err := e.embedder.Embed(ctx, []string{question})
	if err != nil {
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "embedding")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Failed to embed query", err, map[string]interface{}{"question": question})
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if len(queryEmbeddings) == 0 {
		err := fmt.Errorf("failed to generate query embedding")
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "embedding")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Failed to generate query embedding", err, map[string]interface{}{"question": question})
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, err
	}

	var results []vectorstore.Result

	// Execute search based on route type
	switch routeResult.Type {
	case "keyword":
		// Use keyword search if retriever is available
		if e.retriever != nil {
			results, err = e.retriever.KeywordSearch(ctx, question, topK*2)
		} else {
			// Fall back to vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = e.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	case "vector":
		// Use vector search
		searchOpts := vectorstore.SearchOptions{
			TopK: topK * 2, // Get more results for reranking
		}
		results, err = e.store.Search(ctx, queryEmbeddings[0], searchOpts)
	case "hybrid":
		fallthrough
	default:
		// Use hybrid retriever if available
		if e.retriever != nil {
			results, err = e.retriever.Search(ctx, question, queryEmbeddings[0], topK*2)
		} else {
			// Use default vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = e.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	}

	if err != nil {
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "search")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Failed to search", err, map[string]interface{}{"question": question, "route_type": routeResult.Type})
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Use reranker if available
	if e.reranker != nil && len(results) > 0 {
		rerankStartTime := time.Now()
		results, err = e.reranker.Rerank(ctx, question, results)
		if err != nil {
			if e.metrics != nil {
				e.metrics.RecordErrorCount(ctx, "reranking")
			}
			if e.logger != nil {
				e.logger.Error(ctx, "Failed to rerank", err, map[string]interface{}{"question": question})
			}
			if e.tracer != nil {
				if span, ok := e.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return nil, fmt.Errorf("failed to rerank: %w", err)
		}
		if e.logger != nil {
			e.logger.Debug(ctx, "Reranking completed", map[string]interface{}{
				"duration": time.Since(rerankStartTime).Seconds(),
				"results_count": len(results),
			})
		}
	}

	if len(results) == 0 {
		response := &Response{
			Answer:  "未找到相关信息",
			Sources: results,
		}

		// Cache the response if cache is available
		if e.cache != nil {
			cacheKey := question + opts.PromptTemplate
			e.cache.Set(ctx, cacheKey, response, 1*time.Hour)
		}

		// Record metrics
		if e.metrics != nil {
			duration := time.Since(startTime)
			e.metrics.RecordQueryLatency(ctx, duration)
			e.metrics.RecordQueryCount(ctx, status)
		}

		// Log query
		if e.logger != nil {
			e.logger.Info(ctx, "Query completed with no results", map[string]interface{}{
				"question": question,
				"duration": time.Since(startTime).Seconds(),
			})
		}

		return response, nil
	}

	// Limit to top K results
	if len(results) > topK {
		results = results[:topK]
	}

	contexts := make([]string, len(results))
	for i, result := range results {
		contexts[i] = result.Content
	}

	prompt := buildPrompt(question, contexts, opts.PromptTemplate)

	llmStartTime := time.Now()
	answer, err := e.llm.Complete(ctx, prompt)
	if err != nil {
		if e.metrics != nil {
			e.metrics.RecordErrorCount(ctx, "llm_completion")
		}
		if e.logger != nil {
			e.logger.Error(ctx, "Failed to generate answer", err, map[string]interface{}{"question": question})
		}
		if e.tracer != nil {
			if span, ok := e.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}
	if e.logger != nil {
		e.logger.Debug(ctx, "LLM completion completed", map[string]interface{}{
			"duration": time.Since(llmStartTime).Seconds(),
			"answer_length": len(answer),
		})
	}

	response := &Response{
		Answer:  answer,
		Sources: results,
	}

	// Cache the response if cache is available
	if e.cache != nil {
		cacheKey := question + opts.PromptTemplate
		e.cache.Set(ctx, cacheKey, response, 1*time.Hour)
	}

	// Record metrics
	if e.metrics != nil {
		duration := time.Since(startTime)
		e.metrics.RecordQueryLatency(ctx, duration)
		e.metrics.RecordQueryCount(ctx, status)
	}

	// Log query
	if e.logger != nil {
		e.logger.Info(ctx, "Query completed", map[string]interface{}{
			"question": question,
			"duration": time.Since(startTime).Seconds(),
			"answer_length": len(response.Answer),
			"sources_count": len(response.Sources),
			"route_type": routeResult.Type,
		})
	}

	return response, nil
}

// QueryStream performs a streaming RAG query
func (e *Engine) QueryStream(ctx context.Context, question string, opts QueryOptions) (<-chan StreamResponse, error) {
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	// Determine routing for the query
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Use router if available
	var routeResult RouteResult
	var err error
	if e.router != nil {
		routeResult, err = e.router.Route(ctx, question)
		if err != nil {
			// Fall back to default routing if router fails
			routeResult = RouteResult{
				Type: "hybrid",
				Params: map[string]interface{}{
					"topK": topK,
				},
			}
		}
	} else {
		// Default routing
		routeResult = RouteResult{
			Type: "hybrid",
			Params: map[string]interface{}{
				"topK": topK,
			},
		}
	}

	// Override topK if specified in route params
	if routeTopK, ok := routeResult.Params["topK"].(int); ok {
		topK = routeTopK
	}

	queryEmbeddings, err := e.embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if len(queryEmbeddings) == 0 {
		return nil, fmt.Errorf("failed to generate query embedding")
	}

	var results []vectorstore.Result

	// Execute search based on route type
	switch routeResult.Type {
	case "keyword":
		// Use keyword search if retriever is available
		if e.retriever != nil {
			results, err = e.retriever.KeywordSearch(ctx, question, topK*2)
		} else {
			// Fall back to vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = e.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	case "vector":
		// Use vector search
		searchOpts := vectorstore.SearchOptions{
			TopK: topK * 2, // Get more results for reranking
		}
		results, err = e.store.Search(ctx, queryEmbeddings[0], searchOpts)
	case "hybrid":
		fallthrough
	default:
		// Use hybrid retriever if available
		if e.retriever != nil {
			results, err = e.retriever.Search(ctx, question, queryEmbeddings[0], topK*2)
		} else {
			// Use default vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = e.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Use reranker if available
	if e.reranker != nil && len(results) > 0 {
		results, err = e.reranker.Rerank(ctx, question, results)
		if err != nil {
			return nil, fmt.Errorf("failed to rerank: %w", err)
		}
	}

	if len(results) == 0 {
		ch := make(chan StreamResponse, 1)
		ch <- StreamResponse{
			Chunk:   "未找到相关信息",
			Sources: results,
			Done:    true,
		}
		close(ch)
		return ch, nil
	}

	// Limit to top K results
	if len(results) > topK {
		results = results[:topK]
	}

	contexts := make([]string, len(results))
	for i, result := range results {
		contexts[i] = result.Content
	}

	prompt := buildPrompt(question, contexts, opts.PromptTemplate)

	// Get streaming response from LLM
	llmCh, err := e.llm.CompleteStream(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to start streaming: %w", err)
	}

	// Create response channel
	ch := make(chan StreamResponse, 10)

	// Forward LLM stream to response channel
	go func() {
		defer close(ch)

		// Send sources with first chunk
		sourcesSent := false

		for chunk := range llmCh {
			if chunk == "" {
				continue
			}

			resp := StreamResponse{
				Chunk: chunk,
				Done:  false,
			}

			if !sourcesSent {
				resp.Sources = results
				sourcesSent = true
			}

			select {
			case ch <- resp:
			case <-ctx.Done():
				return
			}
		}

		// Send final done response
		select {
		case ch <- StreamResponse{
			Done: true,
		}:
		case <-ctx.Done():
			return
		}
	}()

	return ch, nil
}

// buildPrompt builds the prompt for LLM
func buildPrompt(question string, contexts []string, template string) string {
	if template != "" {
		// Build context string
		var contextStr strings.Builder
		for i, ctx := range contexts {
			contextStr.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
		}
		
		// Replace placeholders
		result := strings.ReplaceAll(template, "{question}", question)
		result = strings.ReplaceAll(result, "{context}", contextStr.String())
		return result
	}

	var buf bytes.Buffer
	buf.WriteString("基于以下上下文信息回答问题。如果上下文中没有相关信息，请说明无法回答。\n\n")
	buf.WriteString("上下文：\n")

	for i, ctx := range contexts {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
	}

	buf.WriteString("\n问题：\n")
	buf.WriteString(question)
	buf.WriteString("\n\n答案：\n")

	return buf.String()
}

// generateChunkID generates a unique chunk ID
func generateChunkID() string {
	return uuid.New().String()
}
