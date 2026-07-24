package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

	if !s.isPathInAllowedDirs(req.Path) {
		writeError(w, http.StatusForbidden, "无权访问该路径")
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

	if !s.isPathInAllowedDirs(path) {
		writeError(w, http.StatusForbidden, "无权访问该路径")
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

	if !s.isPathInAllowedDirs(req.Path) {
		writeError(w, http.StatusForbidden, "无权访问该路径")
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

	if !s.isPathInAllowedDirs(req.Path) {
		writeError(w, http.StatusForbidden, "无权访问该路径")
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

	if !s.isPathInAllowedDirs(req.Source) || !s.isPathInAllowedDirs(req.Target) {
		writeError(w, http.StatusForbidden, "无权访问该路径")
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

// ── 路径访问控制 ──────────────────────────────────────────────────

// allowedDirs 构建允许访问的目录白名单（ragDir + 所有索引目录）。
// 返回 nil 表示白名单未初始化。
func (s *Server) allowedDirs() []string {
	if s.svc == nil {
		return nil
	}
	ragDir := s.svc.DataDir()
	if ragDir == "" {
		return nil
	}
	dirs := []string{ragDir}
	indexDirs := loadIndexDirs(ragDir)
	dirs = append(dirs, indexDirs...)
	return dirs
}

// isPathInAllowedDirs 检查路径是否在白名单目录范围内。
func (s *Server) isPathInAllowedDirs(targetPath string) bool {
	dirs := s.allowedDirs()
	if dirs == nil {
		// 白名单未初始化，允许通过（避免服务启动瞬间误拦截）
		return true
	}
	for _, dir := range dirs {
		if strings.HasPrefix(targetPath, dir+string(filepath.Separator)) || targetPath == dir {
			return true
		}
	}
	return false
}
