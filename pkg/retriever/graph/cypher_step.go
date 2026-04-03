package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// CypherStep allows using Cypher templates for deep relationship retrieval.
// It is specifically designed for GraphStores that support Cypher (like Neo4j).
type CypherStep struct {
	store    core.GraphStore
	template string // Cypher template, e.g., MATCH (p:Entity {id: $id})-[:WORKS_AT]->(c)-[:CEO_OF]-(ceo) RETURN ceo.id as name
	logger   logging.Logger
}

// NewCypherStep creates a new step for Cypher-based graph retrieval.
func NewCypherStep(store core.GraphStore, template string, logger logging.Logger) *CypherStep {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &CypherStep{
		store:    store,
		template: template,
		logger:   logger,
	}
}

func (s *CypherStep) Name() string {
	return "CypherQuery"
}

func (s *CypherStep) Execute(ctx context.Context, retrievalCtx *core.RetrievalContext) error {
	// Get extracted entities from context
	entities, ok := retrievalCtx.Custom["extracted_entities"].([]string)
	if !ok || len(entities) == 0 {
		return nil
	}

	var results []string
	for _, entity := range entities {
		// Execute Cypher query with entity ID as $id parameter
		params := map[string]any{"id": entity}
		records, err := s.store.Query(ctx, s.template, params)
		if err != nil {
			s.logger.Warn("failed to execute cypher template", map[string]any{
				"error":  err,
				"entity": entity,
			})
			continue
		}

		// Parse results and add to context
		for _, record := range records {
			// We look for a 'name' or 'result' field in the returned map
			val, ok := record["name"].(string)
			if !ok {
				// Fallback to first string value found
				for _, v := range record {
					if sVal, ok := v.(string); ok {
						val = sVal
						break
					}
				}
			}

			if val != "" {
				results = append(results, fmt.Sprintf("Graph Insight: Entity '%s' has a deep relationship with '%s'", entity, val))
			}
		}
	}

	if len(results) > 0 {
		// Append insights to graph_context for LLM to use
		existingCtx, _ := retrievalCtx.Custom["graph_context"].(string)
		separator := ""
		if existingCtx != "" {
			separator = "\n"
		}
		retrievalCtx.Custom["graph_context"] = existingCtx + separator + "[Deep Reasoning Insights]:\n" + strings.Join(results, "\n")
	}

	return nil
}
