// Package crag provides evaluation steps for RAG retrieval quality assessment.
//
// This package contains reusable steps for evaluating retrieval quality:
//   - Evaluate: CRAG (Correctness, Relevance, Accuracy, Grounding) evaluation
//
// Example usage:
//
//	p := pipeline.New[*entity.PipelineState]()
//	p.AddSteps(
//	    crag.Evaluate(cragEvaluator, logger, metrics),
//	)
package crag
