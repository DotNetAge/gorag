package chunker

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	"github.com/DotNetAge/gorag/v2/utils"
)

// =====================================================================
// Chunker 接口 + 工厂方法
// =====================================================================
//
// 设计要点：
//   - Chunker 接口仅 Chunk(doc RawDoc) (ChunkResult, error) 一个方法
//   - 输入为 document.RawDoc 接口
//   - 输出为 ChunkResult（包含 Chunks/Nodes/Edges）
//   - 按 RawDoc.Type 路由到 4 个实现：ImageChunker / MarkdownChunker / CodeChunker / DatumChunker

// ChunkResult 分块结果容器。
//
// Chunker 在分块过程中可同时产出结构化的 Nodes/Edges，
// 因此接口返回 ChunkResult 而非单纯的 []core.Chunk。
type ChunkResult struct {
	Chunks []core.Chunk // 分片列表
	Nodes  []core.Node  // 结构节点（如 heading、函数、类、数据表等）
	Edges  []core.Edge  // 结构关系（主要为 CONTAINS 层级关系）
}

// Chunker 分块器接口。
type Chunker interface {
	// Chunk 对原始文档进行分块，返回分块结果（含 Chunks/Nodes/Edges）。
	// 实现按 RawDoc.Type 路由：
	//   - RawDocImage → ImageChunker（整个图片作为一个 Chunk）
	//   - RawDocDoc   → MarkdownChunker（按 Markdown heading 切分）
	//   - RawDocText  → CodeChunker（代码按函数/类边界切分）
	//   - RawDocData  → DatumChunker（按数据结构边界切分）
	Chunk(doc document.RawDoc) (ChunkResult, error)
}

// New 工厂方法：按 doc.Type() 路由到对应的 Chunker 实现。
//
// 路由规则：
//   - RawDocImage → ImageChunker
//   - RawDocDoc   → MarkdownChunker
//   - RawDocText  → CodeChunker（代码 + 纯文本，纯文本走简易实现）
//   - RawDocData  → DatumChunker
//
// 未知类型返回 error，避免静默兜底导致数据被错误分块。
func New(doc document.RawDoc) (Chunker, error) {
	if doc == nil {
		return nil, fmt.Errorf("chunker.New: doc is nil")
	}

	switch doc.Type() {
	case document.RawDocImage:
		return &ImageChunker{}, nil
	case document.RawDocDoc:
		return &MarkdownChunker{}, nil
	case document.RawDocText:
		return &CodeChunker{}, nil
	case document.RawDocData:
		return &DatumChunker{}, nil
	default:
		return nil, fmt.Errorf("chunker.New: unknown RawDocType %q", doc.Type())
	}
}

// =====================================================================
// 共用辅助函数
// =====================================================================

// buildChunk 构造单个 Chunk，统一填充 ID/DocID/Metadata。
//
// 设计要点：
//   - Chunk.ID 使用 utils.GenerateID([]byte(docID + ":" + title + ":" + content)) 生成
//     将 title（路径/符号名/数据路径等）纳入盐值，可避免数据文件中相同内容的记录（如数组里多个相同对象）产生重复 ID
//   - Summary 字段默认留空，由各 Chunker 在返回前按统一策略填充
//   - Metadata["source"] 统一记录 doc.FileName()
func buildChunk(
	doc document.RawDoc,
	idx int,
	start, end int,
	title, content string,
) core.Chunk {
	return core.Chunk{
		ID:       utils.GenerateID([]byte(doc.ID() + ":" + title + ":" + content)),
		DocID:    doc.ID(),
		Title:    title,
		Content:  content,
		Index:    idx,
		StartPos: start,
		EndPos:   end,
		Metadata: map[string]any{
			"source": doc.FileName(),
		},
	}
}

// enrichChunksMetadata 为每个 Chunk 补充通用元数据：
//   - start_line / end_line：在源文件中的行号（从 1 开始）
//   - language：从文件扩展名推导的语言标识
//   - directory：文件所在目录（绝对路径）
//
// 每个 Chunker 在返回结果前应调用此函数统一 enriched。
func enrichChunksMetadata(chunks []core.Chunk, fullContent, fileName string) []core.Chunk {
	lang := deriveLanguage(fileName)
	directory := ""
	if fileName != "" {
		directory = filepath.Dir(fileName)
	}
	for i := range chunks {
		if chunks[i].Metadata == nil {
			chunks[i].Metadata = map[string]any{}
		}
		chunks[i].Metadata["start_line"] = byteOffsetToLine(fullContent, chunks[i].StartPos)
		chunks[i].Metadata["end_line"] = byteOffsetToLine(fullContent, chunks[i].EndPos)
		chunks[i].Metadata["language"] = lang
		if directory != "" {
			chunks[i].Metadata["directory"] = directory
		}
	}
	return chunks
}

// deriveSummary 从内容中提取前 maxSentences 个句子作为 Summary。
//
// 句子切分支持中英文句号；若内容为空或无法切分，返回原文或空字符串。
// 用于统一 Markdown/Datum/Image 等 Chunker 的默认 Summary 生成策略。
func deriveSummary(content string, maxSentences int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if maxSentences <= 0 {
		maxSentences = 1
	}

	var sentences []string
	start := 0
	for i, r := range content {
		size := utf8.RuneLen(r)
		if r == '。' {
			s := strings.TrimSpace(content[start : i+size])
			if s != "" {
				sentences = append(sentences, s)
			}
			start = i + size
		} else if r == '.' && i+size < len(content) {
			next := content[i+size]
			if next == ' ' || next == '\n' {
				s := strings.TrimSpace(content[start : i+size])
				if s != "" {
					sentences = append(sentences, s)
				}
				start = i + size
			}
		}
	}
	if start < len(content) {
		s := strings.TrimSpace(content[start:])
		if s != "" {
			sentences = append(sentences, s)
		}
	}

	if len(sentences) == 0 {
		return content
	}
	if len(sentences) > maxSentences {
		sentences = sentences[:maxSentences]
	}
	return strings.Join(sentences, " ")
}

// byteOffsetToLine 计算字节偏移量对应的行号（从 1 开始）。
func byteOffsetToLine(content string, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(content) {
		offset = len(content)
	}
	line := 1
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}

// deriveLanguage 从文件扩展名推导语言标识。
//
// 常见扩展名映射为可读语言名（如 .md -> markdown，.ts -> typescript），
// 未知扩展名直接去掉前导点返回。
func deriveLanguage(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh":
		return "cpp"
	case ".rs":
		return "rust"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".swift":
		return "swift"
	case ".php":
		return "php"
	case ".lua":
		return "lua"
	case ".sh", ".bash":
		return "bash"
	case ".md", ".markdown", ".mdx":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".xml":
		return "xml"
	case ".csv":
		return "csv"
	case ".toml":
		return "toml"
	case ".log":
		return "log"
	case "":
		return ""
	default:
		return strings.TrimPrefix(ext, ".")
	}
}

// deriveTitle 从文件路径提取标题（去扩展名，取 basename）。
// 路径为空时返回 "Untitled"。
func deriveTitle(fileName string) string {
	if fileName == "" {
		return "Untitled"
	}
	base := filepath.Base(fileName)
	// 去除扩展名
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" {
		return "Untitled"
	}
	return base
}

// buildNode 构造结构节点，用于 Chunker 产出的语法/结构节点。
//
// 设计要点：
//   - Node.ID 使用 utils.GenerateID([]byte(scope + ":" + name + ":" + labels[0])) 生成
//   - 默认 scope 为 doc.ID()；传入 scope 可实现跨文件实体合并（如 Go 的 package 作用域）
//   - Node.SourceChunkIDs 绑定到对应 Chunk.ID；当 chunkID 为空时，SourceChunkIDs 为空，
//     表示该 Node 是纯图实体，上层代码不应假设每个 Node 都对应 Chunk
//   - Node.SourceDocIDs 绑定到 doc.ID()
func buildNode(doc document.RawDoc, name string, labels []string, chunkID string, props map[string]any, scope ...string) core.Node {
	if props == nil {
		props = map[string]any{}
	}
	label := ""
	if len(labels) > 0 {
		label = labels[0]
	}
	idScope := doc.ID()
	if len(scope) > 0 && scope[0] != "" {
		idScope = scope[0]
	}
	node := core.Node{
		ID:           utils.GenerateID([]byte(idScope + ":" + name + ":" + label)),
		Labels:       labels,
		Name:         name,
		Properties:   props,
		SourceDocIDs: []string{doc.ID()},
	}
	if chunkID != "" {
		node.SourceChunkIDs = []string{chunkID}
	}
	return node
}

// buildEdge 构造结构关系边，用于 Chunker 产出的层级/包含关系。
//
// 设计要点：
//   - Edge.ID 使用 utils.GenerateID([]byte(docID + ":" + source + ":" + edgeType + ":" + target)) 生成
//   - Edge.SourceChunkIDs 合并父子节点来源 Chunk
//   - Edge.SourceDocIDs 绑定到 doc.ID()
func buildEdge(doc document.RawDoc, source, target, edgeType string, srcChunkIDs []string) core.Edge {
	return core.Edge{
		ID:             utils.GenerateID([]byte(doc.ID() + ":" + source + ":" + edgeType + ":" + target)),
		Type:           edgeType,
		Source:         source,
		Target:         target,
		SourceChunkIDs: uniqueStrings(srcChunkIDs),
		SourceDocIDs:   []string{doc.ID()},
	}
}

// uniqueStrings 去重并保留顺序。
func uniqueStrings(s []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(s))
	for _, v := range s {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
