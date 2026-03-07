package query

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/observability"
	"github.com/DotNetAge/gorag/rag/retrieval"
	"github.com/DotNetAge/gorag/vectorstore"
)

// QueryOptions configures query behavior
type QueryOptions struct {
	TopK              int
	PromptTemplate    string
	Stream            bool
	UseMultiHopRAG    bool   // Use multi-hop RAG for complex questions
	UseAgenticRAG     bool   // Use agentic RAG with autonomous retrieval
	MaxHops           int    // Maximum number of hops for multi-hop RAG
	AgentInstructions string // Instructions for agentic RAG
}

// StreamResponse represents a streaming RAG query response
type StreamResponse struct {
	Chunk   string
	Sources []core.Result
	Done    bool
	Error   error
}

// Response represents the RAG query response
type Response struct {
	Answer  string
	Sources []core.Result
}

// Cache defines the interface for query result caching
type Cache interface {
	Get(ctx context.Context, key string) (*Response, bool)
	Set(ctx context.Context, key string, value *Response, expiration time.Duration)
}

// Router defines the interface for query routing
type Router interface {
	Route(ctx context.Context, question string) (RouteResult, error)
}

// RouteResult represents the result of query routing
type RouteResult struct {
	Type   string
	Params map[string]interface{}
}

// HyDE defines the interface for Hypothetical Document Embeddings
type HyDE interface {
	EnhanceQuery(ctx context.Context, question string) (string, error)
}

// ContextCompressor defines the interface for context compression
type ContextCompressor interface {
	Compress(ctx context.Context, question string, results []core.Result) ([]core.Result, error)
}

// Embedder defines the interface for embedding providers
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Metrics defines the interface for metrics collection
type Metrics interface {
	RecordErrorCount(ctx context.Context, errorType string)
	RecordQueryLatency(ctx context.Context, duration time.Duration)
	RecordQueryCount(ctx context.Context, status string)
}

// Logger defines the interface for logging
type Logger interface {
	Info(ctx context.Context, message string, fields map[string]interface{})
	Debug(ctx context.Context, message string, fields map[string]interface{})
	Error(ctx context.Context, message string, err error, fields map[string]interface{})
	Warn(ctx context.Context, message string, fields map[string]interface{})
}

// Tracer defines the interface for tracing
type Tracer interface {
	StartSpan(ctx context.Context, name string) (context.Context, observability.Span)
	Extract(ctx context.Context) (observability.Span, bool)
}

// QueryHandler handles RAG query operations
type QueryHandler struct {
	embedder        Embedder
	store           vectorstore.Store
	llm             llm.Client
	retriever       *retrieval.HybridRetriever
	reranker        *retrieval.Reranker
	hydration       HyDE
	compressor      ContextCompressor
	multiHopRAG     *retrieval.MultiHopRAG
	agenticRAG      *retrieval.AgenticRAG
	cache           Cache
	router          Router
	metrics         Metrics
	logger          Logger
	tracer          Tracer
}

// NewQueryHandler creates a new query handler
func NewQueryHandler(
	embedder Embedder,
	store vectorstore.Store,
	llm llm.Client,
	retriever *retrieval.HybridRetriever,
	reranker *retrieval.Reranker,
	hydration HyDE,
	compressor ContextCompressor,
	multiHopRAG *retrieval.MultiHopRAG,
	agenticRAG *retrieval.AgenticRAG,
	cache Cache,
	router Router,
	metrics Metrics,
	logger Logger,
	tracer Tracer,
) *QueryHandler {
	return &QueryHandler{
		embedder:        embedder,
		store:           store,
		llm:             llm,
		retriever:       retriever,
		reranker:        reranker,
		hydration:       hydration,
		compressor:      compressor,
		multiHopRAG:     multiHopRAG,
		agenticRAG:      agenticRAG,
		cache:           cache,
		router:          router,
		metrics:         metrics,
		logger:          logger,
		tracer:          tracer,
	}
}

// Query performs a RAG query
func (q *QueryHandler) Query(ctx context.Context, question string, opts QueryOptions) (*Response, error) {
	startTime := time.Now()
	status := "success"

	// Start span
	if q.tracer != nil {
		var span observability.Span
		ctx, span = q.tracer.StartSpan(ctx, "RAGQuery")
		if span != nil {
			defer span.End()
			span.SetAttribute("question", question)
			span.SetAttribute("stream", opts.Stream)
			span.SetAttribute("topK", opts.TopK)
		}
	}

	if opts.Stream {
		// For streaming, use QueryStream
		ch, err := q.QueryStream(ctx, question, opts)
		if err != nil {
			if q.metrics != nil {
				q.metrics.RecordErrorCount(ctx, "streaming_query")
			}
			if q.logger != nil {
				q.logger.Error(ctx, "Failed to start streaming query", err, map[string]interface{}{"question": question})
			}
			if q.tracer != nil {
				if span, ok := q.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return nil, err
		}

		// Collect all chunks
		var answer strings.Builder
		var sources []core.Result
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
			if q.metrics != nil {
				q.metrics.RecordErrorCount(ctx, "streaming_query")
			}
			if q.logger != nil {
				q.logger.Error(ctx, "Failed to collect streaming response", finalErr, map[string]interface{}{"question": question})
			}
			if q.tracer != nil {
				if span, ok := q.tracer.Extract(ctx); ok {
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
		if q.cache != nil {
			cacheKey := question + opts.PromptTemplate
			q.cache.Set(ctx, cacheKey, response, 1*time.Hour)
		}

		// Record metrics
		if q.metrics != nil {
			duration := time.Since(startTime)
			q.metrics.RecordQueryLatency(ctx, duration)
			q.metrics.RecordQueryCount(ctx, status)
		}

		// Log query
		if q.logger != nil {
			q.logger.Info(ctx, "Streaming query completed", map[string]interface{}{
				"question":      question,
				"duration":      time.Since(startTime).Seconds(),
				"answer_length": len(response.Answer),
				"sources_count": len(response.Sources),
			})
		}

		return response, nil
	}

	// Non-streaming path
	if question == "" {
		err := fmt.Errorf("question is required")
		if q.metrics != nil {
			q.metrics.RecordErrorCount(ctx, "invalid_input")
		}
		if q.logger != nil {
			q.logger.Error(ctx, "Invalid query input", err, nil)
		}
		if q.tracer != nil {
			if span, ok := q.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, err
	}

	// Check cache if available
	if q.cache != nil {
		cacheKey := question + opts.PromptTemplate
		if cachedResponse, found := q.cache.Get(ctx, cacheKey); found {
			// Record metrics
			if q.metrics != nil {
				duration := time.Since(startTime)
				q.metrics.RecordQueryLatency(ctx, duration)
				q.metrics.RecordQueryCount(ctx, status)
			}

			// Log query
			if q.logger != nil {
				q.logger.Info(ctx, "Query completed from cache", map[string]interface{}{
					"question":      question,
					"duration":      time.Since(startTime).Seconds(),
					"answer_length": len(cachedResponse.Answer),
					"sources_count": len(cachedResponse.Sources),
				})
			}

			return cachedResponse, nil
		}
	}

	// Check if we should use advanced RAG patterns
	if opts.UseMultiHopRAG && q.multiHopRAG != nil {
		// Use multi-hop RAG for complex questions
		maxHops := opts.MaxHops
		if maxHops <= 0 {
			maxHops = 3 // Default maximum hops
		}

		multiHopStartTime := time.Now()
		retrievalResp, err := q.multiHopRAG.Query(ctx, question, maxHops, opts.PromptTemplate)
		if err != nil {
			if q.metrics != nil {
				q.metrics.RecordErrorCount(ctx, "multi_hop_rag")
			}
			if q.logger != nil {
				q.logger.Error(ctx, "Failed to perform multi-hop RAG", err, map[string]interface{}{"question": question})
			}
			if q.tracer != nil {
				if span, ok := q.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return nil, fmt.Errorf("failed to perform multi-hop RAG: %w", err)
		}

		// Convert to QueryHandler Response
		response := &Response{
			Answer:  retrievalResp.Answer,
			Sources: retrievalResp.Sources,
		}

		if q.logger != nil {
			q.logger.Debug(ctx, "Multi-hop RAG completed", map[string]interface{}{
				"duration":      time.Since(multiHopStartTime).Seconds(),
				"answer_length": len(response.Answer),
				"sources_count": len(response.Sources),
			})
		}

		// Cache the response if cache is available
		if q.cache != nil {
			cacheKey := question + opts.PromptTemplate
			q.cache.Set(ctx, cacheKey, response, 1*time.Hour)
		}

		// Record metrics
		if q.metrics != nil {
			duration := time.Since(startTime)
			q.metrics.RecordQueryLatency(ctx, duration)
			q.metrics.RecordQueryCount(ctx, status)
		}

		// Log query
		if q.logger != nil {
			q.logger.Info(ctx, "Multi-hop RAG query completed", map[string]interface{}{
				"question":      question,
				"duration":      time.Since(startTime).Seconds(),
				"answer_length": len(response.Answer),
				"sources_count": len(response.Sources),
			})
		}

		return response, nil
	} else if opts.UseAgenticRAG && q.agenticRAG != nil {
		// Use agentic RAG with autonomous retrieval
		agenticStartTime := time.Now()
		retrievalResp, err := q.agenticRAG.Query(ctx, question, opts.AgentInstructions, opts.PromptTemplate)
		if err != nil {
			if q.metrics != nil {
				q.metrics.RecordErrorCount(ctx, "agentic_rag")
			}
			if q.logger != nil {
				q.logger.Error(ctx, "Failed to perform agentic RAG", err, map[string]interface{}{"question": question})
			}
			if q.tracer != nil {
				if span, ok := q.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return nil, fmt.Errorf("failed to perform agentic RAG: %w", err)
		}

		// Convert to QueryHandler Response
		response := &Response{
			Answer:  retrievalResp.Answer,
			Sources: retrievalResp.Sources,
		}

		if q.logger != nil {
			q.logger.Debug(ctx, "Agentic RAG completed", map[string]interface{}{
				"duration":      time.Since(agenticStartTime).Seconds(),
				"answer_length": len(response.Answer),
				"sources_count": len(response.Sources),
			})
		}

		// Cache the response if cache is available
		if q.cache != nil {
			cacheKey := question + opts.PromptTemplate
			q.cache.Set(ctx, cacheKey, response, 1*time.Hour)
		}

		// Record metrics
		if q.metrics != nil {
			duration := time.Since(startTime)
			q.metrics.RecordQueryLatency(ctx, duration)
			q.metrics.RecordQueryCount(ctx, status)
		}

		// Log query
		if q.logger != nil {
			q.logger.Info(ctx, "Agentic RAG query completed", map[string]interface{}{
				"question":      question,
				"duration":      time.Since(startTime).Seconds(),
				"answer_length": len(response.Answer),
				"sources_count": len(response.Sources),
			})
		}

		return response, nil
	}

	// Standard RAG path
	// Determine routing for the query
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Use router if available
	var routeResult RouteResult
	var err error
	if q.router != nil {
		routeResult, err = q.router.Route(ctx, question)
		if err != nil {
			// Fall back to default routing if router fails
			routeResult = RouteResult{
				Type: "hybrid",
				Params: map[string]interface{}{
					"topK": topK,
				},
			}
			if q.logger != nil {
				q.logger.Warn(ctx, "Router failed, falling back to default routing", map[string]interface{}{"error": err.Error()})
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

	// Enhance query using HyDE if available
	queryToEmbed := question
	if q.hydration != nil {
		enhancedQuery, err := q.hydration.EnhanceQuery(ctx, question)
		if err == nil {
			queryToEmbed = enhancedQuery
			if q.logger != nil {
				q.logger.Debug(ctx, "Query enhanced with HyDE", map[string]interface{}{"original_query": question, "enhanced_query_length": len(enhancedQuery)})
			}
		}
	}

	queryEmbeddings, err := q.embedder.Embed(ctx, []string{queryToEmbed})
	if err != nil {
		if q.metrics != nil {
			q.metrics.RecordErrorCount(ctx, "embedding")
		}
		if q.logger != nil {
			q.logger.Error(ctx, "Failed to embed query", err, map[string]interface{}{"question": question})
		}
		if q.tracer != nil {
			if span, ok := q.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if len(queryEmbeddings) == 0 {
		err := fmt.Errorf("failed to generate query embedding")
		if q.metrics != nil {
			q.metrics.RecordErrorCount(ctx, "embedding")
		}
		if q.logger != nil {
			q.logger.Error(ctx, "Failed to generate query embedding", err, map[string]interface{}{"question": question})
		}
		if q.tracer != nil {
			if span, ok := q.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, err
	}

	var results []core.Result

	// Execute search based on route type
	switch routeResult.Type {
	case "keyword":
		// Use keyword search if retriever is available
		if q.retriever != nil {
			results, err = q.retriever.KeywordSearch(ctx, question, topK*2)
		} else {
			// Fall back to vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = q.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	case "vector":
		// Use vector search
		searchOpts := vectorstore.SearchOptions{
			TopK: topK * 2, // Get more results for reranking
		}
		results, err = q.store.Search(ctx, queryEmbeddings[0], searchOpts)
	case "hybrid":
		fallthrough
	default:
		// Use hybrid retriever if available
		if q.retriever != nil {
			results, err = q.retriever.Search(ctx, question, queryEmbeddings[0], topK*2)
		} else {
			// Use default vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = q.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	}

	if err != nil {
		if q.metrics != nil {
			q.metrics.RecordErrorCount(ctx, "search")
		}
		if q.logger != nil {
			q.logger.Error(ctx, "Failed to search", err, map[string]interface{}{"question": question, "route_type": routeResult.Type})
		}
		if q.tracer != nil {
			if span, ok := q.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Use reranker if available
	if q.reranker != nil && len(results) > 0 {
		rerankStartTime := time.Now()
		results, err = q.reranker.Rerank(ctx, question, results)
		if err != nil {
			if q.metrics != nil {
				q.metrics.RecordErrorCount(ctx, "reranking")
			}
			if q.logger != nil {
				q.logger.Error(ctx, "Failed to rerank", err, map[string]interface{}{"question": question})
			}
			if q.tracer != nil {
				if span, ok := q.tracer.Extract(ctx); ok {
					span.SetError(err)
				}
			}
			status = "error"
			return nil, fmt.Errorf("failed to rerank: %w", err)
		}
		if q.logger != nil {
			q.logger.Debug(ctx, "Reranking completed", map[string]interface{}{
				"duration":      time.Since(rerankStartTime).Seconds(),
				"results_count": len(results),
			})
		}
	}

	// Use context compressor if available
	if q.compressor != nil && len(results) > 0 {
		compressStartTime := time.Now()
		results, err = q.compressor.Compress(ctx, question, results)
		if err != nil {
			if q.logger != nil {
				q.logger.Warn(ctx, "Failed to compress context, using original results", map[string]interface{}{"error": err.Error()})
			}
		} else if q.logger != nil {
			q.logger.Debug(ctx, "Context compression completed", map[string]interface{}{
				"duration":         time.Since(compressStartTime).Seconds(),
				"compressed_count": len(results),
			})
		}
	}

	if len(results) == 0 {
		response := &Response{
			Answer:  "未找到相关信息",
			Sources: results,
		}

		// Cache the response if cache is available
		if q.cache != nil {
			cacheKey := question + opts.PromptTemplate
			q.cache.Set(ctx, cacheKey, response, 1*time.Hour)
		}

		// Record metrics
		if q.metrics != nil {
			duration := time.Since(startTime)
			q.metrics.RecordQueryLatency(ctx, duration)
			q.metrics.RecordQueryCount(ctx, status)
		}

		// Log query
		if q.logger != nil {
			q.logger.Info(ctx, "Query completed with no results", map[string]interface{}{
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
	answer, err := q.llm.Complete(ctx, prompt)
	if err != nil {
		if q.metrics != nil {
			q.metrics.RecordErrorCount(ctx, "llm_completion")
		}
		if q.logger != nil {
			q.logger.Error(ctx, "Failed to generate answer", err, map[string]interface{}{"question": question})
		}
		if q.tracer != nil {
			if span, ok := q.tracer.Extract(ctx); ok {
				span.SetError(err)
			}
		}
		status = "error"
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}
	if q.logger != nil {
		q.logger.Debug(ctx, "LLM completion completed", map[string]interface{}{
			"duration":      time.Since(llmStartTime).Seconds(),
			"answer_length": len(answer),
		})
	}

	response := &Response{
		Answer:  answer,
		Sources: results,
	}

	// Cache the response if cache is available
	if q.cache != nil {
		cacheKey := question + opts.PromptTemplate
		q.cache.Set(ctx, cacheKey, response, 1*time.Hour)
	}

	// Record metrics
	if q.metrics != nil {
		duration := time.Since(startTime)
		q.metrics.RecordQueryLatency(ctx, duration)
		q.metrics.RecordQueryCount(ctx, status)
	}

	// Log query
	if q.logger != nil {
		q.logger.Info(ctx, "Query completed", map[string]interface{}{
			"question":      question,
			"duration":      time.Since(startTime).Seconds(),
			"answer_length": len(response.Answer),
			"sources_count": len(response.Sources),
			"route_type":    routeResult.Type,
		})
	}

	return response, nil
}

// QueryStream performs a streaming RAG query
func (q *QueryHandler) QueryStream(ctx context.Context, question string, opts QueryOptions) (<-chan StreamResponse, error) {
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	// Check if advanced RAG patterns are requested
	if opts.UseMultiHopRAG || opts.UseAgenticRAG {
		return nil, fmt.Errorf("streaming is not supported for advanced RAG patterns (multi-hop or agentic RAG)")
	}

	// Determine routing for the query
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Use router if available
	var routeResult RouteResult
	var err error
	if q.router != nil {
		routeResult, err = q.router.Route(ctx, question)
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

	// Enhance query using HyDE if available
	queryToEmbed := question
	if q.hydration != nil {
		enhancedQuery, err := q.hydration.EnhanceQuery(ctx, question)
		if err == nil {
			queryToEmbed = enhancedQuery
			if q.logger != nil {
				q.logger.Debug(ctx, "Query enhanced with HyDE", map[string]interface{}{"original_query": question, "enhanced_query_length": len(enhancedQuery)})
			}
		}
	}

	queryEmbeddings, err := q.embedder.Embed(ctx, []string{queryToEmbed})
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if len(queryEmbeddings) == 0 {
		return nil, fmt.Errorf("failed to generate query embedding")
	}

	var results []core.Result

	// Execute search based on route type
	switch routeResult.Type {
	case "keyword":
		// Use keyword search if retriever is available
		if q.retriever != nil {
			results, err = q.retriever.KeywordSearch(ctx, question, topK*2)
		} else {
			// Fall back to vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = q.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	case "vector":
		// Use vector search
		searchOpts := vectorstore.SearchOptions{
			TopK: topK * 2, // Get more results for reranking
		}
		results, err = q.store.Search(ctx, queryEmbeddings[0], searchOpts)
	case "hybrid":
		fallthrough
	default:
		// Use hybrid retriever if available
		if q.retriever != nil {
			results, err = q.retriever.Search(ctx, question, queryEmbeddings[0], topK*2)
		} else {
			// Use default vector search
			searchOpts := vectorstore.SearchOptions{
				TopK: topK * 2, // Get more results for reranking
			}
			results, err = q.store.Search(ctx, queryEmbeddings[0], searchOpts)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Use reranker if available
	if q.reranker != nil && len(results) > 0 {
		results, err = q.reranker.Rerank(ctx, question, results)
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
	llmCh, err := q.llm.CompleteStream(ctx, prompt)
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

// BatchQuery performs multiple RAG queries in batch
func (q *QueryHandler) BatchQuery(ctx context.Context, questions []string, opts QueryOptions) ([]*Response, error) {
	if len(questions) == 0 {
		return []*Response{}, nil
	}

	responses := make([]*Response, len(questions))

	// Process each question in batch
	for i, question := range questions {
		response, err := q.Query(ctx, question, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to query: %w", err)
		}
		responses[i] = response
	}

	return responses, nil
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
