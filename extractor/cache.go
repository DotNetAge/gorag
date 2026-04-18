package extractor

import (
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/utils"
)

// Extraction 原始提取结果，与特定 chunk/doc ID 无关
// 缓存的是"这段文字包含哪些实体和关系"
type Extraction struct {
	Entities  []ExtractedEntity  `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// ExtractedEntity 提取的实体
type ExtractedEntity struct {
	Name       string `json:"name"`
	EntityType string `json:"entity_type"`
}

// ExtractedRelation 提取的关系
type ExtractedRelation struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// ContentHash FNV-1a 哈希，将内容映射为 16 字符的十六进制 key
func ContentHash(content string) string {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
		hex      = "0123456789abcdef"
	)

	h := uint64(offset64)
	for i := 0; i < len(content); i++ {
		h ^= uint64(content[i])
		h *= prime64
	}

	result := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		result[i] = hex[h&0xf]
		h >>= 4
	}
	return string(result)
}

// GetCachedExtraction 从 CacheStore 中查询实体提取缓存
// 返回 nil 表示缓存未命中
func GetCachedExtraction(store core.CacheStore, hashKey string) (*Extraction, error) {
	if store == nil {
		return nil, nil
	}
	var ext Extraction
	if err := store.Get(hashKey, &ext); err != nil {
		return nil, nil
	}
	if len(ext.Entities) == 0 && len(ext.Relations) == 0 {
		return nil, nil
	}
	return &ext, nil
}

// SetCachedExtraction 将实体提取结果写入 CacheStore
func SetCachedExtraction(store core.CacheStore, hashKey string, ext *Extraction) error {
	if store == nil || ext == nil {
		return nil
	}
	return store.Set(hashKey, ext)
}

// BuildFromExtraction 将缓存的原始提取结果绑定到当前 chunk 的 ID/DocID
func BuildFromExtraction(ext *Extraction, chunk *core.Chunk) ([]core.Node, []core.Edge) {
	entityMap := make(map[string]*core.Node)
	nodes := make([]core.Node, 0, len(ext.Entities))

	for _, ent := range ext.Entities {
		if ent.Name == "" {
			continue
		}
		if _, exists := entityMap[ent.Name]; exists {
			continue
		}

		node := core.Node{
			ID:   utils.GenerateID([]byte(ent.Name + chunk.DocID)),
			Type: ent.EntityType,
			Name: ent.Name,
			Properties: map[string]any{
				"confidence": 0.9,
			},
			SourceChunkIDs: []string{chunk.ID},
			SourceDocIDs:   []string{chunk.DocID},
		}
		entityMap[ent.Name] = &node
		nodes = append(nodes, node)
	}

	edges := make([]core.Edge, 0, len(ext.Relations))
	for _, rel := range ext.Relations {
		subjectNode, hasSubject := entityMap[rel.Subject]
		objectNode, hasObject := entityMap[rel.Object]
		if !hasSubject || !hasObject {
			continue
		}

		edge := core.Edge{
			ID:        utils.GenerateID([]byte(rel.Subject + rel.Predicate + rel.Object + chunk.DocID)),
			Type:      rel.Predicate,
			Source:    subjectNode.ID,
			Target:    objectNode.ID,
			Predicate: rel.Predicate,
			Properties: map[string]any{
				"confidence": 0.9,
			},
			SourceChunkIDs: []string{chunk.ID},
			SourceDocIDs:   []string{chunk.DocID},
		}
		edges = append(edges, edge)
	}

	return nodes, edges
}
