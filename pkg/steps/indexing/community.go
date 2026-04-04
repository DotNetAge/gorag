package stepinx

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type detectCommunities struct {
	detector core.CommunityDetector
	store    core.GraphStore
	logger   logging.Logger
}

// DetectCommunities creates a step that detects communities in the knowledge graph.
// Communities are hierarchical groups of related nodes that enable:
// - Global search (searching community summaries)
// - Hierarchical summarization (multi-level understanding)
func DetectCommunities(detector core.CommunityDetector, graphStore core.GraphStore, logger logging.Logger) pipeline.Step[*core.IndexingContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &detectCommunities{
		detector: detector,
		store:    graphStore,
		logger:   logger,
	}
}

func (s *detectCommunities) Name() string {
	return "DetectCommunities"
}

func (s *detectCommunities) Execute(ctx context.Context, state *core.IndexingContext) error {
	if s.detector == nil {
		return fmt.Errorf("community detector not configured")
	}

	if s.store == nil {
		s.logger.Warn("GraphStore is nil, skipping community detection", nil)
		return nil
	}

	s.logger.Info("Starting community detection", map[string]any{
		"file": state.FilePath,
	})

	// Detect communities
	communities, err := s.detector.Detect(ctx, s.store)
	if err != nil {
		return fmt.Errorf("community detection failed: %w", err)
	}

	// Store communities in state for downstream steps (e.g., summary generation)
	state.Communities = communities

	s.logger.Info("Community detection completed", map[string]any{
		"communities": len(communities),
	})

	return nil
}
