package indexer

import "fmt"

// EntityDef 定义一个实体类型的 Prompt 与 Schema，供 WithSchemas 注入 LLM Prompt。
// Prompt 写入 ### Entity Types 段，Schema 写入 ### Entity Schema 段。
// LLM 在 output 中使用 Prompt 中的类型名作为 entity.type，该 type 最终成为
// core.Node.Labels[0]（即 gograph 的 node Label），可用 MATCH (n:TypeName) 查询。
type EntityDef struct {
	Prompt string // 实体类型描述文本（如 "**Person** — author, expert, contributor"）
	Schema string // JSON Schema 文本（如 `{"type":"object","properties":{...}}`），可选
}

// ModelConfig LLM 模型连接配置
type ModelConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	Language       string // 内容语言英文名（如 "Chinese", "English", "Japanese"），直接注入提示词
	MaxTokens      int    // 模型的最大输出 token 数，也用于分页预算计算，0 表示使用默认值 128000
	ContextLength  int    // 模型的总上下文长度（token 数），0 表示使用默认值 defaultMaxTokens
	ThinkingBudget int    // 思考模式的 token 预算（0 = 模型默认值），GraphIndexer 始终启用思考模式
}

// TokenUsage 单次 LLM 调用的 Token 消耗
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// IndexData LLM 返回的顶层 JSON 结构。
// 字段设计对齐 core.Chunk / core.Node / core.Edge，减少后处理映射。
// ID 字段使用序数（1, 2, 3...），由 writeToStores 解析为真实存储 ID。
type IndexData struct {
	Chunks []struct {
		Content   string         `json:"content"`
		Metadata  map[string]any `json:"metadata,omitempty"`
		ChunkMeta struct {
			Positions [][2]int `json:"positions"`
		} `json:"chunk_meta,omitempty"`
	} `json:"chunks"`
	Entities []struct {
		ID         int            `json:"id"`
		Type       string         `json:"type"`
		Name       string         `json:"name"`
		Properties map[string]any `json:"properties,omitempty"`
	} `json:"entities"`
	Relations []struct {
		Source     int            `json:"source"`
		Target     int            `json:"target"`
		Type       string         `json:"type"`
		Predicate  string         `json:"predicate,omitempty"`
		Properties map[string]any `json:"properties,omitempty"`
	} `json:"relations"`
}

// mergeIndexData 合并多次 LLM 调用的结果。
// 每个切片内的 ID 是独立序数（1, 2, 3...），合并时 rebase 为全局连续 ID，
// 同时更新所有 cross-reference（chunk.entity_ids、relation.source/target）。
//
// 去重策略：
//   - Entities：不主动去重，graphDB.UpsertNodes 基于 SHA NodeID 天然去重
//   - Relations：按 Source|Type|Target 去重，避免重复边写入
func mergeIndexData(datas ...*IndexData) *IndexData {
	merged := &IndexData{}
	nextID := 1 // 全局序数 ID 起点
	seenChunkContent := make(map[string]struct{}) // Chunk 内容去重

	for _, d := range datas {
		if d == nil {
			continue
		}

		// 1. 建立本切片的旧ID→新ID映射
		reID := make(map[int]int, len(d.Entities))
		for j := range d.Entities {
			reID[d.Entities[j].ID] = nextID
			d.Entities[j].ID = nextID
			nextID++
		}

		// 2. 更新 chunk.entity_ids
		for j := range d.Chunks {
			for k := range d.Chunks[j].Metadata {
				// entity_ids 存储在 metadata 中，类型为 []any
				if k == "entity_ids" {
					if ids, ok := d.Chunks[j].Metadata["entity_ids"].([]any); ok {
						newIDs := make([]any, len(ids))
						for idx, oldID := range ids {
							if oldInt, ok := oldID.(float64); ok {
								if newID, ok2 := reID[int(oldInt)]; ok2 {
									newIDs[idx] = newID
								}
							}
						}
						d.Chunks[j].Metadata["entity_ids"] = newIDs
					}
				}
			}
		}

		// 3. 更新 relation.source/target
		for j := range d.Relations {
			if newSrc, ok := reID[d.Relations[j].Source]; ok {
				d.Relations[j].Source = newSrc
			}
			if newTgt, ok := reID[d.Relations[j].Target]; ok {
				d.Relations[j].Target = newTgt
			}
		}

		// 4. 追加 Chunk（按内容去重：相邻切片可能在切点处产生相似 chunk）
		for _, chunk := range d.Chunks {
			if _, ok := seenChunkContent[chunk.Content]; ok {
				continue
			}
			seenChunkContent[chunk.Content] = struct{}{}
			merged.Chunks = append(merged.Chunks, chunk)
		}
		merged.Entities = append(merged.Entities, d.Entities...)
		merged.Relations = append(merged.Relations, d.Relations...)
	}

	merged.Relations = dedupRelations(merged.Relations)
	return merged
}

func dedupRelations(relations []struct {
	Source     int            `json:"source"`
	Target     int            `json:"target"`
	Type       string         `json:"type"`
	Predicate  string         `json:"predicate,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}) []struct {
	Source     int            `json:"source"`
	Target     int            `json:"target"`
	Type       string         `json:"type"`
	Predicate  string         `json:"predicate,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
} {
	seen := make(map[string]bool, len(relations))
	deduped := make([]struct {
		Source     int            `json:"source"`
		Target     int            `json:"target"`
		Type       string         `json:"type"`
		Predicate  string         `json:"predicate,omitempty"`
		Properties map[string]any `json:"properties,omitempty"`
	}, 0, len(relations))
	for _, r := range relations {
		// 使用序数 ID 构建唯一键（跨切片已 rebase，不会误判）
		key := fmt.Sprintf("%d|%s|%d", r.Source, r.Type, r.Target)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, r)
	}
	return deduped
}
