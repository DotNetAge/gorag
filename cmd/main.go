package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/formatter"
	"github.com/DotNetAge/gorag/utils"
	"github.com/spf13/cobra"
)

var (
	// 搜索参数
	searchText   string
	outputFormat string
	topK         int
	showScore    bool
	showDocID    bool
	contentMax   int

	// 初始化参数
	initType      string
	initName      string
	initModel     string
	initModelID   string
	initModelFile string
)

var ui = NewUI()

func main() {
	var rootCmd = &cobra.Command{
		Use:   "gorag",
		Short: "GoRAG - 高性能 RAG 检索增强生成工具",
		Long: `GoRAG 是一个高性能的 RAG (Retrieval-Augmented Generation) 工具，
支持向量检索、全文检索和图检索的混合索引。

使用方法:
  gorag init ./my-rag -t hybrid -i Xenova/chinese-clip-vit-base-patch16
  gorag init ./my-rag -t semantic -m ./model.onnx
  gorag add ./my-rag "要索引的文本"
  gorag add ./my-rag -f document.txt
  gorag watch ./my-rag ./documents
  gorag search ./my-rag -s "搜索内容"`,
	}

	// init 子命令
	var initCmd = &cobra.Command{
		Use:   "init <dataDir>",
		Short: "初始化 RAG 库",
		Long: `初始化一个新的 RAG 库，创建目录结构和配置文件。

支持的索引器类型:
  - hybrid:   混合索引（向量 + 全文 + 图）
  - semantic: 语义向量索引
  - graph:    图索引
  - fulltext: 全文索引

模型指定方式:
  1. 使用 -i/--model-id 从 HuggingFace 自动下载模型
  2. 使用 -m/--model 指定本地模型文件路径

环境变量:
  GORAG_MODEL_PATH - 模型存储目录（默认: ~/.embeddings）

示例:
  gorag init ./my-rag -t hybrid -i Xenova/chinese-clip-vit-base-patch16
  gorag init ./my-rag -t semantic -m /path/to/model.onnx
  gorag init ./my-rag -t fulltext`,
		Args: cobra.ExactArgs(1),
		Run:  runInit,
	}
	initCmd.Flags().StringVarP(&initType, "type", "t", "hybrid", "索引器类型: hybrid, semantic, graph, fulltext")
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "RAG 库命名")
	initCmd.Flags().StringVarP(&initModel, "model", "m", "", "本地模型文件路径")
	initCmd.Flags().StringVarP(&initModelID, "model-id", "i", "", "HuggingFace 模型 ID")
	initCmd.Flags().StringVar(&initModelFile, "model-file", "onnx/model.onnx", "模型文件名")

	// search 子命令
	var searchCmd = &cobra.Command{
		Use:   "search <dataDir>",
		Short: "搜索 RAG 库",
		Long: `在已存在的 RAG 库中执行搜索查询。

使用方法:
  gorag search ./my-rag -s "机器学习"
  gorag search ./my-rag -s "机器学习" -o json -k 5`,
		Args: cobra.ExactArgs(1),
		Run:  runSearch,
	}
	searchCmd.Flags().StringVarP(&searchText, "search", "s", "", "搜索文本")
	searchCmd.Flags().StringVarP(&outputFormat, "output", "o", "terminal", "输出格式: terminal, json, prompt")
	searchCmd.Flags().IntVarP(&topK, "topk", "k", 10, "返回结果数量")
	searchCmd.Flags().BoolVar(&showScore, "score", true, "显示相似度分数")
	searchCmd.Flags().BoolVar(&showDocID, "docid", true, "显示文档ID")
	searchCmd.Flags().IntVar(&contentMax, "max", 500, "内容最大显示长度")

	// 默认命令
	var defaultCmd = &cobra.Command{
		Use:   "<dataDir>",
		Short: "搜索 RAG 库（简写形式）",
		Args:  cobra.ExactArgs(1),
		Run:   runSearch,
	}
	defaultCmd.Flags().StringVarP(&searchText, "search", "s", "", "搜索文本")
	defaultCmd.Flags().StringVarP(&outputFormat, "output", "o", "terminal", "输出格式")
	defaultCmd.Flags().IntVarP(&topK, "topk", "k", 10, "返回结果数量")
	defaultCmd.Flags().BoolVar(&showScore, "score", true, "显示相似度分数")
	defaultCmd.Flags().BoolVar(&showDocID, "docid", true, "显示文档ID")
	defaultCmd.Flags().IntVar(&contentMax, "max", 500, "内容最大显示长度")

	rootCmd.AddCommand(initCmd, addCmd, watchCmd, searchCmd, defaultCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runInit 初始化命令
func runInit(cmd *cobra.Command, args []string) {
	dataDir := args[0]

	ui.Title("🚀 GoRAG 初始化")

	var modelPath string
	var err error

	// 处理模型
	if initModelID != "" {
		ui.Section("模型下载")
		ui.KeyValue("模型目录", getModelDir())
		ui.KeyValue("模型 ID", initModelID)
		ui.KeyValue("模型文件", initModelFile)

		// 使用下载观察者（UI 层只负责显示）
		observer := NewDownloadObserver()
		modelPath, err = utils.CheckAndDownload(initModelID, initModelFile, observer)
		if err != nil {
			ui.Error("模型下载失败: %v", err)
			os.Exit(1)
		}
		ui.Success("模型已就绪")
	} else if initModel != "" {
		if _, err := os.Stat(initModel); os.IsNotExist(err) {
			ui.Error("模型文件不存在: %s", initModel)
			os.Exit(1)
		}
		modelPath = initModel
	}

	// 检查索引器类型是否需要模型
	if needsModel(initType) && modelPath == "" {
		ui.Error("%s 索引器需要模型，请使用 -i 或 -m 参数", initType)
		os.Exit(1)
	}

	ui.Section("创建 RAG 库")

	cfg := &gorag.Config{
		Name:      initName,
		Type:      initType,
		ModelFile: modelPath,
	}

	// 创建 RAG 库
	spinner := ui.NewSpinner("正在初始化...")
	spinner.Start()

	idx, err := gorag.New(dataDir, cfg)
	if err != nil {
		spinner.Stop()
		ui.Error("初始化失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()

	ui.Success("RAG 库初始化成功")
	ui.Section("配置信息")
	ui.KeyValue("目录", dataDir)
	ui.KeyValue("类型", initType)
	if modelPath != "" {
		ui.KeyValue("模型", modelPath)
	}
	if initName != "" {
		ui.KeyValue("名称", initName)
	}
	ui.KeyValue("索引器", idx.Name())

	if closer, ok := idx.(interface{ Close() error }); ok {
		closer.Close()
	}
}

// runSearch 搜索命令
func runSearch(cmd *cobra.Command, args []string) {
	dataDir := args[0]

	if searchText == "" {
		ui.Error("请使用 -s 参数指定搜索文本")
		os.Exit(1)
	}

	ui.Title("🔍 搜索")

	spinner := ui.NewSpinner("正在加载 RAG 库...")
	spinner.Start()

	idx, err := gorag.Open(dataDir)
	if err != nil {
		spinner.Stop()
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("RAG 库已加载")
	ui.Info("查询: %s", searchText)

	spinner = ui.NewSpinner("正在搜索...")
	spinner.Start()

	hits, err := idx.Search(context.Background(), idx.NewQuery(searchText))
	if err != nil {
		spinner.Stop()
		ui.Error("搜索失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()

	if len(hits) > topK {
		hits = hits[:topK]
	}

	ui.Success("找到 %d 个结果", len(hits))

	// 格式化输出
	fmt.Println(formatOutput(hits))
}

// needsModel 检查索引器类型是否需要模型
func needsModel(indexerType string) bool {
	return indexerType == "hybrid" || indexerType == "semantic"
}

// getModelDir 获取模型目录
func getModelDir() string {
	dir := os.Getenv("GORAG_MODEL_PATH")
	if dir != "" {
		return dir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./models"
	}
	return filepath.Join(homeDir, ".embeddings")
}

// formatOutput 格式化输出
func formatOutput(hits []core.Hit) string {
	switch outputFormat {
	case "json":
		return formatter.NewJSONFormatter().FormatAll(hits)
	case "prompt":
		return formatter.NewPromptFormatter(
			formatter.WithContentMaxPrompt(contentMax),
			formatter.WithIncludeScore(showScore),
		).FormatForRAG(hits, searchText)
	default:
		return formatter.NewTerminalFormatter(
			formatter.WithShowScore(showScore),
			formatter.WithShowDocID(showDocID),
			formatter.WithContentMax(contentMax),
		).FormatAll(hits)
	}
}
