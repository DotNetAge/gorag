package view

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// normalizePage 归一化分页参数：page ≥ 1, size ∈ [1, 200]。
func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

// parseCountResult 解析 Cypher count(*) 查询的单行结果。
func parseCountResult(rows []map[string]any) int {
	if len(rows) == 0 {
		return 0
	}
	row := rows[0]
	v, ok := row["cnt"]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return int(x)
	case int:
		return x
	case int32:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

// isCypherIdentifier 校验字符串是否为合法的 Cypher 标识符（防御注入）。
// 只允许字母、数字、下划线，且首字符为字母或下划线。
func isCypherIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(i > 0 && c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// translateWhere 将 Where 翻译为 Cypher WHERE 片段与命名参数。
// 多个子句之间使用 AND 连接。
func (s *Service) translateWhere(label string, w *Where) (string, map[string]any, error) {
	if w == nil {
		return "", nil, nil
	}
	var parts []string
	params := make(map[string]any)
	paramIndex := 0

	// 1. Equals
	for field, val := range w.Equals {
		if err := s.validateField(label, field); err != nil {
			return "", nil, err
		}
		pName := fmt.Sprintf("p%d", paramIndex)
		paramIndex++
		parts = append(parts, fmt.Sprintf("n.%s = $%s", field, pName))
		params[pName] = val
	}

	// 2. IN
	for field, list := range w.In {
		if err := s.validateField(label, field); err != nil {
			return "", nil, err
		}
		if len(list) == 0 {
			continue
		}
		pName := fmt.Sprintf("p%d", paramIndex)
		paramIndex++
		parts = append(parts, fmt.Sprintf("n.%s IN $%s", field, pName))
		params[pName] = list
	}

	// 3. Range
	for field, rng := range w.Range {
		if err := s.validateField(label, field); err != nil {
			return "", nil, err
		}
		if rng.Gte != nil {
			pName := fmt.Sprintf("p%d", paramIndex)
			paramIndex++
			parts = append(parts, fmt.Sprintf("n.%s >= $%s", field, pName))
			params[pName] = rng.Gte
		}
		if rng.Lte != nil {
			pName := fmt.Sprintf("p%d", paramIndex)
			paramIndex++
			parts = append(parts, fmt.Sprintf("n.%s <= $%s", field, pName))
			params[pName] = rng.Lte
		}
	}

	// 4. Contains
	for field, sub := range w.Contains {
		if err := s.validateField(label, field); err != nil {
			return "", nil, err
		}
		pName := fmt.Sprintf("p%d", paramIndex)
		paramIndex++
		parts = append(parts, fmt.Sprintf("n.%s CONTAINS $%s", field, pName))
		params[pName] = sub
	}

	if len(parts) == 0 {
		return "", nil, nil
	}
	return "WHERE " + strings.Join(parts, " AND "), params, nil
}

// translateOrder 将 Order 列表翻译为 Cypher ORDER BY 片段。
func (s *Service) translateOrder(label string, orders []Order) string {
	if len(orders) == 0 {
		return ""
	}
	parts := make([]string, 0, len(orders))
	for _, o := range orders {
		if err := s.validateField(label, o.Field); err != nil {
			continue
		}
		if o.Desc {
			parts = append(parts, fmt.Sprintf("n.%s DESC", o.Field))
		} else {
			parts = append(parts, fmt.Sprintf("n.%s ASC", o.Field))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "ORDER BY " + strings.Join(parts, ", ")
}

// validateField 校验字段名合法性。
func (s *Service) validateField(label, field string) error {
	if !isCypherIdentifier(field) {
		return newError(ErrUnknownField, "非法字段名: %q", field)
	}
	if s.validator != nil {
		if err := s.validator.ValidateField(label, field); err != nil {
			return err
		}
	}
	return nil
}

// projectRows 把 Cypher 返回的 n 行投影为业务字段行。
// 处理 n 字段是 map[string]any 的常见情况。
func projectRows(rawRows []map[string]any, columns []Column) []map[string]any {
	out := make([]map[string]any, 0, len(rawRows))
	for _, row := range rawRows {
		node, _ := row["n"].(map[string]any)
		if node == nil {
			// 兜底：直接用整行
			out = append(out, row)
			continue
		}
		flat := make(map[string]any, len(node))
		for k, v := range node {
			flat[k] = v
		}
		// 字段白名单：只保留 columns 中声明的字段
		if len(columns) > 0 {
			filtered := make(map[string]any, len(columns))
			for _, c := range columns {
				if v, ok := flat[c.Name]; ok {
					filtered[c.Name] = v
				}
			}
			out = append(out, filtered)
		} else {
			out = append(out, flat)
		}
	}
	return out
}

// applyFieldWhitelist 应用字段白名单。
func applyFieldWhitelist(columns []Column, fields []string) ([]Column, error) {
	if len(fields) == 0 {
		return columns, nil
	}
	colSet := make(map[string]Column, len(columns))
	for _, c := range columns {
		colSet[c.Name] = c
	}
	out := make([]Column, 0, len(fields))
	for _, f := range fields {
		c, ok := colSet[f]
		if !ok {
			return nil, newError(ErrUnknownField, "字段 %q 不在 Schema 中", f)
		}
		out = append(out, c)
	}
	return out, nil
}

// ── 列解析（无 Schema 时走图内省）──────────────────────────

// resolveColumns 解析 Spec 返回的列定义。
// 优先级：SchemaProvider > 图内省（采样节点属性）。
// 应用字段白名单。
func (s *Service) resolveColumns(ctx context.Context, label, category string, fields []string) ([]Column, error) {
	columns, _, _, err := s.resolveSchemaInfo(ctx, label, category)
	if err != nil {
		return nil, err
	}
	return applyFieldWhitelist(columns, fields)
}

// resolveSchemaInfo 解析标签的完整 Schema 信息。
func (s *Service) resolveSchemaInfo(ctx context.Context, label, category string) ([]Column, string, string, error) {
	// 1. 优先用 SchemaProvider
	if s.schema != nil {
		columns, cat, desc, err := s.schema.Describe(ctx, label, category)
		if err == nil {
			return columns, cat, desc, nil
		}
	}

	// 2. 回退到图内省：采样一个节点，列出所有属性键
	columns, err := s.introspectColumns(ctx, label)
	if err != nil {
		return nil, "", "", err
	}
	return columns, "", "", nil
}

// introspectColumns 通过 Cypher 采样节点属性，构造最简列定义。
func (s *Service) introspectColumns(ctx context.Context, label string) ([]Column, error) {
	rows, err := s.cypher.CypherQuery(ctx,
		fmt.Sprintf(`MATCH (n:%s) RETURN n LIMIT 1`, label), nil)
	if err != nil {
		return nil, fmt.Errorf("采样节点失败: %w", err)
	}
	if len(rows) == 0 {
		// 没有数据：返回空列集（调用方拿到空 rows 即可）
		return []Column{}, nil
	}

	node, _ := rows[0]["n"].(map[string]any)
	if node == nil {
		return []Column{}, nil
	}

	// 收集所有属性键（按字典序稳定排序）
	keys := make([]string, 0, len(node))
	for k := range node {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	columns := make([]Column, 0, len(keys))
	for _, k := range keys {
		v := node[k]
		columns = append(columns, Column{
			Name: k,
			Type: inferType(v),
		})
	}
	return columns, nil
}

// inferType 推断 JSON 值类型。
func inferType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "string"
}
