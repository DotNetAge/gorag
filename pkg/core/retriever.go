package core

import "context"

// Retriever defines the interface for retrieving relevant chunks based on queries.
// It is the core component responsible for finding and ranking relevant information
// from the knowledge base. Implementations can use various strategies including:
//   - Vector similarity search
//   - Keyword-based search (BM25, TF-IDF)
//   - Graph traversal (GraphRAG)
//   - Hybrid approaches combining multiple methods
//
// The retriever is a key component in the RAG pipeline, bridging the gap between
// user queries and the knowledge base content.
type Retriever interface {
	// Retrieve fetches the most relevant chunks for the given queries.
	// It returns a RetrievalResult for each query, containing the matched chunks
	// and their relevance scores.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - queries: List of query strings to search for
	//   - topK: Maximum number of chunks to return per query
	//
	// Returns:
	//   - []*RetrievalResult: Retrieval results for each query
	//   - error: Any error that occurred during retrieval
	Retrieve(ctx context.Context, queries []string, topK int) ([]*RetrievalResult, error)
}
