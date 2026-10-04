// Package native 提供极简的语义索引与查询层（朴素 RAG）。
//
// 设计定位：只做「语义索引 + 语义查询」一件事，不涉及文档解析、分块、图谱
// 等 GoRAG 完整流水线。适用于技能注册表、工具目录、配置项语义匹配等
// 「键值条目 + 语义检索」场景。
//
// 核心概念：
//   - 族（family）：索引命名空间，直接对应底层一个 Collection（bbolt bucket），
//     物理隔离，互不干扰。族隔离是正确性必需而非偏好——govector HNSW 检索
//     采用 post-filtering 策略（先取 topK*10 近邻再过滤），多族混存时其他族
//     会挤占过打名额导致目标族漏召回
//   - 条目：一次 Add 即一个条目。主值向量化为主体记录，meta 中的每个键值对
//     额外生成一条子键记录（多维命中，任一维度相似即可召回该条目）
//   - schema：每个族的子键名集合以一条特殊记录存储（ID = __schema__），
//     供定向子键搜索使用
//
// 存储形态：一个数据库文件承载全部族，共享一个 bbolt 连接（无锁冲突）；
// 族 = Collection = bucket。底层直接基于 govector，不经由 core.VectorStore
// 抽象——本包定位本地极简用法，多路搜索后按条目聚合取最高分。
//
// 幂等与覆盖语义：
//   - 同族同值重复 Add 视为更新索引（主键 = 族:sha256(值)，同键覆盖）
//   - 不同条目若某子键值完全相同，视为同一个东西，后写覆盖
package native

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/DotNetAge/gorag/v2/core"
	gvcore "github.com/DotNetAge/govector/core"
)

// payload 键名常量（业务记录与 schema 记录共用同一命名空间，统一加前缀防冲突）。
const (
	pKind    = "seg_kind"    // 记录类型：schema 记录为 "schema"，业务记录不带此键
	pField   = "seg_field"   // 子键字段名（主值记录不带此键）
	pValue   = "seg_value"   // 主值（子键记录冗余存储所属条目的主值）
	pMeta    = "seg_meta"    // 用户元数据 JSON（子键记录冗余存储所属条目的元数据）
	pSubkeys = "seg_subkeys" // schema 记录专用：该族的子键名列表（逗号分隔）
)

// 常量定义。
const (
	kindSchema       = "schema"     // schema 记录的类型值
	schemaID         = "__schema__" // schema 记录的固定 ID（每族一条）
	searchOversample = 4            // 搜索过采系数：结果按条目去重后截取 topK
)

// SegHit 语义索引命中：一条 KV 条目 + 相似度分数。
//
// 不复用 core.Hit——那是为 Chunks/Nodes/Edges 三类命中与 RRF 融合设计的
// 重容器；本包命中即「一条记录 + 分数」，专用结构极简到底。
type SegHit struct {
	Key   string            // 条目主键：族:sha256(主值)
	Field string            // 命中来源字段名（"" 表示主值本身命中）
	Value string            // 条目主值
	Meta  map[string]string // 条目元数据（无论命中主值还是子键，均返回完整元数据）
	Score float32           // 相似度分数（Cosine，越大越相似）
}

// SegIndexer 极简语义索引器：族 = Collection，一库多族共享一个存储连接。
type SegIndexer struct {
	dbPath   string
	embedder core.Embedder
	dim      int

	mu      sync.Mutex
	storage *gvcore.Storage
	cols    map[string]*gvcore.Collection // 族名 → Collection（惰性创建）
}

// NewSegIndexer 创建语义索引器。
//
// 参数：
//   - dbPath: 数据库文件路径，必须为绝对路径（索引系统禁止相对路径）
//   - embedder: 向量计算器，所有族共用（维度统一）
func NewSegIndexer(dbPath string, embedder core.Embedder) (*SegIndexer, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("native.NewSegIndexer: 数据库路径不能为空")
	}
	if !filepath.IsAbs(dbPath) {
		return nil, fmt.Errorf("native.NewSegIndexer: 数据库路径必须为绝对路径，收到 %q", dbPath)
	}
	if embedder == nil {
		return nil, fmt.Errorf("native.NewSegIndexer: embedder 不能为空")
	}
	if embedder.Dim() <= 0 {
		return nil, fmt.Errorf("native.NewSegIndexer: embedder 维度非法: %d", embedder.Dim())
	}

	storage, err := gvcore.NewStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("native.NewSegIndexer: 打开存储失败: %w", err)
	}
	return &SegIndexer{
		dbPath:   dbPath,
		embedder: embedder,
		dim:      embedder.Dim(),
		storage:  storage,
		cols:     make(map[string]*gvcore.Collection),
	}, nil
}

// Add 向指定族添加（或更新）一个条目。
//
// 流程：
//  1. 主值向量化，写主记录（ID = 族:sha256(主值)）
//  2. meta 每个键值对向量化，写子键记录（ID = 族:键:sha256(键值)），
//     并冗余主值与元数据，命中即返回、无需二次查询
//  3. 合并既有 schema，重写 schema 记录（幂等）
//
// 同值重复 Add 即覆盖更新；写入后立即 Flush 防止进程异常退出丢数据。
func (s *SegIndexer) Add(ctx context.Context, family, value string, meta map[string]string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("native.Add: %w", err)
	}
	if err := validFamily(family); err != nil {
		return fmt.Errorf("native.Add: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("native.Add: 值不能为空")
	}

	col, err := s.collection(family)
	if err != nil {
		return err
	}

	// 主值向量
	mainVec, err := s.embedder.CalcText(value)
	if err != nil {
		return fmt.Errorf("native.Add: 主值向量化失败: %w", err)
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("native.Add: 元数据序列化失败: %w", err)
	}

	points := make([]gvcore.PointStruct, 0, len(meta)+2)
	points = append(points, gvcore.PointStruct{
		ID:      entryKey(family, value),
		Vector:  mainVec.Values,
		Payload: gvcore.Payload{pValue: value, pMeta: string(metaJSON)},
	})

	// 子键记录：冗余主值与元数据，保证子键命中时直接取回完整条目
	for k, v := range meta {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("native.Add: 子键 %q 处理前取消: %w", k, err)
		}
		subVec, err := s.embedder.CalcText(v)
		if err != nil {
			return fmt.Errorf("native.Add: 子键 %q 向量化失败: %w", k, err)
		}
		points = append(points, gvcore.PointStruct{
			ID:      family + ":" + k + ":" + shaHex(v),
			Vector:  subVec.Values,
			Payload: gvcore.Payload{pField: k, pValue: value, pMeta: string(metaJSON)},
		})
	}

	// schema 记录：合并既有子键名，去重排序后重写
	subkeys := s.loadSubkeys(col)
	for k := range meta {
		subkeys = append(subkeys, k)
	}
	points = append(points, s.schemaPoint(dedupeSort(subkeys)))

	if err := col.Upsert(points); err != nil {
		return fmt.Errorf("native.Add: 写入失败: %w", err)
	}
	if err := col.Flush(); err != nil {
		return fmt.Errorf("native.Add: 刷盘失败: %w", err)
	}
	return nil
}

// Search 在指定键下执行语义检索。
//
// key 支持两种形式：
//   - "skill"：在族 skill 的主值与全部子键上检索，结果按条目聚合取最高分
//   - "skill:title"：仅在子键 title 上定向检索
//
// 返回按相似度降序排列、按条目去重后的 topK 条命中。
func (s *SegIndexer) Search(ctx context.Context, key, query string, topK int) ([]SegHit, error) {
	family, field, err := splitKey(key)
	if err != nil {
		return nil, fmt.Errorf("native.Search: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return nil, fmt.Errorf("native.Search: %w", err)
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("native.Search: 查询值不能为空")
	}
	if topK <= 0 {
		topK = 1
	}

	col, err := s.collection(family)
	if err != nil {
		return nil, err
	}
	if col.Count() == 0 {
		return []SegHit{}, nil
	}

	qvec, err := s.embedder.CalcText(query)
	if err != nil {
		return nil, fmt.Errorf("native.Search: 查询向量化失败: %w", err)
	}

	// schema 记录不参与检索，一律排除
	filter := &gvcore.Filter{
		MustNot: []gvcore.Condition{{
			Key:   pKind,
			Type:  gvcore.MatchTypeExact,
			Match: gvcore.MatchValue{Value: kindSchema},
		}},
	}
	// 定向子键：前置过滤只看该字段的记录
	if field != "" {
		filter.Must = []gvcore.Condition{{
			Key:   pField,
			Type:  gvcore.MatchTypeExact,
			Match: gvcore.MatchValue{Value: field},
		}}
	}

	// 过采后按条目去重，避免同一条目的多条记录挤占 topK
	fetch := topK * searchOversample
	if total := col.Count(); fetch > total {
		fetch = total
	}
	scored, err := col.Search(qvec.Values, filter, fetch)
	if err != nil {
		return nil, fmt.Errorf("native.Search: 检索失败: %w", err)
	}

	return aggregate(family, scored, topK), nil
}

// List 列出指定族的条目（仅主记录视图，不携带命中来源与分数）。
//
// 返回按主键去重后的条目列表，无排序保证；limit 截断结果（<=0 取 100）。
func (s *SegIndexer) List(ctx context.Context, family string, limit int) ([]SegHit, error) {
	if err := validFamily(family); err != nil {
		return nil, fmt.Errorf("native.List: %w", err)
	}
	if limit <= 0 {
		limit = 100
	}

	col, err := s.collection(family)
	if err != nil {
		return nil, err
	}

	// 排除 schema 记录后遍历全部业务记录，主记录 = 无子键字段名的记录
	pts, err := col.GetPointsByFilter(&gvcore.Filter{
		MustNot: []gvcore.Condition{{
			Key:   pKind,
			Type:  gvcore.MatchTypeExact,
			Match: gvcore.MatchValue{Value: kindSchema},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("native.List: 遍历失败: %w", err)
	}

	seen := make(map[string]struct{}, len(pts))
	hits := make([]SegHit, 0, len(pts))
	for _, pt := range pts {
		if _, hasField := pt.Payload[pField]; hasField {
			continue // 子键记录，仅列主记录
		}
		value, _ := pt.Payload[pValue].(string)
		if value == "" {
			continue
		}
		key := entryKey(family, value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hits = append(hits, SegHit{
			Key:   key,
			Value: value,
			Meta:  parseMeta(pt.Payload[pMeta]),
		})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

// Close 刷盘并关闭底层存储。Close 后索引器不可再用。
//
// 任一族 Flush 失败不中断流程：storage.Close 必然执行以释放 bbolt
// 连接与文件锁（否则长驻进程在 Flush 失败后将无法重开同库），
// 全部错误经 errors.Join 合并返回。
func (s *SegIndexer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storage == nil {
		return nil
	}
	var errs []error
	for name, col := range s.cols {
		if err := col.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("native.Close: 刷盘族 %q 失败: %w", name, err))
		}
	}
	if err := s.storage.Close(); err != nil {
		errs = append(errs, fmt.Errorf("native.Close: 关闭存储失败: %w", err))
	}
	s.storage = nil
	s.cols = make(map[string]*gvcore.Collection)
	return errors.Join(errs...)
}

// collection 取指定族的 Collection，不存在则惰性创建。
// 每族独立 Collection（bbolt bucket），共享同一存储连接。
func (s *SegIndexer) collection(family string) (*gvcore.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.storage == nil {
		return nil, fmt.Errorf("native: 索引器已关闭")
	}
	if col, ok := s.cols[family]; ok {
		return col, nil
	}
	col, err := gvcore.NewCollection(family, s.dim, gvcore.Cosine, s.storage, true)
	if err != nil {
		return nil, fmt.Errorf("native: 创建族 %q 失败: %w", family, err)
	}
	s.cols[family] = col
	return col, nil
}

// loadSubkeys 读取族 schema 记录中的子键名列表。
// 无 schema 记录时返回空切片（首条 Add 前的正常状态）。
func (s *SegIndexer) loadSubkeys(col *gvcore.Collection) []string {
	pts, err := col.GetPointsByFilter(&gvcore.Filter{
		Must: []gvcore.Condition{{
			Key:   pKind,
			Type:  gvcore.MatchTypeExact,
			Match: gvcore.MatchValue{Value: kindSchema},
		}},
	})
	if err != nil || len(pts) == 0 {
		return nil
	}
	raw, _ := pts[0].Payload[pSubkeys].(string)
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// schemaPoint 构造族 schema 记录。向量取单位占位向量：
// 该记录仅按 ID 与 payload 读写，不参与相似度检索（检索时被 MustNot 排除），
// 取单位向量避免零向量在 Cosine 距离计算中产生除零。
func (s *SegIndexer) schemaPoint(subkeys []string) gvcore.PointStruct {
	vec := make([]float32, s.dim)
	vec[0] = 1
	return gvcore.PointStruct{
		ID:      schemaID,
		Vector:  vec,
		Payload: gvcore.Payload{pKind: kindSchema, pSubkeys: strings.Join(subkeys, ",")},
	}
}

// aggregate 将检索结果按条目去重聚合：同一条目的主记录与子键记录
// 合并为一条命中，取最高分，按分数降序截取 topK。
func aggregate(family string, scored []gvcore.ScoredPoint, topK int) []SegHit {
	type best struct {
		score float32
		pt    gvcore.ScoredPoint
	}
	m := make(map[string]best, len(scored))
	for _, pt := range scored {
		value, _ := pt.Payload[pValue].(string)
		if value == "" {
			continue // 无主值的异常记录，跳过
		}
		key := entryKey(family, value)
		if cur, ok := m[key]; !ok || pt.Score > cur.score {
			m[key] = best{pt.Score, pt}
		}
	}

	hits := make([]SegHit, 0, len(m))
	for key, cur := range m {
		field, _ := cur.pt.Payload[pField].(string)
		value, _ := cur.pt.Payload[pValue].(string)
		hits = append(hits, SegHit{
			Key:   key,
			Field: field,
			Value: value,
			Meta:  parseMeta(cur.pt.Payload[pMeta]),
			Score: cur.score,
		})
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

// parseMeta 解析元数据 JSON。空值或 "null"（Add 未传 meta）返回 nil。
func parseMeta(raw any) map[string]string {
	str, ok := raw.(string)
	if !ok || str == "" {
		return nil
	}
	var meta map[string]string
	if err := json.Unmarshal([]byte(str), &meta); err != nil {
		return nil
	}
	return meta
}

// entryKey 生成条目主键：族:sha256(主值)。
func entryKey(family, value string) string {
	return family + ":" + shaHex(value)
}

// shaHex 计算字符串的 SHA-256 十六进制摘要，用作记录 ID 的去重因子。
func shaHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// splitKey 拆分检索键："skill" → (skill, "")；"skill:title" → (skill, title)。
// 族名不允许包含冒号（冒号是键语法保留字符）。
func splitKey(key string) (family, field string, err error) {
	if strings.TrimSpace(key) == "" {
		return "", "", fmt.Errorf("键不能为空")
	}
	family, field, ok := strings.Cut(key, ":")
	if err := validFamily(family); err != nil {
		return "", "", err
	}
	if !ok {
		return family, "", nil
	}
	if strings.TrimSpace(field) == "" {
		return "", "", fmt.Errorf("键 %q 的子键名为空", key)
	}
	return family, field, nil
}

// validFamily 校验族名：非空且不含冒号。
func validFamily(family string) error {
	if strings.TrimSpace(family) == "" {
		return fmt.Errorf("族名不能为空")
	}
	if strings.Contains(family, ":") {
		return fmt.Errorf("族名 %q 不能包含冒号（冒号为键语法保留字符）", family)
	}
	return nil
}

// dedupeSort 子键名去重并排序，保证 schema 记录内容稳定。
func dedupeSort(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
