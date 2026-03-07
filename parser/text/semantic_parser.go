package text

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedding"
	"github.com/google/uuid"
)

// SemanticParser implements semantic chunking based on sentence boundaries and semantic similarity
type SemanticParser struct {
	chunkSize    int
	chunkOverlap int
	embedder     embedding.Provider
	maxChunkSize int
	minChunkSize int
	similarityThreshold float32
}

// NewSemanticParser creates a new semantic parser
func NewSemanticParser(embedder embedding.Provider) *SemanticParser {
	return &SemanticParser{
		chunkSize:    500,
		chunkOverlap: 50,
		embedder:     embedder,
		maxChunkSize: 1000,
		minChunkSize: 100,
		similarityThreshold: 0.8,
	}
}

// Parse parses text into semantically meaningful chunks
func (p *SemanticParser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	text := string(content)
	chunks := p.splitSemantically(text, ctx)

	result := make([]core.Chunk, len(chunks))
	for i, chunk := range chunks {
		result[i] = core.Chunk{
			ID:      uuid.New().String(),
			Content: chunk,
			Metadata: map[string]string{
				"type":     "text",
				"position": fmt.Sprintf("%d", i),
				"method":   "semantic",
			},
		}
	}

	return result, nil
}

// SupportedFormats returns supported formats
func (p *SemanticParser) SupportedFormats() []string {
	return []string{".txt", ".md"}
}

// splitSemantically splits text into semantically meaningful chunks
func (p *SemanticParser) splitSemantically(text string, ctx context.Context) []string {
	// First split by sentences
	sentences := p.splitIntoSentences(text)
	if len(sentences) == 0 {
		return []string{text}
	}

	// Then group sentences into chunks based on semantic similarity
	return p.groupSentences(sentences, ctx)
}

// splitIntoSentences splits text into sentences
func (p *SemanticParser) splitIntoSentences(text string) []string {
	var sentences []string
	var currentSentence strings.Builder

	text = strings.TrimSpace(text)
	if text == "" {
		return sentences
	}

	for i, r := range text {
		currentSentence.WriteRune(r)

		// Check for sentence boundaries
		if r == '.' || r == '!' || r == '?' {
			// Check if this is a real sentence boundary
			if i+1 < len(text) {
				nextRune, _ := utf8.DecodeRuneInString(text[i+1:])
				if unicode.IsSpace(nextRune) || unicode.IsUpper(nextRune) {
					sentence := strings.TrimSpace(currentSentence.String())
					if sentence != "" {
						sentences = append(sentences, sentence)
						currentSentence.Reset()
					}
				}
			} else {
				// End of text
				sentence := strings.TrimSpace(currentSentence.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
			}
		}
	}

	// Add any remaining text
	remaining := strings.TrimSpace(currentSentence.String())
	if remaining != "" {
		sentences = append(sentences, remaining)
	}

	return sentences
}

// groupSentences groups sentences into semantically coherent chunks
func (p *SemanticParser) groupSentences(sentences []string, ctx context.Context) []string {
	var chunks []string
	var currentChunk strings.Builder
	var currentSentences []string

	for i, sentence := range sentences {
		// Add sentence to current chunk
		currentSentences = append(currentSentences, sentence)
		currentChunk.WriteString(sentence)
		currentChunk.WriteString(" ")

		// Check if chunk is getting too large
		if currentChunk.Len() >= p.chunkSize {
			// Check semantic coherence before splitting
			if p.embedder != nil && i+1 < len(sentences) {
				// Get embedding for current chunk
				currentText := strings.TrimSpace(currentChunk.String())
				nextText := sentences[i+1]
				
				// Get embeddings
				currentEmbedding, err := p.embedder.Embed(ctx, []string{currentText})
				if err != nil {
					// Fallback to simple chunking if embedding fails
					chunks = append(chunks, currentText)
					currentChunk.Reset()
					currentSentences = []string{}
					continue
				}
				
				nextEmbedding, err := p.embedder.Embed(ctx, []string{nextText})
				if err != nil {
					// Fallback to simple chunking if embedding fails
					chunks = append(chunks, currentText)
					currentChunk.Reset()
					currentSentences = []string{}
					continue
				}
				
				// Calculate similarity
				similarity := p.cosineSimilarity(currentEmbedding[0], nextEmbedding[0])
				
				if similarity < p.similarityThreshold {
					// Split here if not semantically similar
					chunks = append(chunks, currentText)
					currentChunk.Reset()
					currentSentences = []string{}
				}
			} else {
				// No embedder or last sentence, just split
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
				currentSentences = []string{}
			}
		}
	}

	// Add final chunk
	finalChunk := strings.TrimSpace(currentChunk.String())
	if finalChunk != "" {
		chunks = append(chunks, finalChunk)
	}

	// Handle chunks that are too small or too large
	return p.adjustChunkSizes(chunks, ctx)
}

// adjustChunkSizes adjusts chunk sizes to be within acceptable range
func (p *SemanticParser) adjustChunkSizes(chunks []string, ctx context.Context) []string {
	var adjustedChunks []string

	for _, chunk := range chunks {
		// If chunk is too small, combine with next chunk
		if len(chunk) < p.minChunkSize && len(adjustedChunks) > 0 {
			// Combine with previous chunk
			lastIdx := len(adjustedChunks) - 1
			combined := adjustedChunks[lastIdx] + " " + chunk
			if len(combined) <= p.maxChunkSize {
				adjustedChunks[lastIdx] = combined
				continue
			}
		}

		// If chunk is too large, split it
		if len(chunk) > p.maxChunkSize {
			subChunks := p.splitLargeChunk(chunk, ctx)
			adjustedChunks = append(adjustedChunks, subChunks...)
		} else {
			adjustedChunks = append(adjustedChunks, chunk)
		}
	}

	return adjustedChunks
}

// splitLargeChunk splits a large chunk into smaller ones
func (p *SemanticParser) splitLargeChunk(chunk string, ctx context.Context) []string {
	// Split by paragraphs first
	paragraphs := strings.Split(chunk, "\n\n")
	if len(paragraphs) > 1 {
		return p.adjustChunkSizes(paragraphs, ctx)
	}

	// Fallback to sentence-based splitting
	sentences := p.splitIntoSentences(chunk)
	return p.groupSentences(sentences, ctx)
}

// cosineSimilarity calculates cosine similarity between two vectors
func (p *SemanticParser) cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float32
	var normA float32
	var normB float32

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
