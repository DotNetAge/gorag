package indexer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/gochat/client/openai"
	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/utils"
)

// RegionFileName 是 Region 描述文件的文件名。
// 每个目录下的该文件作为 Region 的实体化载体，
// 其内容会被索引为普通文档，提取的实体/关系成为 Region 的知识源。
//
// 若目录下已存在该文件（用户编写或上次生成），IndexRegion 会直接复用其内容，
// 跳过 LLM 聚合；仅在文件不存在时才走 LLM 聚合生成流程。
const RegionFileName = "README.md"

// IsRegionDescriptor 判断文件是否为 Region 描述文件。
func IsRegionDescriptor(path string) bool {
	if path == "" {
		return false
	}
	return filepath.Base(path) == RegionFileName
}

// StripExt 去除文件名中的扩展名。
// "file.md" → "file", "notes.txt" → "notes", 无扩展名时原样返回。
func StripExt(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return filename
	}
	return filename[:len(filename)-len(ext)]
}

// RegionIndexer 对一个已索引目录生成 Region 描述文件（README.md）。
//
// Region 是"摘要的摘要"——它的聚合输入只包含该目录下两类摘要级向量：
//
//  1. 本层文件的 Root Chunk（chunk_type = "root"）：文件级摘要
//  2. 子目录的子 Region（README.md）：子目录级摘要
//
// 普通分段（chunk_type = "segment"）不参与 Region 聚合，确保 Region
// 只做层级抽象而不被细节污染。
//
// 生成流程：
//  1. 若 README.md 已存在 → 直接复用，跳过 LLM 聚合
//  2. 若 README.md 不存在 → 聚合摘要 → 写入 README.md 文件
//  3. 在图数据库中创建 Region Node + CONTAINS 边
//  4. 调用方负责将 README.md 通过 GraphIndexer 索引
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

// RegionResult 是 IndexRegion 的返回结果。
type RegionResult struct {
	*core.Region
	RegionFilePath string // Region 描述文件路径（已存在或新生成的 README.md），为空表示无需索引
}

// IndexRegion 对指定目录生成或复用 Region 描述文件（README.md）并创建图结构。
//
// 核心策略：优先复用目录下已存在的 README.md，避免覆盖用户编写的内容；
// 仅在 README.md 不存在时才走 LLM 聚合生成流程。
//
// 流程：
//
//  1. 通过 region_id 过滤查询 VectorStore，获取该目录下全部向量
//  2. 检查目录下 README.md 是否已存在：
//     - 已存在 → 直接复用，跳过 LLM 聚合（仍会更新图结构）
//     - 不存在 → 进入聚合流程
//  3. 聚合时只取摘要级向量：
//     - chunk_type = "root"  → 文件级文档摘要
//     - region = true        → 子目录级 Region 摘要（兼容旧版，新版来自 README.md 的向量）
//     - chunk_type = "segment" → 忽略（不入聚合）
//  4. 用 LLM 对摘要做二次抽象（摘要的摘要 + 实体关系补充）
//  5. 将内容写入目录下的 README.md 文件
//  6. 在图数据库中创建/更新 Region Node + CONTAINS 边
//
// 返回的 RegionResult 包含 RegionFilePath，调用方应将其通过 GraphIndexer.AddFile 索引。
//
// 特殊情况：
//   - 目录无内容（无任何向量）且 README.md 不存在：返回 nil
//   - 目录无内容但 README.md 已存在：复用 README.md，仍创建图结构
//   - 目录有文件但无摘要级向量：生成降级摘要，仍会写入 README.md
func (r *RegionIndexer) IndexRegion(ctx context.Context, dir string) (*RegionResult, error) {
	regionID := regionIDFromDir(dir)
	title := StripExt(filepath.Base(dir))
	regionFilePath := filepath.Join(dir, RegionFileName)

	r.logger.Info("region: indexing", "dir", dir, "region_id", regionID)

	// 1. 查询该 Region 下的全部向量
	vectors, err := r.queryRegionVectors(ctx, regionID)
	if err != nil {
		return nil, fmt.Errorf("region: query vectors: %w", err)
	}

	// 2. 检查 README.md 是否已存在
	//    已存在 → 直接复用（用户编写或上次生成），跳过 LLM 聚合
	//    不存在 → 走 LLM 聚合生成流程
	_, statErr := os.Stat(regionFilePath)
	readmeExists := statErr == nil

	// ── 空目录处理 ──
	// 无向量且 README.md 不存在 → 目录无任何内容，直接返回
	// 无向量但 README.md 已存在 → 用户编写的 README.md，仍需索引
	if len(vectors) == 0 && !readmeExists {
		r.logger.Info("region: no vectors and no README.md, nothing to do",
			"dir", dir, "region_id", regionID)
		return nil, nil
	}

	// 3. 分离摘要级向量和 tags（用于图结构构建；README.md 已存在时仅用于图）
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
			// 跳过 README.md 自身的 Root Chunk，防止自引用循环。
			// README.md 由 RegionIndexer 在上一次 Sync 中生成（或用户编写），
			// 其摘要不应作为本次聚合的输入。
			if sf, _ := v.Metadata["source_file"].(string); IsRegionDescriptor(sf) {
				continue
			}
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

	// 4. 生成或复用 README.md 内容
	var summary string
	if readmeExists {
		// README.md 已存在 → 直接复用，跳过 LLM 聚合
		// summary 留空：摘要以 README.md 文件实际内容为准，由 GraphIndexer 索引时提取
		r.logger.Info("region: README.md exists, using as-is, skip LLM aggregation",
			"path", regionFilePath)
	} else {
		// README.md 不存在 → 走 LLM 聚合生成流程

		// ── 无实质性内容（所有向量均来自 README.md 或全为 segment）──
		if docCount == 0 && subRegionCount == 0 {
			r.logger.Info("region: no substantive content, skip README.md generation",
				"dir", dir, "region_id", regionID)
			return nil, nil
		}

		if len(abstractSummaries) == 0 {
			r.logger.Warn("region: no abstract-level vectors, creating minimal region",
				"dir", dir, "total_vectors", len(vectors))
			summary = ""
		} else {
			summary = r.aggregateSummary(ctx, title, abstractSummaries, docCount, subRegionCount)
		}

		// 写入 README.md 文件
		if err := r.writeRegionFile(regionFilePath, title, summary, allTags); err != nil {
			return nil, fmt.Errorf("region: write region file: %w", err)
		}
	}

	r.logger.Info("region: completed",
		"dir", dir,
		"doc_summaries", docCount,
		"sub_regions", subRegionCount,
		"tags", len(allTags),
		"summary_len", len(summary),
		"readme_preexisting", readmeExists)

	// 5. 图层面：Region Node + 边
	if r.graphDB != nil {
		if err := r.upsertRegionGraph(ctx, regionID, title, dir, docIDs, childRegionIDs); err != nil {
			return nil, fmt.Errorf("region: upsert graph: %w", err)
		}
	}

	return &RegionResult{
		Region: &core.Region{
			ID:      regionID,
			Title:   title,
			Summary: summary,
			Tags:    allTags,
			Dir:     dir,
			Meta: map[string]any{
				"child_region_ids":  childRegionIDs,
				"readme_preexisting": readmeExists,
			},
		},
		RegionFilePath: regionFilePath,
	}, nil
}

// queryRegionVectors 通过 metadata filter 查询指定 Region 下的全部向量。
func (r *RegionIndexer) queryRegionVectors(ctx context.Context, regionID string) ([]*core.Vector, error) {
	zeroVec := make([]float32, r.embedder.Dim())
	results, _, err := r.vectorDB.Search(ctx, zeroVec, 10000, map[string]any{
		"region_id": regionID,
	})
	return results, err
}

// writeRegionFile 将摘要内容写入 README.md 文件。
// 仅在 README.md 不存在时调用；已存在时由调用方直接复用，不覆盖。
func (r *RegionIndexer) writeRegionFile(path, title, summary string, tags []string) error {
	var content string
	if summary == "" {
		content = fmt.Sprintf("# %s\n\n_This directory has no indexed content._\n", title)
	} else {
		var tagLine string
		if len(tags) > 0 {
			tagLine = "\n\nTags: " + strings.Join(tags, ", ")
		}
		content = fmt.Sprintf("# %s\n\n%s%s\n", title, summary, tagLine)
	}

	// 确保目录存在（理论上已存在，但安全起见）
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// upsertRegionGraph 在 graphDB 中创建/更新 Region Node 和关联边。
func (r *RegionIndexer) upsertRegionGraph(
	ctx context.Context, regionID, title, dir string,
	docIDs, childRegionIDs []string,
) error {
	regionNodeID := utils.GenerateID([]byte("region:" + regionID))

	// Region Node
	regionNode := &core.Node{
		ID:     regionNodeID,
		Labels: []string{"Region"},
		Name:   title,
		Properties: map[string]any{
			"dir":        dir,
			"confidence": 0.9,
		},
	}
	if err := r.graphDB.UpsertNodes(ctx, []*core.Node{regionNode}); err != nil {
		return fmt.Errorf("upsert region node: %w", err)
	}

	// 构建边：Region CONTAINS Document + CHILD_REGION
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
			return fmt.Errorf("upsert region edges: %w", err)
		}
	}

	r.logger.Info("region: graph nodes and edges created",
		"region_id", regionID,
		"doc_nodes", len(docIDs),
		"child_regions", len(childRegionIDs))

	return nil
}

// aggregateSummary 用 LLM 对摘要级内容做二次抽象并生成 README.md 内容。
// 仅在目录下 README.md 不存在时调用；LLM 不可用或不返回内容时降级为统计摘要。
//
// 特殊优化：仅 1 个输入摘要时直接透传，零 LLM 调用。
// 此种情形发生在单文件目录的首次摘要——Region 摘要 = 文件摘要，
// 不需要也做不出更有意义的聚合。
func (r *RegionIndexer) aggregateSummary(
	ctx context.Context, title string, summaries []string, docCount, subRegionCount int,
) string {
	if len(summaries) == 0 {
		return fmt.Sprintf("Directory: %s (no summaries)", title)
	}
	// 仅 1 个输入摘要 → 直接透传，零 LLM
	if len(summaries) == 1 {
		return summaries[0]
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

Generate a README.md document that describes this directory's overall purpose and structure.
Include:
1. A high-level summary of what this directory is collectively about
2. Key entities (concepts, technologies, components) mentioned across files
3. Relationships between these entities where applicable

This document serves as a navigation entry point for knowledge retrieval.

Summaries:
%s

README.md content:`,
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
