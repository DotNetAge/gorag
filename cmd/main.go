package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gorag "github.com/DotNetAge/gorag/v2"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/formatter"
	"github.com/DotNetAge/gorag/v2/utils"
	"github.com/spf13/cobra"
)

var (
	// 查询参数
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
		Use:   "grag",
		Short: "grag - RAG 检索增强生成工具",
		Long: `grag 是一个 RAG (Retrieval-Augmented Generation) 工具，
支持语义检索和图检索的混合索引。

使用方法:
  grag init                   # 在当前目录创建 .rag 库
  grag index [./dir/]         # 索引文件
  grag query "搜索内容"        # 查询
  grag info                   # 查看库信息
  grag doctor                 # 诊断配置
  grag logs                   # 查看日志`,
	}

	// init 子命令
	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "在当前目录创建 .rag 库",
		Long: `初始化一个新的 RAG 库，创建目录结构和配置文件。

在当前目录下创建 ./<basename>.rag，basename 取当前目录名。

支持的索引器类型:
  - semantic: 语义向量索引（默认）
  - graph:    图索引
  - hyper:    混合索引（语义 + 图，需要 LLM）

模型指定方式:
  1. 使用 -i/--model-id 从 HuggingFace 自动下载模型
  2. 使用 -m/--model 指定本地模型文件路径

示例:
  grag init -t hyper -i Xenova/bge-base-zh-v1.5 -f onnx/model.onnx`,
		Args: cobra.NoArgs,
		Run:  runInit,
	}
	initCmd.Flags().StringVarP(&initType, "type", "t", "hyper", "索引器类型: semantic, graph, hyper")
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "RAG 库命名")
	initCmd.Flags().StringVarP(&initModel, "model", "m", "", "本地模型文件路径")
	initCmd.Flags().StringVarP(&initModelID, "model-id", "i", "", "HuggingFace 模型 ID")
	initCmd.Flags().StringVarP(&initModelFile, "model-file", "f", "", "模型文件名")

	// index 子命令
	var indexCmd = &cobra.Command{
		Use:   "index [dest_path]",
		Short: "索引文件或目录",
		Long: `对当前目录或指定目录/文件执行索引。

.rag 文件必须在当前工作目录（自动检测）。

示例:
  grag index                  # 索引当前目录
  grag index ./docs/          # 索引指定子目录
  grag index /abs/path/file   # 索引指定文件`,
		Args: cobra.MaximumNArgs(1),
		Run:  runIndex,
	}

	// query 子命令
	var queryCmd = &cobra.Command{
		Use:   "query <text>",
		Short: "搜索 RAG 库",
		Long: `在已存在的 RAG 库中执行搜索查询。

.rag 文件必须在当前工作目录（自动检测）。

示例:
  grag query "机器学习"
  grag query "机器学习" -o json -k 5`,
		Args: cobra.ExactArgs(1),
		Run:  runQuery,
	}
	queryCmd.Flags().StringVarP(&outputFormat, "output", "o", "terminal", "输出格式: terminal, json, prompt")
	queryCmd.Flags().IntVarP(&topK, "topk", "k", 10, "返回结果数量")
	queryCmd.Flags().BoolVar(&showScore, "score", true, "显示相似度分数")
	queryCmd.Flags().BoolVar(&showDocID, "docid", true, "显示文档ID")
	queryCmd.Flags().IntVar(&contentMax, "max", 500, "内容最大显示长度")

	// doctor 子命令
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "诊断 RAG 库配置",
		Long: `诊断当前 RAG 库的配置完整性，引导补全缺失项。

示例:
  grag doctor`,
		Args: cobra.NoArgs,
		Run:  runDoctor,
	}

	// logs 子命令
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "输出 RAG 库日志",
		Long: `输出当前 RAG 库的日志文件内容。

示例:
  grag logs`,
		Args: cobra.NoArgs,
		Run:  runLogs,
	}

	// update 子命令
	updateCmd := &cobra.Command{
		Use:   "update [dest_path]",
		Short: "更新 RAG 库的实体关系",
		Long: `对已索引的文档执行跨文件实体关系发现与重建。

.rag 文件必须在当前工作目录（自动检测）。

示例:
  grag update
  grag update ./docs/`,
		Args: cobra.MaximumNArgs(1),
		Run:  runUpdate,
	}

	// tree 子命令
	treeCmd := &cobra.Command{
		Use:   "tree",
		Short: "以目录树查看已索引的文件结构",
		Long: `基于 Chunk 的 Source 属性重建完整目录树。

每个文件下展示顶层 Chunk（ParentID=""）及其子块。

示例:
  grag tree`,
		Args: cobra.NoArgs,
		Run:  runTree,
	}

	rootCmd.AddCommand(initCmd, indexCmd, queryCmd, infoCmd, doctorCmd, logsCmd, updateCmd, treeCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// findRAGInCWD 查找当前工作目录下的 .rag 子目录。
func findRAGInCWD() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前工作目录失败: %w", err)
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		return "", fmt.Errorf("读取当前目录失败: %w", err)
	}

	var ragDirs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".rag") {
			ragDirs = append(ragDirs, filepath.Join(cwd, e.Name()))
		}
	}

	if len(ragDirs) == 0 {
		return "", fmt.Errorf("当前目录下未找到 .rag 库，请先运行 grag init")
	}
	if len(ragDirs) > 1 {
		return "", fmt.Errorf("当前目录有多个 .rag 库，请进入具体目录运行")
	}
	return ragDirs[0], nil
}

// ── init ──────────────────────────────────────────────────────────

func runInit(cmd *cobra.Command, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		ui.Error("获取当前目录失败: %v", err)
		os.Exit(1)
	}

	basename := filepath.Base(cwd)
	ragDir := filepath.Join(cwd, basename+".rag")

	ui.Title("grag 初始化")

	// 确定默认模型
	if initType == "hyper" || initType == "semantic" {
		if initModel == "" && initModelID == "" {
			initModelID = "Xenova/bge-base-zh-v1.5"
			initModelFile = "onnx/model.onnx"
		}
	}

	// 创建下载观察者
	var observer utils.DownloadObserver
	if initModelID != "" {
		ui.Section("模型下载")
		ui.KeyValue("模型 ID", initModelID)
		ui.KeyValue("模型文件", initModelFile)
		observer = NewDownloadObserver()
	}

	ui.Section("创建 RAG 库")

	spinner := ui.NewSpinner("正在初始化...")
	spinner.Start()

	result, err := gorag.InitRAG(gorag.InitOptions{
		RagDir:    ragDir,
		IndexType: initType,
		ModelPath: initModel,
		ModelID:   initModelID,
		ModelFile: initModelFile,
		Observer:  observer,
	})
	spinner.Stop()

	if err != nil {
		ui.Error("初始化失败: %v", err)
		os.Exit(1)
	}

	if initModelID != "" {
		ui.Success("模型已就绪")
	}

	ui.Success("RAG 库初始化成功")
	ui.Section("配置信息")
	ui.KeyValue("目录", result.RagDir)
	ui.KeyValue("类型", result.IndexType)
	if result.ModelPath != "" {
		ui.KeyValue("模型", result.ModelPath)
	}
	if result.IndexerName != "" {
		ui.KeyValue("索引器", result.IndexerName)
	}
}

// ── index ─────────────────────────────────────────────────────────

func runIndex(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	indexTarget := "."
	if len(args) > 0 {
		indexTarget = args[0]
	}

	absTarget, err := filepath.Abs(indexTarget)
	if err != nil {
		ui.Error("无法解析路径: %v", err)
		os.Exit(1)
	}

	ui.Title("索引")
	ui.KeyValue("RAG 库", ragDir)
	ui.KeyValue("目标", absTarget)

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("创建索引服务失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	spinner := ui.NewSpinner("正在索引...")
	spinner.Start()

	if err := svc.Index(context.Background(), absTarget); err != nil {
		spinner.Stop()
		ui.Error("索引失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("索引完成")
}

// ── query ─────────────────────────────────────────────────────────

func runQuery(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	searchText = args[0]

	ui.Title("查询")

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	ui.Info("查询: %s", searchText)

	spinner := ui.NewSpinner("正在搜索...")
	spinner.Start()

	hit, err := svc.Query(context.Background(), searchText)
	if err != nil {
		spinner.Stop()
		ui.Error("搜索失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()

	if hit != nil && len(hit.Chunks) > topK {
		hit.Chunks = hit.Chunks[:topK]
	}

	resultCount := 0
	if hit != nil {
		resultCount = len(hit.Chunks)
	}
	ui.Success("找到 %d 个结果", resultCount)

	fmt.Println(formatOutput(hit))
}

// ── doctor ────────────────────────────────────────────────────────

func runDoctor(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	ui.Title("诊断")

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	checks := svc.Doctor()

	allOK := true
	for _, c := range checks {
		if c.OK {
			ui.Success("%s", c.Name)
		} else {
			allOK = false
			if c.Hint != "" {
				ui.Warning("%s — %s", c.Name, c.Hint)
			} else {
				ui.Warning("%s", c.Name)
			}
		}
	}

	if allOK {
		ui.Success("所有检查通过")
	} else {
		ui.Info("提示：embedder 未配置时，语义检索不可用")
	}
}

// ── logs ──────────────────────────────────────────────────────────

func runLogs(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	data, err := svc.Logs()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	fmt.Print(data)
}

// ── update ────────────────────────────────────────────────────────

func runUpdate(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	updateTarget := "."
	if len(args) > 0 {
		updateTarget = args[0]
	}

	absTarget, err := filepath.Abs(updateTarget)
	if err != nil {
		ui.Error("无法解析路径: %v", err)
		os.Exit(1)
	}

	ui.Title("更新实体关系")
	ui.KeyValue("RAG 库", ragDir)
	ui.KeyValue("目标", absTarget)

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	spinner := ui.NewSpinner("正在更新...")
	spinner.Start()

	if err := svc.Update(context.Background(), absTarget); err != nil {
		spinner.Stop()
		ui.Error("更新失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("更新完成")
}

// ── tree ─────────────────────────────────────────────────────────

func runTree(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	ui.Title("目录树")

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	root, err := svc.Tree(context.Background())
	if err != nil {
		ui.Error("构建目录树失败: %v", err)
		os.Exit(1)
	}

	renderTree(root, "")
}

// renderTree 递归渲染目录树。
func renderTree(node *gorag.SourceTreeNode, prefix string) {
	if node == nil {
		return
	}

	// 对子节点排序：目录在前，字母序
	sorted := sortTreeChildren(node.Children)
	nodeChunks := node.Chunks

	for i, child := range sorted {
		isLast := i == len(sorted)-1 && len(nodeChunks) == 0
		branch := "├── "
		connector := "│   "
		if isLast {
			branch = "└── "
			connector = "    "
		}

		if child.IsDir {
			fmt.Printf("%s%s%s/\n", prefix, branch, child.Name)
			renderTree(child, prefix+connector)
		} else {
			renderFileNode(child, prefix, branch, connector, isLast)
		}
	}

	// 渲染当前层级下的文件节点（顶层文件）
	for i, chunk := range nodeChunks {
		isLast := i == len(nodeChunks)-1
		branch := "├── "
		connector := "│   "
		if isLast && len(node.Children) == 0 {
			branch = "└── "
			connector = "    "
		}
		renderChunkNode(chunk, prefix+branch, prefix+connector, true)
	}
}

// renderFileNode 渲染文件节点及 Chunk 子树。
func renderFileNode(node *gorag.SourceTreeNode, prefix, branch, connector string, isLast bool) {
	if len(node.Chunks) == 0 {
		fmt.Printf("%s%s%s  [size:%s]\n", prefix, branch, node.Path, formatBytes(node.Size))
		return
	}

	// 渲染第一个 Chunk 与文件名同一行
	first := node.Chunks[0]
	if isLast {
		branch = "└── "
		connector = "    "
	}

	fmt.Printf("%s%s[%s] %s - %s\n", prefix, branch, first.Type, first.Title, first.Summary)
	fmt.Printf("%s%s%s  [size:%s]\n", prefix+connector, connector, node.Path, formatBytes(node.Size))

	// 渲染第一个 Chunk 的子块
	renderChunkChildren(first, prefix+connector+connector)

	// 渲染其余 Chunk
	for i := 1; i < len(node.Chunks); i++ {
		chunkBranch := "├── "
		chunkConn := "│   "
		if i == len(node.Chunks)-1 {
			chunkBranch = "└── "
			chunkConn = "    "
		}
		renderChunkNode(node.Chunks[i], prefix+connector+chunkBranch, prefix+connector+chunkConn, false)
	}
}

// renderChunkNode 渲染单个 Chunk 节点。
func renderChunkNode(node gorag.SourceChunkNode, branch, connector string, showSource bool) {
	fmt.Printf("%s[%s] %s - %s\n", branch, node.Type, node.Title, node.Summary)
	renderChunkChildren(node, connector)
}

// renderChunkChildren 递归渲染 Chunk 子块。
func renderChunkChildren(node gorag.SourceChunkNode, prefix string) {
	for i, child := range node.Children {
		isLast := i == len(node.Children)-1
		branch := "├── "
		connector := "│   "
		if isLast {
			branch = "└── "
			connector = "    "
		}
		fmt.Printf("%s%s[%s] %s - %s\n", prefix+branch, "", child.Type, child.Title, child.Summary)
		renderChunkChildren(child, prefix+connector)
	}
}

// sortTreeChildren 对子节点排序：目录在前，文件在后，各自按名排序。
func sortTreeChildren(children []*gorag.SourceTreeNode) []*gorag.SourceTreeNode {
	sorted := make([]*gorag.SourceTreeNode, len(children))
	copy(sorted, children)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			less := false
			if sorted[i].IsDir && !sorted[j].IsDir {
				less = true
			} else if !sorted[i].IsDir && sorted[j].IsDir {
				less = false
			} else {
				less = sorted[i].Name < sorted[j].Name
			}
			if !less {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

// ── 工具函数 ──────────────────────────────────────────────────────

func formatOutput(hit *core.Hit) string {
	switch outputFormat {
	case "json":
		return formatter.NewJSONFormatter().FormatAll(hit)
	case "prompt":
		return formatter.NewPromptFormatter(
			formatter.WithContentMaxPrompt(contentMax),
			formatter.WithIncludeScore(showScore),
		).FormatForRAG(hit, searchText)
	default:
		return formatter.NewTerminalFormatter(
			formatter.WithShowScore(showScore),
			formatter.WithShowDocID(showDocID),
			formatter.WithContentMax(contentMax),
		).FormatAll(hit)
	}
}
