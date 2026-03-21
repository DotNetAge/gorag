package types

import (
	"sync"
	"github.com/DotNetAge/gorag/pkg/core"
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
)

var (
	DefaultRegistry = NewParserRegistry()
	once            sync.Once
)

// ParserRegistry 解析器注册表
type ParserRegistry struct {
	parsers map[ParserType]core.Parser
	lock    sync.RWMutex
}

// NewParserRegistry 创建新的注册表
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make(map[ParserType]core.Parser),
	}
}

// DefaultParserRegistry 兼容性函数
func DefaultParserRegistry() *ParserRegistry {
	return DefaultRegistry
}

// EnsureInitialized 懒加载初始化
func (r *ParserRegistry) EnsureInitialized() {
	once.Do(func() {
		r.lock.Lock()
		defer r.lock.Unlock()
		
		r.parsers[TEXT] = text.DefaultTextStreamParser(1024)
		r.parsers[MARKDOWN] = markdown.DefaultMarkdownStreamParser(1)
		r.parsers[GOCODE] = gocode.DefaultGocodeStreamParser()
		r.parsers[JAVACODE] = javacode.DefaultJavacodeStreamParser()
		r.parsers[PYCODE] = pycode.DefaultPycodeStreamParser()
		r.parsers[TSCODE] = tscode.DefaultParser()
		r.parsers[JSCODE] = jscode.DefaultJscodeStreamParser()
		r.parsers[PDF] = pdf.DefaultParser()
		r.parsers[DOCX] = docx.DefaultParser()
		r.parsers[EXCEL] = excel.DefaultExcelStreamParser()
		r.parsers[CSV] = csv.DefaultCSVStreamParser(100, true)
		r.parsers[JSON] = json.DefaultJsonStreamParser()
		r.parsers[XML] = xml.DefaultParser()
		r.parsers[YAML] = yaml.DefaultParser()
		r.parsers[LOG] = log.DefaultParser()
		r.parsers[HTML] = html.DefaultHtmlStreamParser()
		r.parsers[IMAGE] = image.DefaultParser(nil)
		r.parsers[EMAIL] = email.DefaultEmailStreamParser()
		r.parsers[PPT] = ppt.DefaultParser()
		r.parsers[DBSCHEMA] = dbschema.DefaultDBSchemaStreamParser()
	})
}

// Register 注册一个解析器
func (r *ParserRegistry) Register(parserType ParserType, parser core.Parser) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.parsers[parserType] = parser
}

// Get 获取解析器
func (r *ParserRegistry) Get(parserType ParserType) (core.Parser, bool) {
	r.EnsureInitialized()
	r.lock.RLock()
	defer r.lock.RUnlock()
	p, ok := r.parsers[parserType]
	return p, ok
}

// GetAll 获取所有解析器
func (r *ParserRegistry) GetAll() []core.Parser {
	r.EnsureInitialized()
	r.lock.RLock()
	defer r.lock.RUnlock()
	
	result := make([]core.Parser, 0, len(r.parsers))
	for _, parser := range r.parsers {
		result = append(result, parser)
	}
	return result
}

// GetByTypes 根据类型获取
func (r *ParserRegistry) GetByTypes(types ...ParserType) []core.Parser {
	r.EnsureInitialized()
	r.lock.RLock()
	defer r.lock.RUnlock()
	
	result := make([]core.Parser, 0, len(types))
	for _, t := range types {
		if parser, ok := r.parsers[t]; ok {
			result = append(result, parser)
		}
	}
	return result
}

// Parsers 导出函数
func Parsers(parserTypes ...ParserType) []core.Parser {
	return DefaultRegistry.GetByTypes(parserTypes...)
}

// AllParsers 导出函数
func AllParsers() []core.Parser {
	return DefaultRegistry.GetAll()
}

// DefaultParser 导出函数
func DefaultParser(parserType ParserType) (core.Parser, error) {
	p, ok := DefaultRegistry.Get(parserType)
	if !ok {
		return nil, ErrParserNotFound
	}
	return p, nil
}
