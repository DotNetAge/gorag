package core

import "strings"

// MIME类型常量
const (
	// 基础文本格式
	MimeTypeTextPlain       = "text/plain"
	MimeTypeTextMarkdown    = "text/markdown"
	MimeTypeTextHTML        = "text/html"
	MimeTypeApplicationJSON = "application/json"
	MimeTypeTextCSS         = "text/css"
	MimeTypeTextJavaScript  = "text/javascript"
	MimeTypeApplicationXML  = "application/xml"
	MimeTypeTextXML         = "text/xml"
	MimeTypeTextYAML        = "text/yaml"
	MimeTypeTextTOML        = "text/toml"
	MimeTypeTextCSV         = "text/csv"
	MimeTypeTextTSV         = "text/tab-separated-values"
	MimeTypeTextSQL         = "text/sql"
	// 编程语言
	MimeTypeTextPython     = "text/x-python"
	MimeTypeTextGo         = "text/x-go"
	MimeTypeTextJava       = "text/x-java"
	MimeTypeTextC          = "text/x-c"
	MimeTypeTextCPP        = "text/x-c++"
	MimeTypeTextCsharp     = "text/x-csharp"
	MimeTypeTextPHP        = "text/x-php"
	MimeTypeTextRuby       = "text/x-ruby"
	MimeTypeTextPerl       = "text/x-perl"
	MimeTypeTextBash       = "text/x-sh"
	MimeTypeTextPowerShell = "text/x-powershell"
	MimeTypeTextRust       = "text/x-rust"
	MimeTypeTextSwift      = "text/x-swift"
	MimeTypeTextKotlin     = "text/x-kotlin"
	MimeTypeTextTypeScript = "text/typescript"
	MimeTypeTextVue        = "text/vue"
	MimeTypeTextSvelte     = "text/svelte"
	MimeTypeTextGraphQL    = "application/graphql"
	// 图片格式
	MimeTypeImageJPEG  = "image/jpeg"
	MimeTypeImagePNG   = "image/png"
	MimeTypeImageGIF   = "image/gif"
	MimeTypeImageWebP  = "image/webp"
	MimeTypeImageBMP   = "image/bmp"
	MimeTypeImageSVG   = "image/svg+xml"
	// Office 文档
	MimeTypeApplicationMsWord                                                                    = "application/msword"
	MimeTypeApplicationWordOpenXML                                                               = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	MimeTypeApplicationMsExcel                                                                   = "application/vnd.ms-excel"
	MimeTypeApplicationExcelOpenXML                                                              = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	MimeTypeApplicationMsPowerpoint                                                              = "application/vnd.ms-powerpoint"
	MimeTypeApplicationPowerpointOpenXML                                                        = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	MimeTypeApplicationPDF                                                                       = "application/pdf"
	// 其他
	MimeTypeApplicationXYAML = "application/x-yaml"
	MimeTypeApplicationToml  = "application/toml"
)

// MimeTypes 是文件扩展名到MIME类型的映射
var MimeTypes = map[string]string{
	// 基础文本格式
	".txt":  MimeTypeTextPlain,
	".md":   MimeTypeTextMarkdown,
	".html": MimeTypeTextHTML,
	".htm":  MimeTypeTextHTML,
	".json": MimeTypeApplicationJSON,
	".css":  MimeTypeTextCSS,
	".js":   MimeTypeTextJavaScript,
	".xml":  MimeTypeTextXML,
	".yaml": MimeTypeTextYAML,
	".yml":  MimeTypeTextYAML,
	".toml": MimeTypeTextTOML,
	".csv":  MimeTypeTextCSV,
	".tsv":  MimeTypeTextTSV,
	".sql":  MimeTypeTextSQL,
	// 编程语言
	".py":      MimeTypeTextPython,
	".go":      MimeTypeTextGo,
	".java":    MimeTypeTextJava,
	".c":       MimeTypeTextC,
	".cpp":     MimeTypeTextCPP,
	".h":       MimeTypeTextC,
	".hpp":     MimeTypeTextCPP,
	".cs":      MimeTypeTextCsharp,
	".php":     MimeTypeTextPHP,
	".rb":      MimeTypeTextRuby,
	".pl":      MimeTypeTextPerl,
	".sh":      MimeTypeTextBash,
	".ps1":     MimeTypeTextPowerShell,
	".rs":      MimeTypeTextRust,
	".swift":   MimeTypeTextSwift,
	".kt":      MimeTypeTextKotlin,
	".ts":      MimeTypeTextTypeScript,
	".vue":     MimeTypeTextVue,
	".svelte":  MimeTypeTextSvelte,
	".graphql": MimeTypeTextGraphQL,
	".gql":     MimeTypeTextGraphQL,
	// 配置文件
	".ini":  MimeTypeTextPlain,
	".conf": MimeTypeTextPlain,
	".cfg":  MimeTypeTextPlain,
	".env":  MimeTypeTextPlain,
	// 图片格式
	".jpg":  MimeTypeImageJPEG,
	".jpeg": MimeTypeImageJPEG,
	".png":  MimeTypeImagePNG,
	".gif":  MimeTypeImageGIF,
	".webp": MimeTypeImageWebP,
	".bmp":  MimeTypeImageBMP,
	".svg":  MimeTypeImageSVG,
	// Office 文档
	".doc":  MimeTypeApplicationMsWord,
	".docx": MimeTypeApplicationWordOpenXML,
	".xls":  MimeTypeApplicationMsExcel,
	".xlsx": MimeTypeApplicationExcelOpenXML,
	".ppt":  MimeTypeApplicationMsPowerpoint,
	".pptx": MimeTypeApplicationPowerpointOpenXML,
	".pdf":  MimeTypeApplicationPDF,
}

// ParseMimeTypeFromText 根据文本内容推断 MIME 类型
// 只支持纯文本格式的检测，返回最可能的 MIME 类型
func ParseMimeTypeFromText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return MimeTypeTextPlain
	}

	// JSON: 通常以 { 或 [ 开头
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		// 简单验证：检查是否是有效的 JSON 语法（可选）
		if isJSON(text) {
			return MimeTypeApplicationJSON
		}
	}

	// XML/HTML: 以 <?xml、<!DOCTYPE、<html、<head、<body、<div、<p、<span 等标签开头
	if strings.HasPrefix(text, "<?xml") ||
		strings.HasPrefix(text, "<!DOCTYPE") ||
		strings.HasPrefix(text, "<html") ||
		strings.HasPrefix(text, "<head") ||
		strings.HasPrefix(text, "<body") ||
		strings.HasPrefix(text, "<div") ||
		strings.HasPrefix(text, "<p>") ||
		strings.HasPrefix(text, "<span") ||
		strings.HasPrefix(text, "<!") ||
		strings.HasPrefix(text, "<?") {
		return MimeTypeTextHTML
	}

	// YAML: 以 --- 开头，或者是 key: value 格式
	if strings.HasPrefix(text, "---") {
		return MimeTypeTextYAML
	}

	// TOML: 以 [ 开头（section），或 key = value 格式（不含冒号）
	if strings.HasPrefix(text, "[") || (strings.Contains(text, "=") && !strings.Contains(text, ":")) {
		if isTOML(text) {
			return MimeTypeTextTOML
		}
	}

	// Markdown: 以 #、-、*、>、===、--- 等 Markdown 语法开头
	if isMarkdown(text) {
		return MimeTypeTextMarkdown
	}

	// CSS: 以 { 开头，或者包含 property: value 模式
	if strings.HasPrefix(text, "{") || isCSS(text) {
		return MimeTypeTextCSS
	}

	// GraphQL: 以 type、query、mutation、schema、fragment 等关键字开头
	if isGraphQL(text) {
		return MimeTypeTextGraphQL
	}

	// SQL: 以 CREATE、SELECT、INSERT、UPDATE、DELETE、ALTER、DROP 等 SQL 关键字开头
	if isSQL(text) {
		return MimeTypeTextSQL
	}

	// 编程语言检测
	if mime := detectCodeMime(text); mime != "" {
		return mime
	}

	// 默认纯文本
	return MimeTypeTextPlain
}

// isJSON 简单验证是否是有效的 JSON
func isJSON(text string) bool {
	text = strings.TrimSpace(text)
	if (strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}")) ||
		(strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]")) {
		// 尝试解析（这里用简单的括号匹配作为初步判断）
		return true
	}
	return false
}

// isTOML 简单验证是否是有效的 TOML
func isTOML(text string) bool {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// TOML section: [section]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		// TOML key = value
		if strings.Contains(line, "=") {
			continue
		}
		return false
	}
	return true
}

// isMarkdown 判断文本是否是 Markdown 格式
func isMarkdown(text string) bool {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Markdown 标题: # ## ### 等
		if strings.HasPrefix(line, "#") {
			return true
		}
		// Markdown 列表: - * 1. 2. 等
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") ||
			(strings.HasPrefix(line, "1.") && len(line) > 2 && line[2] == ' ') ||
			(strings.HasPrefix(line, "2.") && len(line) > 2 && line[2] == ' ') {
			return true
		}
		// Markdown 引用: >
		if strings.HasPrefix(line, "> ") {
			return true
		}
		// Markdown 分隔线: --- === ***
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "***") {
			return true
		}
		// Markdown 代码块: ```
		if strings.HasPrefix(line, "```") {
			return true
		}
		// Markdown 斜体/粗体: *text* **text**
		if strings.HasPrefix(line, "*") || strings.HasPrefix(line, "**") {
			return true
		}
		// 超过一行且不是代码相关内容，可能是 Markdown
		if len(line) > 3 {
			return false // 如果第一行非空且不是 Markdown 语法，大概率不是 Markdown
		}
	}
	return false
}

// isCSS 判断文本是否是 CSS 格式
func isCSS(text string) bool {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "/*") {
			continue
		}
		// CSS property: value;
		if strings.Contains(line, "{") || strings.Contains(line, "}") {
			return true
		}
		// 选择器 { 格式
		if strings.Contains(line, ":") && strings.Contains(line, ";") {
			return true
		}
	}
	return false
}

// isGraphQL 判断文本是否是 GraphQL 格式
func isGraphQL(text string) bool {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// GraphQL 关键字
		if strings.HasPrefix(line, "type ") ||
			strings.HasPrefix(line, "query ") ||
			strings.HasPrefix(line, "mutation ") ||
			strings.HasPrefix(line, "subscription ") ||
			strings.HasPrefix(line, "fragment ") ||
			strings.HasPrefix(line, "schema ") ||
			strings.HasPrefix(line, "extend ") ||
			strings.HasPrefix(line, "enum ") ||
			strings.HasPrefix(line, "interface ") ||
			strings.HasPrefix(line, "union ") ||
			strings.HasPrefix(line, "input ") ||
			strings.HasPrefix(line, "scalar ") ||
			strings.HasPrefix(line, "directive ") {
			return true
		}
	}
	return false
}

// isSQL 判断文本是否是 SQL 格式
func isSQL(text string) bool {
	upper := strings.ToUpper(strings.TrimSpace(text))
	keywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP", "TABLE", "INDEX", "VIEW", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "GROUP BY", "ORDER BY", "HAVING", "LIMIT", "OFFSET"}
	for _, kw := range keywords {
		if strings.HasPrefix(upper, kw) {
			return true
		}
	}
	return false
}

// detectCodeMime 检测编程语言的 MIME 类型
func detectCodeMime(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		// Go: package xxx, import (
		if strings.HasPrefix(line, "package ") || strings.HasPrefix(line, "import ") {
			if strings.Contains(text, "func ") {
				return MimeTypeTextGo
			}
		}
		// Python: def xxx, class xxx, import xxx, from xxx
		if strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ") {
			return MimeTypeTextPython
		}
		// Java: public class, private, protected
		if strings.HasPrefix(line, "public class") || strings.HasPrefix(line, "private ") ||
			strings.HasPrefix(line, "protected ") {
			return MimeTypeTextJava
		}
		// C/C++: #include, #define, int main()
		if strings.HasPrefix(line, "#include") || strings.HasPrefix(line, "#define") ||
			strings.HasPrefix(line, "int main(") || strings.HasPrefix(line, "void main(") {
			return MimeTypeTextC
		}
		// Rust: fn main(), use xxx::
		if strings.HasPrefix(line, "fn main(") || strings.HasPrefix(line, "use ") ||
			strings.HasPrefix(line, "let mut ") || strings.HasPrefix(line, "impl ") {
			return MimeTypeTextRust
		}
		// JavaScript/TypeScript: const, let, var, function, =>
		if strings.HasPrefix(line, "const ") || strings.HasPrefix(line, "let ") ||
			strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "function ") ||
			strings.Contains(line, "=>") {
			// 可能是 JS 或 TS，需要进一步判断
			if strings.Contains(line, ": ") && (strings.Contains(line, "interface") || strings.Contains(line, "type ")) {
				return MimeTypeTextTypeScript
			}
			return MimeTypeTextJavaScript
		}
		// Ruby: def xxx, class xxx, module xxx
		if strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "module ") {
			return MimeTypeTextRuby
		}
		// PHP: <?php, $xxx =
		if strings.HasPrefix(line, "<?php") || strings.HasPrefix(line, "$") {
			return MimeTypeTextPHP
		}
		// Swift: func xxx, var xxx, let xxx, import xxx
		if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "var ") ||
			strings.HasPrefix(line, "let ") || strings.HasPrefix(line, "import ") {
			return MimeTypeTextSwift
		}
		// Kotlin: fun xxx, val xxx, var xxx, data class
		if strings.HasPrefix(line, "fun ") || strings.HasPrefix(line, "val ") ||
			strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "data class") {
			return MimeTypeTextKotlin
		}
		break
	}
	return ""
}

// ExtMimeTypes 是 MIME 类型到文件扩展名的反向映射
var ExtMimeTypes = map[string]string{
	// 基础文本格式
	MimeTypeTextPlain:       ".txt",
	MimeTypeTextMarkdown:    ".md",
	MimeTypeTextHTML:        ".html",
	MimeTypeApplicationJSON: ".json",
	MimeTypeTextCSS:         ".css",
	MimeTypeTextJavaScript:  ".js",
	MimeTypeTextXML:         ".xml",
	MimeTypeTextYAML:        ".yaml",
	MimeTypeTextTOML:        ".toml",
	MimeTypeTextCSV:         ".csv",
	MimeTypeApplicationXYAML: ".yaml",
	MimeTypeApplicationToml:  ".toml",
	// 编程语言
	MimeTypeTextPython:     ".py",
	MimeTypeTextGo:         ".go",
	MimeTypeTextJava:       ".java",
	MimeTypeTextC:          ".c",
	MimeTypeTextCPP:        ".cpp",
	MimeTypeTextCsharp:     ".cs",
	MimeTypeTextPHP:        ".php",
	MimeTypeTextRuby:       ".rb",
	MimeTypeTextPerl:       ".pl",
	MimeTypeTextBash:       ".sh",
	MimeTypeTextPowerShell: ".ps1",
	MimeTypeTextRust:       ".rs",
	MimeTypeTextSwift:      ".swift",
	MimeTypeTextKotlin:     ".kt",
	MimeTypeTextTypeScript: ".ts",
	MimeTypeTextVue:        ".vue",
	MimeTypeTextSvelte:     ".svelte",
	MimeTypeTextGraphQL:    ".graphql",
	// 图片格式
	MimeTypeImageJPEG:  ".jpg",
	MimeTypeImagePNG:   ".png",
	MimeTypeImageGIF:   ".gif",
	MimeTypeImageWebP:  ".webp",
	MimeTypeImageBMP:   ".bmp",
	MimeTypeImageSVG:   ".svg",
	// Office 文档
	MimeTypeApplicationMsWord:               ".doc",
	MimeTypeApplicationWordOpenXML:           ".docx",
	MimeTypeApplicationMsExcel:               ".xls",
	MimeTypeApplicationExcelOpenXML:          ".xlsx",
	MimeTypeApplicationMsPowerpoint:          ".ppt",
	MimeTypeApplicationPowerpointOpenXML:    ".pptx",
	MimeTypeApplicationPDF:                  ".pdf",
}
