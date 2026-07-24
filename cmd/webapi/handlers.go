package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	gorag "github.com/DotNetAge/gorag/v2"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/formatter"
	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/llm"
	goragLog "github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/utils"
)

// ── 全局单例 ────────────────────────────────────────────────────────

// globalSvc 是全局唯一的 RAG 服务实例，所有 handler 共享同一个实例。
// 在 Server.Start 中初始化，Server 关闭时 Stop。
var globalSvc *gorag.IndexingService

// SetGlobalService 设置全局 RAG 服务实例（由 Server.Start 调用）。
func SetGlobalService(svc *gorag.IndexingService) {
	globalSvc = svc
}

// withRAG 注入全局 RAG 服务实例到处理器。
func withRAG(next func(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if globalSvc == nil {
			writeError(w, http.StatusBadRequest, "请先初始化 RAG 库")
			return
		}
		next(w, r, globalSvc)
	}
}

// resolveRAGDir 确定 .rag 库目录。
func resolveRAGDir(ragDir string) (string, error) {
	if ragDir != "" {
		info, err := os.Stat(ragDir)
		if err != nil {
			return "", fmt.Errorf("RAG 库目录不存在: %s", ragDir)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("RAG 库路径不是目录: %s", ragDir)
		}
		return ragDir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前工作目录失败: %w", err)
	}
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return "", fmt.Errorf("读取当前目录失败: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".rag") {
			return filepath.Join(cwd, e.Name()), nil
		}
	}
	return "", fmt.Errorf("未找到 .rag 库目录，请通过 --rag-dir 指定或先在当前目录运行 grag init")
}

// ── config ────────────────────────────────────────────────────────

type configRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	ragDir, err := resolveRAGDir("")
	if err != nil {
		// 没有 RAG 库时，GET 返回空配置，POST 返回错误
		if r.Method == http.MethodGet {
			writeSuccess(w, configRequest{})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		cfg, loadErr := gorag.LoadConfig(ragDir)
		if loadErr != nil {
			writeSuccess(w, configRequest{})
			return
		}
		writeSuccess(w, configRequest{
			BaseURL: cfg.LLM.BaseURL,
			APIKey:  cfg.LLM.APIKey,
			Model:   cfg.LLM.Model,
		})

	case http.MethodPost:
		var req configRequest
		if parseErr := json.NewDecoder(r.Body).Decode(&req); parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", parseErr))
			return
		}

		cfg, loadErr := gorag.LoadConfig(ragDir)
		if loadErr != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("读取配置失败: %v", loadErr))
			return
		}
		cfg.LLM.BaseURL = req.BaseURL
		cfg.LLM.Model = req.Model
		cfg.LLM.APIKey = req.APIKey
		if saveErr := gorag.SaveConfig(ragDir, cfg); saveErr != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("保存配置失败: %v", saveErr))
			return
		}

		writeSuccess(w, map[string]string{"message": "LLM 配置已保存"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 和 POST 方法")
	}
}

// ── delete ────────────────────────────────────────────────────────

type deleteRequest struct {
	Path string `json:"path"`
}

func handleDelete(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req deleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "路径不能为空")
		return
	}

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("无法解析路径: %v", err))
		return
	}

	ctx := context.Background()
	if err := svc.IndexerSvc().RemoveDir(ctx, absPath); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("删除失败: %v", err))
		return
	}

	// 同步从 index_dirs.json 中移除该目录
	ragDir := svc.DataDir()
	if ragDir != "" {
		if err := removeFromIndexDirs(ragDir, absPath); err != nil {
			// 非关键操作，仅记录日志
			logger.Warn("从 index_dirs 移除目录失败", "path", absPath, "error", err)
		}
	}

	writeSuccess(w, map[string]string{
		"message": "删除完成",
		"path":    absPath,
	})
}

// ── index-dirs ────────────────────────────────────────────────────

type indexDirEntry struct {
	Path string `json:"path"`
}

type indexDirsResponse struct {
	Dirs   []string `json:"dirs"`
	RagDir string   `json:"rag_dir"`
	RunDir string   `json:"run_dir"`
}

func handleIndexDirs(w http.ResponseWriter, r *http.Request) {
	ragDir, err := resolveRAGDir("")
	if err != nil {
		if r.Method == http.MethodGet {
			runDir, _ := os.Getwd()
			writeSuccess(w, indexDirsResponse{
				Dirs:   []string{},
				RagDir: "",
				RunDir: runDir,
			})
			return
		}
		writeError(w, http.StatusBadRequest, "请先初始化 RAG 库（使用 grag serve -i）")
		return
	}

	switch r.Method {
	case http.MethodGet:
		dirs := loadIndexDirs(ragDir)
		runDir, _ := os.Getwd()
		writeSuccess(w, indexDirsResponse{
			Dirs:   dirs,
			RagDir: ragDir,
			RunDir: runDir,
		})

	case http.MethodPost:
		var req struct {
			Dirs []string `json:"dirs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
			return
		}
		if err := saveIndexDirs(ragDir, req.Dirs); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("保存目录列表失败: %v", err))
			return
		}
		writeSuccess(w, map[string]string{"message": "目录列表已保存"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 和 POST 方法")
	}
}

// ── index_dirs.json 持久化辅助 ───────────────────────────────────

func indexDirsPath(ragDir string) string {
	return filepath.Join(ragDir, "index_dirs.json")
}

func loadIndexDirs(ragDir string) []string {
	data, err := os.ReadFile(indexDirsPath(ragDir))
	if err != nil {
		return nil
	}
	var dirs []string
	if err := json.Unmarshal(data, &dirs); err != nil {
		return nil
	}
	// 去重
	seen := make(map[string]bool)
	var unique []string
	for _, d := range dirs {
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		unique = append(unique, abs)
	}
	return unique
}

func saveIndexDirs(ragDir string, dirs []string) error {
	// 去重并保留顶层目录
	dirs = deduplicateDirs(dirs)
	data, _ := json.MarshalIndent(dirs, "", "  ")
	return os.WriteFile(indexDirsPath(ragDir), data, 0644)
}

func removeFromIndexDirs(ragDir, absPath string) error {
	dirs := loadIndexDirs(ragDir)
	var filtered []string
	for _, d := range dirs {
		if d != absPath && !strings.HasPrefix(d, absPath+string(filepath.Separator)) {
			filtered = append(filtered, d)
		}
	}
	if len(filtered) == len(dirs) {
		return nil // 无变化
	}
	return saveIndexDirs(ragDir, filtered)
}

// ── 进程内进度追踪 ──────────────────────────────────────────────────

// ProcessProgress 记录当前后台处理的状态。
type ProcessProgress struct {
	Running      bool   `json:"running"`
	Phase        string `json:"phase"`         // "indexing" / "updating" / "completed" / "error"
	Dir          string `json:"dir"`           // 当前处理的目录
	DirIndex     int    `json:"dir_index"`     // 从 0 开始
	DirCount     int    `json:"dir_count"`     // 总目录数
	CurrentFile  string `json:"current_file"`  // 当前正在处理的文件
	Message      string `json:"message"`       // 当前状态描述
	TotalFiles   int    `json:"total_files"`   // 当前目录文件总数
	IndexedFiles int    `json:"indexed_files"` // 当前目录已索引文件数
	Error        string `json:"error,omitempty"`
}

// deduplicateDirs 移除子目录，只保留顶层目录。
// 例如 ["/a/b", "/a", "/a/b/c", "/d"] → ["/a", "/d"]
func deduplicateDirs(dirs []string) []string {
	type item struct {
		original string
		abs      string
	}

	var items []item
	for _, d := range dirs {
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		items = append(items, item{original: d, abs: abs})
	}

	if len(items) <= 1 {
		if len(items) == 1 {
			return []string{items[0].original}
		}
		return nil
	}

	// 按路径长度排序，确保父目录排在子目录前
	sort.Slice(items, func(i, j int) bool {
		return len(items[i].abs) < len(items[j].abs)
	})

	var result []item
	for _, it := range items {
		hasParent := false
		absWithSep := it.abs + string(filepath.Separator)
		for _, r := range result {
			if strings.HasPrefix(absWithSep, r.abs+string(filepath.Separator)) {
				hasParent = true
				break
			}
		}
		if !hasParent {
			result = append(result, it)
		}
	}

	out := make([]string, len(result))
	for i, r := range result {
		out[i] = r.original
	}
	return out
}

type processRunner struct {
	mu       sync.RWMutex
	progress ProcessProgress
	hub      *Hub // 非空时每次更新后自动广播 WebSocket 通知
}

var globalRunner = &processRunner{}

// SetHub 设置 processRunner 的 WebSocket Hub，用于实时推送进度。
func (r *processRunner) SetHub(h *Hub) {
	r.hub = h
}

func (r *processRunner) load() ProcessProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.progress
}

func (r *processRunner) update(fn func(p *ProcessProgress)) {
	r.mu.Lock()
	fn(&r.progress)
	r.mu.Unlock()
	// 每次更新后自动广播到所有 WebSocket 客户端
	if r.hub != nil {
		r.hub.Broadcast("progress", r.progress)
	}
}

// start 在后台启动顺序 Index → Update 流程。
// 使用全局唯一的 RAG 服务实例（globalSvc），避免多实例竞争同一 .rag 目录的锁。
func (r *processRunner) start(dirs []string, llmCfg Config) {
	svc := globalSvc
	if svc == nil {
		r.update(func(p *ProcessProgress) {
			p.Running = false
			p.Phase = "error"
			p.Message = "知识库未初始化"
			p.Error = "请先初始化 RAG 库，使用 grag serve -i"
		})
		return
	}

	// 去重：移除子目录条目，只保留顶层目录
	dirs = deduplicateDirs(dirs)
	if len(dirs) == 0 {
		r.update(func(p *ProcessProgress) {
			p.Running = false
			p.Phase = "error"
			p.Message = "没有有效的索引目录"
			p.Error = "索引目录列表为空"
		})
		return
	}

	r.update(func(p *ProcessProgress) {
		p.Running = true
		p.Phase = "indexing"
		p.Dir = ""
		p.DirIndex = 0
		p.DirCount = len(dirs)
		p.Message = "正在准备…"
		p.TotalFiles = 0
		p.IndexedFiles = 0
		p.Error = ""
	})

	go func() {
		ctx := context.Background()
		logger.Info("后台处理协程已启动", "目录数", len(dirs))

		// 设置进度回调
		svc.IndexerSvc().ProgressFn = func(file string, indexed, total int) {
			r.update(func(p *ProcessProgress) {
				p.CurrentFile = file
				p.IndexedFiles = indexed
				if total > p.TotalFiles {
					p.TotalFiles = total
				}
			})
		}

		// 设置事件回调：将 IndexerService 发射的细粒度事件转发到 WebSocket
		svc.IndexerSvc().OnEvent = func(event gorag.IndexerEvent) {
			if r.hub != nil {
				r.hub.Broadcast("indexer_event", event)
			}
		}

		// 阶段 1：依次 Index 每个目录
		logger.Info("开始 Index 阶段", "目录数", len(dirs))
		for i, dir := range dirs {
			total := countFilesInDir(dir)
			logger.Info("开始索引目录", "序号", i+1, "总目录", len(dirs), "目录", dir, "文件数", total)
			r.update(func(p *ProcessProgress) {
				p.Phase = "indexing"
				p.Dir = dir
				p.DirIndex = i
				p.CurrentFile = ""
				p.Message = "正在索引 " + dir + " …"
				p.TotalFiles = total
				p.IndexedFiles = 0
			})

			if err := svc.IndexerSvc().Index(ctx, dir); err != nil {
				logger.Error("目录索引失败", err, "序号", i+1, "总目录", len(dirs), "目录", dir)
				r.update(func(p *ProcessProgress) {
					p.Running = false
					p.Phase = "error"
					p.Message = "索引失败"
					p.Error = err.Error()
				})
				svc.IndexerSvc().ProgressFn = nil
				svc.IndexerSvc().OnEvent = nil
				return
			}

			indexed := countIndexedFiles(svc, dir)
			logger.Info("目录索引完成", "序号", i+1, "总目录", len(dirs), "目录", dir, "已索引", indexed)
			r.update(func(p *ProcessProgress) {
				p.IndexedFiles = indexed
				p.CurrentFile = ""
			})
		}
		logger.Info("Index 阶段全部完成")

		// 阶段 2：依次 Update 每个目录
		if llmCfg.BaseURL != "" && llmCfg.Model != "" {
			logger.Info("开始 Update 阶段", "目录数", len(dirs))
			if err := injectLLMConfig(svc, llmCfg); err != nil {
				logger.Warn("注入 LLM 配置失败", "error", err)
			}

			// 加载 schemas 配置：优先使用目录级配置，无则回退到全局配置
			for i, dir := range dirs {
				logger.Info("开始更新目录", "序号", i+1, "总目录", len(dirs), "目录", dir)
				total := countFilesInDir(dir)
				r.update(func(p *ProcessProgress) {
					p.Phase = "updating"
					p.Dir = dir
					p.DirIndex = i
					p.CurrentFile = ""
					p.Message = "正在完善索引 " + dir + " …"
					p.TotalFiles = total
					p.IndexedFiles = 0
				})

				// 加载 schemas：优先目录级配置，回退到全局配置
				var schemaDir string
				schemas, sErr := llm.LoadEntitySchemasFromDir(gorag.DirSchemaDir(svc.DataDir(), dir))
				if sErr != nil || len(schemas) == 0 {
					schemas, sErr = llm.LoadEntitySchemasFromDir(gorag.DirSchemaDir(svc.DataDir(), ""))
					if sErr == nil && len(schemas) > 0 {
						schemaDir = "schemas/all (全局)"
						logger.Info("Update: 使用全局 schema 配置", "count", len(schemas))
					}
				} else {
					schemaDir = fmt.Sprintf("schemas/%s (目录级)", dir)
					logger.Info("Update: 使用目录级 schema 配置", "dir", dir, "count", len(schemas))
				}
				if len(schemas) > 0 {
					if hyper, ok := svc.Indexer().(*indexer.HyperIndexer); ok {
						hyper.AddSchemas(schemaDir, schemas)
					}
				}

				if err := svc.IndexerSvc().Update(ctx, dir); err != nil {
					logger.Error("目录更新失败", err, "序号", i+1, "总目录", len(dirs), "目录", dir)
					r.update(func(p *ProcessProgress) {
						p.Running = false
						p.Phase = "error"
						p.Message = "完善索引失败"
						p.Error = err.Error()
					})
					svc.IndexerSvc().ProgressFn = nil
					svc.IndexerSvc().OnEvent = nil
					return
				}
				logger.Info("目录更新完成", "序号", i+1, "总目录", len(dirs), "目录", dir)
			}
			logger.Info("Update 阶段全部完成")
		}

		r.update(func(p *ProcessProgress) {
			p.Running = false
			p.Phase = "completed"
			p.Message = "全部完成"
		})
		logger.Info("全部任务完成")
		// 清理回调，避免影响后续同步操作
		svc.IndexerSvc().ProgressFn = nil
		svc.IndexerSvc().OnEvent = nil
	}()
}

// countFilesInDir 统计目录下可索引文件的大致数量。
func countFilesInDir(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var n int
	for _, e := range entries {
		if e.IsDir() {
			sub, _ := os.ReadDir(filepath.Join(dir, e.Name()))
			n += len(sub)
		} else {
			n++
		}
	}
	return n
}

// countIndexedFiles 统计当前已索引的文件数量。
func countIndexedFiles(svc *gorag.IndexingService, dir string) int {
	progs, err := svc.Admin().FileStatuses(context.Background(), "", dir)
	if err != nil {
		return 0
	}
	var n int
	for _, p := range progs {
		if p.IndexStatus == "indexed" {
			n++
		}
	}
	return n
}

// injectLLMConfig 将 LLM 组件注入到 HyperIndexer 中。
func injectLLMConfig(svc *gorag.IndexingService, cfg Config) error {
	hyper, ok := svc.Indexer().(*indexer.HyperIndexer)
	if !ok {
		return fmt.Errorf("索引器不是 HyperIndexer，无法注入 LLM")
	}

	llmCfg := llm.Config{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	}
	consoleLogger := goragLog.DefaultConsoleLogger()

	summarizer, err := llm.NewSummarizer(llmCfg, consoleLogger)
	if err != nil {
		return fmt.Errorf("创建 Summarizer 失败: %w", err)
	}
	hyper.SetSummarizer(summarizer)

	refiller, err := llm.NewRefiller(llmCfg, consoleLogger)
	if err != nil {
		return fmt.Errorf("创建 Refiller 失败: %w", err)
	}
	hyper.SetRefiller(refiller)

	return nil
}

// Config WebAPI 常用配置结构。
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// ── process ────────────────────────────────────────────────────────

type processStartRequest struct {
	Dirs []string `json:"dirs"`
	LLM  Config   `json:"llm"`
}

func handleProcessStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	if globalRunner.load().Running {
		writeError(w, http.StatusConflict, "已有正在执行的索引任务")
		return
	}

	var req processStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	if len(req.Dirs) == 0 {
		writeError(w, http.StatusBadRequest, "目录列表不能为空")
		return
	}

	globalRunner.start(req.Dirs, req.LLM)
	writeSuccess(w, map[string]string{"message": "后台处理已启动"})
}

func handleProcessProgress(w http.ResponseWriter, r *http.Request) {
	prog := globalRunner.load()
	writeSuccess(w, prog)
}

// ── init ──────────────────────────────────────────────────────────

func handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req initRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	ragLibDir := req.RagDir
	if ragLibDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("获取当前目录失败: %v", err))
			return
		}
		ragLibDir = filepath.Join(cwd, ".rag")
	}

	idxType := req.IndexType
	if idxType == "" {
		idxType = "hyper"
	}
	modelID := req.ModelID
	modelFile := req.ModelFile
	modelPath := req.ModelPath

	if (idxType == "hyper" || idxType == "semantic") && modelPath == "" && modelID == "" {
		modelID = "Xenova/chinese-clip-vit-base-patch16"
		modelFile = "onnx/model.onnx"
	}

	var observer utils.DownloadObserver

	result, err := gorag.InitRAG(gorag.InitOptions{
		RagDir:    ragLibDir,
		IndexType: idxType,
		ModelPath: modelPath,
		ModelID:   modelID,
		ModelFile: modelFile,
		Observer:  observer,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("初始化失败: %v", err))
		return
	}

	writeSuccess(w, map[string]interface{}{
		"rag_dir":      result.RagDir,
		"index_type":   result.IndexType,
		"model_path":   result.ModelPath,
		"indexer_name": result.IndexerName,
	})
}

// ── index ─────────────────────────────────────────────────────────

func handleIndex(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req indexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	target := req.Path
	if target == "" {
		target = "."
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("无法解析路径: %v", err))
		return
	}

	consoleLogger := goragLog.DefaultConsoleLogger()
	if hyper, ok := svc.Indexer().(*indexer.HyperIndexer); ok {
		hyper.SetLogger(consoleLogger)
	}

	if err := svc.IndexerSvc().Index(context.Background(), absTarget); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("索引失败: %v", err))
		return
	}

	writeSuccess(w, map[string]string{
		"message": "索引完成",
		"target":  absTarget,
	})
}

// ── query ─────────────────────────────────────────────────────────

func handleQuery(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "查询内容不能为空")
		return
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}
	format := req.Format
	if format == "" {
		format = "json"
	}

	queries := splitQueryGroups(req.Text)

	var hit *core.Hit
	var err error
	if len(queries) == 1 {
		hit, err = svc.Querier().Query(context.Background(), queries[0], req.FilterPath)
	} else {
		hit, err = svc.Querier().QueryMulti(context.Background(), queries, req.FilterPath)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("搜索失败: %v", err))
		return
	}

	if hit != nil && len(hit.Chunks) > topK {
		hit.Chunks = hit.Chunks[:topK]
	}

	switch format {
	case "prompt":
		output := formatter.NewPromptFormatter(
			formatter.WithContentMaxPrompt(req.ContentMax),
			formatter.WithIncludeScore(req.ShowScore),
		).FormatForRAG(hit, req.Text)
		writeSuccess(w, map[string]string{"result": output})
	case "terminal":
		output := formatter.NewTerminalFormatter(
			formatter.WithShowScore(req.ShowScore),
			formatter.WithShowDocID(req.ShowDocID),
			formatter.WithContentMax(req.ContentMax),
		).FormatAll(hit)
		writeSuccess(w, map[string]string{"result": output})
	default:
		writeSuccess(w, hit)
	}
}

// ── info ──────────────────────────────────────────────────────────

func handleInfo(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	info, err := svc.Admin().Info()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("获取信息失败: %v", err))
		return
	}
	writeSuccess(w, info)
}

// ── doctor ────────────────────────────────────────────────────────

func handleDoctor(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	checks := svc.Admin().Doctor()
	writeSuccess(w, checks)
}

// ── logs ──────────────────────────────────────────────────────────

func handleLogs(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	data, err := svc.Admin().Logs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("%v", err))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(data))
}

// ── update ────────────────────────────────────────────────────────

func handleUpdate(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	target := req.Path
	if target == "" {
		target = "."
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("无法解析路径: %v", err))
		return
	}

	writeSuccess(w, map[string]string{
		"message": "增量更新完成",
		"target":  absTarget,
	})
}

// ── tree ──────────────────────────────────────────────────────────

func handleTree(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	root, err := svc.Admin().Tree(context.Background())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("构建目录树失败: %v", err))
		return
	}
	writeSuccess(w, root)
}

// ── chunks ────────────────────────────────────────────────────────

func handleChunks(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	page := 1
	size := 20
	filter := ""

	if p := r.URL.Query().Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if s := r.URL.Query().Get("size"); s != "" {
		fmt.Sscanf(s, "%d", &size)
	}
	filter = r.URL.Query().Get("filter")

	chunks, total, err := svc.Querier().ListChunks(context.Background(), page, size, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("列出分片失败: %v", err))
		return
	}

	writeSuccess(w, map[string]interface{}{
		"page":  page,
		"size":  size,
		"total": total,
		"items": chunks,
	})
}

// ── nodes ─────────────────────────────────────────────────────────

func handleNodes(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req nodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	hops := req.Hops
	if hops < 1 {
		hops = 1
	}
	if hops > 3 {
		hops = 3
	}

	result, err := svc.Explorer().Nodes(context.Background(), req.Dir, hops)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("图节点查询失败: %v", err))
		return
	}

	writeSuccess(w, result)
}

// ── cypher ────────────────────────────────────────────────────────

func handleCypher(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req cypherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "Cypher 查询不能为空")
		return
	}

	rows, err := svc.Explorer().Cypher(context.Background(), req.Query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Cypher 查询失败: %v", err))
		return
	}

	writeSuccess(w, rows)
}

// ── llm-check ────────────────────────────────────────────────────

func handleLLMCheck(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	ragDir := svc.DataDir()

	// 从 config.yml 读取 LLM 配置
	cfg, err := gorag.LoadConfig(ragDir)
	configured := err == nil && cfg.LLM.BaseURL != "" && cfg.LLM.Model != "" && cfg.LLM.APIKey != ""

	// 检查是否有分片需要 LLM 处理
	needsUpdate, _ := svc.IndexerSvc().NeedsUpdate(r.Context())

	writeSuccess(w, map[string]interface{}{
		"configured":   configured,
		"needs_update": needsUpdate,
	})
}

// ── status ────────────────────────────────────────────────────────

func handleStatus(w http.ResponseWriter, r *http.Request, svc *gorag.IndexingService) {
	filter := r.URL.Query().Get("filter")
	status := r.URL.Query().Get("status")
	summary := r.URL.Query().Get("summary") == "true"

	ctx := context.Background()

	if summary {
		counts, err := svc.Admin().StatusSummary(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("查询状态统计失败: %v", err))
			return
		}
		writeSuccess(w, counts)
		return
	}

	progress, err := svc.Admin().FileStatuses(ctx, status, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("查询文件状态失败: %v", err))
		return
	}

	writeSuccess(w, progress)
}

// ── fs-home ───────────────────────────────────────────────────────

func handleFSHome(w http.ResponseWriter, r *http.Request) {
	home, err := os.UserHomeDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("获取用户主目录失败: %v", err))
		return
	}
	writeSuccess(w, map[string]string{"path": home})
}

// ── fs-list ───────────────────────────────────────────────────────

type fsEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

func handleFSList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path 参数不能为空")
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("读取目录失败: %v", err))
		return
	}

	result := make([]fsEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, fsEntry{
			Name:    e.Name(),
			Path:    filepath.Join(path, e.Name()),
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			Mode:    info.Mode().Perm().String(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	writeSuccess(w, result)
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
