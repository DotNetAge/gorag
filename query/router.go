package query

import (
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// QueryRouter 查询路由器
// 根据查询特征自动选择最合适的搜索模式
type QueryRouter struct {
	// 全局搜索关键词：包含这些词的查询倾向于使用 Global Search
	globalKeywords []string
}

// NewQueryRouter 创建查询路由器，使用默认关键词配置
func NewQueryRouter() *QueryRouter {
	return &QueryRouter{
		globalKeywords: []string{
			// 中文全局查询特征词
			"总结", "主题", "整体", "概况", "概述", "概括",
			"主要", "核心", "概要", "大纲", "脉络", "全局",
			"总览", "鸟瞰", "梳理", "分类", "归纳", "综合",
			"有什么", "包括哪些", "分为哪些", "有哪些类别",
			"主要内容", "主要问题", "关键信息", "重点",
			// 英文全局查询特征词
			"summarize", "summary", "overview", "main topic",
			"general", "overall", "broad", "high-level",
			"what are the", "what is", "key themes", "main themes",
			"classify", "categorize", "organize", "structure",
		},
	}
}

// Route 根据查询文本自动选择搜索模式
func (r *QueryRouter) Route(query string) core.SearchMode {
	if query == "" {
		return core.SearchModeLocal
	}

	lower := strings.ToLower(query)

	// 计算全局匹配度
	globalScore := 0
	for _, keyword := range r.globalKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			globalScore++
		}
	}

	// 判断逻辑
	if globalScore >= 2 {
		return core.SearchModeGlobal
	}
	if globalScore == 1 {
		return core.SearchModeHybrid
	}

	return core.SearchModeLocal
}

// IsGlobalQuery 快速判断是否为全局查询
func (r *QueryRouter) IsGlobalQuery(query string) bool {
	return r.Route(query) == core.SearchModeGlobal
}

// IsHybridQuery 快速判断是否为混合查询
func (r *QueryRouter) IsHybridQuery(query string) bool {
	return r.Route(query) == core.SearchModeHybrid
}
