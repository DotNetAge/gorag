package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	api "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gorag"
	gvcore "github.com/DotNetAge/govector/core"
	blevedb "github.com/blevesearch/bleve"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var infoCmd = &cobra.Command{
	Use:   "info <dataDir>",
	Short: "查看 RAG 库信息",
	Long: `查看 RAG 库的详细信息，包括配置、索引统计和目录大小。

显示内容:
  - RAG 类型和名称
  - 完整配置内容
  - 各索引的条目数量（向量/全文/图）
  - 目录总大小及各子目录大小

示例:
  gorag info ./my-rag`,
	Args: cobra.ExactArgs(1),
	Run:  runInfo,
}

func runInfo(cmd *cobra.Command, args []string) {
	dataDir := args[0]

	ui.Title("RAG 库信息")

	// 检查目录是否存在
	dirInfo, err := os.Stat(dataDir)
	if err != nil {
		ui.Error("无法访问目录: %v", err)
		os.Exit(1)
	}
	if !dirInfo.IsDir() {
		ui.Error("%s 不是一个目录", dataDir)
		os.Exit(1)
	}

	absDir, _ := filepath.Abs(dataDir)

	// 1. 读取配置
	ui.Section("配置")
	cfg, cfgRaw, err := readConfig(dataDir)
	if err != nil {
		ui.Error("读取配置失败: %v", err)
		os.Exit(1)
	}

	ui.KeyValue("名称", cfg.Name)
	ui.KeyValue("类型", cfg.Type)
	ui.KeyValue("模型文件", cfg.EmbeddingModelFile)
	ui.KeyValue("绝对路径", absDir)

	if strings.TrimSpace(cfgRaw) != "" {
		fmt.Printf("\n  %sconfig.yml:%s\n", ui.colors.Highlight, ui.colors.Reset)
		for _, line := range strings.Split(cfgRaw, "\n") {
			if line != "" {
				fmt.Printf("    %s%s%s\n", ui.colors.Dim, line, ui.colors.Reset)
			}
		}
	}

	// 2. 目录大小统计
	ui.Section("存储")
	sizes := calcDirSizes(dataDir)
	totalSize := sizes["total"]

	ui.KeyValue("总大小", formatBytes(totalSize))

	subDirs := []string{"vectors", "fulltexts", "graphs", "caches", "history", "logs"}
	for _, sub := range subDirs {
		if size, ok := sizes[sub]; ok && size > 0 {
			ui.KeyValue("  "+sub+"/", formatBytes(size))
		}
	}

	// 3. 索引统计
	ui.Section("索引统计")
	name := filepath.Base(absDir)

	// 3.1 向量索引
	vectorDBPath := filepath.Join(dataDir, "vectors", name+".db")
	if _, err := os.Stat(vectorDBPath); err == nil {
		count := getVectorCount(vectorDBPath)
		if count >= 0 {
			ui.KeyValue("向量索引 (vectors)", fmt.Sprintf("%d 条", count))
		} else {
			ui.KeyValue("向量索引 (vectors)", "读取失败")
		}
	} else if _, err := os.Stat(filepath.Join(dataDir, "vectors")); err == nil {
		ui.KeyValue("向量索引 (vectors)", "目录存在，数据库文件缺失")
	} else {
		ui.KeyValue("向量索引 (vectors)", "未创建")
	}

	// 3.2 全文索引
	fulltextDBPath := filepath.Join(dataDir, "fulltexts", name+".bleve")
	if _, err := os.Stat(fulltextDBPath); err == nil {
		count := getFulltextCount(fulltextDBPath)
		if count >= 0 {
			ui.KeyValue("全文索引 (fulltexts)", fmt.Sprintf("%d 条", count))
		} else {
			ui.KeyValue("全文索引 (fulltexts)", "读取失败")
		}
	} else if _, err := os.Stat(filepath.Join(dataDir, "fulltexts")); err == nil {
		ui.KeyValue("全文索引 (fulltexts)", "目录存在，索引文件缺失")
	} else {
		ui.KeyValue("全文索引 (fulltexts)", "未创建")
	}

	// 3.3 图索引
	graphDBPath := filepath.Join(dataDir, "graphs", name+".db")
	if _, err := os.Stat(graphDBPath); err == nil {
		nodes, edges := getGraphCount(graphDBPath)
		ui.KeyValue("图索引 (graphs)", fmt.Sprintf("%d 个节点, %d 条边", nodes, edges))
	} else if _, err := os.Stat(filepath.Join(dataDir, "graphs")); err == nil {
		ui.KeyValue("图索引 (graphs)", "目录存在，数据库文件缺失")
	} else {
		ui.KeyValue("图索引 (graphs)", "未创建")
	}

	fmt.Println()
}

// readConfig 读取配置文件，返回解析后的 Config 和原始 YAML 内容
func readConfig(dataDir string) (*gorag.Config, string, error) {
	configPath := filepath.Join(dataDir, "config.yml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取 config.yml 失败: %w", err)
	}

	var cfg gorag.Config
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return nil, string(configData), fmt.Errorf("解析 config.yml 失败: %w", err)
	}

	return &cfg, string(configData), nil
}

// calcDirSizes 递归计算各子目录大小
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

// dirSize 递归计算目录大小
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

// getVectorCount 获取向量索引条目数
// 通过打开 govector 的底层 Storage 读取 CollectionMeta，然后用正确的维度创建 Collection
func getVectorCount(dbPath string) int {
	storage, err := gvcore.NewStorage(dbPath)
	if err != nil {
		return -1
	}
	defer storage.Close()

	// 列出所有 collection
	collections, err := storage.ListCollections()
	if err != nil || len(collections) == 0 {
		return -1
	}

	// 读取第一个 collection 的元数据以获取维度和 HNSW 配置
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

// getFulltextCount 获取全文索引条目数
func getFulltextCount(dbPath string) int64 {
	index, err := blevedb.Open(dbPath)
	if err != nil {
		return -1
	}
	defer index.Close()
	count, err := index.DocCount()
	if err != nil {
		return -1
	}
	return int64(count)
}

// getGraphCount 获取图索引的节点和边数量
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

// queryGraphCount 执行图查询获取计数值
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
		// 尝试读取为 any
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
