package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── 文件读写 API ──────────────────────────────────────────────────

// handleReadFile 读取文本文件并返回内容。
// POST /api/read-file
func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}

	data, err := os.ReadFile(req.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("读取文件失败: %v", err))
		return
	}

	writeSuccess(w, map[string]string{
		"content": string(data),
		"path":    req.Path,
	})
}

// handleServeFile 提供文件下载/显示（用于图片等二进制文件）。
// GET /api/file?path=<url-encoded-path>
func (s *Server) handleServeFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}

	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("文件不存在: %v", err))
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "无法获取文件信息")
		return
	}

	// 根据扩展名设置 Content-Type
	ext := strings.ToLower(filepath.Ext(path))
	contentType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".bmp":
		contentType = "image/bmp"
	case ".svg":
		contentType = "image/svg+xml"
	case ".ico":
		contentType = "image/x-icon"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	io.Copy(w, f)
}

// handleSaveFile 将文本内容写入文件。
// POST /api/save-file
func (s *Server) handleSaveFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}

	if err := os.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("写入文件失败: %v", err))
		return
	}

	writeSuccess(w, map[string]string{
		"message": "文件已保存",
		"path":    req.Path,
	})
}

// ── 文件系统操作 API ───────────────────────────────────────────────

// handleFSMkdir 创建目录。
// POST /api/fs-mkdir
func (s *Server) handleFSMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}

	if err := os.MkdirAll(req.Path, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "创建目录失败: "+err.Error())
		return
	}
	writeSuccess(w, map[string]string{"message": "目录已创建"})
}

// handleFSRemove 删除文件或目录。
// POST /api/fs-remove
func (s *Server) handleFSRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}

	if err := os.RemoveAll(req.Path); err != nil {
		writeError(w, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}

	// 同步清理索引数据
	if s.svc != nil {
		ctx := context.Background()
		if err := s.svc.IndexerSvc().RemoveDir(ctx, req.Path); err != nil {
			logger.Warn("文件系统删除后清理索引数据失败", "path", req.Path, "error", err)
		}
		ragDir := s.svc.DataDir()
		// 同时从 index_dirs.json 中移除
		if ragDir != "" {
			if err := removeFromIndexDirs(ragDir, req.Path); err != nil {
				logger.Warn("从 index_dirs 移除目录失败", "path", req.Path, "error", err)
			}
		}
	}

	writeSuccess(w, map[string]string{"message": "已删除"})
}

// handleFSMove 移动/重命名文件或目录。
// POST /api/fs-move
func (s *Server) handleFSMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Source == "" || req.Target == "" {
		writeError(w, http.StatusBadRequest, "缺少 source 或 target 参数")
		return
	}

	if err := os.Rename(req.Source, req.Target); err != nil {
		writeError(w, http.StatusInternalServerError, "移动/重命名失败: "+err.Error())
		return
	}

	// 同步索引数据：删除旧路径的索引，重新索引新路径
	if s.svc != nil {
		ctx := context.Background()
		idx := s.svc.IndexerSvc()
		if err := idx.RemoveDir(ctx, req.Source); err != nil {
			logger.Warn("移动后清理旧路径索引失败", "source", req.Source, "error", err)
		}
		if err := idx.Index(ctx, req.Target); err != nil {
			logger.Warn("移动后重新索引新路径失败", "target", req.Target, "error", err)
		}
		ragDir := s.svc.DataDir()
		if ragDir != "" {
			dirs := loadIndexDirs(ragDir)
			var changed bool
			for i, d := range dirs {
				if d == req.Source || strings.HasPrefix(d, req.Source+string(filepath.Separator)) {
					dirs[i] = req.Target + strings.TrimPrefix(d, req.Source)
					changed = true
				}
			}
			if changed {
				if err := saveIndexDirs(ragDir, dirs); err != nil {
					logger.Warn("移动后更新 index_dirs 失败", "error", err)
				}
			}
		}
	}

	writeSuccess(w, map[string]string{"message": "已移动"})
}

// handleOpenFile 使用本地默认程序打开文件。
// POST /api/open-file
func (s *Server) handleOpenFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}

	cmd := exec.Command("open", req.Path)
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("打开文件失败: %v", err))
		return
	}

	writeSuccess(w, map[string]string{"message": "文件已通过本地程序打开"})
}

// defaultRagignoreContent .ragignore 默认内容
const defaultRagignoreContent = `# 敏感信息
.api_key

# 运行时锁文件
.lock

# 版本控制
.git/
.svn/
.hg/

# 数据库（体积大，不入版本控制）
vectors/
graphs/
meta.db
meta.db-wal
meta.db-shm

# 日志
logs/

# 依赖和构建产物
node_modules/
vendor/
dist*/
build/
target/
.next/
.turbo/
.cache/
__pycache__/
**.pyc

# 样式文件
*.less
*.css
*.scss
*.sass

# 备份和临时文件
.backup/
.DS_Store
*.swp
*.swo

*.vlog
*.sst
go.mod
go.sum
LICENSE
.editorconfig
.goreleaser.yml
.github/
.vscode/
.claude/
`

// handleCreateRagignore 在指定目录中创建 .ragignore 文件并写入默认内容。
// POST /api/ragignore
func (s *Server) handleCreateRagignore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Dir == "" {
		writeError(w, http.StatusBadRequest, "缺少 dir 参数")
		return
	}

	ragignorePath := filepath.Join(req.Dir, ".ragignore")
	if err := os.WriteFile(ragignorePath, []byte(defaultRagignoreContent), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("创建 .ragignore 失败: %v", err))
		return
	}

	writeSuccess(w, map[string]string{"message": ".ragignore 已创建", "path": ragignorePath})
}

// handleAppendRagignore 将指定路径追加到所在目录的 .ragignore 中。
// 若 .ragignore 不存在则自动创建。
// POST /api/ragignore-append
func (s *Server) handleAppendRagignore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}

	parentDir := filepath.Dir(req.Path)
	base := filepath.Base(req.Path)

	// 如果是目录，加 / 后缀以匹配 gitignore 惯例
	info, stErr := os.Stat(req.Path)
	if stErr == nil && info.IsDir() {
		base += "/"
	}

	ragignorePath := filepath.Join(parentDir, ".ragignore")

	var existing []byte
	if _, err := os.Stat(ragignorePath); os.IsNotExist(err) {
		// 文件不存在，创建新文件
		existing = []byte("# 排除规则\n" + base + "\n")
	} else {
		existing, err = os.ReadFile(ragignorePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("读取 .ragignore 失败: %v", err))
			return
		}
		// 检查是否已包含该规则
		lines := strings.Split(string(existing), "\n")
		alreadyExists := false
		for _, line := range lines {
			if strings.TrimSpace(line) == base {
				alreadyExists = true
				break
			}
		}
		if alreadyExists {
			writeSuccess(w, map[string]string{"message": "该路径已在排除规则中", "path": ragignorePath})
			return
		}
		// 追加
		if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
			existing = append(existing, '\n')
		}
		existing = append(existing, []byte(base+"\n")...)
	}

	if err := os.WriteFile(ragignorePath, existing, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("写入 .ragignore 失败: %v", err))
		return
	}

	writeSuccess(w, map[string]string{"message": "已添加到排除规则", "path": ragignorePath})
}
