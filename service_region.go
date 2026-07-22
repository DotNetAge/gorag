package gorag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/indexer"
)

// RegionService 负责 Region README 的自动生成。
type RegionService struct {
	svc *IndexingService
}

// GenerateMissingReadmes 为本次索引涉及但缺少 README.md 的目录自动生成摘要文件。
//
// 参数：
//   - targetPath: 索引目标路径（文件或目录）
//   - indexedFiles: 本次实际索引到的文件列表
//   - indexFile: 单文件索引回调，用于索引生成的 README.md
func (s *RegionService) GenerateMissingReadmes(ctx context.Context, targetPath string, indexedFiles []string, indexFile func(ctx context.Context, absPath string) ([]*core.Chunk, error)) error {
	indexedDirs := collectIndexedDirs(indexedFiles, targetPath)
	for _, dir := range indexedDirs {
		readmePath := filepath.Join(dir, "README.md")
		if fileExists(readmePath) {
			continue
		}
		if err := s.generateRegionReadme(ctx, dir, indexFile); err != nil {
			s.svc.logger.Warn("生成目录 README 失败", "dir", dir, "error", err)
		}
	}
	return nil
}

// collectIndexedDirs 从本次索引的文件路径中提取所有触及的目录，
// 并向上回收到 targetPath 根目录，去重后按深度从深到浅排序。
func collectIndexedDirs(files []string, targetPath string) []string {
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil
	}
	rootDir := targetPath
	if !info.IsDir() {
		rootDir = filepath.Dir(targetPath)
	}
	rootDir = filepath.Clean(rootDir)

	seen := make(map[string]bool)
	var dirs []string
	for _, f := range files {
		dir := filepath.Dir(f)
		for dir != "" && dir != "/" && dir != "." {
			dir = filepath.Clean(dir)
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
			if dir == rootDir {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 按目录深度从深到浅排序
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if strings.Count(dirs[i], string(filepath.Separator)) < strings.Count(dirs[j], string(filepath.Separator)) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}
	return dirs
}

// generateRegionReadme 为指定目录生成摘要式 README.md 并索引。
// 若目录下已存在 README.md 或没有可摘要内容，则生成默认摘要。
func (s *RegionService) generateRegionReadme(ctx context.Context, dir string, indexFile func(ctx context.Context, absPath string) ([]*core.Chunk, error)) error {
	readmePath := filepath.Join(dir, "README.md")
	if fileExists(readmePath) {
		return nil
	}

	admin, ok := s.svc.indexer.(indexer.IndexerAdmin)
	if !ok {
		return fmt.Errorf("索引器不支持列表查询")
	}

	total, err := admin.Count(ctx)
	if err != nil {
		return fmt.Errorf("获取 Chunk 总数失败: %w", err)
	}
	if total == 0 {
		return nil
	}

	prefix := dir + string(filepath.Separator)
	filters := []core.FilterCondition{
		{
			Key:   core.VecMetaSource,
			Type:  "prefix",
			Value: prefix,
		},
	}

	allChunks, _, err := admin.List(ctx, 0, total, filters)
	if err != nil {
		return fmt.Errorf("获取 Chunk 列表失败: %w", err)
	}

	// 去重并筛选当前目录下的顶层 Chunk
	seen := make(map[string]bool)
	var summaries []string
	for _, c := range allChunks {
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		if c.ParentID != "" {
			continue
		}
		if c.Summary == "" {
			continue
		}
		if !contains(summaries, c.Summary) {
			summaries = append(summaries, c.Summary)
		}
	}

	regionName := filepath.Base(dir)
	var content string
	if len(summaries) > 0 {
		var b strings.Builder
		b.WriteString("# ")
		b.WriteString(regionName)
		b.WriteString("\n\n")
		b.WriteString(core.RegionDescriptorMarker)
		b.WriteString("\n\n")
		for _, summary := range summaries {
			b.WriteString("- ")
			b.WriteString(summary)
			b.WriteString("\n")
		}
		content = b.String()
	} else {
		content = fmt.Sprintf("# %s\n\n%s\n\n_该目录暂无摘要。_\n", regionName, core.RegionDescriptorMarker)
	}

	if err := os.WriteFile(readmePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入 README.md 失败: %w", err)
	}

	if _, err := indexFile(ctx, readmePath); err != nil {
		return fmt.Errorf("索引生成的 README.md 失败: %w", err)
	}

	s.svc.logger.Info("目录 README 生成完成", "dir", dir)
	return nil
}
