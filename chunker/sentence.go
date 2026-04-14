package chunker

import (
	"regexp"
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// sentenceInfo contains sentence text with position tracking
type sentenceInfo struct {
	text     string
	startPos int
	endPos   int
}

// SentenceChunker splits text by sentence boundaries
// Ensures each chunk contains complete sentences
type SentenceChunker struct {
	maxSentences int     // maximum sentences per chunk
	options      Options // 可选配置
}

// NewSentenceChunker creates a new SentenceChunker
func NewSentenceChunker(opts ...Option) *SentenceChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &SentenceChunker{
		maxSentences: options.MaxSentences,
		options:      options,
	}
}

// Chunk implements the Chunker interface
func (c *SentenceChunker) Chunk(
	structured *core.StructuredDocument,
	entities []*core.Entity,
) ([]*core.Chunk, error) {
	if structured == nil || structured.RawDoc == nil {
		return []*core.Chunk{}, nil
	}

	text := structured.RawDoc.GetContent()
	if text == "" {
		return []*core.Chunk{}, nil
	}

	sentenceInfos := c.splitSentencesWithPositions(text)

	if len(sentenceInfos) == 0 {
		return []*core.Chunk{}, nil
	}

	var chunks []*core.Chunk
	currentSentences := []sentenceInfo{}
	index := 0

	for i, info := range sentenceInfos {
		currentSentences = append(currentSentences, info)

		// Create chunk when reaching max sentences or last sentence
		if len(currentSentences) >= c.maxSentences || i == len(sentenceInfos)-1 {
			// Calculate chunk position range
			startPos := currentSentences[0].startPos
			endPos := currentSentences[len(currentSentences)-1].endPos
			content := text[startPos:endPos]

			chunk := &core.Chunk{
				ID:       GenerateChunkID(structured.RawDoc.GetID(), index, content),
				ParentID: "",
				DocID:    structured.RawDoc.GetID(),
				MIMEType: structured.RawDoc.GetMimeType(),
				Content:  content,
				Metadata: structured.RawDoc.GetMeta(),
				ChunkMeta: core.ChunkMeta{
					Index:        index,
					StartPos:     startPos,
					EndPos:       endPos,
					HeadingLevel: 0,
					HeadingPath:  []string{},
				},
			}

			// Extract heading info from structured document
			if structured.Root != nil {
				c.extractHeadingInfo(chunk, structured.Root, chunk.ChunkMeta.StartPos, chunk.ChunkMeta.EndPos)
			}

			chunks = append(chunks, chunk)

			// Reset state
			currentSentences = []sentenceInfo{}
			index++
		}
	}

	return chunks, nil
}

// GetStrategy returns the chunk strategy type
func (c *SentenceChunker) GetStrategy() core.ChunkStrategy {
	return StrategySentence
}

// splitSentencesWithPositions splits text into sentences with position tracking
// Supports both Chinese and English sentence delimiters
func (c *SentenceChunker) splitSentencesWithPositions(text string) []sentenceInfo {
	// Sentence delimiter regex: Chinese/English periods, exclamation marks, question marks
	// followed by whitespace or end of string
	sentenceEndPattern := regexp.MustCompile(`([。.！!？?])\s*`)

	// Find all sentence delimiter positions
	matches := sentenceEndPattern.FindAllStringIndex(text, -1)

	if len(matches) == 0 {
		// No sentence delimiters found, return entire text as single sentence
		if strings.TrimSpace(text) != "" {
			return []sentenceInfo{
				{
					text:     text,
					startPos: 0,
					endPos:   len(text),
				},
			}
		}
		return []sentenceInfo{}
	}

	var sentences []sentenceInfo
	lastEnd := 0

	for _, match := range matches {
		// match[0] is start position, match[1] is end position (includes trailing whitespace)
		end := match[1]
		sentence := strings.TrimSpace(text[lastEnd:end])
		if sentence != "" {
			sentences = append(sentences, sentenceInfo{
				text:     sentence,
				startPos: lastEnd,
				endPos:   end,
			})
		}
		lastEnd = end
	}

	// Handle remaining text after last delimiter
	if lastEnd < len(text) {
		remaining := strings.TrimSpace(text[lastEnd:])
		if remaining != "" {
			sentences = append(sentences, sentenceInfo{
				text:     remaining,
				startPos: lastEnd,
				endPos:   len(text),
			})
		}
	}

	return sentences
}

// extractHeadingInfo extracts heading information from the structure tree
func (c *SentenceChunker) extractHeadingInfo(
	chunk *core.Chunk,
	node *core.StructureNode,
	chunkStart, chunkEnd int,
) {
	if node == nil {
		return
	}

	// Check if current node contains the chunk position
	if node.StartPos <= chunkStart && node.EndPos >= chunkEnd {
		if node.NodeType == "heading" {
			chunk.ChunkMeta.HeadingLevel = node.Level
			if len(chunk.ChunkMeta.HeadingPath) == 0 {
				chunk.ChunkMeta.HeadingPath = []string{node.Title}
			}
		}
	}

	// Recursively check child nodes
	for _, child := range node.Children {
		if child.StartPos <= chunkStart && child.EndPos >= chunkEnd {
			c.extractHeadingInfo(chunk, child, chunkStart, chunkEnd)
			break
		}
	}
}
