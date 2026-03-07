package middleware

import (
	"context"

	"github.com/DotNetAge/gorag/core"
)

// Middleware defines the interface for middleware components
// Middleware can intercept and modify the indexing and query pipeline
type Middleware interface {
	// Name returns the middleware name
	Name() string

	// BeforeIndex is called before indexing a document
	BeforeIndex(ctx context.Context, source *Source) error

	// AfterIndex is called after indexing a document
	AfterIndex(ctx context.Context, chunks []core.Chunk) error

	// BeforeQuery is called before executing a query
	BeforeQuery(ctx context.Context, query *Query) error

	// AfterQuery is called after executing a query
	AfterQuery(ctx context.Context, response *Response) error
}

// Source represents a document source for indexing
type Source struct {
	Type    string
	Path    string
	Content string
	Reader  interface{}
}

// Query represents a query request
type Query struct {
	Question string
	Options  map[string]interface{}
}

// Response represents a query response
type Response struct {
	Answer  string
	Sources []core.Result
}

// Chain represents a middleware chain
type Chain struct {
	middlewares []Middleware
}

// NewChain creates a new middleware chain
func NewChain(middlewares ...Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
	}
}

// Add adds a middleware to the chain
func (c *Chain) Add(middleware Middleware) {
	c.middlewares = append(c.middlewares, middleware)
}

// BeforeIndex executes all BeforeIndex middleware
func (c *Chain) BeforeIndex(ctx context.Context, source *Source) error {
	for _, m := range c.middlewares {
		if err := m.BeforeIndex(ctx, source); err != nil {
			return err
		}
	}
	return nil
}

// AfterIndex executes all AfterIndex middleware
func (c *Chain) AfterIndex(ctx context.Context, chunks []core.Chunk) error {
	for _, m := range c.middlewares {
		if err := m.AfterIndex(ctx, chunks); err != nil {
			return err
		}
	}
	return nil
}

// BeforeQuery executes all BeforeQuery middleware
func (c *Chain) BeforeQuery(ctx context.Context, query *Query) error {
	for _, m := range c.middlewares {
		if err := m.BeforeQuery(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

// AfterQuery executes all AfterQuery middleware
func (c *Chain) AfterQuery(ctx context.Context, response *Response) error {
	for _, m := range c.middlewares {
		if err := m.AfterQuery(ctx, response); err != nil {
			return err
		}
	}
	return nil
}

// BaseMiddleware provides a default implementation of Middleware
// Embed this in your custom middleware to only implement the methods you need
type BaseMiddleware struct {
	name string
}

// NewBaseMiddleware creates a new base middleware
func NewBaseMiddleware(name string) *BaseMiddleware {
	return &BaseMiddleware{name: name}
}

// Name returns the middleware name
func (m *BaseMiddleware) Name() string {
	return m.name
}

// BeforeIndex is a no-op by default
func (m *BaseMiddleware) BeforeIndex(ctx context.Context, source *Source) error {
	return nil
}

// AfterIndex is a no-op by default
func (m *BaseMiddleware) AfterIndex(ctx context.Context, chunks []core.Chunk) error {
	return nil
}

// BeforeQuery is a no-op by default
func (m *BaseMiddleware) BeforeQuery(ctx context.Context, query *Query) error {
	return nil
}

// AfterQuery is a no-op by default
func (m *BaseMiddleware) AfterQuery(ctx context.Context, response *Response) error {
	return nil
}
