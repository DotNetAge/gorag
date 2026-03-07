package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoRAGError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *GoRAGError
		contains []string
	}{
		{
			name: "basic error",
			err: NewError(ErrorTypeParsing, "Failed to parse document"),
			contains: []string{
				"[parsing]",
				"Failed to parse document",
			},
		},
		{
			name: "error with cause",
			err: NewError(ErrorTypeParsing, "Failed to parse document").
				WithCause(fmt.Errorf("invalid format")),
			contains: []string{
				"[parsing]",
				"Failed to parse document",
				"Cause: invalid format",
			},
		},
		{
			name: "error with suggestions",
			err: NewError(ErrorTypeEmbedding, "Failed to generate embeddings").
				WithSuggestions("Check API key", "Verify network connection"),
			contains: []string{
				"[embedding]",
				"Failed to generate embeddings",
				"Suggestions:",
				"1. Check API key",
				"2. Verify network connection",
			},
		},
		{
			name: "error with context",
			err: NewError(ErrorTypeStorage, "Failed to store chunks").
				WithContext("chunk_count", 10).
				WithContext("store_type", "milvus"),
			contains: []string{
				"[storage]",
				"Failed to store chunks",
				"Context:",
				"chunk_count: 10",
				"store_type: milvus",
			},
		},
		{
			name: "error with docs URL",
			err: NewError(ErrorTypeConfiguration, "Invalid config").
				WithDocsURL("https://example.com/docs"),
			contains: []string{
				"[configuration]",
				"Invalid config",
				"For more information, see: https://example.com/docs",
			},
		},
		{
			name: "complete error",
			err: NewError(ErrorTypeLLM, "Failed to generate response").
				WithCause(fmt.Errorf("rate limit exceeded")).
				WithSuggestion("Wait and retry").
				WithContext("provider", "openai").
				WithDocsURL("https://example.com/docs"),
			contains: []string{
				"[llm]",
				"Failed to generate response",
				"Cause: rate limit exceeded",
				"Context:",
				"provider: openai",
				"Suggestions:",
				"1. Wait and retry",
				"For more information, see: https://example.com/docs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			for _, substr := range tt.contains {
				assert.Contains(t, errStr, substr)
			}
		})
	}
}

func TestGoRAGError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := NewError(ErrorTypeParsing, "Failed to parse").WithCause(cause)

	unwrapped := errors.Unwrap(err)
	assert.Equal(t, cause, unwrapped)
}

func TestGoRAGError_Is(t *testing.T) {
	err1 := NewError(ErrorTypeParsing, "Error 1")
	err2 := NewError(ErrorTypeParsing, "Error 2")
	err3 := NewError(ErrorTypeEmbedding, "Error 3")

	assert.True(t, err1.Is(err2), "Same type should match")
	assert.False(t, err1.Is(err3), "Different types should not match")
}

func TestGoRAGError_WithMethods(t *testing.T) {
	err := NewError(ErrorTypeParsing, "Test error")

	// Test WithCause
	cause := fmt.Errorf("cause error")
	err.WithCause(cause)
	assert.Equal(t, cause, err.Cause)

	// Test WithSuggestion
	err.WithSuggestion("Suggestion 1")
	assert.Len(t, err.Suggestions, 1)
	assert.Equal(t, "Suggestion 1", err.Suggestions[0])

	// Test WithSuggestions
	err.WithSuggestions("Suggestion 2", "Suggestion 3")
	assert.Len(t, err.Suggestions, 3)

	// Test WithContext
	err.WithContext("key1", "value1")
	assert.Equal(t, "value1", err.Context["key1"])

	// Test WithContextMap
	err.WithContextMap(map[string]interface{}{
		"key2": "value2",
		"key3": 123,
	})
	assert.Equal(t, "value2", err.Context["key2"])
	assert.Equal(t, 123, err.Context["key3"])

	// Test WithDocsURL
	err.WithDocsURL("https://example.com")
	assert.Equal(t, "https://example.com", err.DocsURL)
}

func TestErrInvalidInput(t *testing.T) {
	err := ErrInvalidInput("Invalid parameter")

	assert.Equal(t, ErrorTypeInvalidInput, err.Type)
	assert.Equal(t, "Invalid parameter", err.Message)
	assert.NotEmpty(t, err.Suggestions)
	assert.NotEmpty(t, err.DocsURL)
}

func TestErrParsing(t *testing.T) {
	cause := fmt.Errorf("file corrupted")
	err := ErrParsing("PDF", cause)

	assert.Equal(t, ErrorTypeParsing, err.Type)
	assert.Contains(t, err.Message, "PDF")
	assert.Equal(t, cause, err.Cause)
	assert.NotEmpty(t, err.Suggestions)
	assert.NotEmpty(t, err.DocsURL)
}

func TestErrEmbedding(t *testing.T) {
	cause := fmt.Errorf("API key invalid")
	err := ErrEmbedding("OpenAI", cause)

	assert.Equal(t, ErrorTypeEmbedding, err.Type)
	assert.Contains(t, err.Message, "OpenAI")
	assert.Equal(t, cause, err.Cause)
	assert.NotEmpty(t, err.Suggestions)
	assert.Contains(t, err.Suggestions[0], "API key")
}

func TestErrStorage(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := ErrStorage("add chunks", cause)

	assert.Equal(t, ErrorTypeStorage, err.Type)
	assert.Contains(t, err.Message, "add chunks")
	assert.Equal(t, cause, err.Cause)
	assert.NotEmpty(t, err.Suggestions)
}

func TestErrRetrieval(t *testing.T) {
	cause := fmt.Errorf("no results found")
	err := ErrRetrieval(cause)

	assert.Equal(t, ErrorTypeRetrieval, err.Type)
	assert.Equal(t, cause, err.Cause)
	assert.NotEmpty(t, err.Suggestions)
}

func TestErrLLM(t *testing.T) {
	cause := fmt.Errorf("rate limit exceeded")
	err := ErrLLM("Anthropic", cause)

	assert.Equal(t, ErrorTypeLLM, err.Type)
	assert.Contains(t, err.Message, "Anthropic")
	assert.Equal(t, cause, err.Cause)
	assert.NotEmpty(t, err.Suggestions)
}

func TestErrConfiguration(t *testing.T) {
	err := ErrConfiguration("Missing API key")

	assert.Equal(t, ErrorTypeConfiguration, err.Type)
	assert.Equal(t, "Missing API key", err.Message)
	assert.NotEmpty(t, err.Suggestions)
}

func TestErrNetwork(t *testing.T) {
	cause := fmt.Errorf("connection timeout")
	err := ErrNetwork("API call", cause)

	assert.Equal(t, ErrorTypeNetwork, err.Type)
	assert.Contains(t, err.Message, "API call")
	assert.Equal(t, cause, err.Cause)
	assert.NotEmpty(t, err.Suggestions)
}

func TestErrTimeout(t *testing.T) {
	err := ErrTimeout("embedding generation", "30s")

	assert.Equal(t, ErrorTypeTimeout, err.Type)
	assert.Contains(t, err.Message, "embedding generation")
	assert.Equal(t, "30s", err.Context["timeout"])
	assert.NotEmpty(t, err.Suggestions)
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		errType  ErrorType
		message  string
		wantNil  bool
		wantType ErrorType
	}{
		{
			name:    "wrap nil error",
			err:     nil,
			errType: ErrorTypeParsing,
			message: "Test",
			wantNil: true,
		},
		{
			name:     "wrap standard error",
			err:      fmt.Errorf("standard error"),
			errType:  ErrorTypeParsing,
			message:  "Wrapped error",
			wantNil:  false,
			wantType: ErrorTypeParsing,
		},
		{
			name:     "wrap GoRAGError",
			err:      NewError(ErrorTypeEmbedding, "Original"),
			errType:  ErrorTypeParsing,
			message:  "Wrapped",
			wantNil:  false,
			wantType: ErrorTypeEmbedding, // Should keep original type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := Wrap(tt.err, tt.errType, tt.message)

			if tt.wantNil {
				assert.Nil(t, wrapped)
				return
			}

			require.NotNil(t, wrapped)
			assert.Equal(t, tt.wantType, wrapped.Type)
		})
	}
}

func TestIsType(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		errType ErrorType
		want    bool
	}{
		{
			name:    "matching type",
			err:     NewError(ErrorTypeParsing, "Test"),
			errType: ErrorTypeParsing,
			want:    true,
		},
		{
			name:    "non-matching type",
			err:     NewError(ErrorTypeParsing, "Test"),
			errType: ErrorTypeEmbedding,
			want:    false,
		},
		{
			name:    "standard error",
			err:     fmt.Errorf("standard error"),
			errType: ErrorTypeParsing,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsType(tt.err, tt.errType)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Create a chain of errors
	rootCause := fmt.Errorf("root cause")
	err := NewError(ErrorTypeParsing, "Parsing failed").
		WithCause(rootCause).
		WithSuggestion("Check file format").
		WithContext("file", "test.pdf")

	// Test error message contains all information
	errStr := err.Error()
	assert.Contains(t, errStr, "Parsing failed")
	assert.Contains(t, errStr, "root cause")
	assert.Contains(t, errStr, "Check file format")
	assert.Contains(t, errStr, "file: test.pdf")

	// Test unwrapping
	assert.Equal(t, rootCause, errors.Unwrap(err))
}
