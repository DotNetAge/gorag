# Extractor 接口 NER 实现示例

说明：本示例采用「传统 CRF 模型」实现 NER 实体抽取（非大模型、非 LLM），轻量无外部大模型依赖，适配解析层 Extractor 接口，输出符合设计的 Entity 结构，支持基础实体类型（PERSON/ORG/TERM）。

## 1. 依赖准备（传统 NER 所需轻量依赖）

使用 Go 生态中成熟的 CRF 库（非大模型，无 GPU 依赖），可直接 go get 安装：

```bash
go get github.com/kljensen/snowball
go get github.com/james-bowman/nlp
```

## 2. 完整实现代码（贴合解析层 Extractor 接口）

```go
package gorag

import (
	"github.com/james-bowman/nlp"
	"github.com/kljensen/snowball/english"
	"strings"
	"unicode"
)

// CRFEntityExtractor 基于传统 CRF 模型的实体抽取器（非大模型）
// 实现 Extractor 接口，不依赖 LLM、不依赖大模型，轻量可部署
type CRFEntityExtractor struct {
	// crfModel CRF 模型实例（预训练好的基础 NER 模型，可本地加载）
	crfModel *nlp.CRF
	// entityTypes 支持的实体类型（可扩展）
	entityTypes map[string]bool
}

// NewCRFEntityExtractor 初始化 CRF 实体抽取器（加载预训练 CRF 模型）
func NewCRFEntityExtractor() (*CRFEntityExtractor, error) {
	// 加载本地预训练的 CRF 模型（非大模型，体积小，可嵌入部署）
	// 实际项目中，可将预训练模型文件放入项目目录，此处简化为示例
	crfModel, err := nlp.LoadCRFModel("./crf_ner_model.bin")
	if err != nil {
		return nil, err
	}

	return &CRFEntityExtractor{
		crfModel: crfModel,
		entityTypes: map[string]bool{
			"PERSON": true,  // 人名
			"ORG":    true,  // 机构名
			"TERM":   true,  // 专业术语
		},
	}, nil
}

// Extract 实现 Extractor 接口，从结构化文档中抽取实体
// 输入：清洗后的 StructuredDocument（干净纯文本）
// 输出：符合设计的 []*Entity，置信度来自 CRF 模型预测分数
func (e *CRFEntityExtractor) Extract(structured *StructuredDocument) ([]*Entity, error) {
	var entities []*Entity

	// 遍历结构化文档的所有节点（StructureNode），逐节点抽取实体
	// 递归遍历树形结构，确保所有段落、标题都能被处理
	err := e.traverseStructureNode(structured.Root, &entities)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

// traverseStructureNode 递归遍历 StructureNode，抽取每个节点的实体
func (e *CRFEntityExtractor) traverseStructureNode(node *StructureNode, entities *[]*Entity) error {
	if node == nil || node.Text == "" {
		return nil
	}

	// 1. 文本预处理（轻量，配合 CRF 模型，无需复杂清洗——节点文本已在 Structurizer 中清洗完成）
	processedText := e.preprocessText(node.Text)

	// 2. 用 CRF 模型预测实体（核心步骤，非大模型，本地推理）
	// 输出格式：[]struct{Text string; Label string; Score float32}
	predictions, err := e.crfModel.Predict(processedText)
	if err != nil {
		return err
	}

	// 3. 解析预测结果，转化为设计中的 Entity 结构
	for _, pred := range predictions {
		// 过滤不支持的实体类型
		if !e.entityTypes[pred.Label] {
			continue
		}
		// 过滤低置信度实体（可自定义阈值，此处设为 0.6）
		if pred.Score < 0.6 {
			continue
		}

		// 构建 Entity（完全贴合之前设计的结构）
		entity := &Entity{
			ID:          generateEntityID(pred.Text, pred.Label), // 生成全局唯一ID（简化实现）
			Name:        pred.Text,                              // 实体名称（清洗后纯文本）
			EntityType:  pred.Label,                             // 实体类型（PERSON/ORG/TERM）
			Confidence:  pred.Score,                            // 置信度（来自 CRF 模型预测分数）
			SourceNode:  generateNodeID(node),                  // 实体来源的 StructureNode ID（追溯位置）
			Features: map[string]interface{}{                   // 扩展特征（简单示例）
				"length": len(pred.Text),
				"pos":    []int{node.StartPos, node.EndPos},
			},
		}

		*entities = append(*entities, entity)
	}

	// 递归处理子节点
	for _, child := range node.Children {
		if err := e.traverseStructureNode(child, entities); err != nil {
			return err
		}
	}

	return nil
}

// preprocessText 轻量文本预处理（配合 CRF 模型，不重复清洗）
// 输入：StructureNode 中已清洗的纯文本
func (e *CRFEntityExtractor) preprocessText(text string) string {
	// 1. 统一小写（可选，根据 CRF 模型训练情况调整）
	text = strings.ToLower(text)
	// 2. 分词（简单分词，配合 CRF 模型）
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	// 3. 词干提取（可选，优化实体识别精度）
	for i, word := range words {
		words[i] = english.Stem(word, false)
	}
	return strings.Join(words, " ")
}

// generateEntityID 生成实体全局唯一ID（简化实现，实际可使用 UUID）
func generateEntityID(name, typ string) string {
	return typ + "_" + strings.ReplaceAll(name, " ", "_")
}

// generateNodeID 生成 StructureNode 唯一ID（简化实现，实际可使用节点的位置+标题生成）
func generateNodeID(node *StructureNode) string {
	return node.NodeType + "_" + string(node.Level) + "_" + strings.ReplaceAll(node.Title, " ", "_")
}
```

## 3. 关键说明（贴合解析层设计，无大模型依赖）

- 核心依赖：使用传统 CRF 模型（非大模型），体积小、无 GPU 依赖，可本地嵌入部署，无需调用外部大模型 API；

- 置信度来源：Entity.Confidence 直接取自 CRF 模型的预测分数（0~1），无需手动赋值，符合之前的设计要求；

- 适配性：完全实现 Extractor 接口，输入为 StructuredDocument（清洗后的结构化文本），输出为 []*Entity，可直接接入解析层流水线；

- 可扩展性：支持新增实体类型（如地名、商品名），只需修改 entityTypes 映射，并重新训练 CRF 模型即可；

- 文本处理：仅做轻量预处理（分词、词干提取），不重复清洗——因为 StructureNode.Text 已在 Structurizer 中完成清洗，符合解析层职责划分。

## 4. 使用示例（接入解析层流水线）

```go
// 1. 初始化 CRF 实体抽取器（非大模型）
extractor, err := NewCRFEntityExtractor()
if err != nil {
	log.Fatalf("初始化实体抽取器失败：%v", err)
}

// 2. 假设已通过 Structurizer 得到结构化文档（cleanedStructuredDoc）
// 3. 调用 Extract 接口抽取实体（无大模型依赖）
entities, err := extractor.Extract(cleanedStructuredDoc)
if err != nil {
	log.Fatalf("实体抽取失败：%v", err)
}

// 4. 后续可将 entities 传入 Chunker，用于分块（贴合解析层完整流程）
```

## 5. 补充说明

本示例中 CRF 模型的预训练文件（crf_ner_model.bin），可通过少量标注数据（实体标注文本）训练得到，无需依赖大模型训练资源。对于简单场景（如固定实体类型、规范文本），传统 CRF 模型的识别精度完全满足解析层的实体抽取需求，且部署成本远低于大模型。
> （注：文档部分内容可能由 AI 生成）