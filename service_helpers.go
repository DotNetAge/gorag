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

// textExts 可索引的文本文件扩展名列表
var textExts = []string{
	".txt", ".md", ".json", ".yaml", ".yml",
	".html", ".xml", ".css",
	".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h",
	".sh", ".bash", ".zsh",
	".sql", ".conf", ".cfg", ".ini",
}

// scanDir 扫描目录下的所有文本文件，跳过 .ragignore 匹配的目录。
func scanDir(dir string, ragignore []string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// 跳过 .rag 子目录（避免索引库索引自己）
			if strings.HasSuffix(path, ".rag") {
				return filepath.SkipDir
			}
			// 跳过 .ragignore 匹配的目录
			if matchRagignoreDir(path, dir, ragignore) {
				return filepath.SkipDir
			}
			return nil
		}
		if isTextFile(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// loadRagignore 从 .rag 目录加载 .ragignore 忽略规则。
// 返回非空、非注释的规则行列表。文件不存在时返回空切片。
func loadRagignore(ragDir string) []string {
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

// matchRagignoreDir 判断目录是否匹配任一 .ragignore 规则。
// 规则支持目录匹配（尾随 /）和文件名匹配。
func matchRagignoreDir(dirPath, scanRoot string, patterns []string) bool {
	rel, err := filepath.Rel(scanRoot, dirPath)
	if err != nil {
		return false
	}
	dirName := filepath.Base(dirPath)
	for _, pattern := range patterns {
		// 通配符规则：**.pyc → 检查是否以 .pyc 结尾
		if strings.HasPrefix(pattern, "**.") {
			suffix := strings.TrimPrefix(pattern, "**")
			if strings.HasSuffix(dirName, suffix) {
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
			if strings.HasSuffix(dirName, suffix) {
				return true
			}
			continue
		}
		cleanPattern := strings.TrimSuffix(pattern, "/")
		// 路径中的任意一级匹配
		if strings.HasPrefix(rel, cleanPattern) || strings.Contains("/"+rel+"/", "/"+cleanPattern+"/") {
			return true
		}
	}
	return false
}

// isTextFile 判断是否为可索引的文本文件
func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return slices.Contains(textExts, ext)
}

// computeFileHash 计算文件的 SHA256 哈希值。
func computeFileHash(absPath string) (string, error) {
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

// timePtr 返回 time.Time 的指针。
func timePtr(t time.Time) *time.Time {
	return &t
}

// computeChunkContentHash 计算分片内容的简短哈希（SHA256 前 16 位十六进制）。
func computeChunkContentHash(content string) string {
	if content == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

// abs 返回 int 绝对值。
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// contains 检查字符串切片是否包含指定值。
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// dirExists 判断路径是否为已存在目录。
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists 判断文件是否存在（目录返回 false）。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// calcDirSizes 递归计算各子目录大小。
func calcDirSizes(dataDir string) map[string]int64 {
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
		subSize := dirSize(filepath.Join(dataDir, entry.Name()))
		sizes[entry.Name()] = subSize
		sizes["total"] += subSize
	}
	return sizes
}

// dirSize 递归计算目录总大小。
func dirSize(path string) int64 {
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

// getVectorCount 获取向量索引中的条目数。
func getVectorCount(dbPath string) int {
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

// getGraphCount 获取图索引的节点和边数量。
func getGraphCount(dbPath string) (nodes int64, edges int64) {
	db, err := api.Open(dbPath)
	if err != nil {
		return -1, -1
	}
	defer db.Close()

	ctx := context.Background()
	nodes = queryGraphCount(ctx, db, "MATCH (n) RETURN count(n) AS cnt")
	edges = queryGraphCount(ctx, db, "MATCH ()-[r]->() RETURN count(r) AS cnt")
	return nodes, edges
}

// queryGraphCount 执行图查询获取计数值。
func queryGraphCount(ctx context.Context, db *api.DB, query string) int64 {
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

// sourceHasPrefix 判断 chunk.Source 是否以指定绝对路径开头。
// filterPath 必须是已经转换后的绝对路径。
// 为了避免 /foo 匹配到 /foobar 这类部分路径，函数会自动确保 filterPath 以路径分隔符结尾。
func sourceHasPrefix(source, filterPath string) bool {
	if source == "" || filterPath == "" {
		return false
	}
	// 确保 filterPath 以路径分隔符结尾，避免部分匹配
	if !strings.HasSuffix(filterPath, string(filepath.Separator)) {
		filterPath += string(filepath.Separator)
	}
	return strings.HasPrefix(source, filterPath)
}

// topChunkScore 返回 ChunkHit 切片中的最高分。
func topChunkScore(hits []core.ChunkHit) float32 {
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

// filterHitBySourcePrefix 按 source 路径前缀过滤命中结果。
func filterHitBySourcePrefix(hit *core.Hit, filterPath string) *core.Hit {
	if hit == nil {
		return nil
	}
	absFilter, err := filepath.Abs(filterPath)
	if err != nil {
		absFilter = filterPath
	}
	var filtered []core.ChunkHit
	for _, ch := range hit.Chunks {
		if sourceHasPrefix(ch.Chunk.Source, absFilter) {
			filtered = append(filtered, ch)
		}
	}
	hit.Chunks = filtered
	return hit
}
