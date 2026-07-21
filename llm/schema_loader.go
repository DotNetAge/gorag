package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// =====================================================================
// 外部 JSON Schema 加载器
// =====================================================================

// schemaFile 表示外部 JSON Schema 文件的原始结构。
//
// 仅解析生成 EntitySchema 所需的字段，未知字段忽略。
type schemaFile struct {
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties"`
	Required    []string               `json:"required"`
}

// LoadEntitySchema 从单个 JSON Schema 文件创建 EntitySchema。
//
// 规则：
//   - Type 使用文件名（去除 .json 扩展名）
//   - Prompt 优先使用文件内的 description，缺失时 fallback 到 title，再缺失时使用 Type
//   - JSONSchema 保留原始 JSON 内容（压缩为单行），注入 LLM 提示词
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

	var sf schemaFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return EntitySchema{}, fmt.Errorf("llm.LoadEntitySchema: 解析 JSON 失败 %s: %w", absPath, err)
	}

	name := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	prompt := sf.Description
	if prompt == "" {
		prompt = sf.Title
	}
	if prompt == "" {
		prompt = name
	}

	compact, err := compactJSON(data)
	if err != nil {
		return EntitySchema{}, fmt.Errorf("llm.LoadEntitySchema: 压缩 JSON 失败 %s: %w", absPath, err)
	}

	return EntitySchema{
		Type:       name,
		Prompt:     fmt.Sprintf("**%s** — %s", name, prompt),
		JSONSchema: compact,
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

// compactJSON 将 JSON 字节压缩为单行字符串，减少 prompt 占用。
func compactJSON(data []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
