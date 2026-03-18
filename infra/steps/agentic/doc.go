// Package agentic provides agentic RAG orchestration steps.
//
// This package contains reusable steps for agentic RAG loops:
//   - IntentRouter: Query intent classification and routing
//   - Reasoning: Agent reasoning trace generation
//   - ActionSelection: Next action selection (retrieve/reflect/finish)
//   - ParallelRetrieval: Parallel multi-query retrieval
//   - Observation: Retrieval state snapshot recording
//   - TerminationCheck: Agentic loop termination checking
//   - ToolExecutor: Tool execution for agentic actions
//   - SelfRAG: Self-RAG implementation
//
// Example usage:
//
//	p := pipeline.New[*entity.PipelineState]()
//	p.AddSteps(
//	    agentic.IntentRouter(classifier, logger),
//	    agentic.Reasoning(reasoner, logger),
//	    agentic.ActionSelection(selector, 5, logger),
//	    agentic.ParallelRetrieval(retriever, logger),
//	    agentic.Observation(logger),
//	    agentic.TerminationCheck(checker, logger),
//	)
package agentic
