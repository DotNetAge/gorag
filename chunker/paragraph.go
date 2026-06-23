package chunker

import (
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
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
		chunkSizeHit := false

		for j := i; j < len(paragraphs); j++ {
			paraLen := len(paragraphs[j])
			addLen := paraLen
			if len(selected) > 0 {
				addLen += 2 // "\n\n"
			}

			// 如果加入这段后超过 chunkSize，且已有足够内容，停止
			if currentLen+addLen > c.chunkSize && len(selected) >= 1 && currentLen >= c.minChunkSize {
				chunkSizeHit = true
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

		// 复制 metadata 避免与其他 chunker 共享
		meta := structured.RawDoc.GetMeta()
		metaCopy := make(map[string]any, len(meta))
		for k, v := range meta {
			metaCopy[k] = v
		}

		chunk := &core.Chunk{
			ID:       GenerateChunkID(structured.RawDoc.GetID(), index, content),
			ParentID: "",
			DocID:    structured.RawDoc.GetID(),
			MIMEType: structured.RawDoc.GetMimeType(),
			Content:  content,
			Metadata: metaCopy,
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

		// 应用 overlap：仅当因 chunkSize 限制而停止时（非 maxParagraphs 上限）
		nextStart := selected[len(selected)-1] + 1
		if c.overlap > 0 && nextStart <= len(paragraphs) && chunkSizeHit {
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

// extractHeadingInfo extracts heading information from the structure tree.
// It determines which heading section a chunk belongs to by traversing the
// tree and matching chunkStart against each child's effective span (from
// the child's StartPos up to the next sibling's StartPos).
//
// The tree structure from the structurizer may be:
//   - Flat (MarkdownStructurizer / tree-sitter): headings and paragraphs are
//     siblings under a "document" root.
//   - Nested (PlainTextStructurizer / buildTree): paragraphs are children of
//     the nearest preceding heading.
//
// This algorithm handles both: it tracks the *most recent heading* as it
// walks siblings, and when it finds the child whose span contains chunkStart,
// it assigns the tracked heading info to the chunk. "lastHeading" (not
// position containment) is used because a heading node's own text range
// (StartPos..EndPos) only covers the title line, not the following content.
func (c *ParagraphChunker) extractHeadingInfo(
	chunk *core.Chunk,
	node *core.StructureNode,
	chunkStart, chunkEnd int,
) {
	if node == nil || len(node.Children) == 0 {
		return
	}

	children := node.Children
	var lastHeading *core.StructureNode

	for i, child := range children {
		// Track the most recent heading that starts before the chunk
		if child.NodeType == "heading" && child.StartPos <= chunkStart {
			lastHeading = child
		}

		// Determine the effective end of this child's span
		var spanEnd int
		if i+1 < len(children) {
			spanEnd = children[i+1].StartPos
		} else {
			spanEnd = chunkEnd
		}

		if child.StartPos <= chunkStart && chunkStart < spanEnd {
			// Assign heading info from the nearest preceding heading
			if lastHeading != nil {
				chunk.ChunkMeta.HeadingLevel = lastHeading.Level
				chunk.ChunkMeta.HeadingPath = []string{lastHeading.Title}
			}
			// Recurse into this child's subtree
			c.extractHeadingInfo(chunk, child, chunkStart, chunkEnd)
			break
		}
	}
}
