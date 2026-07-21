package chunker

import (
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
)

// ImageChunker 图片分块器：整图作为一个 Chunk。
//
// 适用 RawDocType：RawDocImage（jpg/png/gif 等，content 已为 Base64）。
// 设计要点：
//   - 整个图片作为一个 Chunk（content 已是 Base64）
//   - Chunk.Title 取文件名（去扩展名）
//   - Chunk.StartPos=0, EndPos=len(content)
//   - Chunk.Summary 留空（由 Extractor 填充）
//   - DocID 使用 doc.ID()，Chunk.Source=doc.FileName()（高频跨包属性提升为字段）
//   - 图片元数据（mime_type/thumbnail_size）保留在 Metadata 中，使用 core.Meta* 常量键名
type ImageChunker struct{}

// NewImageChunker 创建图片分块器。
func NewImageChunker() *ImageChunker { return &ImageChunker{} }

// Chunk 实现 Chunker 接口：整图作为一个 Chunk，不产出结构节点。
func (c *ImageChunker) Chunk(doc document.RawDoc) (ChunkResult, error) {
	if doc == nil {
		return ChunkResult{}, nil
	}

	content := doc.Content()
	if content == "" {
		return ChunkResult{}, nil
	}

	title := deriveTitle(doc.FileName())
	chunk := buildChunk(doc, 0, 0, len(content), title, content)
	// 回填 document 包已提取的图片元数据（使用 core.Meta* 常量键名）
	for k, v := range doc.Meta() {
		switch k {
		case core.MetaMimeType:
			if chunk.Metadata == nil {
				chunk.Metadata = map[string]any{}
			}
			chunk.Metadata[core.MetaMimeType] = v
		case core.MetaThumbnailSize:
			if chunk.Metadata == nil {
				chunk.Metadata = map[string]any{}
			}
			chunk.Metadata[core.MetaThumbnailSize] = v
		}
	}
	chunks := enrichChunksMetadata([]core.Chunk{chunk}, content, doc.FileName())
	return ChunkResult{Chunks: chunks}, nil
}
