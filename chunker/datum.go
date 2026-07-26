package chunker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/yaml"
)

const (
	// datumMaxChunkSize 单个数据 Chunk 的目标最大字符数。
	datumMaxChunkSize = 1500
	// datumMaxDepth 递归切分对象/数组的最大深度，避免无限递归。
	datumMaxDepth = 5
)

// DatumChunker 数据分块器：基于 tree-sitter 按对象边界递归切分。
//
// 适用 RawDocType：RawDocData（csv/json/xml/excel/eml/log/yml 等，已归一化为 JSON/文本）。
// 设计要点：
//   - 用 tree-sitter-yaml 解析数据内容（JSON 是 YAML 子集，可被 yaml 语法解析；
//     同时原生支持 YAML 风格 block_* 节点，便于未来直接处理 YAML 原文）
//   - 递归遍历对象/数组：当属性值是对象或数组时切分为独立子块，子块 Title 带完整路径
//   - 标量属性合并到「其余字段」块；当「其余字段」累积超过 datumMaxChunkSize 时分块
//   - 子对象/数组规模超过 datumMaxChunkSize 时继续递归切分，最深 datumMaxDepth 层
//   - StartPos/EndPos 为字节偏移，准确对应原文
//   - log 类文件走按行切分（非结构化数据）
//   - 解析失败时降级为按行切分
type DatumChunker struct{}

// NewDatumChunker 创建数据分块器。
func NewDatumChunker() *DatumChunker { return &DatumChunker{} }

// Chunk 实现 Chunker 接口：按 tree-sitter 解析的对象/数组边界递归切分，
// 同时产出数据结构层级对应的 Nodes/Edges。
func (d *DatumChunker) Chunk(doc document.RawDoc) (ChunkResult, error) {
	if doc == nil {
		return ChunkResult{}, nil
	}

	content := doc.Content()
	if content == "" {
		return ChunkResult{}, nil
	}

	dataKind := d.detectDataKind(doc.FileName())

	// log 是非结构化文本，直接按行切分
	if strings.EqualFold(dataKind, "Log") {
		chunks := d.chunkByLines(doc, content, dataKind)
		// fillDatumSummary(chunks)
		chunks = enrichChunksMetadata(chunks, content, doc.FileName())
		return ChunkResult{Chunks: chunks}, nil
	}

	// 优先按 tree-sitter 结构化切分
	if chunks := d.tryChunkStructured(doc, content, dataKind); chunks != nil {
		// 按数据路径前缀回填 ParentID，形成分块树
		setDatumParentIDs(chunks, dataKind)
		nodes, edges := buildDatumGraph(doc, chunks, dataKind)
		// fillDatumSummary(chunks)
		chunks = enrichChunksMetadata(chunks, content, doc.FileName())
		return ChunkResult{
			Chunks: chunks,
			Nodes:  nodes,
			Edges:  edges,
		}, nil
	}

	// 解析失败：降级为按行切分
	chunks := d.chunkByLines(doc, content, dataKind)
	// fillDatumSummary(chunks)
	chunks = enrichChunksMetadata(chunks, content, doc.FileName())
	return ChunkResult{Chunks: chunks}, nil
}

// // fillDatumSummary 为数据分块统一填充 Summary。
// //
// // 优先使用 deriveSummary 提取自然语言句子；若数据内容无句子结束符，
// // 则取前 80 个字符作为摘要，避免数据类分片摘要为空。
// func fillDatumSummary(chunks []core.Chunk) {
// 	// NOTE: 此方法是废方法，不进行默认的截断开的Summary
// }

// truncateString 按 UTF-8 字符截断字符串到最大长度，避免切出乱码。
// func truncateString(s string, maxRunes int) string {
// 	if maxRunes <= 0 {
// 		return ""
// 	}
// 	runes := []rune(s)
// 	if len(runes) <= maxRunes {
// 		return s
// 	}
// 	return string(runes[:maxRunes])
// }

// detectDataKind 根据文件扩展名返回数据类型标签（用于 Chunk.Title）。
func (d *DatumChunker) detectDataKind(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".csv":
		return "CSV"
	case ".json":
		return "JSON"
	case ".xml":
		return "XML"
	case ".xls", ".xlsx":
		return "Excel"
	case ".eml", ".msg":
		return "Email"
	case ".yaml", ".yml":
		return "YAML"
	case ".toml":
		return "TOML"
	case ".log":
		return "Log"
	default:
		if ext != "" {
			return strings.ToUpper(strings.TrimPrefix(ext, "."))
		}
		return "Data"
	}
}

// tryChunkStructured 用 tree-sitter-yaml 解析内容并递归切分；解析失败返回 nil。
//
// JSON 是 YAML 的子集，yaml 语法能解析 JSON 字符串；同时原生支持 YAML block_* 节点。
func (d *DatumChunker) tryChunkStructured(doc document.RawDoc, content, dataKind string) []core.Chunk {
	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(yaml.GetLanguage())
	ctx := context.Background()
	tree, err := parser.ParseCtx(ctx, nil, src)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()

	top := findTopContainer(tree.RootNode())
	if top == nil {
		return nil
	}

	state := &datumWalkState{doc: doc, src: src, dataKind: dataKind}
	state.walk(top, "", 0)
	state.flushMisc()

	if len(state.chunks) == 0 {
		return nil
	}
	return state.chunks
}

// findTopContainer 从 stream → document → value_node → container 找到顶层容器节点。
//
// YAML AST 结构：
//
//	stream
//	  document
//	    flow_node | block_node
//	      flow_mapping | flow_sequence | block_mapping | block_sequence
func findTopContainer(root *sitter.Node) *sitter.Node {
	if root == nil || root.Type() != "stream" {
		return nil
	}
	if root.NamedChildCount() == 0 {
		return nil
	}
	docNode := root.NamedChild(0)
	if docNode == nil || docNode.Type() != "document" {
		return nil
	}
	if docNode.NamedChildCount() == 0 {
		return nil
	}
	valueNode := docNode.NamedChild(0)
	inner := unwrapValueNode(valueNode)
	if inner == nil {
		return nil
	}
	if !isContainer(inner) {
		return nil
	}
	return inner
}

// unwrapValueNode 解包 flow_node/block_node，返回内部结构节点。
// flow_node / block_node 是值节点的包装，其第一个 named child 才是真正的结构。
func unwrapValueNode(n *sitter.Node) *sitter.Node {
	if n == nil {
		return nil
	}
	switch n.Type() {
	case "flow_node", "block_node":
		if n.NamedChildCount() == 0 {
			return n
		}
		return n.NamedChild(0)
	default:
		return n
	}
}

// isContainer 判断节点是否为对象或数组容器。
func isContainer(n *sitter.Node) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "flow_mapping", "flow_sequence", "block_mapping", "block_sequence":
		return true
	}
	return false
}

// scalarText 提取标量节点的纯文本，去除引号。
func scalarText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	// node 可能是 flow_node 包装的标量，取其内容
	inner := unwrapValueNode(node)
	if inner != nil && inner != node {
		node = inner
	}
	text := strings.TrimSpace(string(node.Content(src)))
	// 去掉双引号或单引号
	if len(text) >= 2 {
		first, last := text[0], text[len(text)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			text = text[1 : len(text)-1]
		}
	}
	return text
}

// joinPath 拼接属性路径，根路径下不加点。
func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// datumWalkState 数据分块递归状态，收集切分结果与「其余字段」缓冲。
type datumWalkState struct {
	doc       document.RawDoc
	src       []byte
	dataKind  string
	chunks    []core.Chunk
	miscBuf   strings.Builder
	miscStart int
	miscEnd   int
	miscCount int
}

// walk 递归遍历容器节点，按对象边界切分。
func (s *datumWalkState) walk(node *sitter.Node, path string, depth int) {
	if node == nil {
		return
	}
	if depth >= datumMaxDepth {
		// 超过最大深度：作为整体块
		s.emitChunk(path, node)
		return
	}

	switch node.Type() {
	case "flow_mapping", "block_mapping":
		s.walkMapping(node, path, depth)
	case "flow_sequence", "block_sequence":
		s.walkSequence(node, path, depth)
	default:
		// 标量或其他类型：直接作为一个 Chunk
		s.emitChunk(path, node)
	}
}

// walkMapping 遍历对象的所有键值对，对象/数组属性切分为子块，标量属性合并到 misc。
func (s *datumWalkState) walkMapping(node *sitter.Node, path string, depth int) {
	pairType := "flow_pair"
	if node.Type() == "block_mapping" {
		pairType = "block_mapping_pair"
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(i)
		if pair == nil || pair.Type() != pairType {
			continue
		}
		keyNode := pair.ChildByFieldName("key")
		valueNode := pair.ChildByFieldName("value")
		key := scalarText(keyNode, s.src)
		childPath := joinPath(path, key)

		inner := unwrapValueNode(valueNode)
		if isContainer(inner) {
			size := int(inner.EndByte() - inner.StartByte())
			if size > datumMaxChunkSize && depth+1 < datumMaxDepth {
				// 大对象/数组：递归切分
				s.walk(inner, childPath, depth+1)
			} else {
				// 小对象/数组：作为独立 Chunk
				s.emitChunk(childPath, inner)
			}
		} else {
			// 标量属性：合并到 misc
			s.appendMisc(pair)
		}
	}
}

// walkSequence 遍历数组的所有元素，对象元素切分为子块，标量元素合并到 misc。
func (s *datumWalkState) walkSequence(node *sitter.Node, path string, depth int) {
	itemType := "flow_node"
	if node.Type() == "block_sequence" {
		itemType = "block_sequence_item"
	}

	elemIdx := 0
	for i := 0; i < int(node.NamedChildCount()); i++ {
		elem := node.NamedChild(i)
		if elem == nil || elem.Type() != itemType {
			continue
		}
		childPath := fmt.Sprintf("%s[%d]", path, elemIdx)
		elemIdx++

		var inner *sitter.Node
		if elem.Type() == "flow_node" {
			inner = unwrapValueNode(elem)
		} else {
			// block_sequence_item 内含 block_node
			if elem.NamedChildCount() > 0 {
				inner = unwrapValueNode(elem.NamedChild(0))
			}
		}

		if isContainer(inner) {
			size := int(inner.EndByte() - inner.StartByte())
			if size > datumMaxChunkSize && depth+1 < datumMaxDepth {
				s.walk(inner, childPath, depth+1)
			} else {
				s.emitChunk(childPath, inner)
			}
		} else {
			s.appendMisc(elem)
		}
	}
}

// emitChunk 输出一个独立子块，Title 带 dataKind 前缀和属性路径。
func (s *datumWalkState) emitChunk(path string, node *sitter.Node) {
	if node == nil {
		return
	}
	body := strings.TrimSpace(string(node.Content(s.src)))
	if body == "" {
		return
	}

	title := s.dataKind
	if path != "" {
		title = s.dataKind + "." + path
	}
	s.chunks = append(s.chunks, buildChunk(
		s.doc,
		len(s.chunks),
		int(node.StartByte()),
		int(node.StartByte())+len(body),
		title,
		body,
	))
}

// appendMisc 将标量 pair/元素追加到「其余字段」缓冲，超过阈值时自动 flush。
func (s *datumWalkState) appendMisc(node *sitter.Node) {
	if node == nil {
		return
	}
	if s.miscBuf.Len() == 0 {
		s.miscStart = int(node.StartByte())
	}
	if s.miscBuf.Len() > 0 {
		s.miscBuf.WriteString("\n")
	}
	s.miscBuf.WriteString(string(node.Content(s.src)))
	s.miscEnd = int(node.EndByte())

	// 防止 misc 块过大，达到阈值时 flush
	if s.miscBuf.Len() >= datumMaxChunkSize {
		s.flushMisc()
	}
}

// flushMisc 将累积的「其余字段」缓冲输出为一个 Chunk。
func (s *datumWalkState) flushMisc() {
	if s.miscBuf.Len() == 0 {
		return
	}
	s.miscCount++
	title := s.dataKind + ".其余字段"
	if s.miscCount > 1 {
		title = fmt.Sprintf("%s.其余字段%d", s.dataKind, s.miscCount)
	}
	body := strings.TrimRight(s.miscBuf.String(), "\n")
	s.chunks = append(s.chunks, buildChunk(
		s.doc,
		len(s.chunks),
		s.miscStart,
		s.miscStart+len(body),
		title,
		body,
	))
	s.miscBuf.Reset()
}

// chunkByLines 按行切分（Log 等非结构化数据兜底）。
func (d *DatumChunker) chunkByLines(doc document.RawDoc, content, dataKind string) []core.Chunk {
	lines := strings.Split(content, "\n")
	var chunks []core.Chunk

	var buf strings.Builder
	var bufStartLine = 0
	var startByte = 0
	var curByte = 0

	flush := func(endLine, endByte int) {
		if buf.Len() == 0 {
			return
		}
		title := fmt.Sprintf("%s 行 %d-%d", dataKind, bufStartLine, endLine-1)
		body := strings.TrimRight(buf.String(), "\n")
		chunks = append(chunks, buildChunk(doc, len(chunks), startByte, startByte+len(body), title, body))
		buf.Reset()
		bufStartLine = endLine
		startByte = endByte
	}

	for i, line := range lines {
		lineLen := len(line) + 1 // +1 for \n
		if buf.Len() > 0 && buf.Len()+lineLen > datumMaxChunkSize {
			flush(i, curByte)
		}
		if buf.Len() == 0 {
			bufStartLine = i
			startByte = curByte
		}
		buf.WriteString(line)
		buf.WriteString("\n")
		curByte += lineLen
	}
	flush(len(lines), curByte)

	if len(chunks) == 0 {
		title := fmt.Sprintf("%s 行 0-%d", dataKind, len(lines)-1)
		chunks = append(chunks, buildChunk(doc, 0, 0, len(content), title, content))
	}
	return chunks
}

// setDatumParentIDs 根据数据路径前缀为每个 Chunk 设置 ParentID。
//
// 规则：
//   - JSON.users[0].name 的父级是 JSON.users[0]
//   - JSON.users[0] 的父级是 JSON.users
//   - JSON.users 的父级是 JSON（文档根，ParentID 为空）
//   - 「其余字段」没有实际父 chunk，ParentID 为空
func setDatumParentIDs(chunks []core.Chunk, dataKind string) {
	if len(chunks) == 0 {
		return
	}
	titleToIdx := map[string]int{}
	for i, c := range chunks {
		titleToIdx[c.Title] = i
	}

	for i := range chunks {
		title := chunks[i].Title
		if title == dataKind || strings.HasPrefix(title, dataKind+".其余字段") {
			continue
		}
		parentTitle := deriveDatumParentTitle(title, dataKind)
		if parentTitle == "" {
			continue
		}
		if parentIdx, ok := titleToIdx[parentTitle]; ok {
			chunks[i].ParentID = chunks[parentIdx].ID
		}
	}
}

// deriveDatumParentTitle 根据子路径推导父路径标题。
//
// 示例：
//   - JSON.users[0].name -> JSON.users[0]
//   - JSON.users[0] -> JSON.users
//   - JSON.config.debug -> JSON.config
func deriveDatumParentTitle(title, dataKind string) string {
	if !strings.HasPrefix(title, dataKind+".") {
		return ""
	}
	path := strings.TrimPrefix(title, dataKind+".")
	if path == "" {
		return ""
	}

	lastDot := strings.LastIndex(path, ".")
	lastBracket := strings.LastIndex(path, "[")
	lastSep := lastDot
	if lastBracket > lastSep {
		lastSep = lastBracket
	}
	if lastSep <= 0 {
		return dataKind
	}
	return dataKind + "." + path[:lastSep]
}

// buildDatumGraph 根据数据分块的标题层级构建 Data 节点与 CONTAINS 边。
//
// 规则：
//   - dataKind 作为 Document 根节点
//   - 每个非「其余字段」的数据块作为一个 Data 节点
//   - 父路径包含子路径（如 JSON.users 包含 JSON.users[0]）
func buildDatumGraph(doc document.RawDoc, chunks []core.Chunk, dataKind string) ([]core.Node, []core.Edge) {
	docNode := buildNode(doc, dataKind, []string{"Document"}, "", map[string]any{"node_type": "data_kind"})

	nodes := []core.Node{docNode}
	edges := []core.Edge{}

	// title -> nodeID
	nodeIDByTitle := map[string]string{dataKind: docNode.ID}
	// title -> chunkID
	chunkIDByTitle := map[string]string{}

	// 第一步：为每个非「其余字段」的数据块创建节点
	for _, chunk := range chunks {
		title := chunk.Title
		if title == "" || strings.HasPrefix(title, dataKind+".其余字段") {
			continue
		}
		label := "Data"
		nodeType := "field"
		path := ""
		if strings.HasPrefix(title, dataKind+".") {
			path = strings.TrimPrefix(title, dataKind+".")
		}
		if path == "" {
			continue // dataKind 自身已由 docNode 表示
		}
		if strings.Contains(path, "[") {
			nodeType = "record"
		} else if hasChildPath(chunks, title) {
			nodeType = "object"
		}
		if nodeType == "object" || nodeType == "record" {
			label = "Collection"
		}

		node := buildNode(doc, title, []string{label}, chunk.ID, map[string]any{
			"node_type": nodeType,
			"path":      path,
		})
		nodes = append(nodes, node)
		nodeIDByTitle[title] = node.ID
		chunkIDByTitle[title] = chunk.ID
	}

	// 第二步：根据路径前缀构建 CONTAINS 边
	for _, chunk := range chunks {
		childTitle := chunk.Title
		if childTitle == "" || strings.HasPrefix(childTitle, dataKind+".其余字段") {
			continue
		}
		parentTitle := deriveDatumParentTitle(childTitle, dataKind)
		if parentTitle == "" {
			continue
		}
		childID, ok1 := nodeIDByTitle[childTitle]
		parentID, ok2 := nodeIDByTitle[parentTitle]
		if !ok1 || !ok2 {
			continue
		}
		edges = append(edges, buildEdge(doc, parentID, childID, "CONTAINS", []string{
			chunkIDByTitle[parentTitle],
			chunkIDByTitle[childTitle],
		}))
	}

	return nodes, edges
}

// hasChildPath 判断是否存在以当前 title 为前缀的子路径 chunk。
func hasChildPath(chunks []core.Chunk, title string) bool {
	prefix := title + "."
	for _, c := range chunks {
		if strings.HasPrefix(c.Title, prefix) {
			return true
		}
	}
	return false
}

// _ 确保 core 包被引用
var _ = core.Chunk{}
