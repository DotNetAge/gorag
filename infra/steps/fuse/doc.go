// Package fuse provides result fusion steps for RAG retrieval pipelines.
//
// This package contains reusable steps for merging multiple retrieval results:
//   - RRF: Reciprocal Rank Fusion algorithm
//
// Example usage:
//
//	p := pipeline.New[*entity.PipelineState]()
//	p.AddSteps(
//	    fuse.RRF(fusionEngine, 10, logger, metrics),
//	)
package fuse
