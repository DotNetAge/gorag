package types

import "fmt"

// ParserType 解析器类型枚举
type ParserType int

const (
	// UNKNOWN 未知类型
	UNKNOWN ParserType = iota
	// TEXT 纯文本文件 (.txt, .md, .csv, .log)
	TEXT
	// MARKDOWN Markdown 文件 (.md)
	MARKDOWN
	// GOCODE Go 源代码 (.go)
	GOCODE
	// JAVACODE Java 源代码 (.java)
	JAVACODE
	// PYCODE Python 源代码 (.py)
	PYCODE
	// TSCODE TypeScript 源代码 (.ts, .tsx)
	TSCODE
	// JSCODE JavaScript 源代码 (.js, .jsx)
	JSCODE
	// PDF PDF 文档 (.pdf)
	PDF
	// DOCX Word 文档 (.docx)
	DOCX
	// EXCEL Excel 表格 (.xlsx, .xls)
	EXCEL
	// CSV CSV 文件 (.csv)
	CSV
	// JSON JSON 文件 (.json)
	JSON
	// XML XML 文件 (.xml)
	XML
	// YAML YAML 文件 (.yaml, .yml)
	YAML
	// LOG 日志文件 (.log)
	LOG
	// HTML HTML 文件 (.html, .htm)
	HTML
	// IMAGE 图片文件 (.jpg, .png, .gif, .webp)
	IMAGE
	// EMAIL 邮件文件 (.eml)
	EMAIL
	// PPT PowerPoint 演示文稿 (.pptx)
	PPT
	// DBSCHEMA 数据库 Schema 文件 (.sql)
	DBSCHEMA
)

// String 返回 ParserType 的字符串表示
func (p ParserType) String() string {
	switch p {
	case TEXT:
		return "text"
	case MARKDOWN:
		return "markdown"
	case GOCODE:
		return "gocode"
	case JAVACODE:
		return "javacode"
	case PYCODE:
		return "pycode"
	case TSCODE:
		return "tscode"
	case JSCODE:
		return "jscode"
	case PDF:
		return "pdf"
	case DOCX:
		return "docx"
	case EXCEL:
		return "excel"
	case CSV:
		return "csv"
	case JSON:
		return "json"
	case XML:
		return "xml"
	case YAML:
		return "yaml"
	case LOG:
		return "log"
	case HTML:
		return "html"
	case IMAGE:
		return "image"
	case EMAIL:
		return "email"
	case PPT:
		return "ppt"
	case DBSCHEMA:
		return "dbschema"
	default:
		return "unknown"
	}
}

// ErrParserNotFound 解析器未找到错误
var ErrParserNotFound = fmt.Errorf("parser not found")
