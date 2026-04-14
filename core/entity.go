package core

import (
	"github.com/DotNetAge/gorag/utils"
)

// StructureNode 文档结构节点，对应文档中的标题、段落、列表、表格等单元
type StructureNode struct {
	NodeType string           `json:"node_type"` // 节点类型（heading/paragraph/table/list 等）
	Title    string           `json:"title"`     // 节点标题（仅 heading 类型有效）
	Level    int              `json:"level"`     // 标题层级（仅 heading 类型有效，H1=1、H2=2...）
	Text     string           `json:"text"`      // 清洗后的纯文本内容（核心，无任何格式垃圾）
	StartPos int              `json:"start_pos"` // 文本在原始清洗后内容中的起始位置（用于分块定位）
	EndPos   int              `json:"end_pos"`   // 文本在原始清洗后内容中的结束位置（用于分块定位）
	Children []*StructureNode `json:"children"`  // 子节点（如 H1 下的 H2、段落下的列表）
}

func (n *StructureNode) ID() string {
	if n.Text == "" {
		return ""
	}
	return utils.GenerateID([]byte(n.Text))
}

// Clean 清洗当前节点的 Text 和 Title 字段，并递归清洗所有子节点
func (n *StructureNode) Clean() {
	// 清洗 Title 字段（仅对 heading 类型有效）
	if n.NodeType == "heading" && n.Title != "" {
		n.Title = CleanText(n.Title)
	}

	// 清洗 Text 字段
	if n.Text != "" {
		n.Text = CleanText(n.Text)
	}

	// 递归清洗子节点
	for _, child := range n.Children {
		child.Clean()
	}
}

// CleanText 按默认顺序应用所有清洗函数
// 清洗顺序：全角半角 → 噪音字符 → 链接 → 行号 → 水印 → 繁简转换 → 隐私脱敏 → 段落规范化 → 停用词 → 基础清洗
func CleanText(text string) string {
	if text == "" {
		return ""
	}

	// 1. 全角半角转换（先处理字符编码问题）
	text = utils.ToHalfWidth(text)

	// 2. 去除噪音字符（控制字符、多余空白等）
	text = utils.CleanNoise(text)

	// 3. 去除链接（Markdown链接、HTML链接、裸露URL）
	text = utils.RemoveLinks(text)

	// 4. 去除代码行号
	text = utils.RemoveLineNumbers(text)

	// 5. 去除水印
	text = utils.RemoveWatermarks(text)

	// 6. 繁简转换（统一为简体中文）
	text = utils.NormalizeChinese(text)

	// 7. 隐私脱敏（身份证、手机号、银行卡、API密钥、邮箱）
	text = utils.DesensitizePII(text)

	// 8. 段落规范化（合并多余换行、去除行首行尾空格）
	text = utils.NormalizeParagraphs(text)

	// 9. 去除停用词（可选，根据场景决定是否启用）
	// text = utils.RemoveStopWords(text)

	// 10. 基础清洗（去特殊字符、合并空格）
	text = utils.Clean(text)

	return text
}

// StructuredDocument 结构化文档，以树形结构呈现整个文档的层级关系
type StructuredDocument struct {
	RawDoc Document       `json:"raw_doc"` // 原始文档对象
	Title  string         `json:"title"`   // 文档总标题（清洗后）
	Root   *StructureNode `json:"root"`    // 文档结构根节点（顶层节点）
}

func (s *StructuredDocument) ID() string {
	return s.RawDoc.GetID()
}

func (s *StructuredDocument) Meta() map[string]any {
	return s.RawDoc.GetMeta()
}

func (s *StructuredDocument) SetValue(key string, value any) *StructuredDocument {
	s.RawDoc.GetMeta()[key] = value
	return s
}

// Entity 实体结构，统一承载所有类型实体（人名、机构、术语、商品等）
type Entity struct {
	ID         string         `json:"id"`         // 实体全局唯一ID（如 UUID）
	Name       string         `json:"name"`       // 实体名称（清洗后的纯文本）
	EntityType string         `json:"entityType"` // 实体类型（PERSON/ORG/TERM/SKU/OBJECT 等）
	Confidence float32        `json:"confidence"` // 实体抽取置信度（0~1，来自抽取模型/规则）
	SourceNode string         `json:"sourceNode"` // 实体来源的 StructureNode ID（追溯实体位置）
	Features   map[string]any `json:"features"`   // 实体扩展特征（如实体长度、出现次数、语义向量等）
}

// Relation 实体关系结构（预留扩展，用于知识图谱）
type Relation struct {
	ID        string  `json:"id"`        // 关系唯一ID
	Subject   *Entity `json:"subject"`   // 主体实体（关系发起方）
	Predicate string  `json:"predicate"` // 关系类型（如“就职于”“属于”“包含”）
	Object    *Entity `json:"object"`    // 客体实体（关系接收方）
	Score     float32 `json:"score"`     // 关系抽取置信度（0~1）
}

// ChunkMeta Chunk 固定元数据（分块相关位置、层级信息）
type ChunkMeta struct {
	Index        int      `json:"index"`         // 分块在文档中的序号（0,1,2...）
	StartPos     int      `json:"start_pos"`     // 分块在原始清洗后文本中的起始位置
	EndPos       int      `json:"end_pos"`       // 分块在原始清洗后文本中的结束位置
	HeadingLevel int      `json:"heading_level"` // 分块对应的标题层级（来自 StructureNode）
	HeadingPath  []string `json:"heading_path"`  // 分块对应的标题路径（如 ["第一章","1.1节"]）
}

// Chunk 最终可索引单元，由 Chunker 生成，承接解析层所有信息
type Chunk struct {
	ID        string         `json:"id"`         // Chunk 唯一ID
	ParentID  string         `json:"parent_id"`  // 父Chunk/父文档ID（来自 RawDocument.Source）
	DocID     string         `json:"doc_id"`     // 原始文档ID（来自 RawDocument.ID）
	MIMEType  string         `json:"mime_type"`  // 内容类型
	Content   string         `json:"content"`    // 分块内容（清洗后纯文本）
	Metadata  map[string]any `json:"metadata"`   // 扩展元数据（来自 RawDocument.Metadata）
	ChunkMeta ChunkMeta      `json:"chunk_meta"` // 分块固定元数据
}

// Hit 搜索结果结构
type Hit struct {
	ID      string  `json:"id"`      // 结果ID
	Score   float32 `json:"score"`   // 相似度分数
	Content string  `json:"content"` // 结果内容
	DocID   string  `json:"doc_id"`  // 文档ID
}
