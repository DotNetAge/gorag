package types

import (
	"github.com/DotNetAge/gorag/pkg/indexing/parser/csv"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/dbschema"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/docx"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/email"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/excel"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/gocode"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/html"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/image"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/javacode"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/jscode"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/json"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/log"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/markdown"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/pdf"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/ppt"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/pycode"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/text"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/tscode"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/xml"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/yaml"
	"github.com/DotNetAge/gorag/pkg/core"
)

// ParserRegistry 解析器注册表
type ParserRegistry struct {
	parsers map[ParserType]core.Parser
}

// DefaultParserRegistry 创建新的解析器注册表
func DefaultParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make(map[ParserType]core.Parser),
	}
}

// Register 注册一个解析器
func (r *ParserRegistry) Register(parserType ParserType, parser core.Parser) {
	r.parsers[parserType] = parser
}

// Get 获取指定类型的解析器
func (r *ParserRegistry) Get(parserType ParserType) (core.Parser, bool) {
	parser, ok := r.parsers[parserType]
	return parser, ok
}

// GetAll 获取所有已注册的解析器
func (r *ParserRegistry) GetAll() []core.Parser {
	result := make([]core.Parser, 0, len(r.parsers))
	for _, parser := range r.parsers {
		result = append(result, parser)
	}
	return result
}

// GetByTypes 根据类型列表获取解析器
func (r *ParserRegistry) GetByTypes(types ...ParserType) []core.Parser {
	result := make([]core.Parser, 0, len(types))
	for _, t := range types {
		if parser, ok := r.parsers[t]; ok {
			result = append(result, parser)
		}
	}
	return result
}

// DefaultRegistry 默认注册表实例
var DefaultRegistry = DefaultParserRegistry()

// init 初始化默认注册表
func init() {
	// 注册所有内置解析器
	DefaultRegistry.Register(TEXT, text.DefaultTextStreamParser(1024))
	DefaultRegistry.Register(MARKDOWN, markdown.DefaultMarkdownStreamParser(1))
	DefaultRegistry.Register(GOCODE, gocode.DefaultGocodeStreamParser())
	DefaultRegistry.Register(JAVACODE, javacode.DefaultJavacodeStreamParser())
	DefaultRegistry.Register(PYCODE, pycode.DefaultPycodeStreamParser())
	DefaultRegistry.Register(TSCODE, tscode.DefaultParser())
	DefaultRegistry.Register(JSCODE, jscode.DefaultJscodeStreamParser())
	DefaultRegistry.Register(PDF, pdf.DefaultParser())
	DefaultRegistry.Register(DOCX, docx.DefaultParser())
	DefaultRegistry.Register(EXCEL, excel.DefaultExcelStreamParser())
	DefaultRegistry.Register(CSV, csv.DefaultCSVStreamParser(100, true))
	DefaultRegistry.Register(JSON, json.DefaultJsonStreamParser())
	DefaultRegistry.Register(XML, xml.DefaultParser())
	DefaultRegistry.Register(YAML, yaml.DefaultParser())
	DefaultRegistry.Register(LOG, log.DefaultParser())
	DefaultRegistry.Register(HTML, html.DefaultHtmlStreamParser())
	DefaultRegistry.Register(IMAGE, image.DefaultParser(nil)) // LLM client can be injected later
	DefaultRegistry.Register(EMAIL, email.DefaultEmailStreamParser())
	DefaultRegistry.Register(PPT, ppt.DefaultParser())
	DefaultRegistry.Register(DBSCHEMA, dbschema.DefaultDBSchemaStreamParser())
}

// Parsers 根据类型名称创建解析器列表
func Parsers(parserTypes ...ParserType) []core.Parser {
	return DefaultRegistry.GetByTypes(parserTypes...)
}

// AllParsers 加载所有已注册的解析器 (20+)
func AllParsers() []core.Parser {
	return DefaultRegistry.GetAll()
}

// DefaultParser 创建单个解析器实例
func DefaultParser(parserType ParserType) (core.Parser, error) {
	parser, ok := DefaultRegistry.Get(parserType)
	if !ok {
		return nil, ErrParserNotFound
	}
	return parser, nil
}
