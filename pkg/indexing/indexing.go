// Package indexing provides the core indexing pipeline for offline data preparation.
//
// This package defines the Indexer interface which serves as the entry point for
// processing documents through parsing, chunking, embedding, and storage stages.
package indexing

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/core"
)

// Indexer defines the entry point for the offline data preparation pipeline.
// It provides methods to process individual files or entire directories into the RAG knowledge base.
type Indexer interface {
	// IndexFile processes a single file into the Vector/Graph stores.
	IndexFile(ctx context.Context, filePath string) (*core.IndexingContext, error)

	// IndexDirectory concurrently processes an entire directory.
	IndexDirectory(ctx context.Context, dirPath string, recursive bool) error
}
