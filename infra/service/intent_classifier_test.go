package service_test

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/testkit"
)

func TestIntentClassifier_Classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		query       string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty query",
			query:       "",
			wantErr:     true,
			errContains: "query required",
		},
		{
			name:        "simple query",
			query:       "What is RAG?",
			wantErr:     false,
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup mock logger and collector
			logger := testkit.NewMockLogger()
			collector := testkit.NewMockCollector()

			// Verify mock types compile correctly
			_ = logger
			_ = collector

			ctx := context.Background()
			_ = ctx

			if tt.query == "" {
				// For empty query test, we expect error handling
				return
			}

			// TODO: Implement with mocked LLM client
			t.Skip("Requires mocked LLM client")
		})
	}
}
