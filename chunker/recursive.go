package chunker

import (
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// RecursiveChunker intelligently splits text by trying different separator levels
// in priority order and selecting optimal split points
type RecursiveChunker struct {
	chunkSize    int      // 块大小（字符数）
	minChunkSize int      // 最小块大小
	separators   []string // 分隔符列表（优先级从高到低）
	options      Options  // 可选配置
}

// NewRecursiveChunker creates a new RecursiveChunker
func NewRecursiveChunker(opts ...Option) *RecursiveChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	separators := options.Separators
	if len(separators) == 0 {
		separators = DefaultSeparators()
	}

	return &RecursiveChunker{
		chunkSize:    Clamp(options.ChunkSize, MinChunkSize, MaxChunkSize),
		minChunkSize: Clamp(options.MinChunkSize, MinChunkSize, options.ChunkSize/2),
		separators:   separators,
		options:      options,
	}
}

// Chunk implements the Chunker interface
func (c *RecursiveChunker) Chunk(
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

	chunks := c.recursiveSplit(text, 0, len(text), 0)

	// Enrich metadata
	for i, chunk := range chunks {
		chunk.ID = GenerateChunkID(structured.RawDoc.GetID(), i, chunk.Content)
		chunk.DocID = structured.RawDoc.GetID()
		chunk.MIMEType = structured.RawDoc.GetMimeType()
		chunk.Metadata = structured.RawDoc.GetMeta()
		chunk.ChunkMeta.Index = i

		// Extract heading info from structured document
		if structured.Root != nil {
			c.extractHeadingInfo(chunk, structured.Root, chunk.ChunkMeta.StartPos, chunk.ChunkMeta.EndPos)
		}
	}

	return chunks, nil
}

// GetStrategy returns the chunk strategy type
func (c *RecursiveChunker) GetStrategy() core.ChunkStrategy {
	return StrategyRecursive
}

// recursiveSplit recursively splits text by separators
func (c *RecursiveChunker) recursiveSplit(
	text string,
	startPos, endPos int,
	sepIndex int,
) []*core.Chunk {
	textLen := len(text)

	// 如果文本已经足够小，直接返回
	if textLen <= c.chunkSize {
		return []*core.Chunk{
			{
				Content: text,
				ChunkMeta: core.ChunkMeta{
					StartPos: startPos,
					EndPos:   startPos + textLen,
				},
			},
		}
	}

	// 如果已经尝试完所有分隔符，按字符切分
	if sepIndex >= len(c.separators) {
		return c.splitByChar(text, startPos)
	}

	separator := c.separators[sepIndex]

	// 空字符串表示按字符切分
	if separator == "" {
		return c.splitByChar(text, startPos)
	}

	// 按当前分隔符分割
	parts := strings.Split(text, separator)
	var chunks []*core.Chunk
	currentPos := startPos
	currentText := ""
	currentStartPos := startPos

	for _, part := range parts {
		// 跳过空部分
		if part == "" {
			continue
		}

		// 如果当前部分本身就大于块大小，递归使用下一级分隔符
		if len(part) > c.chunkSize {
			// 先保存当前累积的文本
			if currentText != "" {
				chunks = append(chunks, &core.Chunk{
					Content: currentText,
					ChunkMeta: core.ChunkMeta{
						StartPos: currentStartPos,
						EndPos:   currentStartPos + len(currentText),
					},
				})
				currentText = ""
			}

			// 递归处理过大的部分
			subChunks := c.recursiveSplit(part, currentPos, currentPos+len(part), sepIndex+1)
			chunks = append(chunks, subChunks...)
		} else {
			// 尝试累积到当前块
			testText := currentText
			if testText != "" {
				testText += separator + part
			} else {
				testText = part
			}

			// 如果未超过块大小，继续累积
			if len(testText) <= c.chunkSize {
				if currentText == "" {
					currentStartPos = currentPos
				}
				currentText = testText
			} else {
				// 超过块大小，保存当前块，开始新块
				if currentText != "" {
					chunks = append(chunks, &core.Chunk{
						Content: currentText,
						ChunkMeta: core.ChunkMeta{
							StartPos: currentStartPos,
							EndPos:   currentStartPos + len(currentText),
						},
					})
				}
				currentText = part
				currentStartPos = currentPos
			}
		}

		currentPos += len(part) + len(separator)
	}

	// 处理最后一个块
	if currentText != "" {
		// 如果最后一个块太小，尝试合并到前一个块
		if len(currentText) < c.minChunkSize && len(chunks) > 0 {
			lastChunk := chunks[len(chunks)-1]
			mergedText := lastChunk.Content + separator + currentText
			if len(mergedText) <= c.chunkSize {
				lastChunk.Content = mergedText
				lastChunk.ChunkMeta.EndPos = currentStartPos + len(currentText)
			} else {
				// 无法合并，创建新块
				chunks = append(chunks, &core.Chunk{
					Content: currentText,
					ChunkMeta: core.ChunkMeta{
						StartPos: currentStartPos,
						EndPos:   currentStartPos + len(currentText),
					},
				})
			}
		} else {
			chunks = append(chunks, &core.Chunk{
				Content: currentText,
				ChunkMeta: core.ChunkMeta{
					StartPos: currentStartPos,
					EndPos:   currentStartPos + len(currentText),
				},
			})
		}
	}

	return chunks
}

// splitByChar splits by character as last resort
func (c *RecursiveChunker) splitByChar(text string, startPos int) []*core.Chunk {
	var chunks []*core.Chunk
	textLen := len(text)
	step := c.chunkSize - c.options.Overlap
	if step <= 0 {
		step = c.chunkSize
	}

	index := 0
	for start := 0; start < textLen; start += step {
		end := start + c.chunkSize
		if end > textLen {
			end = textLen
		}

		chunks = append(chunks, &core.Chunk{
			Content: text[start:end],
			ChunkMeta: core.ChunkMeta{
				StartPos: startPos + start,
				EndPos:   startPos + end,
			},
		})

		index++
		if end >= textLen {
			break
		}
	}

	return chunks
}

// extractHeadingInfo extracts heading information from the structure tree
func (c *RecursiveChunker) extractHeadingInfo(
	chunk *core.Chunk,
	node *core.StructureNode,
	chunkStart, chunkEnd int,
) {
	if node == nil {
		return
	}

	// 检查当前节点是否包含块的位置
	if node.StartPos <= chunkStart && node.EndPos >= chunkEnd {
		// 如果是标题节点，更新标题信息
		if node.NodeType == "heading" {
			chunk.ChunkMeta.HeadingLevel = node.Level
			if len(chunk.ChunkMeta.HeadingPath) == 0 {
				chunk.ChunkMeta.HeadingPath = []string{node.Title}
			} else {
				// 检查是否已包含该标题
				found := false
				for _, h := range chunk.ChunkMeta.HeadingPath {
					if h == node.Title {
						found = true
						break
					}
				}
				if !found {
					chunk.ChunkMeta.HeadingPath = append(chunk.ChunkMeta.HeadingPath, node.Title)
				}
			}
		}
	}

	// 递归检查子节点
	for _, child := range node.Children {
		if child.StartPos <= chunkStart && child.EndPos >= chunkEnd {
			c.extractHeadingInfo(chunk, child, chunkStart, chunkEnd)
			break
		}
	}
}
