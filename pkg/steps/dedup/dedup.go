package dedup

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type unique struct {
	threshold float64
	logger    logging.Logger
	metrics   core.Metrics
}

func Unique(threshold float64, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.State] {
	if threshold <= 0 {
		threshold = 0.95
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &unique{threshold: threshold, logger: logger, metrics: metrics}
}

func (s *unique) Name() string { return "Unique" }

func (s *unique) Execute(_ context.Context, state *core.State) error {
	var allChunks []*core.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		allChunks = append(allChunks, chunkGroup...)
	}
	for _, chunks := range state.ParallelResults {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	uniqueMap := make(map[string]*core.Chunk)
	var uniqueChunks []*core.Chunk

	for _, chunk := range allChunks {
		if _, ok := uniqueMap[chunk.Content]; !ok {
			uniqueMap[chunk.Content] = chunk
			uniqueChunks = append(uniqueChunks, chunk)
		}
	}

	state.RetrievedChunks = [][]*core.Chunk{uniqueChunks}
	state.ParallelResults = make(map[string][]*core.Chunk)
	return nil
}
