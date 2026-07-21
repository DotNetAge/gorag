package llm

// =====================================================================
// 实体 Schema 定义
// =====================================================================

// EntitySchema 定义 Refiller 要提取的实体类型。
//
// 设计要点：
//   - 采用模板+插件式设计，按文件类型动态注入对应 Schema
//   - 字段保持简化，避免复杂字段和数组字段，提高 LLM 执行有效性
//   - Type 将作为 core.Node.Labels[0]
//   - Prompt 注入 LLM 提示词的实体说明段
//   - JSONSchema 可选，用于约束实体属性
type EntitySchema struct {
	Type       string
	Prompt     string
	JSONSchema string
}

// CodeEntitySchemas 代码文件默认 Schema。
var CodeEntitySchemas = []EntitySchema{
	{Type: "function", Prompt: "**Function** — 可调用单元，包含参数与返回值"},
	{Type: "class", Prompt: "**Class** — 类型定义，包含字段与方法"},
	{Type: "interface", Prompt: "**Interface** — 契约定义"},
	{Type: "method", Prompt: "**Method** — 绑定到类或结构体的方法"},
	{Type: "variable", Prompt: "**Variable** — 模块级或全局绑定"},
	{Type: "module", Prompt: "**Module** — 可导入单元"},
}

// DocumentEntitySchemas 文档类文件默认 Schema。
var DocumentEntitySchemas = []EntitySchema{
	{Type: "person", Prompt: "**Person** — 作者、专家、贡献者"},
	{Type: "organization", Prompt: "**Organization** — 公司、机构、项目团队"},
	{Type: "concept", Prompt: "**Concept** — 核心观点、理论或方法论"},
	{Type: "location", Prompt: "**Location** — 地理位置"},
	{Type: "date", Prompt: "**Date** — 时间引用"},
	{Type: "term", Prompt: "**Term** — 领域专有术语"},
}

// DataEntitySchemas 数据类文件默认 Schema。
var DataEntitySchemas = []EntitySchema{
	{Type: "table", Prompt: "**Table** — 命名数据表或集合"},
	{Type: "field", Prompt: "**Field** — 列或属性名"},
	{Type: "record", Prompt: "**Record** — 具有业务意义的记录"},
	{Type: "key", Prompt: "**Key** — 主键或外键字段"},
}

// SchemasByDocType 按文档类型返回默认 Schema。
func SchemasByDocType(docType string) []EntitySchema {
	switch docType {
	case "code", "text":
		return CodeEntitySchemas
	case "data":
		return DataEntitySchemas
	case "document":
		return DocumentEntitySchemas
	default:
		return DocumentEntitySchemas
	}
}
