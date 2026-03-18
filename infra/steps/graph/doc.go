// Package graph provides graph retrieval steps for RAG pipelines.
//
// This package contains reusable steps for graph-based retrieval:
//   - Local: N-Hop traversal from specific entities
//   - Global: Community summary synthesis for macro-level questions
//
// Example usage:
//
//	p := pipeline.New[*entity.PipelineState]()
//	p.AddSteps(
//	    graph.Local(localSearcher, 2, 10, logger, metrics),
//	    graph.Global(globalSearcher, 1, logger, metrics),
//	)
package graph
