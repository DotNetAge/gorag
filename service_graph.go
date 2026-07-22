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

	return &NodeQueryResult{
		RegionID:   view.RegionID,
		RegionName: view.RegionName,
		Region:     view.Region,
		Nodes:      view.Nodes,
		Edges:      view.Edges,
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
