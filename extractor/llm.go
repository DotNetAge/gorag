package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	gochatcore "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/utils"
)

// ExtractNodesAndEdges 从 Chunk 中提取 Node 和 Edge
// 使用 LLM 进行实体和关系抽取，直接返回 GraphRAG 原语结构
func ExtractNodesAndEdges(ctx context.Context, client gochatcore.Client, chunk *core.Chunk) ([]core.Node, []core.Edge, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("client is required")
	}
	if chunk == nil || chunk.Content == "" {
		return nil, nil, nil
	}

	// 构建 LLM 提示
	prompt := `You are an expert entity extraction specialist. Extract entities and relationships from the given text.
		
IMPORTANT: You MUST output in the same language as the user query.

## Tasks
1. Identify named entities (person names, locations, organizations, terms, etc.)
2. Identify relationships between entities

## Output Format
Output strictly in the following JSON format, no other content:
{
  "entities": [
    {
      "name": "entity name",
      "entity_type": "entity type (PERSON/LOCATION/ORGANIZATION/TECHNOLOGY/PRODUCT/EVENT/OTHER)"
    }
  ],
  "relations": [
    {
      "subject": "subject entity name",
      "predicate": "relationship type (e.g., works_at, belongs_to, contains, develops, creates)",
      "object": "object entity name"
    }
  ]
}

## Notes
- Only extract entities explicitly mentioned in the text, do not infer
- Use the exact text from the original for entity names
- Subject and object of relations must be extracted entities
- Return empty arrays if no entities or relations found`

	messages := []gochatcore.Message{
		gochatcore.NewSystemMessage(prompt),
		gochatcore.NewUserMessage(chunk.Content),
	}

	// 调用 LLM
	response, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// 解析 JSON 响应
	return parseResponse(response.Content, chunk)
}

// DefaultBatchSize 默认批处理大小，每次 LLM 调用处理的 chunk 数量
const DefaultBatchSize = 5

// ExtractBatch 批量从多个 Chunk 中提取 Node 和 Edge
// 将多个 chunk 合并到一次 LLM 调用中，显著降低 LLM 调用次数
// chunks 会被按 batchSize 分组，每组调用一次 LLM
// 如果提供了 cache，会优先从缓存中获取已提取的结果，仅对缓存未命中的 chunk 调用 LLM
func ExtractBatch(ctx context.Context, client gochatcore.Client, chunks []*core.Chunk, batchSize ...int) ([]core.Node, []core.Edge, error) {
	return ExtractBatchWithCache(ctx, client, chunks, nil, batchSize...)
}

// ExtractBatchWithCache 带缓存的批量提取
// cache 可以为 nil，此时行为与 ExtractBatch 相同
func ExtractBatchWithCache(ctx context.Context, client gochatcore.Client, chunks []*core.Chunk, cache core.CacheStore, batchSize ...int) ([]core.Node, []core.Edge, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("client is required")
	}

	size := DefaultBatchSize
	if len(batchSize) > 0 && batchSize[0] > 0 {
		size = batchSize[0]
	}

	var allNodes []core.Node
	var allEdges []core.Edge

	// 阶段一：缓存查找，分离命中和未命中的 chunk
	cachedIndices := make(map[int]bool)
	if cache != nil {
		for i, chunk := range chunks {
			if chunk == nil || chunk.Content == "" {
				cachedIndices[i] = true
				continue
			}
			hashKey := ContentHash(chunk.Content)
			entities, _ := GetCachedExtraction(cache, hashKey)
			if entities != nil {
				// 缓存命中：用缓存的原始提取结果 + 当前 chunk 的 ID 重建 Node/Edge
				nodes, edges := BuildFromExtraction(entities, chunk)
				allNodes = append(allNodes, nodes...)
				allEdges = append(allEdges, edges...)
				cachedIndices[i] = true
			}
		}
	}

	// 阶段二：收集未命中的 chunk，按批次调用 LLM
	uncached := make([]*core.Chunk, 0, len(chunks))
	for i, chunk := range chunks {
		if !cachedIndices[i] {
			uncached = append(uncached, chunk)
		}
	}

	if len(uncached) > 0 {
		for i := 0; i < len(uncached); i += size {
		end := min(i+size, len(uncached))
			batch := uncached[i:end]

			nodes, edges, extractions, err := extractBatchCall(ctx, client, batch, i)
			if err != nil {
				return nil, nil, err
			}

			// 按 chunk 内容哈希写入缓存
			if cache != nil && extractions != nil {
				for batchIdx, chunk := range batch {
					if ext, ok := extractions[batchIdx]; ok && chunk != nil && chunk.Content != "" {
						hashKey := ContentHash(chunk.Content)
						_ = SetCachedExtraction(cache, hashKey, ext)
					}
				}
			}

			allNodes = append(allNodes, nodes...)
			allEdges = append(allEdges, edges...)
		}
	}

	return allNodes, allEdges, nil
}

// extractBatchCall 单次批量 LLM 调用，处理一组 chunk
// offset 是这批 chunk 在原始列表中的起始偏移，用于计算全局 chunk_index
func extractBatchCall(ctx context.Context, client gochatcore.Client, batch []*core.Chunk, offset int) ([]core.Node, []core.Edge, map[int]*Extraction, error) {
	// 过滤空 chunk
	validChunks := make([]*core.Chunk, 0, len(batch))
	for _, c := range batch {
		if c != nil && c.Content != "" {
			validChunks = append(validChunks, c)
		}
	}
	if len(validChunks) == 0 {
		return nil, nil, nil, nil
	}

	// 构建 chunk 编号文本
	var sb strings.Builder
	sb.WriteString("Below are multiple text segments from the same or related documents. Extract entities and relationships from EACH segment separately.\n\n")
	for i, chunk := range validChunks {
		globalIndex := offset + i
		fmt.Fprintf(&sb, "--- Segment %d ---\n%s\n\n", globalIndex, chunk.Content)
	}

	prompt := `You are an expert entity extraction specialist. Extract entities and relationships from the given text segments.

IMPORTANT: You MUST output in the same language as the user query.

## Tasks
1. Identify named entities (person names, locations, organizations, terms, etc.) from EACH segment
2. Identify relationships between entities from EACH segment
3. Each entity and relation MUST include a "chunk_index" field indicating which segment it belongs to

## Output Format
Output strictly in the following JSON format, no other content:
{
  "entities": [
    {
      "name": "entity name",
      "entity_type": "entity type (PERSON/LOCATION/ORGANIZATION/TECHNOLOGY/PRODUCT/EVENT/OTHER)",
      "chunk_index": 0
    }
  ],
  "relations": [
    {
      "subject": "subject entity name",
      "predicate": "relationship type (e.g., works_at, belongs_to, contains, develops, creates)",
      "object": "object entity name",
      "chunk_index": 0
    }
  ]
}

## Notes
- Only extract entities explicitly mentioned in the text, do not infer
- Use the exact text from the original for entity names
- Subject and object of relations must be extracted entities
- The "chunk_index" must match the segment number (e.g., "Segment 0" means chunk_index=0)
- An entity can appear in multiple segments if mentioned in multiple segments
- Return empty arrays if no entities or relations found`

	messages := []gochatcore.Message{
		gochatcore.NewSystemMessage(prompt),
		gochatcore.NewUserMessage(sb.String()),
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("LLM batch extraction failed: %w", err)
	}

	return parseBatchResponse(response.Content, validChunks, offset)
}

// batchLLMResponse 批量 LLM 返回的 JSON 结构
type batchLLMResponse struct {
	Entities  []batchLLMEntity   `json:"entities"`
	Relations []batchLLMRelation `json:"relations"`
}

type batchLLMEntity struct {
	Name       string `json:"name"`
	EntityType string `json:"entity_type"`
	ChunkIndex int    `json:"chunk_index"`
}

type batchLLMRelation struct {
	Subject    string `json:"subject"`
	Predicate  string `json:"predicate"`
	Object     string `json:"object"`
	ChunkIndex int    `json:"chunk_index"`
}

// parseBatchResponse 解析批量 LLM 响应，根据 chunk_index 将实体分配到对应 chunk
// 同时返回 perChunkExtractions：每个 chunk 对应的原始提取结果（用于缓存写入）
func parseBatchResponse(content string, chunks []*core.Chunk, offset int) ([]core.Node, []core.Edge, map[int]*Extraction, error) {
	jsonStr := extractJSON(content)

	var resp batchLLMResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse LLM batch response: %w", err)
	}

	// 按 chunk_index 分组收集实体和原始提取结果
	chunkEntityMaps := make(map[int]map[string]*core.Node)
	perChunkExtractions := make(map[int]*Extraction)
	var allNodes []core.Node

	for _, ent := range resp.Entities {
		if ent.Name == "" {
			continue
		}
		idx := ent.ChunkIndex - offset
		if idx < 0 || idx >= len(chunks) {
			idx = 0
		}
		chunk := chunks[idx]

		if chunkEntityMaps[idx] == nil {
			chunkEntityMaps[idx] = make(map[string]*core.Node)
			perChunkExtractions[idx] = &Extraction{}
		}

		// 收集原始提取结果（用于缓存）
		perChunkExtractions[idx].Entities = append(perChunkExtractions[idx].Entities, ExtractedEntity{
			Name:       ent.Name,
			EntityType: ent.EntityType,
		})

		// 同一 chunk 内去重
		if _, exists := chunkEntityMaps[idx][ent.Name]; exists {
			node := chunkEntityMaps[idx][ent.Name]
			node.SourceChunkIDs = appendUnique(node.SourceChunkIDs, chunk.ID)
			continue
		}

		node := &core.Node{
			ID:   utils.GenerateID([]byte(ent.Name + chunk.DocID)),
			Type: ent.EntityType,
			Name: ent.Name,
			Properties: map[string]any{
				"confidence": 0.9,
			},
			SourceChunkIDs: []string{chunk.ID},
			SourceDocIDs:   []string{chunk.DocID},
		}
		chunkEntityMaps[idx][ent.Name] = node
		allNodes = append(allNodes, *node)
	}

	// 构建关系
	var allEdges []core.Edge
	for _, rel := range resp.Relations {
		idx := rel.ChunkIndex - offset
		if idx < 0 || idx >= len(chunks) {
			idx = 0
		}
		chunk := chunks[idx]
		entityMap := chunkEntityMaps[idx]

		subjectNode, hasSubject := entityMap[rel.Subject]
		objectNode, hasObject := entityMap[rel.Object]

		if !hasSubject || !hasObject {
			continue
		}

		// 收集原始关系（用于缓存）
		if perChunkExtractions[idx] != nil {
			perChunkExtractions[idx].Relations = append(perChunkExtractions[idx].Relations, ExtractedRelation{
				Subject:   rel.Subject,
				Predicate: rel.Predicate,
				Object:    rel.Object,
			})
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
		allEdges = append(allEdges, edge)
	}

	return allNodes, allEdges, perChunkExtractions, nil
}

// appendUnique 向切片追加不重复的元素
func appendUnique(slice []string, item string) []string {
	if slices.Contains(slice, item) {
		return slice
	}
	return append(slice, item)
}

// llmResponse LLM 返回的 JSON 结构（单 chunk 模式）
type llmResponse struct {
	Entities  []llmEntity   `json:"entities"`
	Relations []llmRelation `json:"relations"`
}

type llmEntity struct {
	Name       string `json:"name"`
	EntityType string `json:"entity_type"`
}

type llmRelation struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// parseResponse 解析 LLM 响应并转换为 Node 和 Edge
func parseResponse(content string, chunk *core.Chunk) ([]core.Node, []core.Edge, error) {
	// 提取 JSON 部分（可能在 markdown 代码块中）
	jsonStr := extractJSON(content)

	var resp llmResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// 构建实体映射（名称 -> Node）
	entityMap := make(map[string]*core.Node)
	nodes := make([]core.Node, 0, len(resp.Entities))

	for _, ent := range resp.Entities {
		if ent.Name == "" {
			continue
		}

		// 检查是否已存在（去重）
		if _, exists := entityMap[ent.Name]; exists {
			continue
		}

		// 创建 Node
		node := core.Node{
			ID:   utils.GenerateID([]byte(ent.Name + chunk.DocID)),
			Type: ent.EntityType,
			Name: ent.Name,
			Properties: map[string]any{
				"confidence": 0.9, // LLM 抽取默认置信度
			},
			SourceChunkIDs: []string{chunk.ID},
			SourceDocIDs:   []string{chunk.DocID},
		}

		entityMap[ent.Name] = &node
		nodes = append(nodes, node)
	}

	// 构建关系（Edge）
	edges := make([]core.Edge, 0, len(resp.Relations))
	for _, rel := range resp.Relations {
		subjectNode, hasSubject := entityMap[rel.Subject]
		objectNode, hasObject := entityMap[rel.Object]

		if !hasSubject || !hasObject {
			continue
		}

		// 创建 Edge
		edge := core.Edge{
			ID:        utils.GenerateID([]byte(rel.Subject + rel.Predicate + rel.Object + chunk.DocID)),
			Type:      rel.Predicate,
			Source:    subjectNode.ID,
			Target:    objectNode.ID,
			Predicate: rel.Predicate,
			Properties: map[string]any{
				"confidence": 0.9, // LLM 抽取默认置信度
			},
			SourceChunkIDs: []string{chunk.ID},
			SourceDocIDs:   []string{chunk.DocID},
		}

		edges = append(edges, edge)
	}

	return nodes, edges, nil
}

// extractJSON 从可能包含 markdown 代码块的响应中提取 JSON
func extractJSON(content string) string {
	// 尝试直接解析
	var v any
	if err := json.Unmarshal([]byte(content), &v); err == nil {
		return content
	}

	// 尝试从 markdown 代码块中提取
	start := 0
	if idx := indexOf(content, "```json"); idx >= 0 {
		start = idx + 7
	} else if idx := indexOf(content, "```"); idx >= 0 {
		start = idx + 3
	}

	end := len(content)
	if idx := indexOf(content[start:], "```"); idx >= 0 {
		end = start + idx
	}

	return content[start:end]
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
