package core

import "context"

// Generator defines the interface for generating answers based on query and retrieved context.
// It combines LLM capabilities with retrieved knowledge to produce accurate, contextual responses.
// The generator is responsible for the final "Generation" step in the RAG pipeline.
//
// Implementations typically:
//   - Format the query and context into a prompt
//   - Call an LLM to generate a response
//   - Post-process the response (add citations, format, etc.)
//
// Example usage:
//
//	generator := NewMyGenerator(llm)
//	result, err := generator.Generate(ctx, query, chunks)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Answer)
type Generator interface {
	// Generate produces an answer based on the query and retrieved chunks.
	// It synthesizes information from the chunks to answer the query accurately.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - query: The user's question
	//   - chunks: Retrieved chunks to use as context
	//
	// Returns:
	//   - *Result: The generated answer with confidence score
	//   - error: Any error that occurred during generation
	Generate(ctx context.Context, query *Query, chunks []*Chunk) (*Result, error)

	// GenerateHypotheticalDocument creates a hypothetical document that would answer the query.
	// This is used in HyDE (Hypothetical Document Embedding) to improve retrieval quality
	// by generating a document that would be relevant to the query, then searching for similar real documents.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - query: The query to generate a hypothetical document for
	//
	// Returns:
	//   - string: The generated hypothetical document content
	//   - error: Any error that occurred during generation
	GenerateHypotheticalDocument(ctx context.Context, query *Query) (string, error)
}
