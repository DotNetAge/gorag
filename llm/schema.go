package llm

// =====================================================================
// 实体 Schema 定义
// =====================================================================

// SchemaProperty 描述实体属性的类型约束。
//
// 直接对应 JSON Schema 的 property 定义，保留所有约束信息，
// 用于生成精确的 LLM 提示词。
type SchemaProperty struct {
	Type        string          `json:"type"`
	Description string          `json:"description"`
	Enum        []string        `json:"enum,omitempty"`
	Format      string          `json:"format,omitempty"`
	Items       *SchemaProperty `json:"items,omitempty"`
}

// EntitySchema 定义 Refiller 要提取的实体类型。
//
// 设计要点：
//   - 完整解析外部 JSON Schema 文件的结构化信息，不丢弃任何约束定义
//   - Description 即 Prompt —— 直接来自 JSON Schema 的 description 字段，不做额外包装
//   - Properties 保留所有属性的类型、枚举值、格式等约束，用于生成精确的 LLM 提示词
//   - Type 将作为 core.Node.Labels[0]
type EntitySchema struct {
	Type        string                    // 实体类型名，来自文件名
	Description string                    // 实体说明，直接来自 JSON Schema 的 description 字段
	Properties  map[string]SchemaProperty // 属性定义，来自 JSON Schema 的 properties
	Required    []string                  // 必填属性列表，来自 JSON Schema 的 required
}
