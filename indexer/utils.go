package indexer

import (
	"strings"
	"time"

	"github.com/DotNetAge/gorag/v2/chunker"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/structurizer"
)

// 默认分块策略
var defaultChunkStrategy = chunker.StrategyRecursive

// ChunkOption 分块选项
type ChunkOption func(*chunkOption)

type chunkOption struct {
	strategy core.ChunkStrategy
	logger   logging.Logger
}

// WithChunkStrategy 设置分块策略
func WithChunkStrategy(strategy core.ChunkStrategy) ChunkOption {
	return func(o *chunkOption) {
		o.strategy = strategy
	}
}

// WithChunkLogger attaches a logger to the chunking operation. The chunker
// emits a "chunker.parse" log when the source has been loaded and a
// "chunker.chunked" log when the chunk array is produced.
func WithChunkLogger(logger logging.Logger) ChunkOption {
	return func(o *chunkOption) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// autoSelectStrategy 根据内容自动选择最佳分块策略
func autoSelectStrategy(content string, mime string) core.ChunkStrategy {
	// 1. 根据 MIME 类型选择
	switch mime {
	case core.MimeTypeApplicationJSON, core.MimeTypeTextYAML,
		core.MimeTypeTextXML, core.MimeTypeApplicationXML,
		core.MimeTypeTextTOML:
		return chunker.StrategyRecursive
	case core.MimeTypeTextHTML, core.MimeTypeTextMarkdown:
		return chunker.StrategyParagraph
	}

	// 2. 代码检测 - 包含代码关键字
	if isCodeContent(content) {
		return chunker.StrategyCode
	}

	// 3. 短文本检测
	if len(content) < 200 {
		return chunker.StrategySentence
	}

	// 4. 长文本检测 - 适合 ParentDoc 两级分块
	// 长文本需要精确检索（子块）+ 完整上下文（父块）
	if len(content) > 2000 {
		// 非结构化长文本使用 ParentDoc
		if !isCodeContent(content) && !isMarkdownContent(content) && !isTableContent(content) {
			return chunker.StrategyParentDoc
		}
		// Markdown/表格长文本可以用 ParentDoc 增强
		if isMarkdownContent(content) || isTableContent(content) {
			return chunker.StrategyParentDoc
		}
	}

	// 5. Markdown 检测
	if isMarkdownContent(content) {
		return chunker.StrategyParagraph
	}

	// 6. 表格检测 - 包含表格结构
	if isTableContent(content) {
		return chunker.StrategyRecursive
	}

	// 7. 默认使用递归分块
	return chunker.StrategyRecursive
}

// isCodeContent 检测内容是否为代码
func isCodeContent(content string) bool {
	codeKeywords := []string{
		"func ", "function ", "def ", "class ", "interface ",
		"package ", "import ", "export ", "require(",
		"public ", "private ", "protected ", "static ",
		"const ", "let ", "var ",
		"SELECT ", "FROM ", "WHERE ", "INSERT ", "UPDATE ", "DELETE ",
		"CREATE ", "ALTER ", "DROP ",
		"fn ", "let mut ", "impl ", "pub fn",
	}
	upper := strings.ToUpper(content)
	for _, kw := range codeKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

// isMarkdownContent 检测内容是否为 Markdown
func isMarkdownContent(content string) bool {
	lines := strings.Split(content, "\n")
	markdownCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Markdown 标题
		if strings.HasPrefix(line, "#") {
			markdownCount++
			continue
		}
		// Markdown 列表
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") ||
			strings.HasPrefix(line, "> ") || strings.HasPrefix(line, "1. ") {
			markdownCount++
			continue
		}
		// Markdown 代码块
		if strings.HasPrefix(line, "```") {
			markdownCount++
			continue
		}
	}
	// 超过 3 行 Markdown 语法，认为是 Markdown
	return markdownCount >= 3
}

// isTableContent 检测内容是否包含表格结构
func isTableContent(content string) bool {
	lines := strings.Split(content, "\n")
	tableScore := 0
	for _, line := range lines {
		// 表格通常用 | 分隔
		if strings.Count(line, "|") >= 3 {
			tableScore++
		}
		// CSV 格式
		if strings.Count(line, ",") >= 3 && !strings.Contains(line, " ") {
			tableScore++
		}
	}
	// 超过 2 行表格格式
	return tableScore >= 2
}

// GetChunks 根据文本内容进行结构化和分块
// 如果没有指定策略，会根据内容自动选择最佳分块策略
// 返回完整的分块数组
func GetChunks(content string, opts ...ChunkOption) ([]*core.Chunk, error) {
	// 应用选项
	cfg := &chunkOption{strategy: defaultChunkStrategy, logger: logging.DefaultNoopLogger()}
	for _, opt := range opts {
		opt(cfg)
	}

	parseStart := time.Now()
	// 1. 从文本内容推断 MIME 类型
	mime := core.ParseMimeTypeFromText(content)
	cfg.logger.Debug("chunker.parse",
		"source", "text",
		"mime", mime,
		"bytes", len(content),
		"duration_ms", time.Since(parseStart).Milliseconds(),
	)

	// 2. 如果未指定策略，自动选择
	if cfg.strategy == "" || cfg.strategy == defaultChunkStrategy {
		cfg.strategy = autoSelectStrategy(content, mime)
	}

	// 3. Structurizing 结构化索引内容，获取 StructuredDocument
	doc, err := structurizer.New(content, mime)
	if err != nil {
		return nil, err
	}

	chunkStart := time.Now()
	// 4. Chunking 分块索引内容，获取 Chunk
	chunkerInstance, err := chunker.CreateChunker(cfg.strategy)
	if err != nil {
		return nil, err
	}

	chunks, err := chunkerInstance.Chunk(doc)
	if err != nil {
		return nil, err
	}

	cfg.logger.Debug("chunker.chunked",
		"source", "text",
		"mime", mime,
		"strategy", string(cfg.strategy),
		"chunks", len(chunks),
		"duration_ms", time.Since(chunkStart).Milliseconds(),
	)

	return chunks, nil
}

// GetFileChunks 根据文件路径进行结构化和分块
// 如果没有指定策略，会根据内容自动选择最佳分块策略
// 返回完整的分块数组
func GetFileChunks(file string, opts ...ChunkOption) ([]*core.Chunk, error) {
	// 应用选项
	cfg := &chunkOption{strategy: defaultChunkStrategy, logger: logging.DefaultNoopLogger()}
	for _, opt := range opts {
		opt(cfg)
	}

	parseStart := time.Now()
	// 1. Structurizing 打开并结构化文件，获取 StructuredDocument
	doc, err := structurizer.Open(file)
	if err != nil {
		return nil, err
	}

	// 2. 获取 MIME 类型和内容（通过 RawDoc）
	mime := doc.RawDoc.GetMimeType()
	content := doc.RawDoc.GetContent()
	cfg.logger.Debug("chunker.parse",
		"source", "file",
		"file", file,
		"mime", mime,
		"bytes", len(content),
		"duration_ms", time.Since(parseStart).Milliseconds(),
	)

	// 3. 如果未指定策略，自动选择
	if cfg.strategy == "" || cfg.strategy == defaultChunkStrategy {
		cfg.strategy = autoSelectStrategy(content, mime)
	}

	chunkStart := time.Now()
	// 4. Chunking 分块索引内容，获取 Chunk
	chunkerInstance, err := chunker.CreateChunker(cfg.strategy)
	if err != nil {
		return nil, err
	}

	chunks, err := chunkerInstance.Chunk(doc)
	if err != nil {
		return nil, err
	}

	cfg.logger.Debug("chunker.chunked",
		"source", "file",
		"file", file,
		"mime", mime,
		"strategy", string(cfg.strategy),
		"chunks", len(chunks),
		"duration_ms", time.Since(chunkStart).Milliseconds(),
	)

	// for i := range chunks {
	// 	chunks[i].Metadata["file"] = strings.ToLower(file)
	// }

	return chunks, nil
}
