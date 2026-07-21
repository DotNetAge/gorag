package core

import (
	"fmt"

	"github.com/DotNetAge/gorag/v2/document"
)

// StructuredDoc 结构化文档：RawDoc 的 Wrapper，承载结构化产物的容器。
//
// 设计要点：
//   - 直接持有 Chunks/Nodes/Edges 扁平集合，不构建树形结构
//   - Chunks/Nodes/Edges 全部由 Chunker 填充（Chunker 已取代 Extractor）
//   - StructuredDoc 本身不调用 LLM，仅作为容器传递
type StructuredDoc interface {
	// Raw 返回原始 RawDoc
	Raw() document.RawDoc

	// Title 返回文档标题（由 Chunker 从首行 heading/文件名提取）
	Title() string

	// Summary 返回文档摘要（由 Extractor 生成，未生成时为空）
	Summary() string

	// Chunks 返回所有分片（由 Chunker 填充）
	Chunks() []Chunk

	// Nodes 返回所有实体节点（由 Chunker 填充，仅 GraphIndexer 使用）
	Nodes() []Node

	// Edges 返回所有关系边（由 Chunker 填充，仅 GraphIndexer 使用）
	Edges() []Edge

	// Setters 用于 Chunker 链式填充
	SetTitle(string) StructuredDoc
	SetSummary(string) StructuredDoc
	SetChunks([]Chunk) StructuredDoc
	SetNodes([]Node) StructuredDoc
	SetEdges([]Edge) StructuredDoc
}

// ---- 公共实现 ----

// baseStructuredDoc 是 StructuredDoc 接口的内部基础实现，持有结构化产物的字段。
type baseStructuredDoc struct {
	raw     document.RawDoc
	title   string
	summary string
	chunks  []Chunk
	nodes   []Node
	edges   []Edge
}

func (b *baseStructuredDoc) Raw() document.RawDoc { return b.raw }
func (b *baseStructuredDoc) Title() string        { return b.title }
func (b *baseStructuredDoc) Summary() string      { return b.summary }
func (b *baseStructuredDoc) Chunks() []Chunk      { return b.chunks }
func (b *baseStructuredDoc) Nodes() []Node        { return b.nodes }
func (b *baseStructuredDoc) Edges() []Edge        { return b.edges }

func (b *baseStructuredDoc) SetTitle(s string) StructuredDoc   { b.title = s; return b }
func (b *baseStructuredDoc) SetSummary(s string) StructuredDoc { b.summary = s; return b }
func (b *baseStructuredDoc) SetChunks(c []Chunk) StructuredDoc { b.chunks = c; return b }
func (b *baseStructuredDoc) SetNodes(n []Node) StructuredDoc   { b.nodes = n; return b }
func (b *baseStructuredDoc) SetEdges(e []Edge) StructuredDoc   { b.edges = e; return b }

// Structurize 创建结构化文档容器。
// 接收原始文档 RawDoc，返回空的 StructuredDoc（Chunks/Nodes/Edges 均为空，由 Chunker 后续填充）。
func Structurize(raw document.RawDoc) (StructuredDoc, error) {
	if raw == nil {
		return nil, fmt.Errorf("Structurize: 原文档对象 raw 不能为空")
	}
	return &baseStructuredDoc{raw: raw}, nil
}
