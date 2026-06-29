package indexer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/gochat/client/openai"
	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/utils"
)

// RegionIndexer 对一个已索引目录生成 Region 级聚合摘要并写入 VectorStore。
//
// Region 是"摘要的摘要"——它的聚合输入只包含该目录下两类摘要级向量：
//
//  1. 本层文件的 Root Chunk（chunk_type = "root"）：文件级摘要
//  2. 子目录的子 Region（region = true）：子目录级摘要
//
// 普通分段（chunk_type = "segment"）不参与 Region 聚合，确保 Region
// 只做层级抽象而不被细节污染。
//
// 时序要求：IndexRegion 必须在目标目录下所有文件的 Chunk 已写入 VectorStore
// 之后调用。调用方（通常是编排层）负责确保时序正确。
type RegionIndexer struct {
	model    ModelConfig
	embedder core.Embedder
	vectorDB core.VectorStore
	graphDB  core.GraphStore
	logger   logging.Logger
}

// NewRegionIndexer 创建 RegionIndexer。
func NewRegionIndexer(
	model ModelConfig,
	embedder core.Embedder,
	vectorDB core.VectorStore,
	opts ...RegionOption,
) *RegionIndexer {
	r := &RegionIndexer{
		model:    model,
		embedder: embedder,
		vectorDB: vectorDB,
		logger:   logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RegionOption 配置 RegionIndexer 的可选参数。
type RegionOption func(*RegionIndexer)

// RegionWithLogger 为 RegionIndexer 附加日志记录器。
func RegionWithLogger(logger logging.Logger) RegionOption {
	return func(r *RegionIndexer) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// RegionWithGraphStore 为 RegionIndexer 附加图数据库，用于创建 Region Node 和边。
func RegionWithGraphStore(gs core.GraphStore) RegionOption {
	return func(r *RegionIndexer) {
		r.graphDB = gs
	}
}

// IndexRegion 对指定目录生成 Region 聚合摘要并写入 VectorStore。
//
// 聚合过程：
//
//	1. 通过 region_id 过滤查询 VectorStore
//	2. 只取摘要级向量：
//	   - chunk_type = "root"  → 文件级文档摘要
//	   - region = true        → 子目录级 Region 摘要
//	   - chunk_type = "segment" → 忽略（不入聚合）
//	3. 用 LLM 对摘要做二次抽象（摘要的摘要）
//	4. 向量化后写回 VectorStore（标记为 region 类型向量）
func (r *RegionIndexer) IndexRegion(ctx context.Context, dir string) (*core.Region, error) {
	regionID := regionIDFromDir(dir)
	title := filepath.Base(dir)

	r.logger.Info("region: indexing", "dir", dir, "region_id", regionID)

	// 1. 查询该 Region 下的全部向量
	vectors, err := r.queryRegionVectors(ctx, regionID)
	if err != nil {
		return nil, fmt.Errorf("region: query vectors: %w", err)
	}

	if len(vectors) == 0 {
		r.logger.Warn("region: no vectors found, creating empty region",
			"dir", dir, "region_id", regionID)
		return r.writeRegion(ctx, regionID, title, title, nil, dir,
			nil, nil)
	}

	// 2. 分离摘要级向量和 tags
	//    只取两类：文档级 Root Chunk + 子 Region
	var abstractSummaries []string
	var allTags []string
	var docCount, subRegionCount int
	var docIDs []string
	var childRegionIDs []string

	for _, v := range vectors {
		// ── 子 Region 摘要 ──
		if isRegion, _ := v.Metadata["region"].(bool); isRegion {
			if s, ok := v.Metadata["summary"].(string); ok && s != "" {
				subTitle, _ := v.Metadata["title"].(string)
				abstractSummaries = append(abstractSummaries,
					fmt.Sprintf("[Sub-directory: %s] %s", subTitle, s))
				subRegionCount++
			}
			if crID, ok := v.Metadata["region_id"].(string); ok {
				childRegionIDs = append(childRegionIDs, crID)
			}
			collectTagsFromVector(v, &allTags)
			continue
		}

		// ── 文档级 Root Chunk 摘要 ──
		if ct, _ := v.Metadata["chunk_type"].(string); ct == "root" {
			if s, ok := v.Metadata["summary"].(string); ok && s != "" {
				abstractSummaries = append(abstractSummaries, s)
				docCount++
			}
			if dID, ok := v.Metadata["doc_id"].(string); ok {
				docIDs = append(docIDs, dID)
			}
			collectTagsFromVector(v, &allTags)
			continue
		}

		// ── 普通分段 chunk (chunk_type = "segment") → 忽略 ──
	}

	if len(abstractSummaries) == 0 {
		r.logger.Warn("region: no abstract-level vectors found",
			"dir", dir, "total_vectors", len(vectors))
		return r.writeRegion(ctx, regionID, title, "", allTags, dir,
			docIDs, childRegionIDs)
	}

	// 3. LLM 聚合（输入全是摘要级内容）
	summary := r.aggregateSummary(ctx, title, abstractSummaries, docCount, subRegionCount)

	r.logger.Info("region: completed",
		"dir", dir,
		"doc_summaries", docCount,
		"sub_regions", subRegionCount,
		"tags", len(allTags),
		"summary_len", len(summary))

	return r.writeRegion(ctx, regionID, title, summary, allTags, dir,
		docIDs, childRegionIDs)
}

// queryRegionVectors 通过 metadata filter 查询指定 Region 下的全部向量。
func (r *RegionIndexer) queryRegionVectors(ctx context.Context, regionID string) ([]*core.Vector, error) {
	zeroVec := make([]float32, r.embedder.Dim())
	results, _, err := r.vectorDB.Search(ctx, zeroVec, 10000, map[string]any{
		"region_id": regionID,
	})
	return results, err
}

// writeRegion 将 Region 向量化后写入 VectorStore，并在 graphDB 中创建 Region Node 和边。
func (r *RegionIndexer) writeRegion(
	ctx context.Context, regionID, title, summary string, tags []string, dir string,
	docIDs, childRegionIDs []string,
) (*core.Region, error) {
	region := &core.Region{
		ID:      regionID,
		Title:   title,
		Summary: summary,
		Tags:    tags,
		Dir:     dir,
		Meta: map[string]any{
			"child_region_ids": childRegionIDs,
		},
	}

	// 向量化摘要
	vec, err := r.embedder.CalcText(summary)
	if err != nil {
		return region, fmt.Errorf("region: embed summary: %w", err)
	}
	if vec == nil {
		return region, fmt.Errorf("region: embed summary returned nil vector")
	}
	vec.ID = utils.GenerateID([]byte("region_" + regionID))
	vec.ChunkID = "region:" + regionID
	regionNodeID := utils.GenerateID([]byte("region:" + regionID))
	vec.Metadata = map[string]any{
		"region_id":  regionID,
		"region":     true, // 标记为 Region 向量，区别于普通 Chunk
		"chunk_type": "region",
		"title":      title,
		"tags":       tags,
		"dir":        dir,
		"summary":    summary,
		"entity_ids": []string{regionNodeID},
	}

	// 先删除旧 Region 向量（如存在），防止 HNSW 重复 Add 同一 key 时 panic
	_ = r.vectorDB.Delete(ctx, vec.ID)

	if err := r.vectorDB.Upsert(ctx, []*core.Vector{vec}); err != nil {
		return region, fmt.Errorf("region: upsert vector: %w", err)
	}

	// ── 图层面：Region Node + 边 ──
	if r.graphDB != nil {
		regionNodeID = utils.GenerateID([]byte("region:" + regionID))
		regionNode := &core.Node{
			ID:     regionNodeID,
			Labels: []string{"Region"},
			Name:   title,
			Properties: map[string]any{
				"dir":        dir,
				"confidence": 0.9,
			},
			SourceChunkIDs: []string{"region:" + regionID},
		}
		if err := r.graphDB.UpsertNodes(ctx, []*core.Node{regionNode}); err != nil {
			return region, fmt.Errorf("region: upsert node: %w", err)
		}

		// Region CONTAINS Document（指向该目录下的文档根节点）
		var edges []*core.Edge
		for _, dID := range docIDs {
			docNodeID := utils.GenerateID([]byte(dID + ":document"))
			edges = append(edges, &core.Edge{
				ID:        utils.GenerateID([]byte(regionNodeID + "CONTAINS" + dID)),
				Type:      "CONTAINS",
				Source:    regionNodeID,
				Target:    docNodeID,
				Predicate: "CONTAINS",
				Properties: map[string]any{
					"confidence": 0.9,
				},
			})
		}

		// Parent Region CHILD_REGION 子 Region（指向该目录下的子 Region）
		for _, crID := range childRegionIDs {
			childNodeID := utils.GenerateID([]byte("region:" + crID))
			edges = append(edges, &core.Edge{
				ID:        utils.GenerateID([]byte(regionNodeID + "CHILD_REGION" + crID)),
				Type:      "CHILD_REGION",
				Source:    regionNodeID,
				Target:    childNodeID,
				Predicate: "CHILD_REGION",
				Properties: map[string]any{
					"confidence": 0.9,
				},
			})
		}

		if len(edges) > 0 {
			if err := r.graphDB.UpsertEdges(ctx, edges); err != nil {
				return region, fmt.Errorf("region: upsert edges: %w", err)
			}
		}

		r.logger.Info("region: graph nodes and edges created",
			"region_id", regionID,
			"doc_nodes", len(docIDs),
			"child_regions", len(childRegionIDs))
	}

	return region, nil
}

// aggregateSummary 用 LLM 对摘要级内容做二次抽象（摘要的摘要）。
// LLM 不可用或不返回内容时降级为统计摘要。
func (r *RegionIndexer) aggregateSummary(
	ctx context.Context, title string, summaries []string, docCount, subRegionCount int,
) string {
	if len(summaries) == 0 {
		return fmt.Sprintf("Directory: %s (no summaries)", title)
	}

	client, err := r.getLLMClient()
	if err != nil {
		r.logger.Warn("region: LLM unavailable, using fallback summary", "error", err)
		return r.fallbackSummary(title, docCount, subRegionCount)
	}

	// 截取前 100 条避免超出上下文
	if len(summaries) > 100 {
		summaries = summaries[:100]
	}

	var contextParts []string
	if docCount > 0 {
		contextParts = append(contextParts, fmt.Sprintf("- %d file summaries", docCount))
	}
	if subRegionCount > 0 {
		contextParts = append(contextParts, fmt.Sprintf("- %d sub-directory summaries", subRegionCount))
	}
	contextLine := strings.Join(contextParts, "\n")

	prompt := fmt.Sprintf(`You are a knowledge base architect. Below are summaries of contents in the directory "%s".

%s

Generate a concise 2-3 sentence abstract describing what this directory is collectively about.
Focus on the high-level purpose and structure rather than individual file details.
This abstract will be used as a navigation entry point for knowledge retrieval.

Summaries:
%s

Directory abstract:`,
		title, contextLine, strings.Join(summaries, "\n---\n"))

	messages := []chat.Message{
		chat.NewSystemMessage(prompt),
	}

	resp, err := client.Chat(ctx, messages)
	if err != nil {
		r.logger.Warn("region: LLM summary failed, using fallback", "error", err)
		return r.fallbackSummary(title, docCount, subRegionCount)
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return r.fallbackSummary(title, docCount, subRegionCount)
	}
	return content
}

func (r *RegionIndexer) fallbackSummary(title string, docCount, subRegionCount int) string {
	parts := []string{fmt.Sprintf("Directory: %s", title)}
	if docCount > 0 {
		parts = append(parts, fmt.Sprintf("(%d files)", docCount))
	}
	if subRegionCount > 0 {
		parts = append(parts, fmt.Sprintf("%d sub-directories", subRegionCount))
	}
	return strings.Join(parts, " ")
}

func (r *RegionIndexer) getLLMClient() (chat.Client, error) {
	return openai.NewOpenAI(chat.Config{
		APIKey:  r.model.APIKey,
		Model:   r.model.Model,
		BaseURL: r.model.BaseURL,
		Timeout: 5 * time.Minute,
	})
}

// ---------------------------------------------------------------------------
// 包级辅助函数
// ---------------------------------------------------------------------------

// RegionIDFromDir 计算一个目录路径对应的 Region ID。
func RegionIDFromDir(dir string) string {
	return regionIDFromDir(dir)
}

func regionIDFromDir(dir string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(dir))))
}

// collectTagsFromVector 从向量 metadata 中提取 tags 并入目标切片。
func collectTagsFromVector(v *core.Vector, dest *[]string) {
	seen := make(map[string]bool)
	for _, t := range *dest {
		seen[t] = true
	}
	switch raw := v.Metadata["tags"].(type) {
	case []string:
		for _, t := range raw {
			if !seen[t] {
				seen[t] = true
				*dest = append(*dest, t)
			}
		}
	case []any:
		for _, t := range raw {
			if s, ok := t.(string); ok && !seen[s] {
				seen[s] = true
				*dest = append(*dest, s)
			}
		}
	}
}
