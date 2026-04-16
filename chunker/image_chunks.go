package chunker

import (
	"encoding/base64"

	"github.com/DotNetAge/gorag/core"
)

// ExtractImageChunks 从 structured 文档中提取图片子分块
// 图片作为子分块关联到主文档，通过 ParentID 实现文档级别召回
func ExtractImageChunks(structured *core.StructuredDocument) []*core.Chunk {
	if structured == nil || structured.RawDoc == nil {
		return nil
	}

	images := structured.RawDoc.GetImages()
	if len(images) == 0 {
		return nil
	}

	chunks := make([]*core.Chunk, 0, len(images))
	for i, img := range images {
		chunks = append(chunks, &core.Chunk{
			ID:        GenerateChunkID(structured.RawDoc.GetID(), i, ""),
			ParentID:  structured.RawDoc.GetID(), // 关联到主文档
			DocID:     structured.RawDoc.GetID(),
			MIMEType:  "image/jpeg", // 缩略图统一为 JPEG
			Content:   base64.StdEncoding.EncodeToString(img.Data()),
			Metadata:  map[string]any{
				"image_index": i,
			},
		})
	}

	return chunks
}
