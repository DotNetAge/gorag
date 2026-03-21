package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// routerRetriever implements core.Retriever with intelligent intent-based routing.
type routerRetriever struct {
	classifier core.IntentClassifier
	mapping    map[core.IntentType]core.Retriever
	defaultRet core.Retriever
	logger     logging.Logger
}

// NewSmartRouter creates a new router that dispatches queries to different retrievers based on intent.
func NewSmartRouter(
	classifier core.IntentClassifier,
	mapping map[core.IntentType]core.Retriever,
	defaultRet core.Retriever,
	logger logging.Logger,
) core.Retriever {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &routerRetriever{
		classifier: classifier,
		mapping:    mapping,
		defaultRet: defaultRet,
		logger:     logger,
	}
}

func (r *routerRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, qText := range queries {
		query := core.NewQuery("", qText, nil)
		
		// 1. Classify Intent
		intentRes, err := r.classifier.Classify(ctx, query)
		if err != nil {
			r.logger.Warn("intent classification failed, falling back to default retriever", map[string]any{"error": err, "query": qText})
			res, err := r.defaultRet.Retrieve(ctx, []string{qText}, topK)
			if err != nil {
				return nil, err
			}
			results = append(results, res...)
			continue
		}

		r.logger.Info("Query routed", map[string]any{
			"query":      qText,
			"intent":     intentRes.Intent,
			"confidence": intentRes.Confidence,
		})

		// 2. Dispatch to the mapped retriever
		target, ok := r.mapping[intentRes.Intent]
		if !ok || target == nil {
			r.logger.Debug("no specific retriever for intent, using default", map[string]any{"intent": intentRes.Intent})
			target = r.defaultRet
		}

		if target == nil {
			return nil, fmt.Errorf("no retriever available for intent %s and no default configured", intentRes.Intent)
		}

		// 3. Execute Retrieval with Execution-level Fallback
		res, err := target.Retrieve(ctx, []string{qText}, topK)
		if err != nil {
			// If target is already the default, we can't fallback further
			if target == r.defaultRet {
				return nil, fmt.Errorf("default retrieval failed: %w", err)
			}

			r.logger.Warn("routed retrieval failed, falling back to default", map[string]any{
				"intent": intentRes.Intent,
				"error":  err,
				"query":  qText,
			})
			
			res, err = r.defaultRet.Retrieve(ctx, []string{qText}, topK)
			if err != nil {
				return nil, fmt.Errorf("fallback retrieval failed after routed failure: %w", err)
			}
		}

		// Attach intent info to metadata for traceability
		for _, item := range res {
			if item.Metadata == nil {
				item.Metadata = make(map[string]any)
			}
			item.Metadata["intent"] = intentRes.Intent
			item.Metadata["confidence"] = intentRes.Confidence
			item.Metadata["route_reason"] = intentRes.Reason
		}

		results = append(results, res...)
	}

	return results, nil
}
