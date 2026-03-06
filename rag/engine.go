package rag

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
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
	if source.Type == "" {
		return fmt.Errorf("type is required")
	}

	if source.Content == "" && source.Path == "" {
		return fmt.Errorf("content or path is required")
	}

	var reader io.Reader
	if source.Content != "" {
		reader = strings.NewReader(source.Content)
	} else if source.Reader != nil {
		if r, ok := source.Reader.(io.Reader); ok {
			reader = r
		} else {
			return fmt.Errorf("invalid reader type")
		}
	}

	if reader == nil {
		return fmt.Errorf("no content to index")
	}

	chunks, err := e.parser.Parse(ctx, reader)
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	vsChunks := make([]vectorstore.Chunk, len(chunks))
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		vsChunks[i] = vectorstore.Chunk{
			ID:       chunk.ID,
			Content:  chunk.Content,
			Metadata: chunk.Metadata,
		}
		texts[i] = chunk.Content
	}

	embeddings, err := e.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to embed chunks: %w", err)
	}

	err = e.store.Add(ctx, vsChunks, embeddings)
	if err != nil {
		return fmt.Errorf("failed to store chunks: %w", err)
	}

	return nil
}

// Query performs a RAG query
func (e *Engine) Query(ctx context.Context, question string, opts QueryOptions) (*Response, error) {
	if opts.Stream {
		// For streaming, use QueryStream
		ch, err := e.QueryStream(ctx, question, opts)
		if err != nil {
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
			return nil, finalErr
		}

		return &Response{
			Answer:  answer.String(),
			Sources: sources,
		}, nil
	}

	// Non-streaming path
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	queryEmbeddings, err := e.embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if len(queryEmbeddings) == 0 {
		return nil, fmt.Errorf("failed to generate query embedding")
	}

	var results []vectorstore.Result

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
		return &Response{
			Answer:  "未找到相关信息",
			Sources: results,
		}, nil
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

	answer, err := e.llm.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	return &Response{
		Answer:  answer,
		Sources: results,
	}, nil
}

// QueryStream performs a streaming RAG query
func (e *Engine) QueryStream(ctx context.Context, question string, opts QueryOptions) (<-chan StreamResponse, error) {
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	queryEmbeddings, err := e.embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if len(queryEmbeddings) == 0 {
		return nil, fmt.Errorf("failed to generate query embedding")
	}

	var results []vectorstore.Result

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
		return strings.ReplaceAll(template, "{question}", question)
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
