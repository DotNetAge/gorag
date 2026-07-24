package webapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	gorag "github.com/DotNetAge/gorag/v2"
)

// ── Schema API 处理器 ──────────────────────────────────────────────

// handleSchemaList 返回嵌入的 schema 分类列表。
// GET /api/schema-list
func handleSchemaList(w http.ResponseWriter, r *http.Request) {
	categories, err := gorag.SchemaCategoryList()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("获取 schema 列表失败: %v", err))
		return
	}
	writeSuccess(w, categories)
}

// handleSchemaContent 返回指定 schema 的 JSON 内容。
// GET /api/schema-content?category=enterprise&name=Contract
func handleSchemaContent(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	name := r.URL.Query().Get("name")
	if category == "" || name == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 category 或 name")
		return
	}

	data, err := gorag.SchemaContent(category, name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("schema 未找到: %v", err))
		return
	}

	// 返回 schema 内容作为 JSON 对象
	var schemaObj interface{}
	if err := json.Unmarshal(data, &schemaObj); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("解析 schema JSON 失败: %v", err))
		return
	}
	writeSuccess(w, schemaObj)
}

// ── 目录 Schema 配置管理 ───────────────────────────────────────────

// dirSchemasRequest 保存/查询目录 schema 配置的请求体。
type dirSchemasRequest struct {
	Dir     string              `json:"dir"`     // 目录路径（空=全局默认）
	Schemas []gorag.SchemaEntry `json:"schemas"` // 选定的 schema 列表
}

// handleDirSchemas 获取或保存指定目录的 schema 配置。
// GET  /api/dir-schemas?dir=xxx  → 返回当前选定的 schema 列表
// POST /api/dir-schemas          → 保存选定的 schema 配置
func handleDirSchemas(w http.ResponseWriter, r *http.Request) {
	if globalSvc == nil {
		writeError(w, http.StatusBadRequest, "请先初始化 RAG 库")
		return
	}
	ragDir := globalSvc.DataDir()

	switch r.Method {
	case http.MethodGet:
		dir := r.URL.Query().Get("dir")
		entries, err := gorag.LoadDirSchemaNames(ragDir, dir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("读取目录 schema 配置失败: %v", err))
			return
		}
		if entries == nil {
			entries = []gorag.SchemaEntry{}
		}
		writeSuccess(w, entries)

	case http.MethodPost:
		var req dirSchemasRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
			return
		}
		if req.Schemas == nil {
			req.Schemas = []gorag.SchemaEntry{}
		}
		if err := gorag.SaveDirSchemas(ragDir, req.Dir, req.Schemas); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("保存目录 schema 配置失败: %v", err))
			return
		}
		writeSuccess(w, map[string]string{"message": "Schema 配置已保存"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 和 POST 方法")
	}
}

// ── 自定义 Schema 保存与加载 ───────────────────────────────────────

type schemaCustomRequest struct {
	Dir      string `json:"dir"`      // 目录路径（空=全局默认）
	Category string `json:"category"` // 分类名
	Name     string `json:"name"`     // schema 名称
	Schema   any    `json:"schema"`   // 完整的 schema JSON 对象
}

// handleSchemaCustom 获取或保存自定义 schema。
// GET  /api/schema-custom?category=xxx&name=yyy&dir=zzz  → 返回 schema JSON
// POST /api/schema-custom                                    → 保存自定义 schema
func handleSchemaCustom(w http.ResponseWriter, r *http.Request) {
	if globalSvc == nil {
		writeError(w, http.StatusBadRequest, "请先初始化 RAG 库")
		return
	}
	ragDir := globalSvc.DataDir()

	switch r.Method {
	case http.MethodGet:
		category := r.URL.Query().Get("category")
		name := r.URL.Query().Get("name")
		dir := r.URL.Query().Get("dir")
		if category == "" || name == "" {
			writeError(w, http.StatusBadRequest, "缺少参数 category 或 name")
			return
		}

		data, isEmbedded, err := gorag.LoadSchemaFromEither(ragDir, dir, category, name)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("schema 未找到: %v", err))
			return
		}

		var schemaObj interface{}
		if err := json.Unmarshal(data, &schemaObj); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("解析 schema JSON 失败: %v", err))
			return
		}

		writeSuccess(w, map[string]interface{}{
			"category":    category,
			"name":        name,
			"schema":      schemaObj,
			"is_embedded": isEmbedded,
		})

	case http.MethodPost:
		var req schemaCustomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("请求体解析失败: %v", err))
			return
		}
		if req.Category == "" || req.Name == "" || req.Schema == nil {
			writeError(w, http.StatusBadRequest, "缺少必填参数 category、name 或 schema")
			return
		}

		schemaJSON, err := json.Marshal(req.Schema)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("序列化 schema JSON 失败: %v", err))
			return
		}

		if err := gorag.SaveCustomSchema(ragDir, req.Dir, req.Category, req.Name, schemaJSON); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("保存自定义 schema 失败: %v", err))
			return
		}
		writeSuccess(w, map[string]string{"message": "Schema 已保存"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 和 POST 方法")
	}
}
