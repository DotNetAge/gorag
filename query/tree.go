package query

import "github.com/DotNetAge/gorag/v2/core"

// TreeQuery 树状知识结构查询。
//
// 限定范围（Region 或全局）返回 ChunkNode 树，
// 供 UI 导航展示，避免全量加载力导向图。
type TreeQuery struct {
	BaseQuery

	// RegionID 为空字符串表示全局树，非空表示限定到指定 Region。
	RegionID string

	// Depth 控制树深度：
	//   0/1 = Region → Document（默认）
	//   2   = Region → Document → Entity
	Depth int
}

// NewTreeQuery 创建树查询。
// regionID 为空时返回全局树（所有 Region 为直接子节点）。
// depth 控制深度，超出范围时自动裁剪。
func NewTreeQuery(regionID string, depth int) core.Query {
	if depth < 1 {
		depth = 1
	}
	if depth > 2 {
		depth = 2
	}
	return &TreeQuery{
		BaseQuery: BaseQuery{
			raw:        regionID,
			normalized: core.CleanText(regionID),
			filters:    make(map[string]any),
		},
		RegionID: regionID,
		Depth:    depth,
	}
}
