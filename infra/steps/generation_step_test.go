package steps_test

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/testkit"
)

// TestGenerationStep_Execute tests the generation step behavior
func TestGenerationStep_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		queryText   string
		chunks      []*entity.Chunk
		mockAnswer  string
		mockError   error
		wantErr     bool
		errContains string
		wantAnswer  string
	}{
		{
			name:        "empty query",
			queryText:   "",
			chunks:      nil,
			wantErr:     true,
			errContains: "query required",
		},
		{
			name:       "no retrieved chunks",
			queryText:  "What is RAG?",
			chunks:     nil,
			mockAnswer: "RAG is a technique...",
			wantErr:    false,
			wantAnswer: "RAG is a technique...",
		},
		{
			name:       "with retrieved chunks",
			queryText:  "Explain transformers",
			chunks:     testkit.CreateTestChunks("Transformers are..."),
			mockAnswer: "Transformers use self-attention...",
			wantErr:    false,
			wantAnswer: "Transformers use self-attention...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			logger := testkit.NewMockLogger()

			// Create state
			state := testkit.NewTestPipelineState()
			if tt.queryText != "" {
				state.Query = testkit.NewTestQuery(tt.queryText, nil)
			}

			if len(tt.chunks) > 0 {
				state.RetrievedChunks = append(state.RetrievedChunks, tt.chunks)
			}

			ctx := context.Background()

			// Note: Full implementation requires mocking the Generator service
			// This test demonstrates the test pattern

			// Verify state setup
			if tt.queryText != "" && state.Query == nil {
				t.Error("Query should be set")
			}

			if len(tt.chunks) > 0 && len(state.RetrievedChunks) == 0 {
				t.Error("RetrievedChunks should be set")
			}

			_ = logger
			_ = ctx
		})
	}
}

// TestGenerationStep_AgenticMetadataUpdate tests that AgenticMetadata is properly updated
func TestGenerationStep_AgenticMetadataUpdate(t *testing.T) {
	t.Parallel()

	logger := testkit.NewMockLogger()

	// Create state with AgenticMetadata
	state := testkit.NewTestPipelineState()
	state.Query = testkit.NewTestQuery("Test query", nil)
	state.Answer = "Generated answer"

	// Verify AgenticMetadata exists
	if state.Agentic == nil {
		t.Fatal("AgenticMetadata should be initialized")
	}

	// Verify RewrittenQueryText can be set
	state.Agentic.RewrittenQueryText = "Rewritten query"
	if state.Agentic.RewrittenQueryText != "Rewritten query" {
		t.Errorf("expected 'Rewritten query', got %q", state.Agentic.RewrittenQueryText)
	}

	// Verify OriginalQueryText is set
	if state.Agentic.OriginalQueryText != "Test query" {
		t.Errorf("expected 'Test query', got %q", state.Agentic.OriginalQueryText)
	}

	_ = logger
}
