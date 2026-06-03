package query

import (
	"testing"

	"github.com/DotNetAge/gorag/core"
)

func TestNewQueryRouter(t *testing.T) {
	router := NewQueryRouter()
	if router == nil {
		t.Fatal("NewQueryRouter should not return nil")
	}
	if len(router.globalKeywords) == 0 {
		t.Error("router should have default global keywords configured")
	}
}

func TestRoute_EmptyQuery(t *testing.T) {
	router := NewQueryRouter()
	mode := router.Route("")
	if mode != core.SearchModeLocal {
		t.Errorf("empty query should return SearchModeLocal, got %v", mode)
	}
}

func TestRoute_LocalQuery(t *testing.T) {
	router := NewQueryRouter()

	testCases := []struct {
		name  string
		query string
	}{
		{"specific entity", "张三在哪里工作"},
		{"relationship", "阿里巴巴和腾讯的关系"},
		{"technical detail", "Go语言的goroutine实现原理"},
		{"code question", "如何使用Python的asyncio"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mode := router.Route(tc.query)
			if mode != core.SearchModeLocal {
				t.Errorf("query %q should return SearchModeLocal, got %v", tc.query, mode)
			}
		})
	}
}

func TestRoute_HybridQuery(t *testing.T) {
	router := NewQueryRouter()

	testCases := []struct {
		name  string
		query string
	}{
		{"one global keyword", "请总结这篇文章"},
		{"single summary", "文档概要说明"},
		{"single overview", "项目概况说明"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mode := router.Route(tc.query)
			if mode != core.SearchModeHybrid {
				t.Errorf("query %q should return SearchModeHybrid, got %v", tc.query, mode)
			}
		})
	}
}

func TestRoute_GlobalQuery(t *testing.T) {
	router := NewQueryRouter()

	testCases := []struct {
		name  string
		query string
	}{
		{"multiple keywords", "总结一下文档的主要内容和核心主题"},
		{"summary and overview", "总结并概述整个项目的整体架构"},
		{"global english", "summarize the main topics and overview"},
		{"classify request", "分类归纳这些文档的主要主题和整体脉络"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mode := router.Route(tc.query)
			if mode != core.SearchModeGlobal {
				t.Errorf("query %q should return SearchModeGlobal, got %v", tc.query, mode)
			}
		})
	}
}

func TestIsGlobalQuery(t *testing.T) {
	router := NewQueryRouter()

	if !router.IsGlobalQuery("总结主要内容和主题") {
		t.Error("IsGlobalQuery should return true for global query")
	}
	if router.IsGlobalQuery("张三的工作") {
		t.Error("IsGlobalQuery should return false for local query")
	}
}

func TestIsHybridQuery(t *testing.T) {
	router := NewQueryRouter()

	if !router.IsHybridQuery("总结文章") {
		t.Error("IsHybridQuery should return true for hybrid query")
	}
	if router.IsHybridQuery("张三的工作") {
		t.Error("IsHybridQuery should return false for local query")
	}
	if router.IsHybridQuery("总结主要内容和主题") {
		t.Error("IsHybridQuery should return false for global query")
	}
}

func TestRoute_CaseInsensitive(t *testing.T) {
	router := NewQueryRouter()

	mode1 := router.Route("SUMMARIZE the document")
	mode2 := router.Route("summarize the document")
	mode3 := router.Route("Summarize The Document")

	if mode1 != mode2 || mode2 != mode3 {
		t.Error("Route should be case insensitive")
	}
}
