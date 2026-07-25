package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// =====================================================================
// 外部 JSON Schema 加载器
// =====================================================================

// rawSchema 对应 JSON Schema 文件完整结构，用于直接反序列化。
type rawSchema struct {
	Description string                  `json:"description"`
	Properties  map[string]rawProperty  `json:"properties"`
	Required    []string                `json:"required"`
}

// rawProperty 对应 JSON Schema 中单个 property 的原始结构。
type rawProperty struct {
	Type        string       `json:"type"`
	Description string       `json:"description"`
	Enum        []string     `json:"enum,omitempty"`
	Format      string       `json:"format,omitempty"`
	Items       *rawProperty `json:"items,omitempty"`
}

// LoadEntitySchema 从单个 JSON Schema 文件创建 EntitySchema。
//
// 规则：
//   - Type 使用文件名（去除 .json 扩展名）
//   - Description 直接使用文件内的 description 字段，不做任何格式化包装
//   - Properties 完整解析为 SchemaProperty 结构（类型、描述、枚举值、格式、数组元素类型）
func LoadEntitySchema(path string) (EntitySchema, error) {
	if path == "" {
		return EntitySchema{}, fmt.Errorf("llm.LoadEntitySchema: path 不能为空")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return EntitySchema{}, fmt.Errorf("llm.LoadEntitySchema: 解析路径失败: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return EntitySchema{}, fmt.Errorf("llm.LoadEntitySchema: 读取文件失败 %s: %w", absPath, err)
	}

	var rs rawSchema
	if err := json.Unmarshal(data, &rs); err != nil {
		return EntitySchema{}, fmt.Errorf("llm.LoadEntitySchema: 解析 JSON 失败 %s: %w", absPath, err)
	}

	name := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))

	props := make(map[string]SchemaProperty, len(rs.Properties))
	for k, rp := range rs.Properties {
		props[k] = rawToSchemaProperty(rp)
	}

	return EntitySchema{
		Type:        name,
		Description: rs.Description,
		Properties:  props,
		Required:    rs.Required,
	}, nil
}

// LoadEntitySchemasFromDir 扫描目录下所有 .json 文件并加载为 EntitySchema 列表。
//
// 规则：
//   - 仅加载直接子目录中的 .json 文件，不递归
//   - 加载失败单个文件时跳过并继续，最终汇总错误
//   - 空目录返回空列表
func LoadEntitySchemasFromDir(dir string) ([]EntitySchema, error) {
	if dir == "" {
		return nil, fmt.Errorf("llm.LoadEntitySchemasFromDir: dir 不能为空")
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("llm.LoadEntitySchemasFromDir: 解析目录失败: %w", err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("llm.LoadEntitySchemasFromDir: 读取目录失败 %s: %w", absDir, err)
	}

	var schemas []EntitySchema
	var errs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.EqualFold(filepath.Ext(name), ".json") {
			continue
		}

		schema, err := LoadEntitySchema(filepath.Join(absDir, name))
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		schemas = append(schemas, schema)
	}

	if len(errs) > 0 {
		return schemas, fmt.Errorf("llm.LoadEntitySchemasFromDir: 部分文件加载失败: %s", strings.Join(errs, "; "))
	}
	return schemas, nil
}

// rawToSchemaProperty 将原始 property 定义转换为 SchemaProperty。
func rawToSchemaProperty(rp rawProperty) SchemaProperty {
	sp := SchemaProperty{
		Type:        rp.Type,
		Description: rp.Description,
		Enum:        rp.Enum,
		Format:      rp.Format,
	}
	if rp.Items != nil {
		items := rawToSchemaProperty(*rp.Items)
		sp.Items = &items
	}
	return sp
}
