package rag

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/raya-dev/gorag/embedding"
	"github.com/raya-dev/gorag/llm"
	"github.com/raya-dev/gorag/parser"
	"github.com/raya-dev/gorag/vectorstore"
)

// Engine represents the RAG engine
type Engine struct {
	parser   parser.Parser
	embedder embedding.Provider
	store    vectorstore.Store
	llm      llm.Client
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

	searchOpts := vectorstore.SearchOptions{
		TopK: topK,
	}

	results, err := e.store.Search(ctx, queryEmbeddings[0], searchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	if len(results) == 0 {
		return &Response{
			Answer:  "未找到相关信息",
			Sources: results,
		}, nil
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
