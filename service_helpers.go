package gorag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gorag/v2/core"
	gvcore "github.com/DotNetAge/govector/core"
)

// TextExts 可索引的文本文件扩展名列表
var TextExts = []string{
	".txt", ".md", ".json", ".yaml", ".yml",
	".html", ".xml", ".css",
	".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h",
	".sh", ".bash", ".zsh",
	".sql", ".conf", ".cfg", ".ini",
}

// DataExts 可索引的数据类文件扩展名列表（解析为 JSON 字符串）。
//
// 涵盖：csv/xls/xlsx/json/yaml/yml/xml/toml/log/eml/msg。
// 这些文件不进入 TextExts，因为它们的归一化策略不同（输出 JSON 字符串而非原文）。
var DataExts = []string{
	".csv", ".xls", ".xlsx",
	".json", ".yaml", ".yml",
	".xml", ".toml", ".log",
	".eml", ".msg",
}

// ScanDir 扫描目录下的所有文本文件，跳过 .ragignore 匹配的目录。
//
// 支持层级 .ragignore：每个目录可放置自己的 .ragignore，规则叠加生效。
// basePatterns 来自 .rag 库目录的全局规则，作为根目录的默认规则。每个子目录的
// 本地 .ragignore 规则会附加到父目录规则之上，子目录规则不影响平级目录。
func ScanDir(dir string, basePatterns []string) ([]string, error) {
	// patternCache 缓存每个目录的合并后规则
	patternCache := map[string][]string{}
	patternCache[dir] = basePatterns

	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			// 获取父目录的规则检查文件是否应被跳过
			parent := filepath.Dir(path)
			if patterns, ok := patternCache[parent]; ok {
				rel, relErr := filepath.Rel(dir, path)
				if relErr == nil {
					if MatchRagignoreEntry(d.Name(), rel, patterns) {
						return nil
					}
				}
			}
			if IsIndexableFile(path) {
				files = append(files, path)
			}
			return nil
		}

		// ── 以下为目录处理 ──

		// 跳过 .rag 子目录（避免索引库索引自己）
		if strings.HasSuffix(path, ".rag") {
			return filepath.SkipDir
		}

		// 确定当前目录应使用的规则集
		var curPatterns []string
		if path == dir {
			curPatterns = basePatterns
		} else {
			parent := filepath.Dir(path)
			if p, ok := patternCache[parent]; ok {
				curPatterns = p
			} else {
				curPatterns = basePatterns
			}
			// 用父目录规则判断当前目录是否应被跳过
			if MatchRagignoreDir(path, dir, curPatterns) {
				return filepath.SkipDir
			}
		}

		// 检查当前目录是否有本地 .ragignore，若有则合并后缓存供子目录使用
		localPatterns := LoadRagignore(path)
		if len(localPatterns) > 0 {
			merged := make([]string, len(curPatterns)+len(localPatterns))
			copy(merged, curPatterns)
			copy(merged[len(curPatterns):], localPatterns)
			patternCache[path] = merged
		} else {
			patternCache[path] = curPatterns
		}
		return nil
	})
	return files, err
}

// LoadRagignore 从 .rag 目录加载 .ragignore 忽略规则。
// 返回非空、非注释的规则行列表。文件不存在时返回空切片。
func LoadRagignore(ragDir string) []string {
	data, err := os.ReadFile(filepath.Join(ragDir, ".ragignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// matchRagignoreEntry 判断名称/相对路径是否匹配任一 .ragignore 规则。
// 支持 **.xxx、*.xxx、路径前缀/包含、以及 filepath.Match 通配符。
func matchRagignoreEntry(name, rel string, patterns []string) bool {
	for _, pattern := range patterns {
		// 通配符规则：**.pyc → 检查是否以 .pyc 结尾
		if strings.HasPrefix(pattern, "**.") {
			suffix := strings.TrimPrefix(pattern, "**")
			if strings.HasSuffix(name, suffix) {
				return true
			}
			if strings.HasSuffix(rel, suffix) {
				return true
			}
			continue
		}
		// *.swp, *.swo 等通配符
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(name, suffix) {
				return true
			}
			continue
		}
		// 路径前缀/包含匹配
		cleanPattern := strings.TrimSuffix(pattern, "/")
		if strings.HasPrefix(rel, cleanPattern) || strings.Contains("/"+rel+"/", "/"+cleanPattern+"/") {
			return true
		}
		// filepath.Match 通配（支持 .gitignore 风格精确匹配）
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		// 目录模式（尾随/）尝试匹配 name+"/"
		if strings.HasSuffix(pattern, "/") {
			if matched, _ := filepath.Match(pattern, name+"/"); matched {
				return true
			}
		}
	}
	return false
}

// MatchRagignoreDir 判断目录是否匹配任一 .ragignore 规则。
// 规则支持目录匹配（尾随 /）和文件名匹配。
func MatchRagignoreDir(dirPath, scanRoot string, patterns []string) bool {
	rel, err := filepath.Rel(scanRoot, dirPath)
	if err != nil {
		return false
	}
	return MatchRagignoreEntry(filepath.Base(dirPath), rel, patterns)
}

// MatchRagignoreEntry 是 matchRagignoreEntry 的导出版本，供 webapi 等外部包使用。
func MatchRagignoreEntry(name, rel string, patterns []string) bool {
	return matchRagignoreEntry(name, rel, patterns)
}

// IsTextFile 判断是否为可索引的文本文件
func IsTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return slices.Contains(TextExts, ext)
}

// IsDataFile 判断是否为可索引的数据类文件（CSV / XLSX / JSON / 等）。
//
// 数据类文件的归一化策略是输出 JSON 字符串（不同于文本类输出原文）。
// 与 IsTextFile 是并列关系。
func IsDataFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return slices.Contains(DataExts, ext)
}

// IsIndexableFile 判断文件是否应被索引（文本类 + 数据类）。
// 这是 ScanDir 与 index 单文件入口的统一判断入口。
func IsIndexableFile(filename string) bool {
	return IsTextFile(filename) || IsDataFile(filename)
}

// ComputeFileHash 计算文件的 SHA256 哈希值。
func ComputeFileHash(absPath string) (string, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("计算哈希失败: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// TimePtr 返回 time.Time 的指针。
func TimePtr(t time.Time) *time.Time {
	return &t
}

// ComputeChunkContentHash 计算分片内容的简短哈希（SHA256 前 16 位十六进制）。
func ComputeChunkContentHash(content string) string {
	if content == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// Abs 返回 int 绝对值。
func Abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Contains 检查字符串切片是否包含指定值。
func Contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// DirExists 判断路径是否为已存在目录。
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// FileExists 判断文件是否存在（目录返回 false）。
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// CalcDirSizes 递归计算各子目录大小。
func CalcDirSizes(dataDir string) map[string]int64 {
	sizes := make(map[string]int64)
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return sizes
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			fi, err := entry.Info()
			if err == nil {
				sizes["total"] += fi.Size()
			}
			continue
		}
		subSize := DirSize(filepath.Join(dataDir, entry.Name()))
		sizes[entry.Name()] = subSize
		sizes["total"] += subSize
	}
	return sizes
}

// DirSize 递归计算目录总大小。
func DirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err == nil {
			size += fi.Size()
		}
		return nil
	})
	return size
}

// GetVectorCount 获取向量索引中的条目数。
func GetVectorCount(dbPath string) int {
	storage, err := gvcore.NewStorage(dbPath)
	if err != nil {
		return -1
	}
	defer storage.Close()

	collections, err := storage.ListCollections()
	if err != nil || len(collections) == 0 {
		return -1
	}

	meta, err := storage.LoadCollectionMeta(collections[0])
	if err != nil {
		return -1
	}

	col, err := gvcore.NewCollection(meta.Name, meta.VectorLen, meta.Metric, storage, meta.UseHNSW)
	if err != nil {
		return -1
	}
	return col.Count()
}

// GetGraphCount 获取图索引的节点和边数量。
func GetGraphCount(dbPath string) (nodes int64, edges int64) {
	db, err := api.Open(dbPath)
	if err != nil {
		return -1, -1
	}
	defer db.Close()

	ctx := context.Background()
	nodes = QueryGraphCount(ctx, db, "MATCH (n) RETURN count(n) AS cnt")
	edges = QueryGraphCount(ctx, db, "MATCH ()-[r]->() RETURN count(r) AS cnt")
	return nodes, edges
}

// QueryGraphCount 执行图查询获取计数值。
func QueryGraphCount(ctx context.Context, db *api.DB, query string) int64 {
	rows, err := db.Query(ctx, query)
	if err != nil {
		return -1
	}
	defer rows.Close()

	if !rows.Next() {
		return 0
	}

	var count int64
	if err := rows.Scan(&count); err != nil {
		var val any
		_ = rows.Scan(&val)
		if val != nil {
			switch v := val.(type) {
			case int64:
				return v
			case int:
				return int64(v)
			case float64:
				return int64(v)
			}
		}
		return -1
	}
	return count
}

// SourceHasPrefix 判断 chunk.Source 是否以指定绝对路径开头。
// filterPath 必须是已经转换后的绝对路径。
// 为了避免 /foo 匹配到 /foobar 这类部分路径，函数会自动确保 filterPath 以路径分隔符结尾。
func SourceHasPrefix(source, filterPath string) bool {
	if source == "" || filterPath == "" {
		return false
	}
	// 确保 filterPath 以路径分隔符结尾，避免部分匹配
	if !strings.HasSuffix(filterPath, string(filepath.Separator)) {
		filterPath += string(filepath.Separator)
	}
	return strings.HasPrefix(source, filterPath)
}

// TopChunkScore 返回 ChunkHit 切片中的最高分。
func TopChunkScore(hits []core.ChunkHit) float32 {
	if len(hits) == 0 {
		return 0
	}
	max := hits[0].Score
	for _, h := range hits[1:] {
		if h.Score > max {
			max = h.Score
		}
	}
	return max
}

// FilterHitBySourcePrefix 按 source 路径前缀过滤命中结果。
func FilterHitBySourcePrefix(hit *core.Hit, filterPath string) *core.Hit {
	if hit == nil {
		return nil
	}
	absFilter, err := filepath.Abs(filterPath)
	if err != nil {
		absFilter = filterPath
	}
	var filtered []core.ChunkHit
	for _, ch := range hit.Chunks {
		if SourceHasPrefix(ch.Chunk.Source, absFilter) {
			filtered = append(filtered, ch)
		}
	}
	hit.Chunks = filtered
	return hit
}
