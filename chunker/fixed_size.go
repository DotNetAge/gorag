package chunker

import (
	"github.com/DotNetAge/gorag/core"
)

// FixedSizeChunker splits text into fixed-size chunks
// Simple and fast, splits by character count
type FixedSizeChunker struct {
	chunkSize int     // 块大小（字符数）
	overlap   int     // 重叠大小（字符数）
	options   Options // 可选配置
}

// NewFixedSizeChunker creates a new FixedSizeChunker
func NewFixedSizeChunker(opts ...Option) *FixedSizeChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &FixedSizeChunker{
		chunkSize: Clamp(options.ChunkSize, MinChunkSize, MaxChunkSize),
		overlap:   Clamp(options.Overlap, 0, options.ChunkSize/2),
		options:   options,
	}
}

// Chunk implements the Chunker interface
func (c *FixedSizeChunker) Chunk(
	structured *core.StructuredDocument,
) ([]*core.Chunk, error) {
	if structured == nil || structured.RawDoc == nil {
		return []*core.Chunk{}, nil
	}

	text := structured.RawDoc.GetContent()
	if text == "" {
		return []*core.Chunk{}, nil
	}

	textLen := len(text)

	// 如果文本小于块大小，直接返回单个块
	if textLen <= c.chunkSize {
		return []*core.Chunk{
			c.createChunk(structured, 0, 0, textLen, text),
		}, nil
	}

	// 计算步长（块大小 - 重叠）
	step := c.chunkSize - c.overlap
	if step <= 0 {
		step = c.chunkSize
	}

	var chunks []*core.Chunk
	index := 0

	// 滑动窗口切分
	for start := 0; start < textLen; start += step {
		end := start + c.chunkSize
		if end > textLen {
			end = textLen
		}

		content := text[start:end]
		chunk := c.createChunk(structured, index, start, end, content)
		chunks = append(chunks, chunk)

		index++

		// 如果已经到达文本末尾，退出
		if end >= textLen {
			break
		}
	}

	// Append image chunks as sub-chunks
	if imgChunks := ExtractImageChunks(structured); len(imgChunks) > 0 {
		chunks = append(chunks, imgChunks...)
	}

	return chunks, nil
}

// GetStrategy returns the chunk strategy type
func (c *FixedSizeChunker) GetStrategy() core.ChunkStrategy {
	return StrategyFixedSize
}

// createChunk creates a Chunk object
func (c *FixedSizeChunker) createChunk(
	structured *core.StructuredDocument,
	index int,
	startPos, endPos int,
	content string,
) *core.Chunk {
	rawDoc := structured.RawDoc
	return &core.Chunk{
		ID:       GenerateChunkID(rawDoc.GetID(), index, content),
		ParentID: "", // FixedSize 不使用父子关系
		DocID:    rawDoc.GetID(),
		MIMEType: rawDoc.GetMimeType(),
		Content:  content,
		Metadata: rawDoc.GetMeta(),
		ChunkMeta: core.ChunkMeta{
			Index:        index,
			StartPos:     startPos,
			EndPos:       endPos,
			HeadingLevel: 0, // FixedSize 不识别标题层级
			HeadingPath:  []string{},
		},
	}
}
