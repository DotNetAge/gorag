package core

import (
	"context"
)

// IntentClassifier defines the interface for classifying query intent.
// It determines the type of query (chat, fact-check, relational, etc.) to route to appropriate retrievers.
type IntentClassifier interface {
	Classify(ctx context.Context, query *Query) (*IntentResult, error)
}
