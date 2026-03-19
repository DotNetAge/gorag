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
	"github.com/DotNetAge/gorag/pkg/indexing/parser/base"
)

// ParserRegistry 解析器注册表
type ParserRegistry struct {
	parsers map[ParserType]base.Parser
}

// NewParserRegistry 创建新的解析器注册表
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make(map[ParserType]base.Parser),
	}
}

// Register 注册一个解析器
func (r *ParserRegistry) Register(parserType ParserType, parser base.Parser) {
	r.parsers[parserType] = parser
}

// Get 获取指定类型的解析器
func (r *ParserRegistry) Get(parserType ParserType) (base.Parser, bool) {
	parser, ok := r.parsers[parserType]
	return parser, ok
}

// GetAll 获取所有已注册的解析器
func (r *ParserRegistry) GetAll() []base.Parser {
	result := make([]base.Parser, 0, len(r.parsers))
	for _, parser := range r.parsers {
		result = append(result, parser)
	}
	return result
}

// GetByTypes 根据类型列表获取解析器
func (r *ParserRegistry) GetByTypes(types ...ParserType) []base.Parser {
	result := make([]base.Parser, 0, len(types))
	for _, t := range types {
		if parser, ok := r.parsers[t]; ok {
			result = append(result, parser)
		}
	}
	return result
}

// DefaultRegistry 默认注册表实例
var DefaultRegistry = NewParserRegistry()

// init 初始化默认注册表
func init() {
	// 注册所有内置解析器
	DefaultRegistry.Register(TEXT, text.NewTextStreamParser(1024))
	DefaultRegistry.Register(MARKDOWN, markdown.NewMarkdownStreamParser(1))
	DefaultRegistry.Register(GOCODE, gocode.NewGocodeStreamParser())
	DefaultRegistry.Register(JAVACODE, javacode.NewJavacodeStreamParser())
	DefaultRegistry.Register(PYCODE, pycode.NewPycodeStreamParser())
	DefaultRegistry.Register(TSCODE, tscode.NewParser())
	DefaultRegistry.Register(JSCODE, jscode.NewJscodeStreamParser())
	DefaultRegistry.Register(PDF, pdf.NewParser())
	DefaultRegistry.Register(DOCX, docx.NewParser())
	DefaultRegistry.Register(EXCEL, excel.NewExcelStreamParser())
	DefaultRegistry.Register(CSV, csv.NewCSVStreamParser(100, true))
	DefaultRegistry.Register(JSON, json.NewJsonStreamParser())
	DefaultRegistry.Register(XML, xml.NewParser())
	DefaultRegistry.Register(YAML, yaml.NewParser())
	DefaultRegistry.Register(LOG, log.NewParser())
	DefaultRegistry.Register(HTML, html.NewHtmlStreamParser())
	DefaultRegistry.Register(IMAGE, image.NewParser(nil)) // LLM client can be injected later
	DefaultRegistry.Register(EMAIL, email.NewEmailStreamParser())
	DefaultRegistry.Register(PPT, ppt.NewParser())
	DefaultRegistry.Register(DBSCHEMA, dbschema.NewDBSchemaStreamParser())
}

// Parsers 根据类型名称创建解析器列表
func Parsers(parserTypes ...ParserType) []base.Parser {
	return DefaultRegistry.GetByTypes(parserTypes...)
}

// AllParsers 加载所有已注册的解析器 (20+)
func AllParsers() []base.Parser {
	return DefaultRegistry.GetAll()
}

// NewParser 创建单个解析器实例
func NewParser(parserType ParserType) (base.Parser, error) {
	parser, ok := DefaultRegistry.Get(parserType)
	if !ok {
		return nil, ErrParserNotFound
	}
	return parser, nil
}
