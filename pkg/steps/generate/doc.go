// Package generate provides answer generation steps for RAG pipelines.
//
// This package contains reusable steps for generating answers:
//   - Generate: LLM-based answer generation from retrieved chunks
//
// Example usage:
//
//	p := pipeline.New[*core.RetrievalContext]()
//	p.AddSteps(
//	    generate.Generate(generator, logger, metrics),
//	)
package stepgen
