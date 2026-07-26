package chunker

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/DotNetAge/gorag/v2/chunker/vue"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/swift"
	tstsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	tstypescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// =====================================================================
// CodeChunker：基于 go-tree-sitter 的代码分块器
// =====================================================================
//
// 设计要点：
//   - 按文件扩展名路由到对应 tree-sitter Language
//   - 用 Query 查询语言中的「符号定义节点」（函数/方法/类/结构体/接口等）
//   - 每个定义节点作为一个 Chunk，Chunk.Title 取符号名
//   - 第一个定义之前的内容（import/package/注释等）作为独立 Chunk
//   - 不支持的语言走纯文本兜底（按段落切分，每块不超过 codeMaxChunkSize 字符）
//   - 解析失败或无符号匹配时整体作为单 Chunk
//   - Chunk.StartPos/EndPos 为字节偏移

// codeMaxChunkSize 代码纯文本兜底分块的最大字符数。
const codeMaxChunkSize = 1500

var (
	// goPackageRegexp 匹配 Go 的 package 声明。
	goPackageRegexp = regexp.MustCompile(`(?m)^\s*package\s+(\w+)\s*$`)
	// javaPackageRegexp 匹配 Java/Kotlin 的 package 声明。
	javaPackageRegexp = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)\s*;?\s*$`)
)

// codeSymbol 表示一个 tree-sitter 查询到的代码符号定义节点。
// 用于 CodeChunker 内部收集符号信息后排序与去重。
type codeSymbol struct {
	start         uint32 // 节点起始字节偏移
	end           uint32 // 节点结束字节偏移
	name          string // 符号名（来自 @name capture）
	qualifiedName string // 带作用域的符号名（如 Person.Greet、Animal.__init__）
	nodeType      string // tree-sitter 节点类型（如 function/class）
	summary       string // 符号前的注释或函数体内的 docstring
	signature     string // 符号定义的第一行（签名）
	visibility    string // 可见性（public/private/protected/exported/unexported 等）
	receiver      string // Go 方法 receiver 类型（仅方法有效）
}

// languageSpec 描述一种 tree-sitter 语言的查询规则。
//
//   - lang：tree-sitter Language 指针
//   - query：用于匹配「符号定义节点」的 Query 字符串
//   - defCapture：定义节点 capture 名（默认 "def"）
//   - nameCapture：符号名 capture 名（默认 "name"）
type languageSpec struct {
	lang        *sitter.Language
	query       string
	defCapture  string
	nameCapture string
}

// codeLangRegistry 扩展名 → languageSpec 的路由表。
//
// 每种语言的 Query 同时覆盖函数/方法/类/结构体/接口/枚举/trait 等顶层定义节点，
// 通过 @def capture 标记定义节点本身，@name capture 标记符号名节点。
// 当节点没有合适的符号名子节点时，@name capture 可以省略，CodeChunker 会回退到
// 「取定义节点首行去掉关键字」作为标题。
//
// 注意：`.h` / `.cc` / `.cxx` / `.hpp` / `.hh` 等扩展名通过 spec 函数返回独立副本，
// 不能在初始化时通过 `codeLangRegistry[".c"]` 读取（会引发初始化循环）。
var codeLangRegistry = func() map[string]languageSpec {
	goSpec := languageSpec{
		lang: golang.GetLanguage(),
		query: `
			(function_declaration name: (identifier) @name) @def
			(method_declaration name: (field_identifier) @name) @def
			(type_spec name: (type_identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	cSpec := languageSpec{
		lang: c.GetLanguage(),
		query: `
			(function_definition declarator: (function_declarator declarator: (identifier) @name)) @def
			(struct_specifier name: (type_identifier) @name) @def
			(enum_specifier name: (type_identifier) @name) @def
			(declaration declarator: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	m := map[string]languageSpec{
		// ---- Go ----
		".go": goSpec,
		// ---- C ----
		".c": cSpec,
		".h": cSpec,
	}
	// 注册其他扩展名（覆盖 js/cpp/bash 等多扩展名复用同一 spec 的场景）
	js := jsSpec()
	m[".js"] = js
	m[".jsx"] = js
	m[".mjs"] = js
	m[".cjs"] = js
	cpp := cppSpec()
	m[".cpp"] = cpp
	m[".cc"] = cpp
	m[".cxx"] = cpp
	m[".hpp"] = cpp
	m[".hh"] = cpp
	bashS := bashSpec()
	m[".sh"] = bashS
	m[".bash"] = bashS
	return m
}()

// 其他语言（单扩展名）通过 init 注册，避免 var 初始化顺序问题。
func init() {
	// ---- Python ----
	codeLangRegistry[".py"] = languageSpec{
		lang: python.GetLanguage(),
		query: `
			(function_definition name: (identifier) @name) @def
			(class_definition name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- TypeScript ----
	codeLangRegistry[".ts"] = languageSpec{
		lang: tstypescript.GetLanguage(),
		query: `
			(function_declaration name: (identifier) @name) @def
			(class_declaration name: (type_identifier) @name) @def
			(interface_declaration name: (type_identifier) @name) @def
			(method_definition name: (property_identifier) @name) @def
			(variable_declarator name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	codeLangRegistry[".tsx"] = languageSpec{
		lang: tstsx.GetLanguage(),
		query: `
			(function_declaration name: (identifier) @name) @def
			(class_declaration name: (type_identifier) @name) @def
			(interface_declaration name: (type_identifier) @name) @def
			(method_definition name: (property_identifier) @name) @def
			(variable_declarator name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- Rust ----
	codeLangRegistry[".rs"] = languageSpec{
		lang: rust.GetLanguage(),
		query: `
			(function_item name: (identifier) @name) @def
			(struct_item name: (type_identifier) @name) @def
			(enum_item name: (type_identifier) @name) @def
			(trait_item name: (type_identifier) @name) @def
			(impl_item type: (type_identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- Ruby ----
	codeLangRegistry[".rb"] = languageSpec{
		lang: ruby.GetLanguage(),
		query: `
			(method name: (identifier) @name) @def
			(singleton_method name: (identifier) @name) @def
			(class name: (constant) @name) @def
			(module name: (constant) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- Java ----
	codeLangRegistry[".java"] = languageSpec{
		lang: java.GetLanguage(),
		query: `
			(method_declaration name: (identifier) @name) @def
			(class_declaration name: (identifier) @name) @def
			(interface_declaration name: (identifier) @name) @def
			(constructor_declaration name: (identifier) @name) @def
			(enum_declaration name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- Kotlin ----
	codeLangRegistry[".kt"] = languageSpec{
		lang: kotlin.GetLanguage(),
		query: `
			(function_declaration (simple_identifier) @name) @def
			(class_declaration (type_identifier) @name) @def
			(object_declaration (type_identifier) @name) @def
			(interface_declaration (type_identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- Scala ----
	codeLangRegistry[".scala"] = languageSpec{
		lang: scala.GetLanguage(),
		query: `
			(function_definition name: (identifier) @name) @def
			(class_definition name: (identifier) @name) @def
			(object_definition name: (identifier) @name) @def
			(trait_definition name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- Swift ----
	codeLangRegistry[".swift"] = languageSpec{
		lang: swift.GetLanguage(),
		query: `
			(function_declaration name: (identifier) @name) @def
			(class_declaration name: (type_identifier) @name) @def
			(struct_declaration name: (type_identifier) @name) @def
			(protocol_declaration name: (type_identifier) @name) @def
			(enum_declaration name: (type_identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- C# ----
	codeLangRegistry[".cs"] = languageSpec{
		lang: csharp.GetLanguage(),
		query: `
			(method_declaration name: (identifier) @name) @def
			(class_declaration name: (identifier) @name) @def
			(interface_declaration name: (identifier) @name) @def
			(struct_declaration name: (identifier) @name) @def
			(enum_declaration name: (identifier) @name) @def
			(constructor_declaration name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- PHP ----
	codeLangRegistry[".php"] = languageSpec{
		lang: php.GetLanguage(),
		query: `
			(function_definition name: (name) @name) @def
			(class_declaration name: (name) @name) @def
			(interface_declaration name: (name) @name) @def
			(method_declaration name: (name) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
	// ---- Vue ----
	codeLangRegistry[".vue"] = languageSpec{
		lang: vue.GetLanguage(),
		query: `
			(template_element) @def
			(script_element) @def
			(style_element) @def
		`,
		defCapture:  "def",
		nameCapture: "",
	}
	// ---- Lua ----
	codeLangRegistry[".lua"] = languageSpec{
		lang: lua.GetLanguage(),
		query: `
			(function_declaration name: (identifier) @name) @def
			(function_definition name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
}

// jsSpec 返回 JavaScript 语言的 languageSpec。
func jsSpec() languageSpec {
	return languageSpec{
		lang: javascript.GetLanguage(),
		query: `
			(function_declaration name: (identifier) @name) @def
			(class_declaration name: (identifier) @name) @def
			(method_definition name: (property_identifier) @name) @def
			(variable_declarator name: (identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
}

// cppSpec 返回 C++ 语言的 languageSpec。
func cppSpec() languageSpec {
	return languageSpec{
		lang: cpp.GetLanguage(),
		query: `
			(function_definition declarator: (function_declarator declarator: [
				(identifier) @name
				(qualified_identifier name: (identifier) @name)
				(field_identifier) @name
			])) @def
			(class_specifier name: (type_identifier) @name) @def
			(struct_specifier name: (type_identifier) @name) @def
			(enum_specifier name: (type_identifier) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
}

// bashSpec 返回 Bash 语言的 languageSpec。
func bashSpec() languageSpec {
	return languageSpec{
		lang: bash.GetLanguage(),
		query: `
			(function_definition name: (word) @name) @def
		`,
		defCapture:  "def",
		nameCapture: "name",
	}
}

// CodeChunker 代码分块器：基于 tree-sitter 按符号定义边界切分。
//
// 适用 RawDocType：RawDocText（.go/.py/.js/.ts/.java/.c/.cpp 等代码文件，及纯文本兜底）。
type CodeChunker struct{}

// NewCodeChunker 创建代码分块器。
func NewCodeChunker() *CodeChunker { return &CodeChunker{} }

// Chunk 实现 Chunker 接口：按 tree-sitter 识别的符号定义节点切分代码，
// 同时产出符号层级结构对应的 Nodes/Edges。
func (c *CodeChunker) Chunk(doc document.RawDoc) (ChunkResult, error) {
	if doc == nil {
		return ChunkResult{}, nil
	}

	content := doc.Content()
	if content == "" {
		return ChunkResult{}, nil
	}

	ext := strings.ToLower(filepath.Ext(doc.FileName()))
	spec, ok := codeLangRegistry[ext]
	if !ok {
		// 非代码文件：委托给 MarkdownChunker
		return (&MarkdownChunker{}).Chunk(doc)
	}

	// 1. 用 tree-sitter 解析代码
	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(spec.lang)
	ctx := context.Background()
	tree, err := parser.ParseCtx(ctx, nil, src)
	if err != nil || tree == nil {
		// 解析失败：委托给 MarkdownChunker
		return (&MarkdownChunker{}).Chunk(doc)
	}
	defer tree.Close()

	// 2. 构造 Query
	q, err := sitter.NewQuery([]byte(spec.query), spec.lang)
	if err != nil {
		// Query 构造失败：委托给 MarkdownChunker
		return (&MarkdownChunker{}).Chunk(doc)
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	// 3. 收集所有匹配到的定义节点（按字节偏移升序，去重）
	seen := map[uintptr]bool{}
	var symbols []codeSymbol

	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		// 同一 match 内 @def 与 @name 配对出现
		var defNode *sitter.Node
		var nameNode *sitter.Node
		for _, cap := range match.Captures {
			switch q.CaptureNameForId(cap.Index) {
			case spec.defCapture:
				defNode = cap.Node
			case spec.nameCapture:
				nameNode = cap.Node
			}
		}
		if defNode == nil {
			continue
		}
		// 去重：同一节点可能被多个 pattern 命中
		if seen[defNode.ID()] {
			continue
		}
		seen[defNode.ID()] = true

		name := ""
		if nameNode != nil {
			name = strings.TrimSpace(nameNode.Content(src))
		}
		if name == "" {
			// 回退：取定义节点首行非空内容作为名称
			name = firstNonEmptyLine(defNode.Content(src))
		}
		symbols = append(symbols, codeSymbol{
			start:    defNode.StartByte(),
			end:      defNode.EndByte(),
			name:     name,
			nodeType: normalizeNodeType(defNode.Type()),
		})
	}

	// 4. 没有匹配到任何符号：整体作为单块
	if len(symbols) == 0 {
		title := deriveTitle(doc.FileName())
		chunk := buildChunk(doc, 0, 0, len(content), title, content)
		chunks := enrichChunksMetadata([]core.Chunk{chunk}, content, doc.FileName())
		return ChunkResult{Chunks: chunks}, nil
	}

	// 5. 按字节偏移排序（Query 通常已按文档顺序返回，但保险起见排序一次）
	sortSymbolsByStart(symbols)

	// 5.1 提取每个符号的注释/docstring 作为 Summary（优先），无注释时 fallback 到内容摘要
	//     先走手搓 fallback（Go/Python 命名约定类只能手搓）
	for i, sym := range symbols {
		prevEnd := uint32(0)
		if i > 0 {
			prevEnd = symbols[i-1].end
		}
		symbols[i].summary = extractSymbolSummary(content, sym, prevEnd, tree.RootNode(), spec.lang)
		symbols[i].signature = extractSymbolSignature(content, sym)
		symbols[i].visibility = extractSymbolVisibility(content, sym)
	}

	// 5.2 用 AST 覆盖签名和可见性（精确提取，多行签名正确）
	extractSignaturesFromAST(content, symbols, spec, tree.RootNode())
	extractVisibilityFromAST(content, symbols, spec, tree.RootNode())

	var chunks []core.Chunk
	symbolChunkIDs := make([]string, len(symbols)) // 记录每个 symbol 对应的 chunkID

	// 6. 第一个符号之前的内容（import/package/注释等）作为独立 Chunk
	firstStart := int(symbols[0].start)
	if firstStart > 0 {
		pre := strings.TrimSpace(content[:firstStart])
		if pre != "" {
			title := deriveTitle(doc.FileName()) + " (header)"
			chunks = append(chunks, buildChunk(doc, len(chunks), 0, len(pre), title, pre))
		}
	}

	// 7. 按符号边界切分：每个 Chunk 从当前符号 start 到下一个符号 start（或文档末尾）
	//    同时用栈维护父符号，回填每个 Chunk 的 ParentID
	type parentItem struct {
		start, end uint32
		idx        int
	}
	parentStack := []parentItem{{start: 0, end: 0, idx: -1}} // -1 表示文档根

	for i, sym := range symbols {
		start := int(sym.start)
		var end int
		if i+1 < len(symbols) {
			end = int(symbols[i+1].start)
		} else {
			end = len(content)
		}

		body := strings.TrimRight(content[start:end], "\n\r")
		if body == "" {
			continue
		}

		title := sym.name
		if sym.signature != "" {
			title = sym.signature
		}
		if title == "" {
			title = deriveTitle(doc.FileName())
		}

		// 找到包含当前符号的父 chunk
		for len(parentStack) > 0 {
			top := parentStack[len(parentStack)-1]
			if top.idx < 0 {
				break
			}
			if sym.start >= top.start && sym.end <= top.end {
				break
			}
			parentStack = parentStack[:len(parentStack)-1]
		}
		parent := parentStack[len(parentStack)-1]

		chunk := buildChunk(doc, len(chunks), start, start+len(body), title, body)
		chunk.Summary = sym.summary
		if chunk.Metadata == nil {
			chunk.Metadata = map[string]any{}
		}
		chunk.Metadata[core.MetaSymbolType] = sym.nodeType
		chunk.Metadata[core.MetaSignature] = sym.signature
		chunk.Metadata[core.MetaVisibility] = sym.visibility
		if sym.receiver != "" {
			chunk.Metadata[core.MetaReceiver] = sym.receiver
		}
		if parent.idx >= 0 {
			chunk.ParentID = chunks[parent.idx].ID
		}

		chunkIdx := len(chunks)
		chunks = append(chunks, chunk)
		symbolChunkIDs[i] = chunk.ID
		parentStack = append(parentStack, parentItem{start: sym.start, end: sym.end, idx: chunkIdx})
	}

	if len(chunks) == 0 {
		title := deriveTitle(doc.FileName())
		chunk := buildChunk(doc, 0, 0, len(content), title, content)
		return ChunkResult{Chunks: []core.Chunk{chunk}}, nil
	}

	// 8. 提前提取 Go 方法 receiver，供 Node.Properties 与后续边生成使用
	extractGoMethodReceivers(doc, content, symbols)

	// 9. 构建符号层级结构对应的 Nodes/Edges（同时回填 qualifiedName）
	nodes, edges, symbolNodeIDs := buildCodeGraph(doc, content, symbols, symbolChunkIDs, tree.RootNode(), spec.lang)

	// 10. 提取 CONTAINS 之外的额外代码关系边（如 Go 方法的 BELONGS_TO）
	edges = append(edges, buildGoBelongsToEdges(doc, symbols, symbolChunkIDs, symbolNodeIDs, nodes)...)

	// 11. 提取 CALLS / INHERITS / IMPLEMENTS 等跨符号关系
	relations := extractCodeRelations(doc, content, symbols)
	edges = append(edges, buildCodeRelationEdges(doc, relations, symbols, symbolChunkIDs, symbolNodeIDs)...)

	// 12. 回填 package 元数据（Go/Java/Kotlin 等）
	pkg := extractPackageName(doc.FileName(), content, tree.RootNode(), spec.lang)
	if pkg != "" {
		for i := range chunks {
			if chunks[i].Metadata == nil {
				chunks[i].Metadata = map[string]any{}
			}
			chunks[i].Metadata[core.MetaPackage] = pkg
		}
	}

	// 13. 统一 enriched 通用元数据（行号、语言、目录等）
	chunks = enrichChunksMetadata(chunks, content, doc.FileName())

	return ChunkResult{
		Chunks: chunks,
		Nodes:  nodes,
		Edges:  edges,
	}, nil
}

// sortSymbolsByStart 按起始字节偏移升序排序符号列表（插入排序，规模通常很小）。
func sortSymbolsByStart(s []codeSymbol) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1].start > s[j].start; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// buildCodeGraph 根据符号定义的字节范围构建代码结构 Nodes/Edges。
//
// 规则：
//   - 每个符号对应一个 Node，标签由 tree-sitter 节点类型推导（Function/Method/Class/Interface/Type 等）
//   - 文档节点作为根，包含所有顶层符号
//   - 父符号包含位于其字节范围内的子符号（如 class 包含 method）
//   - Node.ID 使用代码作用域（如 package 名）生成，使同一 package 的跨文件同名实体可合并
//
// 返回值 symbolNodeIDs 与 symbols 一一对应，表示每个符号生成的 Node ID。
func buildCodeGraph(doc document.RawDoc, content string, symbols []codeSymbol, symbolChunkIDs []string, root *sitter.Node, lang *sitter.Language) ([]core.Node, []core.Edge, []string) {
	docTitle := deriveTitle(doc.FileName())
	docProps := map[string]any{
		"node_type": "document",
		"language":  deriveLanguage(doc.FileName()),
	}
	pkg := extractPackageName(doc.FileName(), content, root, lang)
	if pkg != "" {
		docProps[core.PropPackage] = pkg
	}
	// Document 节点保持文档级作用域；符号节点使用 package/模块作用域
	docNode := buildNode(doc, docTitle, []string{"Document"}, "", docProps)
	scope := pkg // Go/Java/Kotlin 使用 package 作为作用域，其他语言为空（fallback 到 docID）

	nodes := []core.Node{docNode}
	edges := []core.Edge{}
	symbolNodeIDs := make([]string, len(symbols))

	// stack 维护当前祖先链，每个元素为 (end, symbolIdx, nodeID, chunkID)
	type stackItem struct {
		end       uint32
		symbolIdx int
		nodeID    string
		chunkID   string
	}
	stack := []stackItem{{end: 0, symbolIdx: -1, nodeID: docNode.ID, chunkID: ""}}

	for i, sym := range symbols {
		chunkID := ""
		if i < len(symbolChunkIDs) {
			chunkID = symbolChunkIDs[i]
		}

		// 找到合适的父节点：弹出直到栈顶范围包含当前符号
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			// 文档节点（end=0）作为根，总是包含顶层符号
			if top.end == 0 {
				break
			}
			// 当前符号在栈顶范围内
			if sym.start >= 0 && sym.end <= top.end {
				break
			}
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]

		// 计算带作用域的符号名
		qualifiedName := sym.name
		if sym.receiver != "" {
			qualifiedName = sym.receiver + "." + sym.name
		} else if parent.symbolIdx >= 0 {
			parentQualified := symbols[parent.symbolIdx].qualifiedName
			if parentQualified != "" {
				qualifiedName = parentQualified + "." + sym.name
			}
		}
		symbols[i].qualifiedName = qualifiedName

		label := codeSymbolLabel(sym.nodeType)
		props := map[string]any{
			"node_type":          sym.nodeType,
			core.PropSignature:  sym.signature,
			core.PropVisibility: sym.visibility,
			"language":           deriveLanguage(doc.FileName()),
		}
		if sym.receiver != "" {
			props[core.PropReceiver] = sym.receiver
		}
		node := buildNode(doc, qualifiedName, []string{label}, chunkID, props, scope)
		nodes = append(nodes, node)
		symbolNodeIDs[i] = node.ID

		edges = append(edges, buildEdge(doc, parent.nodeID, node.ID, "CONTAINS", []string{parent.chunkID, chunkID}))

		stack = append(stack, stackItem{end: sym.end, symbolIdx: i, nodeID: node.ID, chunkID: chunkID})
	}

	return nodes, edges, symbolNodeIDs
}

// extractGoMethodReceivers 提前提取 Go 方法的 receiver 类型并回填到 symbols。
//
// 例如 `func (p Person) Greet()` 会把 `Person` 填入对应 symbol 的 receiver 字段。
func extractGoMethodReceivers(doc document.RawDoc, content string, symbols []codeSymbol) {
	if strings.ToLower(filepath.Ext(doc.FileName())) != ".go" {
		return
	}
	spec, ok := codeLangRegistry[".go"]
	if !ok {
		return
	}

	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(spec.lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return
	}
	defer tree.Close()

	queryText := `
		(method_declaration
		  receiver: (parameter_list
			(parameter_declaration
			  type: [
				(type_identifier)
				(pointer_type (type_identifier))
			  ] @receiver))
		  name: (field_identifier) @name) @def
	`
	q, err := sitter.NewQuery([]byte(queryText), spec.lang)
	if err != nil {
		return
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	// 方法起始字节 -> receiver 类型名
	receiverByStart := map[uint32]string{}
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var defNode *sitter.Node
		var receiverName string
		for _, cap := range match.Captures {
			switch q.CaptureNameForId(cap.Index) {
			case "def":
				defNode = cap.Node
			case "receiver":
				receiverName = strings.TrimSpace(cap.Node.Content(src))
			}
		}
		if defNode != nil && receiverName != "" {
			receiverByStart[defNode.StartByte()] = receiverName
		}
	}

	for i, sym := range symbols {
		if receiverName, ok := receiverByStart[sym.start]; ok {
			symbols[i].receiver = receiverName
		}
	}
}

// buildGoBelongsToEdges 根据已回填的 receiver 信息生成 BELONGS_TO 边。
func buildGoBelongsToEdges(doc document.RawDoc, symbols []codeSymbol, symbolChunkIDs, symbolNodeIDs []string, nodes []core.Node) []core.Edge {
	// 类型名 -> NodeID（Go 文件中类型名唯一）
	typeNodeIDs := map[string]string{}
	for _, node := range nodes {
		for _, label := range node.Labels {
			if label == "Class" || label == "Interface" || label == "Type" {
				typeNodeIDs[node.Name] = node.ID
				break
			}
		}
	}

	var extra []core.Edge
	for i, sym := range symbols {
		if sym.receiver == "" {
			continue
		}
		typeNodeID, ok := typeNodeIDs[sym.receiver]
		if !ok {
			continue
		}
		methodChunkID := ""
		if i < len(symbolChunkIDs) {
			methodChunkID = symbolChunkIDs[i]
		}
		methodNodeID := ""
		if i < len(symbolNodeIDs) {
			methodNodeID = symbolNodeIDs[i]
		}
		if methodNodeID == "" {
			continue
		}
		extra = append(extra, buildEdge(doc, methodNodeID, typeNodeID, "BELONGS_TO", []string{methodChunkID, ""}))
	}

	return extra
}

// codeRelation 表示从 AST 中提取出的符号间关系。
//
// sourceSymbolIdx：关系出发的符号在 symbols 中的索引
// targetName：目标符号名（优先使用 qualifiedName；找不到时回退到本地符号名）
// edgeType：关系类型，如 CALLS / INHERITS / IMPLEMENTS / BELONGS_TO
// sourceChunkID / targetChunkID：关系两端的来源 Chunk ID（可能为空）
type codeRelation struct {
	sourceSymbolIdx int
	targetName      string
	edgeType        string
	sourceChunkID   string
	targetChunkID   string
}

// extractCodeRelations 按语言分发，提取 CONTAINS / BELONGS_TO 之外的代码关系。
//
// 当前支持：
//   - Go：CALLS
//   - Python：CALLS
//   - Java / TypeScript / TSX：INHERITS / IMPLEMENTS
//
// 后续可扩展：
//   - Rust：IMPLEMENTS
//   - C#：INHERITS / IMPLEMENTS
//   - 跨文件 IMPORTS
func extractCodeRelations(doc document.RawDoc, content string, symbols []codeSymbol) []codeRelation {
	ext := strings.ToLower(filepath.Ext(doc.FileName()))
	switch ext {
	case ".go":
		return extractGoCalls(content, symbols)
	case ".py":
		return extractPythonCalls(content, symbols)
	case ".java", ".kt":
		return extractJavaInheritance(content, symbols)
	case ".ts", ".tsx":
		return extractTypeScriptInheritance(content, symbols)
	}
	return nil
}

// extractGoCalls 提取 Go 的同文件函数/方法调用关系。
//
// 支持两种调用形式：
//   - 普通函数调用：add(1, 2)
//   - 方法/选择器调用：p.Greet()、fmt.Println
//
// 对选择器调用，会尝试根据变量声明/参数推断 receiver 类型，从而生成 Type.Method 形式的
// 目标名；推断失败时回退到方法名，依赖 findSymbolIndexByName 进行匹配。
// 只创建指向本文件内已定义符号的 CALLS 边；外部调用（如 fmt.Println）会被后续过滤掉。
func extractGoCalls(content string, symbols []codeSymbol) []codeRelation {
	spec, ok := codeLangRegistry[".go"]
	if !ok {
		return nil
	}

	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(spec.lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()

	// 构建变量 -> 类型映射，用于把 p.Greet() 解析为 Person.Greet
	varTypeMap := buildGoVariableTypeMap(tree.RootNode(), src)

	q, err := sitter.NewQuery([]byte(`
		(call_expression function: (identifier) @callee) @call
		(call_expression
		  function: (selector_expression
			operand: (identifier) @recv
			field: (field_identifier) @callee)) @call
	`), spec.lang)
	if err != nil {
		return nil
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	var rels []codeRelation
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var callNode *sitter.Node
		var callee, recv string
		for _, cap := range match.Captures {
			switch q.CaptureNameForId(cap.Index) {
			case "call":
				callNode = cap.Node
			case "callee":
				callee = strings.TrimSpace(cap.Node.Content(src))
			case "recv":
				recv = strings.TrimSpace(cap.Node.Content(src))
			}
		}
		if callNode == nil || callee == "" {
			continue
		}
		sourceIdx := findSymbolIndexContaining(symbols, callNode.StartByte())
		if sourceIdx < 0 {
			continue
		}

		targetName := callee
		if recv != "" {
			if typ, ok := varTypeMap[recv]; ok && typ != "" {
				targetName = typ + "." + callee
			}
		}

		rels = append(rels, codeRelation{
			sourceSymbolIdx: sourceIdx,
			targetName:      targetName,
			edgeType:        "CALLS",
		})
	}

	return rels
}

// extractPythonCalls 提取 Python 的同文件函数/方法调用关系。
//
// 支持：
//   - 普通函数调用：greet("x")
//   - 属性方法调用：self.speak()、a.speak()
//
// 对 self.Method() 会尝试解析为 ClassName.Method()；其他属性调用回退到方法名。
func extractPythonCalls(content string, symbols []codeSymbol) []codeRelation {
	spec, ok := codeLangRegistry[".py"]
	if !ok {
		return nil
	}

	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(spec.lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()

	q, err := sitter.NewQuery([]byte(`
		(call
		  function: (identifier) @callee) @call
		(call
		  function: (attribute
			object: (identifier) @obj
			attribute: (identifier) @callee)) @call
	`), spec.lang)
	if err != nil {
		return nil
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	var rels []codeRelation
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var callNode *sitter.Node
		var callee, obj string
		for _, cap := range match.Captures {
			switch q.CaptureNameForId(cap.Index) {
			case "call":
				callNode = cap.Node
			case "callee":
				callee = strings.TrimSpace(cap.Node.Content(src))
			case "obj":
				obj = strings.TrimSpace(cap.Node.Content(src))
			}
		}
		if callNode == nil || callee == "" {
			continue
		}
		sourceIdx := findSymbolIndexContaining(symbols, callNode.StartByte())
		if sourceIdx < 0 {
			continue
		}

		targetName := callee
		if obj == "self" {
			if cls := parentClassQualifiedName(symbols, sourceIdx); cls != "" {
				targetName = cls + "." + callee
			}
		}

		rels = append(rels, codeRelation{
			sourceSymbolIdx: sourceIdx,
			targetName:      targetName,
			edgeType:        "CALLS",
		})
	}

	return rels
}

// extractJavaInheritance 提取 Java 的 class extends / implements 关系。
func extractJavaInheritance(content string, symbols []codeSymbol) []codeRelation {
	spec, ok := codeLangRegistry[".java"]
	if !ok {
		return nil
	}

	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(spec.lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()

	queryText := `
		(class_declaration
		  name: (identifier) @class
		  superclass: (superclass (type_identifier) @super)) @decl
		(class_declaration
		  name: (identifier) @class
		  interfaces: (super_interfaces (type_list (type_identifier) @iface))) @decl
	`
	q, err := sitter.NewQuery([]byte(queryText), spec.lang)
	if err != nil {
		return nil
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	var rels []codeRelation
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var className, superName, ifaceName string
		var declNode *sitter.Node
		for _, cap := range match.Captures {
			switch q.CaptureNameForId(cap.Index) {
			case "decl":
				declNode = cap.Node
			case "class":
				className = strings.TrimSpace(cap.Node.Content(src))
			case "super":
				superName = strings.TrimSpace(cap.Node.Content(src))
			case "iface":
				ifaceName = strings.TrimSpace(cap.Node.Content(src))
			}
		}
		if declNode == nil || className == "" {
			continue
		}
		sourceIdx := findSymbolIndexByName(symbols, className)
		if sourceIdx < 0 {
			continue
		}
		if superName != "" {
			rels = append(rels, codeRelation{
				sourceSymbolIdx: sourceIdx,
				targetName:      superName,
				edgeType:        "INHERITS",
			})
		}
		if ifaceName != "" {
			rels = append(rels, codeRelation{
				sourceSymbolIdx: sourceIdx,
				targetName:      ifaceName,
				edgeType:        "IMPLEMENTS",
			})
		}
	}

	return rels
}

// extractTypeScriptInheritance 提取 TypeScript 的 extends / implements 关系。
//
// 策略：直接查询 extends_clause / implements_clause，再向上回溯到所属的 class_declaration，
// 避免依赖不同版本 grammar 中 class_heritage 的字段命名差异。
func extractTypeScriptInheritance(content string, symbols []codeSymbol) []codeRelation {
	spec, ok := codeLangRegistry[".ts"]
	if !ok {
		return nil
	}

	src := []byte(content)
	parser := sitter.NewParser()
	parser.SetLanguage(spec.lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()

	queryText := `
		(extends_clause (identifier) @super) @clause
		(implements_clause (type_identifier) @iface) @clause
	`
	q, err := sitter.NewQuery([]byte(queryText), spec.lang)
	if err != nil {
		return nil
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	var rels []codeRelation
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var clauseNode *sitter.Node
		var targetName string
		edgeType := ""
		for _, cap := range match.Captures {
			switch q.CaptureNameForId(cap.Index) {
			case "clause":
				clauseNode = cap.Node
			case "super":
				targetName = strings.TrimSpace(cap.Node.Content(src))
				edgeType = "INHERITS"
			case "iface":
				targetName = strings.TrimSpace(cap.Node.Content(src))
				edgeType = "IMPLEMENTS"
			}
		}
		if clauseNode == nil || targetName == "" || edgeType == "" {
			continue
		}

		classNode := findParentClassDeclaration(clauseNode)
		if classNode == nil {
			continue
		}
		nameNode := classNode.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		className := strings.TrimSpace(nameNode.Content(src))
		sourceIdx := findSymbolIndexByName(symbols, className)
		if sourceIdx < 0 {
			continue
		}
		rels = append(rels, codeRelation{
			sourceSymbolIdx: sourceIdx,
			targetName:      targetName,
			edgeType:        edgeType,
		})
	}

	return rels
}

// findParentClassDeclaration 向上回溯父节点，直到找到 class_declaration。
func findParentClassDeclaration(node *sitter.Node) *sitter.Node {
	for cur := node; cur != nil; cur = cur.Parent() {
		if cur.Type() == "class_declaration" {
			return cur
		}
	}
	return nil
}

// findSymbolIndexContaining 查找包含指定字节偏移的最内层符号索引。
//
// 对嵌套结构（如 class 包含 method），返回范围最小的那个符号，确保调用关系归属于
// 真正包含调用的函数/方法，而不是外层类。
func findSymbolIndexContaining(symbols []codeSymbol, offset uint32) int {
	best := -1
	for i, sym := range symbols {
		if offset >= sym.start && offset <= sym.end {
			if best < 0 || (sym.start >= symbols[best].start && sym.end <= symbols[best].end) {
				best = i
			}
		}
	}
	return best
}

// findSymbolIndexByName 按符号名查找符号索引（优先匹配 qualifiedName，其次 name）。
func findSymbolIndexByName(symbols []codeSymbol, name string) int {
	for i, sym := range symbols {
		if sym.qualifiedName == name || sym.name == name {
			return i
		}
	}
	return -1
}

// buildCodeRelationEdges 将 codeRelation 转换为 core.Edge。
//
// 只生成目标符号存在于当前文件的边；外部符号暂不创建桩节点。
func buildCodeRelationEdges(doc document.RawDoc, relations []codeRelation, symbols []codeSymbol, symbolChunkIDs, symbolNodeIDs []string) []core.Edge {
	if len(relations) == 0 {
		return nil
	}

	var edges []core.Edge
	for _, rel := range relations {
		if rel.sourceSymbolIdx < 0 || rel.sourceSymbolIdx >= len(symbolNodeIDs) {
			continue
		}
		sourceNodeID := symbolNodeIDs[rel.sourceSymbolIdx]
		if sourceNodeID == "" {
			continue
		}

		targetIdx := findSymbolIndexByName(symbols, rel.targetName)
		if targetIdx < 0 || targetIdx >= len(symbolNodeIDs) {
			continue
		}
		targetNodeID := symbolNodeIDs[targetIdx]
		if targetNodeID == "" {
			continue
		}

		// CALLS 边不指向类/类型/接口节点，避免把实例化或类型引用误判为函数调用
		if rel.edgeType == "CALLS" && isClassLikeSymbol(symbols[targetIdx]) {
			continue
		}

		sourceChunkID := ""
		if rel.sourceSymbolIdx < len(symbolChunkIDs) {
			sourceChunkID = symbolChunkIDs[rel.sourceSymbolIdx]
		}
		targetChunkID := ""
		if targetIdx < len(symbolChunkIDs) {
			targetChunkID = symbolChunkIDs[targetIdx]
		}

		edges = append(edges, buildEdge(doc, sourceNodeID, targetNodeID, rel.edgeType, []string{sourceChunkID, targetChunkID}))
	}

	return edges
}

// buildGoVariableTypeMap 构建文件内简单变量 -> 类型映射。
//
// 覆盖最常见的三种场景：
//   - 短变量声明：p := Person{}
//   - var 声明：var p Person
//   - 参数声明：func (p Person) Method() 或 func f(p Person)
//
// 对指针类型会去掉前导 *。该映射用于把 p.Greet() 解析为 Person.Greet。
func buildGoVariableTypeMap(root *sitter.Node, src []byte) map[string]string {
	spec := codeLangRegistry[".go"]
	queryText := `
		(short_var_declaration
		  left: (expression_list (identifier) @var)
		  right: (expression_list (composite_literal type: (type_identifier) @type))) @binding
		(var_spec
		  name: (identifier) @var
		  type: [(type_identifier) @type (pointer_type (type_identifier) @type)]) @binding
		(parameter_declaration
		  name: (identifier) @var
		  type: [(type_identifier) @type (pointer_type (type_identifier) @type)]) @binding
	`
	q, err := sitter.NewQuery([]byte(queryText), spec.lang)
	if err != nil {
		return nil
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	result := map[string]string{}
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var varName, typ string
		for _, cap := range match.Captures {
			switch q.CaptureNameForId(cap.Index) {
			case "var":
				varName = strings.TrimSpace(cap.Node.Content(src))
			case "type":
				typ = strings.TrimSpace(cap.Node.Content(src))
			}
		}
		if varName != "" && typ != "" {
			typ = strings.TrimPrefix(typ, "*")
			result[varName] = typ
		}
	}
	return result
}

// parentClassQualifiedName 返回包含指定符号的最近 class 的 qualifiedName。
func parentClassQualifiedName(symbols []codeSymbol, idx int) string {
	if idx < 0 || idx >= len(symbols) {
		return ""
	}
	sym := symbols[idx]
	var result string
	for _, s := range symbols {
		if s.nodeType == "class" && sym.start >= s.start && sym.end <= s.end {
			result = s.qualifiedName
		}
	}
	return result
}

// isClassLikeSymbol 判断符号是否为类/类型/接口等，不应作为 CALLS 的目标。
func isClassLikeSymbol(sym codeSymbol) bool {
	switch sym.nodeType {
	case "class", "struct", "interface", "trait", "enum", "type":
		return true
	}
	return false
}

// extractPackageName 从源码中提取包/模块名。
//
// 当前支持：
//   - Go：package xxx
//   - Java：package xxx.yyy;
//   - Python：从文件路径推断模块名（后续可扩展）
func extractPackageName(fileName, content string, root *sitter.Node, lang *sitter.Language) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	src := []byte(content)

	// AST 优先
	if pkg := extractPackageNameFromAST(src, ext, root, lang); pkg != "" {
		return pkg
	}

	// 手搓正则 fallback（AST 不可用时）
	switch ext {
	case ".go":
		if m := goPackageRegexp.FindStringSubmatch(content); len(m) >= 2 {
			return m[1]
		}
	case ".java", ".kt":
		if m := javaPackageRegexp.FindStringSubmatch(content); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

// extractPackageNameFromAST 用 tree-sitter (package_clause) 等查询提取包名。
func extractPackageNameFromAST(src []byte, ext string, root *sitter.Node, lang *sitter.Language) string {
	var queryText string
	switch ext {
	case ".go":
		queryText = `(package_clause name: (package_identifier) @pkg)`
	case ".java", ".kt":
		queryText = `(package_declaration (scoped_identifier) @pkg)`
	default:
		return ""
	}

	q, err := sitter.NewQuery([]byte(queryText), lang)
	if err != nil {
		return ""
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, cap := range match.Captures {
			if q.CaptureNameForId(cap.Index) == "pkg" {
				return strings.TrimSpace(cap.Node.Content(src))
			}
		}
	}
	return ""
}

// extractSignaturesFromAST 用 AST body 字段边界精确提取多行签名。
// 覆盖 extractSymbolSignature 的 firstNonEmptyLine 盲切结果。
func extractSignaturesFromAST(content string, symbols []codeSymbol, spec languageSpec, root *sitter.Node) {
	src := []byte(content)
	q, err := sitter.NewQuery([]byte(spec.query), spec.lang)
	if err != nil {
		return
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	sigByStart := map[uint32]string{}
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var defNode *sitter.Node
		for _, cap := range match.Captures {
			if q.CaptureNameForId(cap.Index) == spec.defCapture {
				defNode = cap.Node
			}
		}
		if defNode == nil {
			continue
		}
		if sig := extractSignatureFromNode(src, defNode); sig != "" {
			sigByStart[defNode.StartByte()] = sig
		}
	}

	for i, sym := range symbols {
		if sig, ok := sigByStart[sym.start]; ok {
			symbols[i].signature = sig
		}
	}
}

// extractSignatureFromNode 从定义节点中提取签名（节点开头到 body 字段前）。
// 支持多种语言：body 字段在 tree-sitter 中几乎通用（function_declaration、
// method_declaration、class_declaration 等均有 body）
func extractSignatureFromNode(src []byte, node *sitter.Node) string {
	body := node.ChildByFieldName("body")
	if body != nil {
		sig := strings.TrimSpace(string(src[node.StartByte():body.StartByte()]))
		sig = strings.TrimRight(sig, "{ \t")
		return strings.TrimSpace(sig)
	}
	return ""
}

// extractVisibilityFromAST 用 AST visibility/modifiers 字段精确提取可见性。
// 未命中时保留 symbols 中已有的值（由 extractSymbolVisibility 的命名约定 fallback 设置）。
func extractVisibilityFromAST(content string, symbols []codeSymbol, spec languageSpec, root *sitter.Node) {
	src := []byte(content)
	q, err := sitter.NewQuery([]byte(spec.query), spec.lang)
	if err != nil {
		return
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	visByStart := map[uint32]string{}
	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		var defNode *sitter.Node
		for _, cap := range match.Captures {
			if q.CaptureNameForId(cap.Index) == spec.defCapture {
				defNode = cap.Node
			}
		}
		if defNode == nil {
			continue
		}
		if vis := extractVisibilityFromNode(src, defNode); vis != "" {
			visByStart[defNode.StartByte()] = vis
		}
	}

	for i, sym := range symbols {
		if vis, ok := visByStart[sym.start]; ok {
			symbols[i].visibility = vis
		}
	}
}

// extractVisibilityFromNode 检查定义节点的 visibility/modifiers 子节点提取可见性。
// Go/Python 等无 AST 可见性的语言返回 ""，保留命名约定 fallback。
func extractVisibilityFromNode(src []byte, node *sitter.Node) string {
	for _, field := range []string{"visibility", "modifiers"} {
		child := node.ChildByFieldName(field)
		if child == nil {
			continue
		}
		text := child.Content(src)
		if strings.Contains(text, "pub") || strings.Contains(text, "public") {
			return "public"
		}
		if strings.Contains(text, "private") {
			return "private"
		}
		if strings.Contains(text, "protected") {
			return "protected"
		}
	}
	return ""
}

// extractSymbolSignature 提取符号定义的第一行作为签名。
//
// 注意：此函数保留为 fallback，实际签名提取由 extractSignaturesFromAST（AST body
// 字段方式）优先覆盖。此函数仅在 AST 无法解析时生效。
func extractSymbolSignature(content string, sym codeSymbol) string {
	body := content[sym.start:sym.end]
	sig := firstNonEmptyLine(body)
	// 去除定义行末尾的 {（Go、C、Java 等语言函数/结构体定义末尾的 {）
	sig = strings.TrimRight(sig, "{ \t")
	sig = strings.TrimSpace(sig)
	return sig
}

// extractSymbolVisibility 根据语言特征推断符号可见性。
//
// 规则：
//   - Go：首字母大写为 exported，小写为 unexported
//   - Python：以 __ 开头为 private，_ 开头为 protected，其余 public
//   - Java/C#/C++/Kotlin/Scala/PHP/Swift：从签名行检测 public/private/protected 关键字
//   - Rust：签名行含 pub 为 public
func extractSymbolVisibility(content string, sym codeSymbol) string {
	// Go 导出规则
	ext := filepath.Ext(sym.name)
	nameOnly := strings.TrimSuffix(sym.name, ext)
	if nameOnly == "" {
		nameOnly = sym.name
	}

	firstRune := rune(0)
	for _, r := range nameOnly {
		firstRune = r
		break
	}

	// Python 命名约定
	if strings.HasPrefix(nameOnly, "__") && strings.HasSuffix(nameOnly, "__") == false {
		return "private"
	}
	if strings.HasPrefix(nameOnly, "_") {
		return "protected"
	}

	sig := extractSymbolSignature(content, sym)
	sigLower := strings.ToLower(sig)

	// Rust
	if strings.Contains(sigLower, "pub ") || strings.Contains(sigLower, "pub(") {
		return "public"
	}

	// C-family / Java / C# / Kotlin / Scala / PHP / Swift
	if strings.Contains(sigLower, "public ") {
		return "public"
	}
	if strings.Contains(sigLower, "private ") {
		return "private"
	}
	if strings.Contains(sigLower, "protected ") {
		return "protected"
	}

	// Go 默认规则（未检测到关键字时按首字母判断）
	if firstRune != 0 {
		if unicode.IsUpper(firstRune) {
			return "exported"
		}
		return "unexported"
	}

	return ""
}

// normalizeNodeType 把 tree-sitter 原始节点类型规范化为干净的元素类型。
//
// 去掉对语义无贡献的后缀，如：
//
//	function_declaration -> function
//	class_definition     -> class
//	struct_item          -> struct
//	trait_item           -> trait
//	impl_item            -> impl
//	type_spec            -> type
func normalizeNodeType(nodeType string) string {
	suffixes := []string{"_declaration", "_definition", "_item", "_spec", "_statement"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(nodeType, suffix) {
			return strings.TrimSuffix(nodeType, suffix)
		}
	}
	return nodeType
}

// codeSymbolLabel 根据 tree-sitter 节点类型推导 Node 标签。
func codeSymbolLabel(nodeType string) string {
	switch {
	case strings.Contains(nodeType, "method"):
		return "Method"
	case strings.Contains(nodeType, "function") || strings.Contains(nodeType, "function_item"):
		return "Function"
	case strings.Contains(nodeType, "class") || strings.Contains(nodeType, "struct") || strings.Contains(nodeType, "enum"):
		return "Class"
	case strings.Contains(nodeType, "interface") || strings.Contains(nodeType, "trait") || strings.Contains(nodeType, "protocol"):
		return "Interface"
	case strings.Contains(nodeType, "type") || strings.Contains(nodeType, "impl"):
		return "Type"
	case strings.Contains(nodeType, "variable") || strings.Contains(nodeType, "declaration"):
		return "Variable"
	case strings.Contains(nodeType, "module") || strings.Contains(nodeType, "object") || strings.Contains(nodeType, "package"):
		return "Module"
	default:
		return "Symbol"
	}
}

// extractSymbolSummary 提取符号的 Summary。
//
// 策略：
//  1. 优先用 AST (comment) 查询取符号前的注释（精确识取注释边界）
//  2. AST 失败时回退到字符串方式（行注释 // / #，或块注释 /* */）
//  3. 都没有时 fallback 取函数体内的第一个 docstring（主要针对 Python）
func extractSymbolSummary(content string, sym codeSymbol, prevEnd uint32, root *sitter.Node, lang *sitter.Language) string {
	if astSummary := extractPrecedingCommentAST([]byte(content), sym.start, prevEnd, root, lang); astSummary != "" {
		return astSummary
	}
	prefix := ""
	if sym.start > prevEnd {
		prefix = content[prevEnd:sym.start]
	}
	if summary := extractPrecedingComment(prefix); summary != "" {
		return summary
	}
	body := content[sym.start:sym.end]
	if summary := extractFirstDocstring(body); summary != "" {
		return summary
	}
	return ""
}

// extractPrecedingCommentAST 用 tree-sitter (comment) 查询从 AST 中提取
// 符号 symStart 前面连续的注释块。
//
// 步骤：
//  1. 查询 AST 中 (prevEnd, symStart) 范围内所有 comment 节点
//  2. 从最末一个 comment 向前收拢，直到遇到空行（或非 comment 间隙）
//  3. 检查末 comment 与 symStart 之间无空行；有空行则认为不属于该符号
func extractPrecedingCommentAST(src []byte, symStart, prevEnd uint32, root *sitter.Node, lang *sitter.Language) string {
	q, err := sitter.NewQuery([]byte("(comment) @c"), lang)
	if err != nil {
		return ""
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)

	type commentNode struct {
		start, end uint32
		text       string
	}
	var comments []commentNode

	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, cap := range match.Captures {
			if q.CaptureNameForId(cap.Index) != "c" {
				continue
			}
			n := cap.Node
			s, e := n.StartByte(), n.EndByte()
			if s >= prevEnd && e <= symStart {
				comments = append(comments, commentNode{s, e, n.Content(src)})
			}
		}
	}

	if len(comments) == 0 {
		return ""
	}

	// 检查末位 comment 与 symStart 之间是否有空行
	last := comments[len(comments)-1]
	gapAfter := string(src[last.end:symStart])
	if strings.Count(gapAfter, "\n") > 1 {
		return "" // 有空行 → 注释属于上一个代码块
	}

	// 从末位向前收拢连续的注释（之间无空行）
	lines := []string{cleanCommentText(last.text)}
	for i := len(comments) - 2; i >= 0; i-- {
		c := comments[i]
		next := comments[i+1]
		gap := string(src[c.end:next.start])
		if strings.Count(gap, "\n") > 1 {
			break // 有空行 → 不属于同一个注释块
		}
		lines = append([]string{cleanCommentText(c.text)}, lines...)
	}

	return strings.Join(lines, "\n")
}

// cleanCommentText 去除常见注释前缀（// / # / /* / """ / '''）并 TrimSpace。
func cleanCommentText(text string) string {
	text = strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(text, "//") || strings.HasPrefix(text, "#"):
		return stripLineComment(text)
	case strings.HasPrefix(text, "/*"):
		inner := strings.TrimPrefix(text, "/*")
		if idx := strings.LastIndex(inner, "*/"); idx >= 0 {
			inner = inner[:idx]
		}
		return strings.TrimSpace(stripBlockCommentStars(inner))
	default:
		// Python """ 或 ''' docstring
		for _, quote := range []string{`"""`, `'''`} {
			if idx := strings.Index(text, quote); idx >= 0 {
				inner := text[idx+3:]
				if ri := strings.LastIndex(inner, quote); ri >= 0 {
					inner = inner[:ri]
				}
				return strings.TrimSpace(inner)
			}
		}
	}
	return text
}

// extractPrecedingComment 从符号前的文本中提取注释。
//
// 支持：
//   - 块注释 /* ... */
//   - 三引号字符串 """...""" / ”'...”'（Python 模块/类的前置 docstring）
//   - 行注释 // / #（支持连续多行）
func extractPrecedingComment(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}

	// 块注释 /* ... */
	if strings.HasPrefix(prefix, "/*") {
		if idx := strings.LastIndex(prefix, "*/"); idx >= 2 {
			return stripBlockCommentStars(prefix[2:idx])
		}
	}

	// 三引号 docstring
	for _, quote := range []string{`"""`, "'''"} {
		if strings.HasPrefix(prefix, quote) {
			if idx := strings.Index(prefix[len(quote):], quote); idx >= 0 {
				return strings.TrimSpace(prefix[len(quote) : len(quote)+idx])
			}
		}
	}

	// 行注释：从末尾向上收集连续注释行
	lines := strings.Split(prefix, "\n")
	var comments []string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			if len(comments) > 0 {
				break
			}
			continue
		}
		if isLineComment(line) {
			comments = append([]string{stripLineComment(line)}, comments...)
			continue
		}
		break
	}
	if len(comments) > 0 {
		return strings.Join(comments, "\n")
	}

	return ""
}

// extractFirstDocstring 从函数体内提取第一个三引号 docstring（Python 等）。
func extractFirstDocstring(body string) string {
	body = strings.TrimSpace(body)
	lines := strings.SplitN(body, "\n", 2)
	if len(lines) < 2 {
		return ""
	}
	rest := strings.TrimSpace(lines[1])
	for _, quote := range []string{`"""`, "'''"} {
		if strings.HasPrefix(rest, quote) {
			if idx := strings.Index(rest[len(quote):], quote); idx >= 0 {
				return strings.TrimSpace(rest[len(quote) : len(quote)+idx])
			}
		}
	}
	return ""
}

// isLineComment 判断单行文本是否为行注释。
func isLineComment(line string) bool {
	return strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#")
}

// stripLineComment 去掉行注释前缀并清理首尾空白。
func stripLineComment(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "//") {
		return strings.TrimSpace(strings.TrimPrefix(line, "//"))
	}
	if strings.HasPrefix(line, "#") {
		return strings.TrimSpace(strings.TrimPrefix(line, "#"))
	}
	return line
}

// stripBlockCommentStars 去掉块注释中每行开头的 * 和空白。
func stripBlockCommentStars(inner string) string {
	lines := strings.Split(inner, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// firstNonEmptyLine 返回文本中第一行非空内容（去掉常见关键字前缀的简化处理）。
func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line
	}
	return ""
}
