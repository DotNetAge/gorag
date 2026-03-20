package stepinx

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
)

// chunk performs semantic chunking on documents.
type chunk struct {
	chunker core.SemanticChunker
}

// Chunk creates a semantic chunking step.
//
// Parameters:
//   - chunker: semantic chunker implementation
//
// Example:
//
//	p.AddStep(indexing.Chunk(chunker))
func Chunk(chunker core.SemanticChunker) pipeline.Step[*core.IndexingContext] {
	return &chunk{chunker: chunker}
}

// Name returns the step name
func (s *chunk) Name() string {
	return "Chunk"
}

// Execute chunks all documents from state.Documents channel.
func (s *chunk) Execute(ctx context.Context, state *core.IndexingContext) error {
	if s.chunker == nil {
		return fmt.Errorf("chunker not configured")
	}

	// Get Documents from state (note: Documents is a channel)
	if state.Documents == nil {
		return fmt.Errorf("no documents to chunk")
	}

	// Iterate through all documents and chunk them
	var allChunks []*core.Chunk

	for doc := range state.Documents {
		if doc == nil {
			continue
		}

		// Use chunker to split document into chunks
		chunks, err := s.chunker.Chunk(ctx, doc)
		if err != nil {
			return fmt.Errorf("failed to chunk document %s: %w", doc.ID, err)
		}

		allChunks = append(allChunks, chunks...)
	}

	// Create channel and pass chunks to next step
	chunkChan := make(chan *core.Chunk, len(allChunks))
	for _, chunk := range allChunks {
		chunkChan <- chunk
	}
	close(chunkChan) // Close channel to notify downstream steps

	// Store chunks channel in state for next step (EmbedStep)
	state.Chunks = chunkChan

	return nil
}
