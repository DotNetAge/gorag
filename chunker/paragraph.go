package chunker

import (
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// ParagraphChunker splits text by paragraph boundaries
// Merges consecutive paragraphs until reaching chunkSize (target), using maxParagraphs as hard cap
// Overlap is applied between consecutive chunks by re-including trailing paragraphs from previous chunk
type ParagraphChunker struct {
	maxParagraphs int
	chunkSize     int // target chunk size in characters
	minChunkSize  int
	overlap       int // overlap size in characters
	options       Options
}

// NewParagraphChunker creates a new ParagraphChunker
func NewParagraphChunker(opts ...Option) *ParagraphChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &ParagraphChunker{
		maxParagraphs: options.MaxParagraphs,
		chunkSize:     options.ChunkSize,
		minChunkSize:  options.MinChunkSize,
		overlap:       options.Overlap,
		options:       options,
	}
}

// Chunk implements the Chunker interface
func (c *ParagraphChunker) Chunk(
	structured *core.StructuredDocument,
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

	// 计算每个段落的累积位置（在原文中的偏移）
	paraPositions := make([]int, len(paragraphs))
	pos := 0
	for i, p := range paragraphs {
		paraPositions[i] = pos
		pos += len(p)
		if i < len(paragraphs)-1 {
			pos += 2 // "\n\n"
		}
	}

	var chunks []*core.Chunk
	index := 0

	i := 0
	for i < len(paragraphs) {
		// 收集段落直到达到 chunkSize 或 maxParagraphs
		var selected []int // 选中的段落索引
		currentLen := 0

		for j := i; j < len(paragraphs); j++ {
			paraLen := len(paragraphs[j])
			addLen := paraLen
			if len(selected) > 0 {
				addLen += 2 // "\n\n"
			}

			// 如果加入这段后超过 chunkSize，且已有足够内容，停止
			if currentLen+addLen > c.chunkSize && len(selected) >= 1 && currentLen >= c.minChunkSize {
				break
			}

			selected = append(selected, j)
			currentLen += addLen

			// 硬上限：maxParagraphs
			if len(selected) >= c.maxParagraphs {
				break
			}
		}

		if len(selected) == 0 {
			break
		}

		// 构建 chunk 内容
		strs := make([]string, len(selected))
		for k, idx := range selected {
			strs[k] = paragraphs[idx]
		}
		content := strings.Join(strs, "\n\n")
		startPos := paraPositions[selected[0]]
		endPos := paraPositions[selected[len(selected)-1]] + len(paragraphs[selected[len(selected)-1]])

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

		if structured.Root != nil {
			c.extractHeadingInfo(chunk, structured.Root, chunk.ChunkMeta.StartPos, chunk.ChunkMeta.EndPos)
		}

		chunks = append(chunks, chunk)
		index++

		// 应用 overlap：从最后一个选中段落向前回溯 overlap 字符
		nextStart := selected[len(selected)-1] + 1
		if c.overlap > 0 && nextStart <= len(paragraphs) {
			overlapUsed := 0
			for k := len(selected) - 1; k >= 1; k-- { // k >= 1 确保至少前进一个段落
				paraLen := len(paragraphs[selected[k]])
				if overlapUsed+paraLen > c.overlap {
					break
				}
				overlapUsed += paraLen
				overlapUsed += 2 // "\n\n"
				nextStart = selected[k]
			}
		}

		i = nextStart
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
func (c *ParagraphChunker) splitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	parts := strings.Split(text, "\n\n")

	var paragraphs []string
	for _, part := range parts {
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

	if node.StartPos <= chunkStart && node.EndPos >= chunkEnd {
		if node.NodeType == "heading" {
			chunk.ChunkMeta.HeadingLevel = node.Level
			if len(chunk.ChunkMeta.HeadingPath) == 0 {
				chunk.ChunkMeta.HeadingPath = []string{node.Title}
			}
		}
	}

	for _, child := range node.Children {
		if child.StartPos <= chunkStart && child.EndPos >= chunkEnd {
			c.extractHeadingInfo(chunk, child, chunkStart, chunkEnd)
			break
		}
	}
}
