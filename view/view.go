// Package view 提供基于 Cypher 的结构化视图查询能力。
//
// 视图查询是「按 Schema 标签的结构化分页列表」语义：
//   - 不引入 SQL 引擎、不物化数据；
//   - 每次查询通过 Cypher 实时执行，零 LLM 成本；
//   - 适合人浏览分页列表（业务系统花名册式），也适合 Agent 精确过滤。
//
// 适用场景：
//   - AgentHarness 集成：提供 ListLabels / Describe / Query 三类结构化调用；
//   - UI 集成：把同标签节点集合以表格方式展示给用户。
//
// 与 mindstore/internal/service/view.go 的关系：
//   - mindstore 的实现带有 Schema 元数据补全（来自 embed 资源 / 目录级配置）；
//   - 本包提供无 Schema 依赖的基础实现，可由调用方注入 SchemaProvider 来补全。
package view

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
)

// ── 公共类型 ─────────────────────────────────────────────────

// Spec 定义一次结构化视图查询。
//
// 字段语义参考 mindstore/internal/service/view.go ViewSpec。
type Spec struct {
	Label     string   // 节点标签，如 "Person"、"Asset"
	Category  string   // 可选，强制指定 Schema 类别（"finance" / "general" 等）
	Fields    []string // 可选，列白名单；空 = 返回所有属性
	Where     *Where   // 可选，过滤条件；nil = 不过滤
	OrderBy   []Order  // 可选，排序；空 = 按节点 ID 稳定序
	Page      int      // 1-based；< 1 时按 1 处理
	Size      int      // ≤ 0 时按默认 20 处理；上限 200
	Path      string   // 可选，源文件路径前缀过滤
	WithTotal bool     // 是否统计 total；默认 true
}

// Where 过滤条件组合，所有子条件之间为 AND 关系。
type Where struct {
	Equals   map[string]any    // 等值匹配
	In       map[string][]any  // IN 匹配
	Range    map[string]Range  // 范围匹配
	Contains map[string]string // 字符串包含匹配
}

// Range 表示一个闭区间 [Gte, Lte]；任一端可省略。
type Range struct {
	Gte any `json:"gte,omitempty"`
	Lte any `json:"lte,omitempty"`
}

// Order 描述一个排序规则。
type Order struct {
	Field string // 列名（属性名）
	Desc  bool   // true = DESC，false = ASC
}

// Column 描述一个视图列的元数据。
type Column struct {
	Name        string `json:"name"`
	Type        string `json:"type"`        // 推断的 JSON 类型
	Description string `json:"description"` // 来自 Schema（若有）
	Required    bool   `json:"required"`
}

// Result 视图查询结果。
type Result struct {
	Label   string           `json:"label"`
	Columns []Column         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Total   int              `json:"total"`
	Page    int              `json:"page"`
	Size    int              `json:"size"`
}

// LabelInfo 描述一个可用标签的概览。
type LabelInfo struct {
	Label       string `json:"label"`
	Count       int    `json:"count"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// Schema 描述一个标签的完整列结构。
type Schema struct {
	Label       string   `json:"label"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Columns     []Column `json:"columns"`
}

// ── 错误码规约 ─────────────────────────────────────────────

// Error 携带机器可读的错误码。
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }

// 错误码规约
const (
	ErrInvalidParam       = 40001 // 参数缺失或非法
	ErrUnknownField       = 40002 // 字段名不在 Schema 中
	ErrTypeMismatch       = 40003 // 字段值类型不匹配
	ErrLabelNotFound      = 40401 // 标签不存在
	ErrUnsupportedIndexer = 50101 // 当前索引器不支持视图查询
	ErrGraphNotReady      = 50001 // 图存储未初始化
)

func newError(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// ── Cypher 注入接口 ────────────────────────────────────────

// CypherQuerier 是视图查询所依赖的最小接口。
// 任何实现此接口的对象都可以作为 ViewService 的后端。
// 典型实现：gorag/indexer.GraphIndexer.CypherQuery / HyperIndexer.CypherQuery。
type CypherQuerier interface {
	CypherQuery(ctx context.Context, q string, params map[string]any) ([]map[string]any, error)
}

// ── Service ───────────────────────────────────────────────

// Service 结构化视图查询服务。
//
// Service 通过 CypherQuerier 接口直接与底层图存储对话，不依赖 Schema；
// 调用方可通过 ServiceOption 注入 SchemaProvider / FieldValidator 等可选能力。
type Service struct {
	cypher    CypherQuerier
	schema    SchemaProvider // 可选；为 nil 时 Describe 走图内省
	validator FieldValidator // 可选；为 nil 时不校验字段
}

// ServiceOption 是 Service 的配置选项。
type ServiceOption func(*Service)

// WithSchemaProvider 注入 Schema 提供者，用于补全 Describe 的元数据。
type SchemaProvider interface {
	// Describe 返回标签的列定义。若标签无对应 Schema，返回 (nil, nil)。
	Describe(ctx context.Context, label, category string) ([]Column, string, string, error)
}

// WithValidator 注入字段名校验器（防御 Cypher 注入）。
type FieldValidator interface {
	ValidateField(label, field string) error
}

// WithSchemaProvider 设置 Schema 提供者。
func WithSchemaProvider(p SchemaProvider) ServiceOption {
	return func(s *Service) { s.schema = p }
}

// WithValidator 设置字段校验器。
func WithValidator(v FieldValidator) ServiceOption {
	return func(s *Service) { s.validator = v }
}

// New 构造一个 ViewService。
// cypher 不能为 nil（必须支持 CypherQuery）。
func New(cypher CypherQuerier, opts ...ServiceOption) (*Service, error) {
	if cypher == nil {
		return nil, newError(ErrInvalidParam, "CypherQuerier 不能为空")
	}
	s := &Service{cypher: cypher}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Query 执行一次结构化视图查询。
// 自动归一化 page ≥ 1、size ∈ [1, 200]。
func (s *Service) Query(ctx context.Context, spec Spec) (*Result, error) {
	if strings.TrimSpace(spec.Label) == "" {
		return nil, newError(ErrInvalidParam, "标签不能为空")
	}

	// 1. 确定返回列（无 Schema 时从图内省）
	columns, err := s.resolveColumns(ctx, spec.Label, spec.Category, spec.Fields)
	if err != nil {
		return nil, err
	}

	// 2. 归一化分页
	page, size := normalizePage(spec.Page, spec.Size)

	// 3. 翻译 Where → Cypher WHERE 片段与参数
	whereClause, whereParams, err := s.translateWhere(spec.Label, spec.Where)
	if err != nil {
		return nil, err
	}

	// 4. 计算 total
	total := 0
	if spec.WithTotal {
		totalCypher := fmt.Sprintf(
			`MATCH (n:%s) %s RETURN count(n) AS cnt`,
			spec.Label, whereClause,
		)
		totalRows, totalErr := s.cypher.CypherQuery(ctx, totalCypher, whereParams)
		if totalErr != nil {
			return nil, fmt.Errorf("统计视图总行数失败: %w", totalErr)
		}
		total = parseCountResult(totalRows)
	}

	// 5. 翻译 Order By
	orderClause := s.translateOrder(spec.Label, spec.OrderBy)
	if orderClause == "" {
		orderClause = "ORDER BY n.ID ASC"
	}

	// 6. 执行分页查询
	offset := (page - 1) * size
	listCypher := fmt.Sprintf(
		`MATCH (n:%s) %s RETURN n %s SKIP %d LIMIT %d`,
		spec.Label, whereClause, orderClause, offset, size,
	)
	rows, err := s.cypher.CypherQuery(ctx, listCypher, whereParams)
	if err != nil {
		return nil, fmt.Errorf("执行视图查询失败: %w", err)
	}

	// 7. 投影为行
	resultRows := projectRows(rows, columns)

	return &Result{
		Label:   spec.Label,
		Columns: columns,
		Rows:    resultRows,
		Total:   total,
		Page:    page,
		Size:    size,
	}, nil
}

// ListLabels 列出知识库中已存在的所有标签及其计数。
func (s *Service) ListLabels(ctx context.Context) ([]LabelInfo, error) {
	// 1. 收集所有不重复的标签
	distinctRows, err := s.cypher.CypherQuery(ctx,
		`MATCH (n) RETURN DISTINCT labels(n) AS labels`, nil)
	if err != nil {
		return nil, fmt.Errorf("枚举标签失败: %w", err)
	}

	labelSet := make(map[string]struct{})
	for _, row := range distinctRows {
		rawLabels, _ := row["labels"].([]any)
		for _, l := range rawLabels {
			if str, ok := l.(string); ok && str != "" {
				labelSet[str] = struct{}{}
			}
		}
	}

	// 2. 过滤内部标签（Region、文档根节点四类、__Node__）
	internal := map[string]struct{}{
		core.LabelRegion:   {},
		core.LabelDocument: {},
		core.LabelCode:     {},
		core.LabelImage:    {},
		core.LabelDataFile: {},
		"__Node__":         {},
	}
	results := make([]LabelInfo, 0, len(labelSet))
	for label := range labelSet {
		if _, skip := internal[label]; skip {
			continue
		}
		results = append(results, LabelInfo{Label: label})
	}

	// 3. 按标签名字典序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Label < results[j].Label
	})

	// 4. 为每个标签补全 count + Schema 元数据
	for i := range results {
		label := results[i].Label

		countRows, cErr := s.cypher.CypherQuery(ctx,
			fmt.Sprintf(`MATCH (n:%s) RETURN count(n) AS cnt`, label), nil)
		if cErr == nil {
			results[i].Count = parseCountResult(countRows)
		}

		// 尝试用 SchemaProvider 补全 category / description
		if s.schema != nil {
			_, cat, desc, sErr := s.schema.Describe(ctx, label, "")
			if sErr == nil {
				results[i].Category = cat
				results[i].Description = desc
			}
		}
	}

	return results, nil
}

// Describe 返回指定标签的列结构。
func (s *Service) Describe(ctx context.Context, label, category string) (*Schema, error) {
	if strings.TrimSpace(label) == "" {
		return nil, newError(ErrInvalidParam, "标签不能为空")
	}

	columns, cat, desc, err := s.resolveSchemaInfo(ctx, label, category)
	if err != nil {
		return nil, err
	}

	return &Schema{
		Label:       label,
		Category:    cat,
		Description: desc,
		Columns:     columns,
	}, nil
}
