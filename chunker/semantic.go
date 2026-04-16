package chunker

import (
	"github.com/DotNetAge/gorag/core"
)

// SemanticChunker splits text based on semantic similarity
// Detects topic changes where similarity drops below threshold
type SemanticChunker struct {
	embedder            core.Embedder // embedder for generating embeddings
	similarityThreshold float32       // similarity threshold
	maxSentences        int           // max sentences (fallback)
	options             Options       // optional configuration
}

// NewSemanticChunker creates a new SemanticChunker
func NewSemanticChunker(embedder core.Embedder, opts ...Option) *SemanticChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &SemanticChunker{
		embedder:            embedder,
		similarityThreshold: options.SimilarityThreshold,
		maxSentences:        options.MaxSentences,
		options:             options,
	}
}

// Chunk implements the Chunker interface
func (c *SemanticChunker) Chunk(
	structured *core.StructuredDocument,
) ([]*core.Chunk, error) {
	if structured == nil || structured.RawDoc == nil {
		return []*core.Chunk{}, nil
	}

	text := structured.RawDoc.GetContent()
	if text == "" {
		return []*core.Chunk{}, nil
	}

	// Fallback to sentence chunking if no embedder
	if c.embedder == nil {
		chunks, err := NewSentenceChunker(WithMaxSentences(c.maxSentences)).Chunk(structured)
		if err != nil {
			return nil, err
		}
		// Append image chunks
		if imgChunks := ExtractImageChunks(structured); len(imgChunks) > 0 {
			chunks = append(chunks, imgChunks...)
		}
		return chunks, nil
	}

	// 1. Split into sentences
	sentenceChunker := NewSentenceChunker()
	sentences, err := sentenceChunker.Chunk(structured)
	if err != nil {
		return nil, err
	}

	if len(sentences) == 0 {
		return []*core.Chunk{}, nil
	}

	// 2. Compute sentence embeddings
	sentenceTexts := make([]string, len(sentences))
	for i, s := range sentences {
		sentenceTexts[i] = s.Content
	}

	embeddings, err := embedBatch(c.embedder, sentenceTexts)
	if err != nil {
		// Fallback to sentence chunking on embed error
		chunks := sentences
		// Append image chunks
		if imgChunks := ExtractImageChunks(structured); len(imgChunks) > 0 {
			chunks = append(chunks, imgChunks...)
		}
		return chunks, nil
	}

	// 3. Calculate similarity between adjacent sentences
	similarities := c.calculateSimilarities(embeddings)

	// 4. Detect topic change points (where similarity drops below threshold)
	breakPoints := c.detectTopicChanges(similarities)

	// 5. Create chunks based on break points
	chunks := c.createChunksFromBreakPoints(sentences, breakPoints, structured.RawDoc.GetMimeType())

	// 6. Append image chunks as sub-chunks
	imageChunks := ExtractImageChunks(structured)
	if len(imageChunks) > 0 {
		chunks = append(chunks, imageChunks...)
	}

	return chunks, nil
}

// GetStrategy returns the chunk strategy type
func (c *SemanticChunker) GetStrategy() core.ChunkStrategy {
	return StrategySemantic
}

// calculateSimilarities computes similarity between adjacent sentences
func (c *SemanticChunker) calculateSimilarities(embeddings [][]float32) []float32 {
	if len(embeddings) < 2 {
		return []float32{}
	}

	similarities := make([]float32, len(embeddings)-1)
	for i := 0; i < len(embeddings)-1; i++ {
		similarities[i] = c.cosineSimilarity(embeddings[i], embeddings[i+1])
	}

	return similarities
}

// cosineSimilarity computes cosine similarity (delegates to package function)
func (c *SemanticChunker) cosineSimilarity(a, b []float32) float32 {
	return CosineSimilarity(a, b)
}

// detectTopicChanges detects topic change points
// Returns indices where chunks should be split (in the sentences array)
func (c *SemanticChunker) detectTopicChanges(similarities []float32) []int {
	if len(similarities) == 0 {
		return []int{}
	}

	var breakPoints []int

	for i, sim := range similarities {
		// Mark as break point if similarity is below threshold
		if sim < c.similarityThreshold {
			breakPoints = append(breakPoints, i+1) // i+1 is next sentence index
		}
	}

	return breakPoints
}

// createChunksFromBreakPoints creates chunks based on detected break points
func (c *SemanticChunker) createChunksFromBreakPoints(
	sentences []*core.Chunk,
	breakPoints []int,
	mimeType string,
) []*core.Chunk {
	if len(sentences) == 0 {
		return []*core.Chunk{}
	}

	// If no break points, return all sentences as single chunk
	if len(breakPoints) == 0 {
		return sentences
	}

	var chunks []*core.Chunk
	start := 0

	for _, bp := range breakPoints {
		if bp > start && bp <= len(sentences) {
			// Merge sentences from start to bp-1
			chunk := c.mergeSentences(sentences[start:bp], len(chunks), mimeType)
			chunks = append(chunks, chunk)
			start = bp
		}
	}

	// Handle remaining sentences
	if start < len(sentences) {
		chunk := c.mergeSentences(sentences[start:], len(chunks), mimeType)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// mergeSentences merges multiple sentences into a single chunk
func (c *SemanticChunker) mergeSentences(sentences []*core.Chunk, index int, mimeType string) *core.Chunk {
	if len(sentences) == 0 {
		return nil
	}

	if len(sentences) == 1 {
		chunk := sentences[0]
		chunk.ChunkMeta.Index = index
		return chunk
	}

	// Merge multiple sentences
	var content string
	for i, s := range sentences {
		if i > 0 {
			content += " "
		}
		content += s.Content
	}

	// Use first sentence info as base
	first := sentences[0]
	last := sentences[len(sentences)-1]

	return &core.Chunk{
		ID:       GenerateChunkID(first.DocID, index, content),
		ParentID: "",
		DocID:    first.DocID,
		MIMEType: mimeType,
		Content:  content,
		Metadata: first.Metadata,
		ChunkMeta: core.ChunkMeta{
			Index:        index,
			StartPos:     first.ChunkMeta.StartPos,
			EndPos:       last.ChunkMeta.EndPos,
			HeadingLevel: first.ChunkMeta.HeadingLevel,
			HeadingPath:  first.ChunkMeta.HeadingPath,
		},
	}
}
