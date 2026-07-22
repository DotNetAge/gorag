package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gorag "github.com/DotNetAge/gorag/v2"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/formatter"
	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/llm"
	"github.com/DotNetAge/gorag/v2/logging"
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
	filterPath   string // source 路径前缀过滤

	// chunks 列表参数
	chunksPage   int
	chunksSize   int
	chunksFilter string
	chunksJSON   bool

	// nodes 图导航参数
	nodesHops int
	nodesJSON bool

	// cypher 查询参数
	cypherJSON bool

	// 初始化参数
	initType      string
	initName      string
	initModel     string
	initModelID   string
	initModelFile string

	// update 命令参数（LLM + Schema）
	updateLLMKey   string
	updateLLMURL   string
	updateLLMModel string
	updateSchema   string
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

在当前目录下创建 .rag 作为 RAG 库目录。

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
		Short: "索引文件或目录（快路径，无 LLM）",
		Long: `对当前目录或指定目录/文件执行基础索引（扫描、分块、向量化）。

.rag 文件必须在当前工作目录（自动检测）。

此命令不调用 LLM，仅做基础索引。LLM 增强（摘要、实体提取）请使用 update 命令。

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
	queryCmd.Flags().StringVarP(&filterPath, "filter", "f", "", "source 路径前缀过滤（只返回 Source 以该路径开头的分片）")

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
		Short: "增量更新：重新索引 + LLM 增强（摘要 + 实体提取）",
		Long: `双阶段增量更新，自动处理文件变更并可选执行 LLM 增强。

第一阶段：检测文件系统变更，对变更文件重新分块 + 向量化（同 index 快路径）
第二阶段：对需要 LLM 处理的分片执行摘要生成和实体关系提取

.rag 文件必须在当前工作目录（自动检测）。

LLM 参数可选。不传时只执行第一阶段（等效于 index + 增量检测）。
传入 LLM 参数时执行完整的双阶段流程。

LLM 参数:
  --llm-key        API Key（支持 GORAG_API_KEY 环境变量）
  --llm-url        LLM Base URL
  --llm-model      LLM 模型名
  --schema         实体 Schema 目录（目录下所有 .json 文件加载为 EntitySchema）

示例:
  grag update                                              # 只做重新索引
  grag update --llm-key sk-xxx --llm-url https://... --llm-model gpt-4o    # 完整双阶段
  grag update --schema ./schemas/                          # 重新索引 + Schema 准备（下次有 LLM 时可用）`,
		Args: cobra.MaximumNArgs(1),
		Run:  runUpdate,
	}
	updateCmd.Flags().StringVarP(&updateLLMKey, "llm-key", "", "", "API Key（支持 GORAG_API_KEY 环境变量）")
	updateCmd.Flags().StringVarP(&updateLLMURL, "llm-url", "", "", "LLM Base URL")
	updateCmd.Flags().StringVarP(&updateLLMModel, "llm-model", "", "", "LLM 模型名")
	updateCmd.Flags().StringVarP(&updateSchema, "schema", "", "", "实体 Schema 目录（目录下所有 .json 文件）")

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

	// chunks 子命令
	chunksCmd := &cobra.Command{
		Use:   "chunks",
		Short: "分页列出已索引的 Chunk",
		Long: `分页列出本地知识库中的 Chunk，显示 id、title、summary、tags。

支持按 source 路径前缀过滤，支持 JSON 输出。

示例:
  grag chunks                    # 第 1 页，每页 20 条
  grag chunks -p 2 -s 10         # 第 2 页，每页 10 条
  grag chunks -f ./docs/         # 只列出 ./docs/ 下的 Chunk
  grag chunks --json             # JSON 输出`,
		Args: cobra.NoArgs,
		Run:  runChunks,
	}
	chunksCmd.Flags().IntVarP(&chunksPage, "page", "p", 1, "页码（从 1 开始）")
	chunksCmd.Flags().IntVarP(&chunksSize, "size", "s", 20, "每页数量")
	chunksCmd.Flags().StringVarP(&chunksFilter, "filter", "f", "", "source 路径前缀过滤")
	chunksCmd.Flags().BoolVar(&chunksJSON, "json", false, "以 JSON 格式输出")

	// nodes 子命令
	nodesCmd := &cobra.Command{
		Use:   "nodes [dir]",
		Short: "目录级多跳图节点查询",
		Long: `以指定目录的 Region 节点为起点，查询其 N 跳相邻的图节点与关系。

不指定 dir 时使用当前工作目录。跳数范围 1-3，默认 1。

示例:
  grag nodes                   # 当前目录，1 跳
  grag nodes ./docs/           # ./docs/ 目录，1 跳
  grag nodes ./docs/ -n 2      # ./docs/ 目录，2 跳
  grag nodes --json            # JSON 输出`,
		Args: cobra.MaximumNArgs(1),
		Run:  runNodes,
	}
	nodesCmd.Flags().IntVarP(&nodesHops, "hops", "n", 1, "跳数（1-3）")
	nodesCmd.Flags().BoolVar(&nodesJSON, "json", false, "以 JSON 格式输出")

	// cypher 子命令
	cypherCmd := &cobra.Command{
		Use:   "cypher <query>",
		Short: "执行原始 Cypher 图查询",
		Long: `直接对底层图存储执行 Cypher 查询并返回结果。

仅支持图索引器（graph / hyper）。查询语句请用引号包裹。

示例:
  grag cypher "MATCH (n) RETURN count(n) AS cnt"
  grag cypher "MATCH (n:Person) RETURN n.name LIMIT 10"
  grag cypher "MATCH (n)-[r]->(m) RETURN n.name, r.type, m.name LIMIT 20" --json`,
		Args: cobra.ExactArgs(1),
		Run:  runCypher,
	}
	cypherCmd.Flags().BoolVar(&cypherJSON, "json", false, "以 JSON 格式输出")

	rootCmd.AddCommand(initCmd, indexCmd, queryCmd, infoCmd, doctorCmd, logsCmd, updateCmd, treeCmd, chunksCmd, nodesCmd, cypherCmd)

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

	ragDir := filepath.Join(cwd, ".rag")

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

	// 创建控制台日志器（实时输出到屏幕）
	consoleLogger := logging.DefaultConsoleLogger()

	// 创建索引服务
	svc, err := gorag.NewRAGService(ragDir, gorag.WithLogger(consoleLogger))
	if err != nil {
		ui.Error("创建索引服务失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	// 将控制台日志器注入到 HyperIndexer
	if hyper, ok := svc.Indexer().(*indexer.HyperIndexer); ok {
		hyper.SetLogger(consoleLogger)
	}

	// 执行索引（快路径：无 LLM）
	consoleLogger.Info("开始批量索引", "target", absTarget)
	if err := svc.Index(context.Background(), absTarget); err != nil {
		ui.Error("索引失败: %v", err)
		os.Exit(1)
	}
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

	// 解析多关键字组，以 | 分隔
	queries := splitQueryGroups(searchText)

	ui.Title("查询")

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	if len(queries) > 1 {
		ui.Info("查询组: %v", queries)
	} else {
		ui.Info("查询: %s", searchText)
	}

	spinner := ui.NewSpinner("正在搜索...")
	spinner.Start()

	var hit *core.Hit
	if len(queries) == 1 {
		hit, err = svc.Query(context.Background(), queries[0], filterPath)
	} else {
		hit, err = svc.QueryMulti(context.Background(), queries, filterPath)
	}
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

	ui.Title("增量 LLM 处理")
	ui.KeyValue("RAG 库", ragDir)
	ui.KeyValue("目标", absTarget)

	// 1. 加载配置
	cfg, _ := gorag.LoadConfig(ragDir)

	// 2. 持久化 LLM 配置到 .rag 库
	hasLLMFlag := false
	configUpdated := false

	if updateLLMKey != "" {
		hasLLMFlag = true
		if err := gorag.WriteAPIKey(ragDir, updateLLMKey); err != nil {
			ui.Warning("写入 API Key 失败: %v", err)
		}
	}

	if updateLLMURL != "" {
		hasLLMFlag = true
		cfg.LLM.BaseURL = updateLLMURL
		configUpdated = true
	}

	if updateLLMModel != "" {
		hasLLMFlag = true
		cfg.LLM.Model = updateLLMModel
		configUpdated = true
	}

	if hasLLMFlag {
		if cfg.LLM.Language == "" {
			cfg.LLM.Language = "Chinese"
			configUpdated = true
		}
		if configUpdated {
			if err := gorag.SaveConfig(ragDir, cfg); err != nil {
				ui.Warning("保存 LLM 配置失败: %v", err)
			}
		}
	}

	// 3. 创建日志器和索引服务
	consoleLogger := logging.DefaultConsoleLogger()

	svc, err := gorag.NewRAGService(ragDir, gorag.WithLogger(consoleLogger))
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	// 4. 确定 API Key
	var apiKey string
	if updateLLMKey != "" {
		apiKey = updateLLMKey
	} else {
		if k, err := gorag.ResolveAPIKey(ragDir); err == nil {
			apiKey = k
		}
	}

	// 5. 确定 LLM 配置
	llmBaseURL := updateLLMURL
	llmModel := updateLLMModel
	if llmBaseURL == "" {
		llmBaseURL = cfg.LLM.BaseURL
	}
	if llmModel == "" {
		llmModel = cfg.LLM.Model
	}

	hasLLM := apiKey != "" && llmBaseURL != "" && llmModel != ""

	if hasLLM {
		llmCfg := llm.Config{
			APIKey:  apiKey,
			BaseURL: llmBaseURL,
			Model:   llmModel,
		}

		// 5a. 注入 Summarizer
		summarizer, sErr := llm.NewSummarizer(llmCfg, consoleLogger)
		if sErr != nil {
			ui.Warning("创建 Summarizer 失败: %v", sErr)
		} else if hyper, ok := svc.Indexer().(*indexer.HyperIndexer); ok {
			hyper.SetSummarizer(summarizer)
			hyper.SetLogger(consoleLogger)
			consoleLogger.Info("Summarizer 已注入", "model", llmModel)
		}

		// 5b. 注入 Refiller + Schema
		refiller, rErr := llm.NewRefiller(llmCfg, consoleLogger)
		if rErr != nil {
			ui.Warning("创建 Refiller 失败: %v", rErr)
		} else if hyper, ok := svc.Indexer().(*indexer.HyperIndexer); ok {
			hyper.SetRefiller(refiller)
			consoleLogger.Info("Refiller 已注入", "model", llmModel)

			if updateSchema != "" {
				schemas, schErr := llm.LoadEntitySchemasFromDir(updateSchema)
				if schErr != nil {
					ui.Warning("加载 Schema 失败: %v", schErr)
				} else {
					hyper.AddSchemas(updateSchema, schemas)
					consoleLogger.Info("Schema 已加载", "文件数", len(schemas), "目录", updateSchema)
				}
			}
		}

		ui.Info("包含 LLM 增强：将执行摘要 + 实体提取")
	} else {
		ui.Warning("LLM 未配置，仅执行重新索引（跳过摘要 + 实体提取）")
		ui.Info("提示：可通过 --llm-key/--llm-url/--llm-model 参数启用 LLM 增强")
	}

	// 6. 执行 update
	ui.Info("正在执行增量更新（重新索引 → LLM 增强）...")
	spinner := ui.NewSpinner("正在处理...")
	spinner.Start()

	if err := svc.Update(context.Background(), absTarget); err != nil {
		spinner.Stop()
		ui.Error("更新失败: %v", err)
		os.Exit(1)
	}

	spinner.Stop()
	ui.Success("增量更新完成")
}

// ── tree ─────────────────────────────────────────────────────────

func runTree(cmd *cobra.Command, args []string) {
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

	root, err := svc.Tree(context.Background())
	if err != nil {
		ui.Error("构建目录树失败: %v", err)
		os.Exit(1)
	}

	// 以项目根目录为树根，不显示上方无关的全路径
	projectRoot := filepath.Dir(ragDir)
	if trimmed := trimTreeToRoot(root, projectRoot); trimmed != nil {
		root = trimmed
	}

	renderTree(root, "")
}

// runChunks 执行 chunks 子命令：分页列出 Chunk。
func runChunks(cmd *cobra.Command, args []string) {
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

	chunks, total, err := svc.ListChunks(context.Background(), chunksPage, chunksSize, chunksFilter)
	if err != nil {
		ui.Error("列出分片失败: %v", err)
		os.Exit(1)
	}

	if chunksJSON {
		output := struct {
			Page  int         `json:"page"`
			Size  int         `json:"size"`
			Total int         `json:"total"`
			Items []chunkItem `json:"items"`
		}{
			Page:  chunksPage,
			Size:  chunksSize,
			Total: total,
			Items: formatChunkItems(chunks),
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			ui.Error("序列化 JSON 失败: %v", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	// 终端输出
	ui.Title("Chunk 列表")
	ui.KeyValue("页码", fmt.Sprintf("%d", chunksPage))
	ui.KeyValue("每页", fmt.Sprintf("%d", chunksSize))
	ui.KeyValue("总数", fmt.Sprintf("%d", total))
	if chunksFilter != "" {
		ui.KeyValue("过滤", chunksFilter)
	}

	if len(chunks) == 0 {
		ui.Info("没有更多分片")
		return
	}

	for i, ch := range chunks {
		fmt.Printf("\n[%d] %s\n", (chunksPage-1)*chunksSize+i+1, ch.ID)
		fmt.Printf("Title:   %s\n", ch.Title)
		fmt.Printf("Summary: %s\n", truncateString(ch.Summary, 120))
		if len(ch.Tags) > 0 {
			fmt.Printf("Tags:    %s\n", strings.Join(ch.Tags, ", "))
		} else {
			fmt.Printf("Tags:    -\n")
		}
	}
}

// chunkItem 是 chunks 命令 JSON 输出的单条结构。
type chunkItem struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// formatChunkItems 将 core.Chunk 切片转换为 chunkItem 切片。
func formatChunkItems(chunks []core.Chunk) []chunkItem {
	items := make([]chunkItem, 0, len(chunks))
	for _, ch := range chunks {
		items = append(items, chunkItem{
			ID:      ch.ID,
			Title:   ch.Title,
			Summary: ch.Summary,
			Tags:    ch.Tags,
		})
	}
	return items
}

// runNodes 执行 nodes 子命令：目录级多跳图节点查询。
func runNodes(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	dir := ""
	if len(args) > 0 {
		dir = args[0]
	}

	if nodesHops < 1 || nodesHops > 3 {
		ui.Error("跳数范围必须在 1-3 之间")
		os.Exit(1)
	}

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	result, err := svc.Nodes(context.Background(), dir, nodesHops)
	if err != nil {
		ui.Error("图节点查询失败: %v", err)
		os.Exit(1)
	}

	if nodesJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			ui.Error("序列化 JSON 失败: %v", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	ui.Title("图节点")
	ui.KeyValue("Region", result.RegionName)
	ui.KeyValue("RegionID", truncateString(result.RegionID, 16))
	ui.KeyValue("跳数", fmt.Sprintf("%d", nodesHops))
	ui.KeyValue("节点数", fmt.Sprintf("%d", len(result.Nodes)))
	ui.KeyValue("关系数", fmt.Sprintf("%d", len(result.Edges)))

	if len(result.Nodes) == 0 {
		ui.Info("未找到节点")
		return
	}

	for _, n := range result.Nodes {
		label := "-"
		if len(n.Labels) > 0 {
			label = strings.Join(n.Labels, ", ")
		}
		fmt.Printf("\n[%s] %s\n", label, n.Name)
		fmt.Printf("  ID:   %s\n", n.ID)
		if len(n.SourceChunkIDs) > 0 {
			fmt.Printf("  SourceChunks: %s\n", strings.Join(n.SourceChunkIDs, ", "))
		}
	}

	if len(result.Edges) > 0 {
		fmt.Println("\n关系:")
		for _, e := range result.Edges {
			fmt.Printf("  %s --[%s]--> %s\n", truncateString(e.Source, 16), e.Type, truncateString(e.Target, 16))
		}
	}
}

// runCypher 执行 cypher 子命令：原始 Cypher 图查询。
func runCypher(cmd *cobra.Command, args []string) {
	ragDir, err := findRAGInCWD()
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	query := args[0]
	if query == "" {
		ui.Error("Cypher 查询不能为空")
		os.Exit(1)
	}

	svc, err := gorag.NewRAGService(ragDir)
	if err != nil {
		ui.Error("打开 RAG 库失败: %v", err)
		os.Exit(1)
	}
	defer svc.Stop()

	rows, err := svc.Cypher(context.Background(), query)
	if err != nil {
		ui.Error("Cypher 查询失败: %v", err)
		os.Exit(1)
	}

	if cypherJSON {
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			ui.Error("序列化 JSON 失败: %v", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	ui.Title("Cypher 查询结果")
	ui.KeyValue("行数", fmt.Sprintf("%d", len(rows)))

	if len(rows) == 0 {
		ui.Info("无结果")
		return
	}

	for i, row := range rows {
		fmt.Printf("\n--- 行 %d ---\n", i+1)
		for k, v := range row {
			fmt.Printf("%s: %v\n", k, v)
		}
	}
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
			line := fmt.Sprintf("%s%s%s/", prefix, branch, child.Name)
			if child.Summary != "" {
				line = fmt.Sprintf("%s - %s", line, truncateString(child.Summary, 80))
			}
			fmt.Println(line)
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
		renderChunkNode(chunk, prefix+branch, prefix+connector)
	}
}

// renderFileNode 渲染文件节点及 Chunk 子树。
func renderFileNode(node *gorag.SourceTreeNode, prefix, branch, connector string, isLast bool) {
	// 文件节点：只显示文件名
	if isLast {
		branch = "└── "
		connector = "    "
	}
	fmt.Printf("%s%s%s  [size:%s]\n", prefix, branch, filepath.Base(node.Path), formatBytes(node.Size))

	// Chunk 子树相对于文件节点缩进
	childPrefix := prefix + connector
	for i, chunk := range node.Chunks {
		chunkBranch := "├── "
		chunkConn := "│   "
		if i == len(node.Chunks)-1 {
			chunkBranch = "└── "
			chunkConn = "    "
		}
		renderChunkNode(chunk, childPrefix+chunkBranch, childPrefix+chunkConn)
	}
}

// renderChunkNode 渲染单个 Chunk 节点。
func renderChunkNode(node gorag.SourceChunkNode, branch, connector string) {
	fmt.Printf("%s%s\n", branch, formatChunkLine(node.Title, node.Summary, node.StartLine, node.EndLine))
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
		fmt.Printf("%s%s%s\n", prefix+branch, "", formatChunkLine(child.Title, child.Summary, child.StartLine, child.EndLine))
		renderChunkChildren(child, prefix+connector)
	}
}

// formatChunkLine 格式化 Chunk 行：[Lstart-Lend] Title [- Summary]
func formatChunkLine(title, summary string, startLine, endLine int) string {
	pos := formatPos(startLine, endLine)
	title = strings.ReplaceAll(title, "\n", " ")
	summary = strings.ReplaceAll(summary, "\n", " ")
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Sprintf("%s %s", pos, title)
	}
	return fmt.Sprintf("%s %s - %s", pos, title, summary)
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

// trimTreeToRoot 从树根向下找到指定路径的节点，作为新的渲染根。
// 用于去掉绝对路径前缀，只显示项目根目录以下的目录树。
func trimTreeToRoot(root *gorag.SourceTreeNode, absPath string) *gorag.SourceTreeNode {
	parts := strings.Split(absPath, string(filepath.Separator))
	current := root
	for _, part := range parts {
		if part == "" {
			continue
		}
		found := false
		for _, child := range current.Children {
			if child.IsDir && child.Name == part {
				current = child
				found = true
				break
			}
		}
		if !found {
			return root
		}
	}
	return current
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

// formatPos 将行号格式化为 [Lstart-Lend]，行号都为 0 时返回空字符串。
func formatPos(start, end int) string {
	if start == 0 && end == 0 {
		return ""
	}
	return fmt.Sprintf("  [L%d-L%d]", start, end)
}

// truncateString 截断字符串到最大长度，超出时追加省略号。
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// splitQueryGroups 将查询字符串按 | 拆分为多个关键字组，并去除首尾空格。
func splitQueryGroups(text string) []string {
	parts := strings.Split(text, "|")
	var groups []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			groups = append(groups, p)
		}
	}
	if len(groups) == 0 {
		groups = append(groups, strings.TrimSpace(text))
	}
	return groups
}
