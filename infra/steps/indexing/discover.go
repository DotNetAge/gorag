// Package indexing provides document indexing pipeline steps for RAG data preparation.
package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/indexing"
)

// discover discovers and validates files for indexing.
type discover struct{}

// Discover creates a new file discovery step.
//
// Example:
//
//	p.AddStep(indexing.Discover())
func Discover() pipeline.Step[*indexing.State] {
	return &discover{}
}

// Name returns the step name
func (s *discover) Name() string {
	return "FileDiscovery"
}

// Execute discovers and validates the file, extracting metadata.
func (s *discover) Execute(ctx context.Context, state *indexing.State) error {
	// Check if file exists
	info, err := os.Stat(state.FilePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", state.FilePath)
	}

	// Update metadata in state
	state.Metadata = indexing.Metadata{
		Source:   state.FilePath,
		FileName: filepath.Base(state.FilePath),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
	}

	return nil
}
