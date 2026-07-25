package webapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	gorag "github.com/DotNetAge/gorag/v2"
	goragLog "github.com/DotNetAge/gorag/v2/logging"
)

// logger 是 webapi 包全局使用的结构化日志实例。
var logger = goragLog.DefaultConsoleLogger()

// Server 是 WebAPI 服务的顶层结构体，持有 RAG 服务实例和 WebSocket Hub，
// 所有 HTTP 处理器作为 Server 的方法实现，避免全局变量。
type Server struct {
	svc    *gorag.IndexingService
	hub    *Hub
	runner *processRunner
	ragDir string
}

// Start 启动 HTTP 服务，阻塞直到收到中断信号。
func Start(port, ragDir string) error {
	if port == "" {
		port = "8080"
	}

	// 如果未指定 rag-dir，尝试自动检测
	if ragDir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			entries, err := os.ReadDir(cwd)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() && strings.HasSuffix(e.Name(), ".rag") {
						ragDir = filepath.Join(cwd, e.Name())
						break
					}
				}
			}
		}
	}

	s := &Server{ragDir: ragDir}

	// 初始化 WebSocket Hub（必须在启动后台任务之前）
	s.hub = NewHub()
	s.runner = &processRunner{hub: s.hub}

	// 初始化 RAG 服务（所有 handler 共享同一实例）
	if ragDir != "" {
		svc, err := gorag.NewRAGService(ragDir)
		if err != nil {
			logger.Error("初始化 RAG 服务失败", err, "ragDir", ragDir)
		} else {
			s.svc = svc
			logger.Info("RAG 服务已初始化", "知识库", ragDir)

			// 自动续跑中断的索引任务
			pending, indexing, countErr := svc.IndexerSvc().UnfinishedWorkCount()
			needsUpdate, updateErr := svc.IndexerSvc().NeedsUpdate(context.Background())

			// 读取 LLM 配置，判断能否执行 Update 阶段
			llmCfg := loadLLMConfigFromFile(ragDir)
			hasLLM := llmCfg.BaseURL != "" && llmCfg.Model != ""

			if countErr != nil {
				logger.Error("检查未完成索引任务失败", countErr)
			} else if updateErr != nil {
				logger.Error("检查未完成 Update 任务失败", updateErr)
			} else if pending > 0 || indexing > 0 || (needsUpdate && hasLLM) {
				dirs := loadIndexDirs(ragDir)
				if len(dirs) > 0 {
					if hasLLM {
						logger.Info("LLM 已配置，将执行 Index + Update 阶段", "model", llmCfg.Model)
					} else {
						logger.Info("未配置 LLM，仅执行 Index 阶段")
					}
					s.runner.start(s.svc, dirs, llmCfg)
				} else {
					logger.Warn("未配置索引目录，跳过自动续跑")
				}
			} else if needsUpdate {
				logger.Info("LLM 未配置，跳过自动续跑（已在状态栏提示配置）")
			} else {
				logger.Info("索引状态正常，无中断任务")
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/init", s.handleInit)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/delete", s.handleDelete)
	mux.HandleFunc("/api/index-dirs", s.handleIndexDirs)
	mux.HandleFunc("/api/index", s.handleIndex)
	mux.HandleFunc("/api/query", s.handleQuery)
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/doctor", s.handleDoctor)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/update", s.handleUpdate)
	mux.HandleFunc("/api/tree", s.handleTree)
	mux.HandleFunc("/api/chunks", s.handleChunks)
	mux.HandleFunc("/api/nodes", s.handleNodes)
	mux.HandleFunc("/api/file-nodes", s.handleFileNodes)
	mux.HandleFunc("/api/cypher", s.handleCypher)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/llm-check", s.handleLLMCheck)
	mux.HandleFunc("/api/usage", s.handleUsage)
	mux.HandleFunc("/api/process", s.handleProcessStart)
	mux.HandleFunc("/api/process-progress", s.handleProcessProgress)
	mux.HandleFunc("/api/fs-home", s.handleFSHome)
	mux.HandleFunc("/api/fs-list", s.handleFSList)
	mux.HandleFunc("/api/schema-list", s.handleSchemaList)
	mux.HandleFunc("/api/schema-content", s.handleSchemaContent)
	mux.HandleFunc("/api/dir-schemas", s.handleDirSchemas)
	mux.HandleFunc("/api/schema-custom", s.handleSchemaCustom)
	mux.HandleFunc("/api/schema-status", s.handleSchemaStatus)
	mux.HandleFunc("/api/read-file", s.handleReadFile)
	mux.HandleFunc("/api/save-file", s.handleSaveFile)
	mux.HandleFunc("/api/fs-mkdir", s.handleFSMkdir)
	mux.HandleFunc("/api/fs-remove", s.handleFSRemove)
	mux.HandleFunc("/api/fs-move", s.handleFSMove)
	mux.HandleFunc("/api/open-file", s.handleOpenFile)
	mux.HandleFunc("/api/ragignore", s.handleCreateRagignore)
	mux.HandleFunc("/api/ragignore-append", s.handleAppendRagignore)
	mux.HandleFunc("/api/file", s.handleServeFile)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ServeWS(s.hub, w, r)
	})
	mux.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%s", port)
	logger.Info("RAG Web API 服务启动", "port", port)
	if ragDir != "" {
		logger.Info("RAG 库", "path", ragDir)
	}

	server := &http.Server{
		Addr:    addr,
		Handler: loggingMiddleware(mux),
	}

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("服务启动失败", err)
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("正在关闭服务")

	// 关闭 RAG 服务
	if s.svc != nil {
		if err := s.svc.Stop(); err != nil {
			logger.Error("关闭 RAG 服务时出错", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

// loggingMiddleware 记录每个请求的方法、路径、状态码和耗时，同时添加 CORS 头。
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// CORS 头（内部服务器使用，全开放即可）
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 包装 ResponseWriter 以捕获状态码
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)

		// 过滤 /health 的重复日志
		if r.URL.Path != "/health" {
			duration := time.Since(start)
			logger.Info("HTTP 请求",
				"method", r.Method,
				"path", r.URL.Path,
				"status", lrw.statusCode,
				"duration", duration.Round(time.Millisecond))
		}
	})
}

// loggingResponseWriter 包装 http.ResponseWriter 以捕获状态码。
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack 实现 http.Hijacker 接口，支持 WebSocket 升级。
func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("loggingResponseWriter: 底层 ResponseWriter 不支持 Hijack")
}

// handleHealth 健康检查端点。
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── 请求/响应类型 ──────────────────────────────────────────────────

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type initRequest struct {
	RagDir    string `json:"rag_dir"`
	IndexType string `json:"index_type"`
	ModelID   string `json:"model_id"`
	ModelFile string `json:"model_file"`
	ModelPath string `json:"model_path"`
}

type indexRequest struct {
	Path string `json:"path"`
}

type queryRequest struct {
	Text       string `json:"text"`
	TopK       int    `json:"top_k"`
	FilterPath string `json:"filter_path"`
	Format     string `json:"format"`
	ShowScore  bool   `json:"show_score"`
	ShowDocID  bool   `json:"show_doc_id"`
	ContentMax int    `json:"content_max"`
}

type updateRequest struct {
	Path      string `json:"path"`
	LLMKey    string `json:"llm_key"`
	LLMURL    string `json:"llm_url"`
	LLMModel  string `json:"llm_model"`
	SchemaDir string `json:"schema_dir"`
}

type nodesRequest struct {
	Dir  string `json:"dir"`
	Hops int    `json:"hops"`
}

type fileNodesRequest struct {
	File string `json:"file"`
	Hops int    `json:"hops"`
}

type cypherRequest struct {
	Query string `json:"query"`
}

// ── 工具函数 ──────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Success: false, Error: msg})
}

func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: data})
}

// loadLLMConfigFromFile 从 .rag/config.yml 加载 LLM 配置。
// config.yml 不存在或无 LLM 配置时返回空配置（跳过 Update 阶段）。
func loadLLMConfigFromFile(ragDir string) Config {
	goragCfg, err := gorag.LoadConfig(ragDir)
	if err != nil {
		return Config{}
	}
	return Config{
		BaseURL: goragCfg.LLM.BaseURL,
		APIKey:  goragCfg.LLM.APIKey,
		Model:   goragCfg.LLM.Model,
	}
}
