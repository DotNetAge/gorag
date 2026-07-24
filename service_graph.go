package gorag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/indexer"
)

// NodeQueryResult 是 nodes 命令的查询结果容器。
type NodeQueryResult struct {
	RegionID   string       `json:"region_id"`
	RegionName string       `json:"region_name"`
	Region     *core.Node   `json:"region,omitempty"`
	Nodes      []*core.Node `json:"nodes"`
	Edges      []*core.Edge `json:"edges"`
	DocCount   int          `json:"doc_count"`    // 该目录下已索引的 Document 节点数
	HasGraph   bool         `json:"has_graph"`    // 是否有实体图谱数据（非仅 Document）
}

// GraphService 负责图探索与原始 Cypher 查询。
type GraphService struct {
	svc *IndexingService
}

// Nodes 按目录查询 Region 节点及其 N 跳相邻节点。
// dir 为空时使用当前工作目录；hops 范围 1-3，默认 1。
func (s *GraphService) Nodes(ctx context.Context, dir string, hops int) (*NodeQueryResult, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
		}
		dir = cwd
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("解析目录路径失败: %w", err)
	}

	if hops < 1 {
		hops = 1
	}
	if hops > 3 {
		hops = 3
	}

	explorer, ok := s.svc.indexer.(indexer.GraphExplorer)
	if !ok {
		return nil, fmt.Errorf("当前索引器不支持图探索")
	}

	view, err := explorer.ExploreRegion(ctx, absDir, hops, 100)
	if err != nil {
		return nil, err
	}

	// 统计该目录下的 Document 节点数（用于区分「未索引」和「有文档但无实体」）
	var docCount int
	if counter, ok := s.svc.indexer.(interface {
		CountByRegion(ctx context.Context, path string) (int, error)
	}); ok {
		if c, err := counter.CountByRegion(ctx, absDir); err == nil {
			docCount = c
		}
	}

	// 判断是否包含实体节点（非 Document/Region 的节点）
	hasGraph := false
	if len(view.Nodes) > 0 {
		for _, n := range view.Nodes {
			if n == nil {
				continue
			}
			isEntity := true
			for _, l := range n.Labels {
				if l == "Document" || l == "Region" {
					isEntity = false
					break
				}
			}
			if isEntity {
				hasGraph = true
				break
			}
		}
	}

	return &NodeQueryResult{
		RegionID:   view.RegionID,
		RegionName: view.RegionName,
		Region:     view.Region,
		Nodes:      view.Nodes,
		Edges:      view.Edges,
		DocCount:   docCount,
		HasGraph:   hasGraph,
	}, nil
}

// Cypher 执行原始 Cypher 查询并返回结果。
// 仅当索引器支持图存储时可用（如 GraphIndexer / HyperIndexer）。
func (s *GraphService) Cypher(ctx context.Context, query string) ([]map[string]any, error) {
	if query == "" {
		return nil, fmt.Errorf("Cypher 查询不能为空")
	}

	type cypherer interface {
		CypherQuery(ctx context.Context, q string, params map[string]any) ([]map[string]any, error)
	}

	c, ok := s.svc.indexer.(cypherer)
	if !ok {
		return nil, fmt.Errorf("当前索引器不支持 Cypher 查询")
	}

	rows, err := c.CypherQuery(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("Cypher 查询执行失败: %w", err)
	}
	return rows, nil
}
