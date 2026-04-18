package gorag

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestBenchmark_ChunkSize 快速模拟不同配置的 chunk 数量（不实际调用 chunker，避免 GenerateID 开销）
func TestBenchmark_ChunkSize(t *testing.T) {
	files := listTestFiles(t)
	if len(files) == 0 {
		t.Skip("无测试数据文件")
	}

	// 预解析所有文件，提取段落
	type fileParas struct {
		name  string
		size  int
		paras []string // 段落文本
	}

	var allFiles []fileParas
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := string(data)
		text = strings.ReplaceAll(text, "\r\n", "\n")
		parts := strings.Split(text, "\n\n")

		var paras []string
		for _, part := range parts {
			lines := strings.Split(part, "\n")
			var cleaned []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					cleaned = append(cleaned, line)
				}
			}
			if len(cleaned) > 0 {
				paras = append(paras, strings.Join(cleaned, "\n"))
			}
		}
		allFiles = append(allFiles, fileParas{
			name:  filepath.Base(f),
			size:  len(data),
			paras: paras,
		})
	}
	t.Logf("解析 %d 个文件", len(allFiles))

	// 模拟 ParagraphChunker 的段落合并逻辑
	simulate := func(paras []string, chunkSize, maxParagraphs, minChunkSize, overlap int) (chunkCount int, totalChars int) {
		i := 0
		for i < len(paras) {
			var selectedLen int
			var selectedCount int

			for j := i; j < len(paras); j++ {
				addLen := len(paras[j])
				if selectedCount > 0 {
					addLen += 2
				}
				if selectedLen+addLen > chunkSize && selectedCount >= 1 && selectedLen >= minChunkSize {
					break
				}
				selectedLen += addLen
				selectedCount++
				if selectedCount >= maxParagraphs {
					break
				}
			}

			if selectedCount == 0 {
				break
			}

			chunkCount++
			totalChars += selectedLen

			// overlap 回溯
			nextStart := i + selectedCount
			if overlap > 0 {
				overlapUsed := 0
				for k := selectedCount - 1; k >= 1; k-- { // k >= 1 确保至少前进一个段落
					pLen := len(paras[i+k])
					if overlapUsed+pLen > overlap {
						break
					}
					overlapUsed += pLen
					overlapUsed += 2 // 段落间分隔符
					nextStart = i + k
				}
			}
			i = nextStart
		}
		return
	}

	// 候选配置
	type config struct {
		label         string
		chunkSize     int
		maxParagraphs int
		minChunkSize  int
		overlap       int
	}
	configs := []config{
		{"当前(800/3p)", 800, 3, 50, 0},
		{"A:800/10p", 800, 10, 50, 0},
		{"B:1000/10p", 1000, 10, 80, 0},
		{"C:1500/15p", 1500, 15, 150, 0},
		{"D:2000/20p", 2000, 20, 200, 0},
		{"E:1000/8p", 1000, 8, 80, 0},
		{"F:1500/10p", 1500, 10, 150, 0},
		{"G:1200/8p", 1200, 8, 100, 0},
		{"C+ov:1500/15p/200ov", 1500, 15, 150, 200},
		{"D+ov:2000/20p/300ov", 2000, 20, 200, 300},
		{"B+ov:1000/10p/150ov", 1000, 10, 80, 150},
	}

	type configResult struct {
		label       string
		totalChunks int
		totalChars  int
		avgChunkLen int
		perFile     []struct {
			name  string
			size  int
			count int
		}
	}

	results := make([]configResult, len(configs))

	for ci, cfg := range configs {
		r := &results[ci]
		r.label = cfg.label

		for _, fp := range allFiles {
			count, chars := simulate(fp.paras, cfg.chunkSize, cfg.maxParagraphs, cfg.minChunkSize, cfg.overlap)
			r.totalChunks += count
			r.totalChars += chars
			r.perFile = append(r.perFile, struct {
				name  string
				size  int
				count int
			}{fp.name, fp.size, count})
		}
		if r.totalChunks > 0 {
			r.avgChunkLen = r.totalChars / r.totalChunks
		}
	}

	// ── 汇总表 ──
	fmt.Println("\n" + strings.Repeat("=", 100))
	fmt.Println("Chunk Size 配置对比（47 个文件，Paragraph 策略模拟）")
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("%-25s %10s %10s %12s\n", "配置", "总chunks", "总字符", "平均长度")
	fmt.Println(strings.Repeat("-", 100))
	for _, r := range results {
		fmt.Printf("%-25s %10d %10d %10d ch\n",
			r.label, r.totalChunks, r.totalChars, r.avgChunkLen)
	}

	// ── Top 10 大文件 ──
	type fileWithSize struct {
		name string
		size int
	}
	var largeFiles []fileWithSize
	for _, fp := range allFiles {
		largeFiles = append(largeFiles, fileWithSize{fp.name, fp.size})
	}
	sort.Slice(largeFiles, func(i, j int) bool {
		return largeFiles[i].size > largeFiles[j].size
	})
	if len(largeFiles) > 10 {
		largeFiles = largeFiles[:10]
	}

	fmt.Println("\n" + strings.Repeat("=", 100))
	fmt.Println("Top 10 大文件 chunk 数量")
	fmt.Println(strings.Repeat("=", 100))

	header := fmt.Sprintf("%-35s %7s", "文件", "大小")
	for _, r := range results {
		short := r.label
		if len(short) > 12 {
			short = short[:12]
		}
		header += fmt.Sprintf(" %12s", short)
	}
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", 100))

	for _, lf := range largeFiles {
		row := fmt.Sprintf("%-35s %5dch", lf.name, lf.size)
		for _, r := range results {
			for _, pf := range r.perFile {
				if pf.name == lf.name {
					row += fmt.Sprintf(" %12d", pf.count)
					break
				}
			}
		}
		fmt.Println(row)
	}

	// ── 推荐 ──
	fmt.Println("\n" + strings.Repeat("=", 100))
	fmt.Println("推荐分析")
	fmt.Println(strings.Repeat("=", 100))

	bestIdx := 0
	for i, r := range results {
		if r.avgChunkLen >= 400 && r.totalChunks < results[bestIdx].totalChunks {
			if results[bestIdx].avgChunkLen < 400 || r.totalChunks < results[bestIdx].totalChunks {
				bestIdx = i
			}
		}
	}

	fmt.Printf("当前默认: %s -> %d chunks, 平均 %d 字符\n", results[0].label, results[0].totalChunks, results[0].avgChunkLen)
	reduction := float64(results[0].totalChunks-results[bestIdx].totalChunks) / float64(results[0].totalChunks) * 100
	fmt.Printf("推荐配置: %s -> %d chunks (减少 %.1f%%), 平均 %d 字符\n",
		results[bestIdx].label, results[bestIdx].totalChunks, reduction, results[bestIdx].avgChunkLen)
	oldTime := results[0].totalChunks * 137
	newTime := results[bestIdx].totalChunks * 137
	fmt.Printf("预估向量化耗时: %.1fs -> %.1fs (节省 %.1fs)\n",
		float64(oldTime)/1000, float64(newTime)/1000, float64(oldTime-newTime)/1000)

	// 列出所有 avgChunkLen >= 500 的配置
	fmt.Println("\n所有平均长度 >= 500 字符的配置:")
	for _, r := range results {
		if r.avgChunkLen >= 500 {
			red := float64(results[0].totalChunks-r.totalChunks) / float64(results[0].totalChunks) * 100
			et := float64(r.totalChunks*137) / 1000
			fmt.Printf("  %s: %d chunks (-%.0f%%), 平均 %d ch, 预估向量化 %.1fs\n",
				r.label, r.totalChunks, red, r.avgChunkLen, et)
		}
	}
}

func listTestFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(testDataDir)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", testDataDir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Join(testDataDir, e.Name()))
	}
	return files
}
