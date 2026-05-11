package core

import (
	"sort"
	"strings"
)

// ChunkInfo 文档还原中的单一块信息
type ChunkInfo struct {
	ChunkID   string   `json:"chunk_id"`   // Chunk 唯一ID
	ParentID  string   `json:"parent_id"`  // 父Chunk/父文档ID
	Index     int      `json:"index"`      // 分块序号（0,1,2...）
	Content   string   `json:"content"`    // 分块内容
	StartPos  int      `json:"start_pos"`  // 在原始文本中的起始位置
	EndPos    int      `json:"end_pos"`    // 在原始文本中的结束位置
	Heading   string   `json:"heading"`    // 标题（最内层）
	HeadingPath []string `json:"heading_path"` // 标题路径
}

// ReconstructedDocument 从向量数据库碎片还原出的完整文档
type ReconstructedDocument struct {
	DocID  string      `json:"doc_id"`  // 原始文档ID
	Title  string      `json:"title"`   // 文档标题（从 Metadata 推测）
	Chunks []ChunkInfo `json:"chunks"`  // 所有分块（按 index 排序）
	Content string     `json:"content"` // 完整还原的文档内容
}

// ReconstructDocument 从向量碎片还原完整文档
// vectors 必须包含 metadata 中的 doc_id, content, chunk_meta 字段
func ReconstructDocument(vectors []*Vector) *ReconstructedDocument {
	if len(vectors) == 0 {
		return &ReconstructedDocument{}
	}

	// 提取 doc_id
	docID := ""
	title := ""
	for _, v := range vectors {
		if v == nil || v.Metadata == nil {
			continue
		}
		if d, ok := v.Metadata["doc_id"].(string); ok && d != "" {
			docID = d
		}
		// 从 metadata 中提取标题
		if title == "" {
			if t, ok := v.Metadata["title"].(string); ok && t != "" {
				title = t
			}
		}
		break
	}

	// 提取 ChunkInfo 并排序
	infos := make([]ChunkInfo, 0, len(vectors))
	for _, v := range vectors {
		if v == nil || v.Metadata == nil {
			continue
		}

		content, _ := v.Metadata["content"].(string)
		chunkID := v.ChunkID
		parentID, _ := v.Metadata["parent_id"].(string)

		// 提取 chunk_meta
		index := 0
		startPos := 0
		endPos := 0
		headingPath := []string{}

		if cm, ok := v.Metadata["chunk_meta"].(map[string]any); ok {
			if idx, ok := cm["index"].(float64); ok {
				index = int(idx)
			}
			if sp, ok := cm["start_pos"].(float64); ok {
				startPos = int(sp)
			}
			if ep, ok := cm["end_pos"].(float64); ok {
				endPos = int(ep)
			}
			if hp, ok := cm["heading_path"].([]any); ok {
				for _, h := range hp {
					if hs, ok := h.(string); ok {
						headingPath = append(headingPath, hs)
					}
				}
			}
		}

		heading := ""
		if len(headingPath) > 0 {
			heading = headingPath[len(headingPath)-1]
		}

		infos = append(infos, ChunkInfo{
			ChunkID:     chunkID,
			ParentID:    parentID,
			Index:       index,
			Content:     content,
			StartPos:    startPos,
			EndPos:      endPos,
			Heading:     heading,
			HeadingPath: headingPath,
		})
	}

	// 按 index 排序
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Index < infos[j].Index
	})

	// 拼接完整内容
	var sb strings.Builder
	for i, info := range infos {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		// 如果有标题，在内容前插入标题
		if info.Heading != "" {
			sb.WriteString("【" + info.Heading + "】")
			sb.WriteString("\n")
		}
		sb.WriteString(info.Content)
	}

	return &ReconstructedDocument{
		DocID:   docID,
		Title:   title,
		Chunks:  infos,
		Content: sb.String(),
	}
}
