package structurizer

import (
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/document"
)

// Structurizer 接口，定义结构化分析器的通用行为
type Structurizer interface {
	Parse(raw core.Document) (*core.StructuredDocument, error)
}

// Open 根据文件路径打开文件并结构化
func Open(file string) (*core.StructuredDocument, error) {
	// 调用 document.Open() 获取 Document
	doc, err := document.Open(file)
	if err != nil {
		return nil, err
	}

	// 根据扩展名选择对应的结构化分析器
	ext := strings.ToLower(filepath.Ext(file))
	structurizer := getStructurizerByExt(ext)

	return structurizer.Parse(doc)
}

// New 根据文本内容和 MIME 类型创建结构化文档
func New(text string, mime string) (*core.StructuredDocument, error) {
	// 调用 document.New() 获取 Document
	doc := document.New(text, mime)

	// 根据 MIME 类型选择对应的结构化分析器
	structurizer := getStructurizerByMime(doc.GetMimeType())

	return structurizer.Parse(doc)
}

// getStructurizerByExt 根据文件扩展名选择对应的结构化分析器
func getStructurizerByExt(ext string) Structurizer {
	switch ext {
	case ".json", ".yaml", ".yml", ".toml":
		return NewConfigStructurizer()
	case ".html", ".htm", ".xml":
		return NewWebStructurizer()
	case ".md", ".markdown":
		return NewMarkdownStructurizer()
	}

	// 代码文件使用代码结构化分析器
	if isCodeExt(ext) {
		return NewCodeStructurizer()
	}

	// 默认使用纯文本结构化分析器
	return NewPlainTextStructurizer()
}

// getStructurizerByMime 根据 MIME 类型选择对应的结构化分析器
func getStructurizerByMime(mime string) Structurizer {
	switch mime {
	case core.MimeTypeApplicationJSON,
		core.MimeTypeTextYAML, core.MimeTypeApplicationXYAML,
		core.MimeTypeTextTOML, core.MimeTypeApplicationToml:
		return NewConfigStructurizer()
	case core.MimeTypeTextHTML:
		return NewWebStructurizer()
	case core.MimeTypeTextMarkdown:
		return NewMarkdownStructurizer()
	case core.MimeTypeTextXML, core.MimeTypeApplicationXML:
		return NewWebStructurizer()
	}

	// 代码文件使用代码结构化分析器
	if isCodeMime(mime) {
		return NewCodeStructurizer()
	}

	// 默认使用纯文本结构化分析器
	return NewPlainTextStructurizer()
}

// isCodeExt 判断扩展名是否为代码文件
func isCodeExt(ext string) bool {
	codeExts := map[string]bool{
		".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".go": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
		".hpp": true, ".cs": true, ".rb": true, ".php": true, ".pl": true,
		".swift": true, ".kt": true, ".rs": true, ".scala": true,
		".sh": true, ".bash": true, ".zsh": true, ".ps1": true,
		".sql": true, ".r": true, ".lua": true, ".ex": true, ".exs": true,
		".erl": true, ".hrl": true, ".fs": true, ".fsx": true,
		".vb": true, ".vbs": true, ".dart": true,
		".groovy": true, ".gradle": true, ".makefile": true, ".dockerfile": true,
		".vue": true, ".svelte": true, ".graphql": true, ".gql": true,
		".toml": true, ".ini": true, ".conf": true, ".cfg": true,
		".css": true, ".scss": true, ".sass": true, ".less": true,
		".html": true, ".htm": true, ".xml": true,
	}
	return codeExts[ext]
}

// isCodeMime 判断 MIME 类型是否为代码文件
func isCodeMime(mime string) bool {
	codeMimes := map[string]bool{
		core.MimeTypeTextPython:     true,
		core.MimeTypeTextJavaScript:  true,
		core.MimeTypeTextTypeScript:  true,
		core.MimeTypeTextGo:           true,
		core.MimeTypeTextJava:         true,
		core.MimeTypeTextC:            true,
		core.MimeTypeTextCPP:          true,
		core.MimeTypeTextCsharp:      true,
		core.MimeTypeTextPHP:          true,
		core.MimeTypeTextRuby:        true,
		core.MimeTypeTextPerl:        true,
		core.MimeTypeTextSwift:       true,
		core.MimeTypeTextKotlin:      true,
		core.MimeTypeTextRust:        true,
		core.MimeTypeTextBash:        true,
		core.MimeTypeTextPowerShell:  true,
		core.MimeTypeTextSQL:         true,
		core.MimeTypeTextCSS:         true,
		core.MimeTypeTextVue:         true,
		core.MimeTypeTextSvelte:     true,
		core.MimeTypeTextGraphQL:    true,
	}
	return codeMimes[mime]
}
