package rag

import (
	"context"
	"strings"
)

// Router defines the interface for query routing
type Router interface {
	// Route determines the appropriate routing for a given query
	Route(ctx context.Context, query string) (RouteResult, error)
}

// RouteResult represents the result of routing a query
type RouteResult struct {
	// Type indicates the type of route (e.g., "vector", "keyword", "hybrid")
	Type string
	// Params contains additional parameters for the route
	Params map[string]interface{}
}

// DefaultRouter implements a simple default router
type DefaultRouter struct {}

// NewDefaultRouter creates a new default router
func NewDefaultRouter() *DefaultRouter {
	return &DefaultRouter{}
}

// Route determines the appropriate routing for a given query
func (r *DefaultRouter) Route(ctx context.Context, query string) (RouteResult, error) {
	// Simple routing logic based on query content
	query = strings.ToLower(query)

	// Check for specific patterns
	if strings.Contains(query, "what") || strings.Contains(query, "who") || strings.Contains(query, "when") || strings.Contains(query, "where") || strings.Contains(query, "why") || strings.Contains(query, "how") {
		// Question-based queries - use hybrid search
		return RouteResult{
			Type: "hybrid",
			Params: map[string]interface{}{
				"topK": 5,
			},
		}, nil
	}

	if strings.Contains(query, "find") || strings.Contains(query, "search") || strings.Contains(query, "lookup") {
		// Search-based queries - use vector search
		return RouteResult{
			Type: "vector",
			Params: map[string]interface{}{
				"topK": 10,
			},
		}, nil
	}

	if strings.Contains(query, "list") || strings.Contains(query, "show") || strings.Contains(query, "display") {
		// List-based queries - use keyword search
		return RouteResult{
			Type: "keyword",
			Params: map[string]interface{}{
				"topK": 8,
			},
		}, nil
	}

	// Default to hybrid search
	return RouteResult{
		Type: "hybrid",
		Params: map[string]interface{}{
			"topK": 5,
		},
	}, nil
}
