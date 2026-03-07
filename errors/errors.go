package errors

import (
	"fmt"
	"strings"
)

// ErrorType represents the category of error
type ErrorType string

const (
	// ErrorTypeInvalidInput indicates invalid input parameters
	ErrorTypeInvalidInput ErrorType = "invalid_input"
	// ErrorTypeParsing indicates document parsing errors
	ErrorTypeParsing ErrorType = "parsing"
	// ErrorTypeEmbedding indicates embedding generation errors
	ErrorTypeEmbedding ErrorType = "embedding"
	// ErrorTypeStorage indicates vector storage errors
	ErrorTypeStorage ErrorType = "storage"
	// ErrorTypeRetrieval indicates retrieval errors
	ErrorTypeRetrieval ErrorType = "retrieval"
	// ErrorTypeLLM indicates LLM client errors
	ErrorTypeLLM ErrorType = "llm"
	// ErrorTypeConfiguration indicates configuration errors
	ErrorTypeConfiguration ErrorType = "configuration"
	// ErrorTypeNetwork indicates network-related errors
	ErrorTypeNetwork ErrorType = "network"
	// ErrorTypeTimeout indicates timeout errors
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeUnknown indicates unknown errors
	ErrorTypeUnknown ErrorType = "unknown"
)

// GoRAGError represents a detailed error with context and suggestions
type GoRAGError struct {
	Type        ErrorType              // Error category
	Message     string                 // Error message
	Cause       error                  // Underlying error
	Suggestions []string               // Recovery suggestions
	Context     map[string]interface{} // Additional context
	DocsURL     string                 // Link to relevant documentation
}

// Error implements the error interface
func (e *GoRAGError) Error() string {
	var buf strings.Builder

	// Error type and message
	buf.WriteString(fmt.Sprintf("[%s] %s", e.Type, e.Message))

	// Underlying cause
	if e.Cause != nil {
		buf.WriteString(fmt.Sprintf("\nCause: %v", e.Cause))
	}

	// Context information
	if len(e.Context) > 0 {
		buf.WriteString("\nContext:")
		for key, value := range e.Context {
			buf.WriteString(fmt.Sprintf("\n  - %s: %v", key, value))
		}
	}

	// Suggestions
	if len(e.Suggestions) > 0 {
		buf.WriteString("\n\nSuggestions:")
		for i, suggestion := range e.Suggestions {
			buf.WriteString(fmt.Sprintf("\n  %d. %s", i+1, suggestion))
		}
	}

	// Documentation link
	if e.DocsURL != "" {
		buf.WriteString(fmt.Sprintf("\n\nFor more information, see: %s", e.DocsURL))
	}

	return buf.String()
}

// Unwrap returns the underlying error
func (e *GoRAGError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error type
func (e *GoRAGError) Is(target error) bool {
	if t, ok := target.(*GoRAGError); ok {
		return e.Type == t.Type
	}
	return false
}

// NewError creates a new GoRAGError
func NewError(errType ErrorType, message string) *GoRAGError {
	return &GoRAGError{
		Type:        errType,
		Message:     message,
		Context:     make(map[string]interface{}),
		Suggestions: []string{},
	}
}

// WithCause adds the underlying cause
func (e *GoRAGError) WithCause(cause error) *GoRAGError {
	e.Cause = cause
	return e
}

// WithSuggestion adds a recovery suggestion
func (e *GoRAGError) WithSuggestion(suggestion string) *GoRAGError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// WithSuggestions adds multiple recovery suggestions
func (e *GoRAGError) WithSuggestions(suggestions ...string) *GoRAGError {
	e.Suggestions = append(e.Suggestions, suggestions...)
	return e
}

// WithContext adds context information
func (e *GoRAGError) WithContext(key string, value interface{}) *GoRAGError {
	e.Context[key] = value
	return e
}

// WithContextMap adds multiple context entries
func (e *GoRAGError) WithContextMap(context map[string]interface{}) *GoRAGError {
	for k, v := range context {
		e.Context[k] = v
	}
	return e
}

// WithDocsURL adds a documentation link
func (e *GoRAGError) WithDocsURL(url string) *GoRAGError {
	e.DocsURL = url
	return e
}

// Common error constructors

// ErrInvalidInput creates an invalid input error
func ErrInvalidInput(message string) *GoRAGError {
	return NewError(ErrorTypeInvalidInput, message).
		WithSuggestion("Check the input parameters and ensure they meet the requirements").
		WithDocsURL("https://github.com/DotNetAge/gorag/blob/main/docs/api.md")
}

// ErrParsing creates a parsing error
func ErrParsing(format string, cause error) *GoRAGError {
	return NewError(ErrorTypeParsing, fmt.Sprintf("Failed to parse %s document", format)).
		WithCause(cause).
		WithSuggestions(
			"Ensure the document is not corrupted",
			"Check if the document format is supported",
			"Try using a different parser if available",
		).
		WithDocsURL("https://github.com/DotNetAge/gorag/blob/main/docs/parsers.md")
}

// ErrEmbedding creates an embedding error
func ErrEmbedding(provider string, cause error) *GoRAGError {
	return NewError(ErrorTypeEmbedding, fmt.Sprintf("Failed to generate embeddings using %s", provider)).
		WithCause(cause).
		WithSuggestions(
			"Check your API key and ensure it's valid",
			"Verify the embedding model is available",
			"Check your network connection",
			"Review rate limits for your API plan",
		).
		WithDocsURL("https://github.com/DotNetAge/gorag/blob/main/docs/embedding.md")
}

// ErrStorage creates a storage error
func ErrStorage(operation string, cause error) *GoRAGError {
	return NewError(ErrorTypeStorage, fmt.Sprintf("Failed to %s in vector store", operation)).
		WithCause(cause).
		WithSuggestions(
			"Check if the vector store is running and accessible",
			"Verify your connection settings",
			"Ensure you have sufficient permissions",
			"Check if the collection/index exists",
		).
		WithDocsURL("https://github.com/DotNetAge/gorag/blob/main/docs/vectorstore.md")
}

// ErrRetrieval creates a retrieval error
func ErrRetrieval(cause error) *GoRAGError {
	return NewError(ErrorTypeRetrieval, "Failed to retrieve documents").
		WithCause(cause).
		WithSuggestions(
			"Check if documents have been indexed",
			"Verify the query parameters",
			"Ensure the vector store is accessible",
		).
		WithDocsURL("https://github.com/DotNetAge/gorag/blob/main/docs/retrieval.md")
}

// ErrLLM creates an LLM error
func ErrLLM(provider string, cause error) *GoRAGError {
	return NewError(ErrorTypeLLM, fmt.Sprintf("Failed to generate response using %s", provider)).
		WithCause(cause).
		WithSuggestions(
			"Check your API key and ensure it's valid",
			"Verify the model name is correct",
			"Check your network connection",
			"Review rate limits and quotas",
			"Try reducing the prompt length if it's too long",
		).
		WithDocsURL("https://github.com/DotNetAge/gorag/blob/main/docs/llm.md")
}

// ErrConfiguration creates a configuration error
func ErrConfiguration(message string) *GoRAGError {
	return NewError(ErrorTypeConfiguration, message).
		WithSuggestions(
			"Check your configuration file syntax",
			"Verify all required fields are set",
			"Review the configuration documentation",
		).
		WithDocsURL("https://github.com/DotNetAge/gorag/blob/main/docs/config.md")
}

// ErrNetwork creates a network error
func ErrNetwork(operation string, cause error) *GoRAGError {
	return NewError(ErrorTypeNetwork, fmt.Sprintf("Network error during %s", operation)).
		WithCause(cause).
		WithSuggestions(
			"Check your internet connection",
			"Verify the service endpoint is correct",
			"Check if the service is experiencing downtime",
			"Try again after a short delay",
		)
}

// ErrTimeout creates a timeout error
func ErrTimeout(operation string, duration string) *GoRAGError {
	return NewError(ErrorTypeTimeout, fmt.Sprintf("Operation timed out: %s", operation)).
		WithContext("timeout", duration).
		WithSuggestions(
			"Increase the timeout duration in configuration",
			"Check if the service is responding slowly",
			"Try processing smaller batches",
		)
}

// Wrap wraps an existing error with GoRAG error context
func Wrap(err error, errType ErrorType, message string) *GoRAGError {
	if err == nil {
		return nil
	}

	// If it's already a GoRAGError, return it
	if goragErr, ok := err.(*GoRAGError); ok {
		return goragErr
	}

	return NewError(errType, message).WithCause(err)
}

// IsType checks if an error is of a specific type
func IsType(err error, errType ErrorType) bool {
	if goragErr, ok := err.(*GoRAGError); ok {
		return goragErr.Type == errType
	}
	return false
}
