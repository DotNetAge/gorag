package types

import (
	"strings"
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
)

// ParserFactory 定义了创建 Parser 实例的工厂函数
type ParserFactory func() core.Parser

// ParserRegistry 解析器工厂注册表, 按文件扩展名注册
type ParserRegistry struct {
	factories map[string]ParserFactory
	lock      sync.RWMutex
	once      sync.Once
}

// NewParserRegistry 创建新的按扩展名注册的注册表
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		factories: make(map[string]ParserFactory),
	}
}

// DefaultParserRegistry 兼容性函数
func DefaultParserRegistry() *ParserRegistry {
	return DefaultRegistry
}

// EnsureInitialized 懒加载初始化内置工厂，按所有支持的扩展名注册
func (r *ParserRegistry) EnsureInitialized() {
	r.once.Do(func() {
		builtins := []ParserFactory{
			func() core.Parser { return text.DefaultTextStreamParser(1024) },
			func() core.Parser { return markdown.DefaultMarkdownStreamParser(1) },
			func() core.Parser { return gocode.DefaultGocodeStreamParser() },
			func() core.Parser { return javacode.DefaultJavacodeStreamParser() },
			func() core.Parser { return pycode.DefaultPycodeStreamParser() },
			func() core.Parser { return tscode.DefaultParser() },
			func() core.Parser { return jscode.DefaultJscodeStreamParser() },
			func() core.Parser { return pdf.DefaultParser() },
			func() core.Parser { return docx.DefaultParser() },
			func() core.Parser { return excel.DefaultExcelStreamParser() },
			func() core.Parser { return csv.DefaultCSVStreamParser(100, true) },
			func() core.Parser { return json.DefaultJsonStreamParser() },
			func() core.Parser { return xml.DefaultParser() },
			func() core.Parser { return yaml.DefaultParser() },
			func() core.Parser { return log.DefaultParser() },
			func() core.Parser { return html.DefaultHtmlStreamParser() },
			func() core.Parser { return image.DefaultParser(nil) },
			func() core.Parser { return email.DefaultEmailStreamParser() },
			func() core.Parser { return ppt.DefaultParser() },
			func() core.Parser { return dbschema.DefaultDBSchemaStreamParser() },
		}

		r.lock.Lock()
		defer r.lock.Unlock()
		for _, factory := range builtins {
			// Get temporary instance to check supported extensions
			tempParser := factory()
			for _, ext := range tempParser.GetSupportedTypes() {
				extStr := strings.ToLower(ext)
				r.factories[extStr] = factory
			}
		}
	})
}

// RegisterByExtension 显式为一个或多个扩展名注册工厂
func (r *ParserRegistry) RegisterByExtension(factory ParserFactory, extensions ...string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, ext := range extensions {
		r.factories[strings.ToLower(ext)] = factory
	}
}

// Register 自动提取工厂支持的扩展名并注册
func (r *ParserRegistry) Register(factory ParserFactory) {
	tempParser := factory()
	exts := tempParser.GetSupportedTypes()
	
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, ext := range exts {
		r.factories[strings.ToLower(ext)] = factory
	}
}

// CreateByExtension 根据文件扩展名动态创建一个对应的 Parser 实例 (O(1) 查找)
func (r *ParserRegistry) CreateByExtension(ext string) (core.Parser, bool) {
	r.EnsureInitialized()
	
	ext = strings.ToLower(ext)
	r.lock.RLock()
	factory, ok := r.factories[ext]
	r.lock.RUnlock()
	
	if !ok {
		return nil, false
	}
	return factory(), true
}

// GetAllFactories 获取所有去重后的工厂函数
func (r *ParserRegistry) GetAllFactories() []ParserFactory {
	r.EnsureInitialized()
	r.lock.RLock()
	defer r.lock.RUnlock()

	// Use pointer addresses or a set logic to deduplicate, 
	// but simply creating an instance and collecting is fine for legacy AllParsers
	unique := make(map[string]ParserFactory)
	for _, f := range r.factories {
		// Just a simple way to get one factory per distinct parser logic
		// We use the first extension as a deduplication key
		temp := f()
		exts := temp.GetSupportedTypes()
		if len(exts) > 0 {
			unique[exts[0]] = f
		}
	}

	var result []ParserFactory
	for _, f := range unique {
		result = append(result, f)
	}
	return result
}

// --- Legacy Compatibility Functions (Optional, but kept to prevent breaking other code) ---

// Get 获取解析器 (Deprecated: use CreateByExtension)
func (r *ParserRegistry) Get(parserType ParserType) (core.Parser, bool) {
	// Simple mapping for legacy support
	str := strings.ToLower(parserType.String())
	ext := "." + str
	if str == "text" {
		ext = ".txt"
	}
	return r.CreateByExtension(ext)
}

// GetAll 获取所有解析器的新实例
func (r *ParserRegistry) GetAll() []core.Parser {
	factories := r.GetAllFactories()
	var result []core.Parser
	for _, f := range factories {
		result = append(result, f())
	}
	return result
}

// GetByTypes 根据类型获取新实例
func (r *ParserRegistry) GetByTypes(types ...ParserType) []core.Parser {
	var result []core.Parser
	for _, t := range types {
		str := strings.ToLower(t.String())
		ext := "." + str
		if str == "text" {
			ext = ".txt"
		}
		if p, ok := r.CreateByExtension(ext); ok {
			result = append(result, p)
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
