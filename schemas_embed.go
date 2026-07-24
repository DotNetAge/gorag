package gorag

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed schemas
var schemasFS embed.FS

// SchemaCategory 表示一个 schema 类别目录。
type SchemaCategory struct {
	Name    string        `json:"name"`    // 类别名（如 enterprise, general）
	Schemas []SchemaEntry `json:"schemas"` // 该类别下的 schema 列表
}

// SchemaEntry 表示一个 schema 文件的信息。
type SchemaEntry struct {
	Category    string `json:"category"`     // 类别名（如 enterprise, general）
	Name        string `json:"name"`         // Schema 名（不含 .json 扩展名）
	DisplayName string `json:"display_name"` // 用于展示的中文/友好名称
}

// SchemaCategoryList 返回嵌入的 schemas 分类列表。
// 每个分类包含该类别下的所有 schema 文件名。
func SchemaCategoryList() ([]SchemaCategory, error) {
	entries, err := fs.ReadDir(schemasFS, "schemas")
	if err != nil {
		return nil, fmt.Errorf("读取嵌入 schema 目录失败: %w", err)
	}

	var categories []SchemaCategory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		categoryName := entry.Name()
		cat := SchemaCategory{Name: categoryName}

		schemaEntries, err := fs.ReadDir(schemasFS, filepath.Join("schemas", categoryName))
		if err != nil {
			continue
		}
		for _, se := range schemaEntries {
			if se.IsDir() {
				continue
			}
			if !strings.EqualFold(filepath.Ext(se.Name()), ".json") {
				continue
			}
			name := strings.TrimSuffix(se.Name(), ".json")
			cat.Schemas = append(cat.Schemas, SchemaEntry{
				Category:    categoryName,
				Name:        name,
				DisplayName: name, // 后续可改为从 schema 内容读取 title 或 description
			})
		}

		if len(cat.Schemas) > 0 {
			categories = append(categories, cat)
		}
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})
	return categories, nil
}

// SchemaContent 从嵌入的 FS 中读取指定 schema 的 JSON 内容。
func SchemaContent(category, name string) ([]byte, error) {
	path := filepath.Join("schemas", category, name+".json")
	data, err := fs.ReadFile(schemasFS, path)
	if err != nil {
		return nil, fmt.Errorf("读取嵌入 schema 失败: %w", err)
	}
	return data, nil
}

// SchemaContentBytes 从嵌入的 FS 读取 schema 文件内容，返回字符串。
func SchemaContentString(category, name string) (string, error) {
	data, err := SchemaContent(category, name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ── .rag/schemas 配置库管理 ──────────────────────────────────────────

// SchemaConfigDir 返回 .rag 库中的 schemas 配置目录。
func SchemaConfigDir(ragDir string) string {
	return filepath.Join(ragDir, "schemas")
}

// DirSchemaDir 返回指定目录对应的 schema 配置子目录。
// 使用目录绝对路径的 SHA256 摘要作为子目录名。
// 若 dir 为空，返回 "all"（默认全局配置）。
func DirSchemaDir(ragDir, dir string) string {
	base := SchemaConfigDir(ragDir)
	if dir == "" {
		return filepath.Join(base, "all")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Join(base, "all")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(absDir)))
	return filepath.Join(base, hash[:16]) // 取前 16 位作为目录名，足够区分
}

// schemasFileName 生成 schema 文件在配置目录中的文件名。
// 使用 category_name 格式避免跨类别文件名冲突。
func schemasFileName(category, name string) string {
	return fmt.Sprintf("%s_%s.json", category, name)
}

// SaveDirSchemas 将用户选定的 schemas 列表保存到指定目录的 schema 配置中。
//
// 流程：
//  1. 创建目标 schema 配置目录
//  2. 从嵌入 FS 读取每个 schema 的 JSON 内容
//  3. 将内容写入目标目录（文件名格式：category_name.json）
//  4. 清理旧的 schema 文件（已不在选定列表中的）
func SaveDirSchemas(ragDir, dir string, schemas []SchemaEntry) error {
	targetDir := DirSchemaDir(ragDir, dir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建 schema 配置目录失败: %w", err)
	}

	// 写入选定的 schema 文件
	written := make(map[string]bool)
	for _, s := range schemas {
		data, err := SchemaContent(s.Category, s.Name)
		if err != nil {
			return fmt.Errorf("读取嵌入 schema %s/%s 失败: %w", s.Category, s.Name, err)
		}
		fname := schemasFileName(s.Category, s.Name)
		destPath := filepath.Join(targetDir, fname)
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("写入 schema 文件失败 %s: %w", destPath, err)
		}
		written[fname] = true
	}

	// 清理不再选定的旧 schema 文件
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil // 目录不存在则无需清理
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !written[entry.Name()] {
			os.Remove(filepath.Join(targetDir, entry.Name()))
		}
	}

	return nil
}

// LoadDirSchemas 读取指定目录选定的 schema 文件列表。
// 返回该目录下所有 .json schema 文件的内容映射（文件名 → 原始 JSON）。
// 若目录不存在或为空，返回空映射。
func LoadDirSchemas(ragDir, dir string) (map[string]string, error) {
	targetDir := DirSchemaDir(ragDir, dir)
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, nil // 目录不存在视为无配置
	}

	result := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(targetDir, entry.Name()))
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		result[name] = string(data)
	}
	return result, nil
}

// ── 自定义 Schema 保存与加载 ─────────────────────────────────────────

// SaveCustomSchema 将用户自定义的 schema JSON 保存到指定目录的 schema 配置中。
// 若目标目录不存在，会自动创建。
func SaveCustomSchema(ragDir, dir, category, name string, schemaJSON []byte) error {
	targetDir := DirSchemaDir(ragDir, dir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建 schema 配置目录失败: %w", err)
	}
	fname := schemasFileName(category, name)
	destPath := filepath.Join(targetDir, fname)
	if err := os.WriteFile(destPath, schemaJSON, 0644); err != nil {
		return fmt.Errorf("写入自定义 schema 文件失败 %s: %w", destPath, err)
	}
	return nil
}

// LoadSchemaFromEither 从配置目录优先加载 schema，若不存在则回退到嵌入资源。
// 返回 schema 的原始 JSON 字节和是否从嵌入资源加载。
// isEmbedded 为 true 表示来自嵌入 FS（只读），false 表示来自配置目录（可读写）。
func LoadSchemaFromEither(ragDir, dir, category, name string) (data []byte, isEmbedded bool, err error) {
	// 1. 尝试从配置目录加载
	targetDir := DirSchemaDir(ragDir, dir)
	fname := schemasFileName(category, name)
	configPath := filepath.Join(targetDir, fname)
	if d, eErr := os.ReadFile(configPath); eErr == nil {
		return d, false, nil
	}

	// 2. 回退到嵌入资源
	d, eErr := SchemaContent(category, name)
	if eErr != nil {
		return nil, true, eErr
	}
	return d, true, nil
}

// LoadDirSchemaNames 读取指定目录选定的 schema 条目列表。
// 从 category_name.json 文件名解析出 category 和 name。
func LoadDirSchemaNames(ragDir, dir string) ([]SchemaEntry, error) {
	schemas, err := LoadDirSchemas(ragDir, dir)
	if err != nil {
		return nil, err
	}
	var entries []SchemaEntry
	for fname := range schemas {
		base := strings.TrimSuffix(fname, ".json")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) == 2 {
			entries = append(entries, SchemaEntry{
				Category:    parts[0],
				Name:        parts[1],
				DisplayName: parts[1],
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}
