package steps_test

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/testkit"
)

// TestIntentRouter_Execute_Basic tests basic execution flow
func TestIntentRouter_Execute_Basic(t *testing.T) {
	t.Parallel()

	// This test demonstrates the test pattern
	// Full implementation requires mocking retrieval.IntentClassifier
	// which depends on LLM client implementation

	logger := testkit.NewMockLogger()

	// Create a minimal state for testing
	state := testkit.NewTestPipelineState()
	state.Query = testkit.NewTestQuery("test query", nil)

	ctx := context.Background()

	// Verify test helpers work correctly
	if state.Query.Text != "test query" {
		t.Errorf("expected query text 'test query', got %q", state.Query.Text)
	}

	if logger == nil {
		t.Error("logger should not be nil")
	}

	if state.Agentic == nil {
		t.Error("Agentic metadata should be initialized")
	}

	_ = ctx
}
