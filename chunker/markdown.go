package chunker

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	"github.com/DotNetAge/gorag/v2/utils"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/markdown"
	tsmd "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
)

// =====================================================================
// MarkdownChunker：基于 go-tree-sitter 的 Markdown 分块器
// =====================================================================
//
// 设计要点：
//   - 使用 tree-sitter-markdown 解析 Markdown 内容得到 block 语法树
//   - 通过 Query 查找 atx_heading（#/##/.../######）和 setext_heading（下划线 =/-）节点
//   - 每个 heading 节点作为一个 Chunk 起点，到下一个 heading 之前结束
//   - 第一个 heading 之前的内容（前言）作为独立 Chunk，标题取文件名
//   - 没有 heading 时整个文档作为一个 Chunk
//   - Chunk.Title 取 heading 行去除 # 前缀后的纯文本
//   - Chunk.StartPos/EndPos 为字节偏移
//   - Chunk.Summary 不取值

// markdownHeadingQuery 匹配 Markdown 的所有 heading 节点。
// 同时识别 ATX heading（#/##/.../######）和 setext heading（=/- 下划线）。
const markdownHeadingQuery = `
(atx_heading) @heading
(setext_heading) @heading
`

// markdownHeadingInfo 记录单个 heading 的位置、标题与层级，
// 供 Chunk 与 buildMarkdownGraph 共用。
type markdownHeadingInfo struct {
	start uint32
	title string
	level int
}

// MarkdownChunker Markdown 分块器：基于 tree-sitter 按 heading 结构切分章节。
//
// 适用 RawDocType：RawDocDoc（pdf/docx/html/epub/pptx/md 等已归一化为 Markdown）。
type MarkdownChunker struct{}

// NewMarkdownChunker 创建 Markdown 分块器。
func NewMarkdownChunker() *MarkdownChunker { return &MarkdownChunker{} }

// Chunk 实现 Chunker 接口：按 tree-sitter 识别的 heading 边界切分 Markdown，
// 同时产出 heading 层级结构对应的 Nodes/Edges。
func (m *MarkdownChunker) Chunk(doc document.RawDoc) (ChunkResult, error) {
	if doc == nil {
		return ChunkResult{}, nil
	}

	content := doc.Content()
	if content == "" {
		return ChunkResult{}, nil
	}

	src := []byte(content)

	dir := filepath.Dir(doc.FileName())
	regionName := filepath.Base(dir)

	// README.md 生成式路径：检测到 RegionDescriptorMarker 直接返回单 Chunk，不再按 heading 切分
	if isRegionDescriptor(doc.FileName()) && strings.Contains(content, core.RegionDescriptorMarker) {
		chunk := buildChunk(doc, 0, 0, len(content), regionName, content)
		chunk.Metadata = map[string]any{
			core.MetaIsRegionDescriptor: true,
			core.MetaRegionGenerated:    true,
		}
		chunks := enrichChunksMetadata([]core.Chunk{chunk}, content, doc.FileName())
		regionNode := buildRegionNode(doc, dir, []string{chunks[0].ID})
		return ChunkResult{
			Chunks: chunks,
			Nodes:  []core.Node{regionNode},
			Edges:  nil,
		}, nil
	}

	// 1. 用 tree-sitter-markdown 解析得到 block 树
	ctx := context.Background()
	tree, err := markdown.ParseCtx(ctx, nil, src)
	if err != nil || tree == nil {
		// 解析失败：兜底为单 Chunk
		title := deriveTitle(doc.FileName())
		chunk := buildChunk(doc, 0, 0, len(content), title, content)
		chunks := enrichChunksMetadata([]core.Chunk{chunk}, content, doc.FileName())
		return ChunkResult{Chunks: chunks}, nil
	}
	defer tree.BlockTree().Close()

	// 2. 用 Query 查找所有 heading 节点
	q, err := sitter.NewQuery([]byte(markdownHeadingQuery), tsmd.GetLanguage())
	if err != nil {
		// Query 构造失败：兜底为单 Chunk
		title := deriveTitle(doc.FileName())
		chunk := buildChunk(doc, 0, 0, len(content), title, content)
		chunks := enrichChunksMetadata([]core.Chunk{chunk}, content, doc.FileName())
		return ChunkResult{Chunks: chunks}, nil
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.BlockTree().RootNode())

	var headings []markdownHeadingInfo

	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range match.Captures {
			n := c.Node
			if n == nil {
				continue
			}
			if n.Type() != "atx_heading" && n.Type() != "setext_heading" {
				continue
			}
			title, level := extractMarkdownHeading(n, src)
			headings = append(headings, markdownHeadingInfo{
				start: n.StartByte(),
				title: title,
				level: level,
			})
		}
	}

	// 4. 没有 heading：整体作为单块
	if len(headings) == 0 {
		title := deriveTitle(doc.FileName())
		chunk := buildChunk(doc, 0, 0, len(content), title, content)
		chunks := enrichChunksMetadata([]core.Chunk{chunk}, content, doc.FileName())
		return ChunkResult{Chunks: chunks}, nil
	}

	var chunks []core.Chunk

	// 4. 第一个 heading 之前的内容（前言/标题块）作为独立 Chunk
	firstStart := int(headings[0].start)
	if firstStart > 0 {
		pre := strings.TrimSpace(content[:firstStart])
		if pre != "" {
			title := deriveTitle(doc.FileName())
			chunks = append(chunks, buildChunk(doc, len(chunks), 0, len(pre), title, pre))
		}
	}

	// 5. 按 heading 切分章节，每段内容从当前 heading 开始到下一个 heading 之前
	for i, h := range headings {
		start := int(h.start)
		var end int
		if i+1 < len(headings) {
			end = int(headings[i+1].start)
		} else {
			end = len(content)
		}

		section := content[start:end]
		body := strings.TrimRight(section, "\n\r")

		title := h.title
		if title == "" {
			title = deriveTitle(doc.FileName())
		}

		chunks = append(chunks, buildChunkWithMeta(doc, len(chunks), start, start+len(body), title, body, h.level))
	}

	// 6. 根据 heading 层级回填每个 Chunk 的 ParentID，形成分块树
	setMarkdownParentIDs(chunks, headings)

	// 7. 构建 heading 层级结构对应的 Nodes/Edges
	nodes, edges := buildMarkdownGraph(doc, chunks, headings)

	// README.md 普通路径：产出 Region 节点，顶层 Chunk Title 改为目录名
	if isRegionDescriptor(doc.FileName()) {
		applyRegionDescriptorMetadata(chunks, dir)
		var topChunkIDs []string
		for _, c := range chunks {
			if c.ParentID == "" {
				topChunkIDs = append(topChunkIDs, c.ID)
			}
		}
		regionNode := buildRegionNode(doc, dir, topChunkIDs)
		nodes = append([]core.Node{regionNode}, nodes...)
	}

	// 8. 不对Summary进行填充，由LLM进行摘要处理，单纯截取字符串效果太差；

	// 9. 统一 enriched 通用元数据（行号、语言、目录等）
	chunks = enrichChunksMetadata(chunks, content, doc.FileName())

	return ChunkResult{
		Chunks: chunks,
		Nodes:  nodes,
		Edges:  edges,
	}, nil
}

// setMarkdownParentIDs 根据 heading 层级为每个 Chunk 设置 ParentID。
//
// 规则：
//   - 第一个 heading Chunk（或前言 Chunk）为文档级根节点，ParentID 为空
//   - 后续 heading Chunk 的父级为最近的层级更低的 heading Chunk
func setMarkdownParentIDs(chunks []core.Chunk, headings []markdownHeadingInfo) {
	if len(chunks) == 0 || len(headings) == 0 {
		return
	}

	// heading chunk 起始索引：如果第一个 chunk 是前言（无 heading_level），则从 1 开始
	headingChunkIdx := 0
	if level, ok := chunks[0].Metadata[core.MetaHeadingLevel].(int); !ok || level == 0 {
		headingChunkIdx = 1
	}

	type stackItem struct {
		level int
		idx   int
	}
	stack := []stackItem{{level: 0, idx: -1}}

	for i, h := range headings {
		chunkIdx := headingChunkIdx + i
		if chunkIdx >= len(chunks) {
			break
		}

		for len(stack) > 0 && stack[len(stack)-1].level >= h.level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		if parent.idx >= 0 {
			chunks[chunkIdx].ParentID = chunks[parent.idx].ID
		}

		stack = append(stack, stackItem{level: h.level, idx: chunkIdx})
	}
}

// buildMarkdownGraph 根据 heading 层级构建 Section 节点与 CONTAINS 边。
//
// 规则：
//   - 文档节点作为根，包含所有 level-1 heading
//   - 每个 heading 作为 Section 节点
//   - 父 heading 包含后续级别更低的子 heading，直到遇到同级或更高级 heading
func buildMarkdownGraph(doc document.RawDoc, chunks []core.Chunk, headings []markdownHeadingInfo) ([]core.Node, []core.Edge) {
	docTitle := deriveTitle(doc.FileName())
	docNode := buildNode(doc, docTitle, []string{"Document"}, "", map[string]any{"node_type": "document"})

	nodes := []core.Node{docNode}
	edges := []core.Edge{}

	// stack 维护当前祖先链，每个元素为 (level, nodeID, chunkID)
	type stackItem struct {
		level   int
		nodeID  string
		chunkID string
	}
	stack := []stackItem{{level: 0, nodeID: docNode.ID, chunkID: ""}}

	headingChunkIdx := 0
	if len(chunks) > 0 {
		// 第一个 chunk 可能是前言，heading chunk 从第二个开始
		if level, ok := chunks[0].Metadata[core.MetaHeadingLevel].(int); !ok || level == 0 {
			headingChunkIdx = 1
		}
	}

	for i, h := range headings {
		if headingChunkIdx+i >= len(chunks) {
			break
		}
		chunk := chunks[headingChunkIdx+i]

		// 找到合适的父节点：弹出直到栈顶 level 小于当前 level
		for len(stack) > 0 && stack[len(stack)-1].level >= h.level {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]

		node := buildNode(doc, h.title, []string{"Section"}, chunk.ID, map[string]any{
			"node_type":     "section",
			"heading_level": h.level,
		})
		nodes = append(nodes, node)
		edges = append(edges, buildEdge(doc, parent.nodeID, node.ID, "CONTAINS", []string{parent.chunkID, chunk.ID}))

		stack = append(stack, stackItem{level: h.level, nodeID: node.ID, chunkID: chunk.ID})
	}

	return nodes, edges
}

// extractMarkdownHeading 从 atx_heading/setext_heading 节点提取标题文本和层级。
//
// ATX heading 结构：(atx_heading (atx_h1_marker) heading_content: (inline))
//   - marker 子节点类型为 atx_h1_marker ~ atx_h6_marker，对应层级 1-6
//   - heading_content 字段为 inline 子节点，取其原文即标题文本
//
// Setext heading 结构：(setext_heading (heading_content: (inline)) (setext_h1_underline | setext_h2_underline))
//   - underline 子节点决定层级（1 或 2）
//   - heading_content 字段为 inline 子节点，取其原文即标题文本
func extractMarkdownHeading(n *sitter.Node, src []byte) (string, int) {
	if n == nil {
		return "", 0
	}

	title := ""
	level := 1

	// 查找 heading_content 字段子节点
	contentNode := n.ChildByFieldName("heading_content")
	if contentNode != nil {
		title = strings.TrimSpace(contentNode.Content(src))
	}

	// 通过子节点类型确定层级
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "atx_h1_marker", "setext_h1_underline":
			level = 1
		case "atx_h2_marker", "setext_h2_underline":
			level = 2
		case "atx_h3_marker":
			level = 3
		case "atx_h4_marker":
			level = 4
		case "atx_h5_marker":
			level = 5
		case "atx_h6_marker":
			level = 6
		}
	}

	// ATX heading 标题可能含有尾随的 # 关闭序列（如 "## Title ##"），统一去除
	title = strings.TrimRight(title, "#")
	title = strings.TrimSpace(title)

	return title, level
}

// buildChunkWithMeta 构造单个 Markdown Chunk，额外写入 heading_level 元数据。
func buildChunkWithMeta(
	doc document.RawDoc,
	idx int,
	start, end int,
	title, content string,
	headingLevel int,
) core.Chunk {
	c := buildChunk(doc, idx, start, end, title, content)
	if c.Metadata == nil {
		c.Metadata = map[string]any{}
	}
	c.Metadata[core.MetaHeadingLevel] = headingLevel
	return c
}

// isRegionDescriptor 判断文件是否为 README.md（不区分大小写）。
func isRegionDescriptor(fileName string) bool {
	return strings.EqualFold(filepath.Base(fileName), "README.md")
}

// applyRegionDescriptorMetadata 为 README.md 分块设置 Region 元数据，
// 并将顶层 Chunk 的 Title 改为目录名。
func applyRegionDescriptorMetadata(chunks []core.Chunk, dir string) {
	regionName := filepath.Base(dir)
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = map[string]any{}
		}
		chunks[i].Metadata[core.MetaIsRegionDescriptor] = true
		if chunks[i].ParentID == "" {
			chunks[i].Title = regionName
		}
	}
}

// buildRegionNode 构造 Region 节点。
// RegionID 与 Chunk.RegionID 保持一致：目录路径的 SHA256。
func buildRegionNode(doc document.RawDoc, dir string, chunkIDs []string) core.Node {
	regionID := utils.GenerateID([]byte(dir))
	regionName := filepath.Base(dir)
	return core.Node{
		ID:             regionID,
		Labels:         []string{core.LabelRegion},
		Name:           regionName,
		Properties:     map[string]any{core.PropDir: dir},
		SourceChunkIDs: chunkIDs,
		SourceDocIDs:   []string{doc.ID()},
	}
}
