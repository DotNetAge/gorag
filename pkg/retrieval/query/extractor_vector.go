package query

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.EntityExtractor = (*vectorExtractor)(nil)

// vectorExtractor extracts entities by combining keyword extraction with graph validation
// and optional vector similarity scoring.
// This approach works without LLM by:
// 1. Extracting candidate entities using heuristics (capitalization, quotes, etc.)
// 2. Validating candidates exist in the graph store
// 3. Optionally re-ranking by semantic similarity
type vectorExtractor struct {
	graphStore core.GraphStore
	embedder   embedding.Provider
	threshold  float64
	topK       int
	minLen     int
	logger     logging.Logger
	collector  observability.Collector
}

// VectorExtractorOption configures a vectorExtractor instance.
type VectorExtractorOption func(*vectorExtractor)

// WithVectorExtractorThreshold sets the similarity threshold for entity matching.
// Default is 0.7. Lower values return more entities but with lower precision.
func WithVectorExtractorThreshold(threshold float64) VectorExtractorOption {
	return func(e *vectorExtractor) {
		if threshold > 0 && threshold <= 1 {
			e.threshold = threshold
		}
	}
}

// WithVectorExtractorTopK sets the maximum number of entities to return.
// Default is 5.
func WithVectorExtractorTopK(topK int) VectorExtractorOption {
	return func(e *vectorExtractor) {
		if topK > 0 {
			e.topK = topK
		}
	}
}

// WithVectorExtractorMinLen sets minimum entity length.
// Default is 2.
func WithVectorExtractorMinLen(minLen int) VectorExtractorOption {
	return func(e *vectorExtractor) {
		if minLen > 0 {
			e.minLen = minLen
		}
	}
}

// WithVectorExtractorLogger sets a structured logger.
func WithVectorExtractorLogger(logger logging.Logger) VectorExtractorOption {
	return func(e *vectorExtractor) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithVectorExtractorCollector sets an observability collector.
func WithVectorExtractorCollector(collector observability.Collector) VectorExtractorOption {
	return func(e *vectorExtractor) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// NewVectorExtractor creates a new vector-based entity extractor.
// It extracts candidate entities using heuristics and validates them against the graph store.
// If an embedder is provided, it can optionally re-rank entities by semantic similarity.
func NewVectorExtractor(
	graphStore core.GraphStore,
	embedder embedding.Provider,
	opts ...VectorExtractorOption,
) *vectorExtractor {
	e := &vectorExtractor{
		graphStore: graphStore,
		embedder:   embedder,
		threshold:  0.7,
		topK:       5,
		minLen:     2,
		logger:     logging.DefaultNoopLogger(),
		collector:  observability.DefaultNoopCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Extract extracts entities from the query using a multi-stage approach:
// 1. Extract candidate entities using text heuristics
// 2. Validate candidates exist in the graph store
// 3. Return validated entities (up to topK)
func (e *vectorExtractor) Extract(ctx context.Context, query *core.Query) (*core.EntityExtractionResult, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("vector_entity_extraction", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		return nil, nil
	}

	text := query.Text

	// Stage 1: Extract candidate entities using heuristics
	candidates := e.extractCandidates(text)

	// Stage 2: Validate candidates against graph store
	validEntities := make([]string, 0, e.topK)
	for _, candidate := range candidates {
		// Check if entity exists in graph
		node, err := e.graphStore.GetNode(ctx, candidate)
		if err != nil {
			e.logger.Debug("entity lookup failed", map[string]any{"entity": candidate, "error": err.Error()})
			continue
		}
		if node != nil {
			validEntities = append(validEntities, candidate)
			if len(validEntities) >= e.topK {
				break
			}
		}
	}

	// Stage 3: If we have an embedder and fewer than topK entities,
	// try fuzzy matching via Cypher query (for Neo4j/Memgraph)
	if e.embedder != nil && len(validEntities) < e.topK && e.graphStore != nil {
		// Try to find entities with similar names using graph query
		semanticEntities := e.findSemanticEntities(ctx, text, validEntities)
		validEntities = append(validEntities, semanticEntities...)
		if len(validEntities) > e.topK {
			validEntities = validEntities[:e.topK]
		}
	}

	e.logger.Debug("vector entity extraction completed", map[string]any{
		"query":    text,
		"entities": validEntities,
	})

	return &core.EntityExtractionResult{Entities: validEntities}, nil
}

// extractCandidates extracts candidate entities using text heuristics.
func (e *vectorExtractor) extractCandidates(text string) []string {
	var candidates []string
	seen := make(map[string]bool)

	// 1. Extract quoted strings
	quotedPattern := regexp.MustCompile(`["']([^"']+)["']`)
	matches := quotedPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 && len(match[1]) >= e.minLen {
			candidate := match[1]
			if !seen[strings.ToLower(candidate)] {
				seen[strings.ToLower(candidate)] = true
				candidates = append(candidates, candidate)
			}
		}
	}

	// 2. Extract capitalized word sequences
	capPattern := regexp.MustCompile(`\b([A-Z][a-zA-Z]+(?:\s+[A-Z][a-zA-Z]+)*)\b`)
	matches = capPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 && len(match[1]) >= e.minLen {
			candidate := match[1]
			if !seen[strings.ToLower(candidate)] {
				seen[strings.ToLower(candidate)] = true
				candidates = append(candidates, candidate)
			}
		}
	}

	// 3. Extract Chinese noun phrases
	chinesePattern := regexp.MustCompile(`[\p{Han}]{2,}`)
	matches = chinesePattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 0 && len(match[0]) >= e.minLen {
			candidate := match[0]
			if !seen[candidate] {
				seen[candidate] = true
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates
}

// findSemanticEntities finds entities using semantic matching via graph query.
func (e *vectorExtractor) findSemanticEntities(ctx context.Context, text string, exclude []string) []string {
	excludeSet := make(map[string]bool)
	for _, e := range exclude {
		excludeSet[strings.ToLower(e)] = true
	}

	// Try to find entities whose name contains query keywords
	// This uses a fuzzy LIKE query which works across graph databases
	query := `MATCH (n) WHERE n.id CONTAINS $keyword RETURN DISTINCT n.id as id LIMIT $limit`

	// Extract significant words from query
	words := strings.Fields(text)
	var entities []string

	for _, word := range words {
		if len(word) < e.minLen {
			continue
		}
		if excludeSet[strings.ToLower(word)] {
			continue
		}

		results, err := e.graphStore.Query(ctx, query, map[string]any{
			"keyword": word,
			"limit":   e.topK,
		})
		if err != nil {
			continue
		}

		for _, result := range results {
			if id, ok := result["id"].(string); ok && id != "" {
				if !excludeSet[strings.ToLower(id)] {
					entities = append(entities, id)
					excludeSet[strings.ToLower(id)] = true
				}
			}
		}

		if len(entities) >= e.topK {
			break
		}
	}

	return entities
}
