// Package indexing provides document indexing pipeline steps for RAG data preparation.
//
// This package contains reusable steps for building indexing pipelines:
//   - Discover: File discovery and validation
//   - Multi: Multi-parser streaming document parsing
//   - Semantic: Semantic chunking of documents
//   - Batch: Batch embedding generation
//   - Upsert: Vector storage upsert operations
//   - Entities: Entity extraction for graph indexing
//
// Example usage:
//
//	p := pipeline.New[*core.State]()
//	p.AddSteps(
//	    indexing.Discover(),
//	    indexing.Multi(parsers...),
//	    indexing.Semantic(chunker),
//	    indexing.Batch(embedder, metrics),
//	    indexing.Upsert(vectorStore, metrics),
//	)
//
//	err := p.Execute(ctx, &indexing.State{FilePath: "document.pdf"})
package stepinx
