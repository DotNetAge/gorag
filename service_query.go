package gorag

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/indexer"
)

// QueryService 负责所有面向 Chunk 的查询与列表。
type QueryService struct {
	svc *IndexingService
}

// Query 执行搜索查询，可选按 source 路径前缀过滤。
func (s *QueryService) Query(ctx context.Context, text string, filterPath string) (*core.Hit, error) {
	if text == "" {
		return nil, fmt.Errorf("查询内容不能为空")
	}
	hit, err := s.svc.indexer.Search(ctx, s.svc.indexer.NewQuery(text))
	if err != nil {
		return nil, err
	}
	if filterPath != "" {
		hit = filterHitBySourcePrefix(hit, filterPath)
	}
	return hit, nil
}

// QueryMulti 执行多个查询并融合结果，可选按 source 路径前缀过滤。
// 融合规则：相同 chunk 取最高分数，按分数降序排列。
func (s *QueryService) QueryMulti(ctx context.Context, queries []string, filterPath string) (*core.Hit, error) {
	if len(queries) == 0 {
		return nil, fmt.Errorf("查询内容不能为空")
	}

	var absFilter string
	if filterPath != "" {
		var err error
		absFilter, err = filepath.Abs(filterPath)
		if err != nil {
			absFilter = filterPath
		}
	}

	merged := make(map[string]core.ChunkHit)
	for _, text := range queries {
		if strings.TrimSpace(text) == "" {
			continue
		}
		hit, err := s.svc.indexer.Search(ctx, s.svc.indexer.NewQuery(text))
		if err != nil {
			return nil, fmt.Errorf("查询 %q 失败: %w", text, err)
		}
		if hit == nil {
			continue
		}
		for _, ch := range hit.Chunks {
			if absFilter != "" && !sourceHasPrefix(ch.Chunk.Source, absFilter) {
				continue
			}
			existing, ok := merged[ch.Chunk.ID]
			if !ok || ch.Score > existing.Score {
				merged[ch.Chunk.ID] = ch
			}
		}
	}

	chunks := make([]core.ChunkHit, 0, len(merged))
	for _, ch := range merged {
		chunks = append(chunks, ch)
	}

	// 按分数降序排列
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})

	combinedText := strings.Join(queries, " | ")
	return &core.Hit{
		Query:  s.svc.indexer.NewQuery(combinedText),
		Score:  topChunkScore(chunks),
		Chunks: chunks,
	}, nil
}

// ListChunks 分页列出已索引的 Chunk，可选按 source 路径前缀过滤。
// page 从 1 开始，size 为每页数量（<=0 时使用默认值 20）。
// 返回当前页 Chunk 列表、过滤后总数和错误。
func (s *QueryService) ListChunks(ctx context.Context, page, size int, filterPath string) ([]core.Chunk, int, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	admin, ok := s.svc.indexer.(indexer.IndexerAdmin)
	if !ok {
		return nil, 0, fmt.Errorf("当前索引器不支持列表查询")
	}

	var filters []core.FilterCondition
	if filterPath != "" {
		absFilter, err := filepath.Abs(filterPath)
		if err != nil {
			absFilter = filterPath
		}
		filters = append(filters, core.FilterCondition{
			Key:   core.VecMetaSource,
			Type:  "prefix",
			Value: absFilter,
		})
	}

	offset := (page - 1) * size
	chunks, total, err := admin.List(ctx, offset, size, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("列出分片失败: %w", err)
	}
	return chunks, total, nil
}
