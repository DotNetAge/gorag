package chunker

import (
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// ParagraphChunker splits text by paragraph boundaries
// Maintains semantic unit integrity
type ParagraphChunker struct {
	maxParagraphs int     // maximum paragraphs per chunk
	options       Options // optional configuration
}

// NewParagraphChunker creates a new ParagraphChunker
func NewParagraphChunker(opts ...Option) *ParagraphChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &ParagraphChunker{
		maxParagraphs: options.MaxParagraphs,
		options:       options,
	}
}

// Chunk implements the Chunker interface
func (c *ParagraphChunker) Chunk(
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

	paragraphs := c.splitParagraphs(text)

	if len(paragraphs) == 0 {
		return []*core.Chunk{}, nil
	}

	var chunks []*core.Chunk
	currentParagraphs := []string{}
	currentStartPos := 0
	currentContentLen := 0
	index := 0

	for _, para := range paragraphs {
		if para == "" {
			continue
		}

		currentParagraphs = append(currentParagraphs, para)

		// Calculate content length including separator
		separator := "\n\n"
		contentLen := len(para)
		if len(currentParagraphs) > 1 {
			currentContentLen += len(separator) + contentLen
		} else {
			currentContentLen = contentLen
		}

		// Reach max paragraphs, create a chunk
		if len(currentParagraphs) >= c.maxParagraphs {
			chunkStartPos := currentStartPos
			chunkEndPos := currentStartPos + currentContentLen
			content := strings.Join(currentParagraphs, separator)
			chunk := &core.Chunk{
				ID:       GenerateChunkID(structured.RawDoc.GetID(), index, content),
				ParentID: "",
				DocID:    structured.RawDoc.GetID(),
				MIMEType: structured.RawDoc.GetMimeType(),
				Content:  content,
				Metadata: structured.RawDoc.GetMeta(),
				ChunkMeta: core.ChunkMeta{
					Index:        index,
					StartPos:     chunkStartPos,
					EndPos:       chunkEndPos,
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
			currentStartPos = chunkEndPos
			currentParagraphs = []string{}
			currentContentLen = 0
			index++
		}
	}

	// Handle remaining paragraphs
	if len(currentParagraphs) > 0 {
		separator := "\n\n"
		chunkStartPos := currentStartPos
		chunkEndPos := currentStartPos + currentContentLen
		content := strings.Join(currentParagraphs, separator)
		chunk := &core.Chunk{
			ID:       GenerateChunkID(structured.RawDoc.GetID(), index, content),
			ParentID: "",
			DocID:    structured.RawDoc.GetID(),
			MIMEType: structured.RawDoc.GetMimeType(),
			Content:  content,
			Metadata: structured.RawDoc.GetMeta(),
			ChunkMeta: core.ChunkMeta{
				Index:        index,
				StartPos:     chunkStartPos,
				EndPos:       chunkEndPos,
				HeadingLevel: 0,
				HeadingPath:  []string{},
			},
		}

		if structured.Root != nil {
			c.extractHeadingInfo(chunk, structured.Root, chunk.ChunkMeta.StartPos, chunk.ChunkMeta.EndPos)
		}

		chunks = append(chunks, chunk)
	}

	// Append image chunks as sub-chunks
	if imgChunks := ExtractImageChunks(structured); len(imgChunks) > 0 {
		chunks = append(chunks, imgChunks...)
	}

	return chunks, nil
}

// GetStrategy returns the chunk strategy type
func (c *ParagraphChunker) GetStrategy() core.ChunkStrategy {
	return StrategyParagraph
}

// splitParagraphs splits text by paragraphs
// Two or more consecutive newlines are treated as paragraph separators
func (c *ParagraphChunker) splitParagraphs(text string) []string {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Split by two or more consecutive newlines
	parts := strings.Split(text, "\n\n")

	// Filter empty paragraphs and trim whitespace
	var paragraphs []string
	for _, part := range parts {
		// Handle possible extra newlines
		lines := strings.Split(part, "\n")
		var cleanedLines []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				cleanedLines = append(cleanedLines, line)
			}
		}

		if len(cleanedLines) > 0 {
			paragraphs = append(paragraphs, strings.Join(cleanedLines, "\n"))
		}
	}

	return paragraphs
}

// extractHeadingInfo extracts heading information from the structure tree
func (c *ParagraphChunker) extractHeadingInfo(
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
