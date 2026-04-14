package core

// Extractor 实体抽取接口，从结构化文档中抽取实体
type Extractor interface {
	// Extract 接收结构化文档，抽取实体列表，返回实体集合
	Extract(structured *StructuredDocument) ([]*Entity, []*Relation, error)
}
