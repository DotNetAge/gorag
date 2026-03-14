package dataprep

import "context"

// Indexer defines the entry point for the offline data preparation pipeline.
type Indexer interface {
	// IndexFile processes a single file into the Vector/Graph stores.
	IndexFile(ctx context.Context, filePath string) error
	
	// IndexDirectory concurrently processes an entire directory.
	IndexDirectory(ctx context.Context, dirPath string, recursive bool) error
}
