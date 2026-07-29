package govector

import (
	"context"
	"fmt"
	"sort"

	"github.com/DotNetAge/gorag/v2/core"
	gvcore "github.com/DotNetAge/govector/core"
)

// chunkMetaKey 是 store 层内部使用的元数据键名，
// 用于在 Vector.Metadata 中嵌套存储 chunk 的序号等元信息（如 index）。
const chunkMetaKey = "chunk_meta"

// Store is an implementation of core.VectorStore using govector.
//
// 采用持久化连接模式：NewStore 时打开一次 bbolt 连接并创建 Collection，
// 所有操作共享同一连接和内存索引，不与 mrag serve 产生文件锁冲突。
type Store struct {
	colName    string
	dimension  int
	dbPath     string
	useHNSW    bool
	storage    *gvcore.Storage    // 持久化 bbolt 连接
	collection *gvcore.Collection // 持久化内存集合（含全部向量的内存索引）
}

// Option is a function that configures a Store.
type Option func(*Store)

// WithCollection sets the collection name.
//
// Parameters:
//   - name: The collection name
//
// Returns:
//   - Option: A configuration function
func WithCollection(name string) Option {
	return func(s *Store) {
		s.colName = name
	}
}

// WithDimension sets the vector dimension.
//
// Parameters:
//   - dim: The vector dimension
//
// Returns:
//   - Option: A configuration function
func WithDimension(dim int) Option {
	return func(s *Store) {
		s.dimension = dim
	}
}

// WithDBPath sets the path for the local bolt database.
//
// Parameters:
//   - path: The database path
//
// Returns:
//   - Option: A configuration function
func WithDBPath(path string) Option {
	return func(s *Store) {
		s.dbPath = path
	}
}

// WithHNSW enables or disables HNSW indexing.
func WithHNSW(use bool) Option {
	return func(s *Store) {
		s.useHNSW = use
	}
}

// DefaultStore returns a govector store configured for local testing.
// It creates a "gorag_vectors.db" file in the current directory and uses a dimension of 1536 (OpenAI default).
//
// Returns:
//   - core.VectorStore: The vector store
//   - error: Any error that occurred
func DefaultStore() (core.VectorStore, error) {
	return NewStore(
		WithDBPath("gorag_vectors.db"),
		WithDimension(1536),
		WithCollection("gorag"),
		WithHNSW(true),
	)
}

// NewStore initializes a new govector store.
//
// 采用持久化连接模式：打开一次 bbolt 连接并创建 Collection，持久持有。
//
// Parameters:
//   - opts: Configuration options
//
// Returns:
//   - core.VectorStore: The vector store
//   - error: Any error that occurred
func NewStore(opts ...Option) (core.VectorStore, error) {
	s := &Store{
		colName:   "gorag",
		dimension: 1536,
		dbPath:    "gorag_vectors.db",
		useHNSW:   true,
	}

	for _, opt := range opts {
		opt(s)
	}

	// 打开持久化 bbolt 连接
	storage, err := gvcore.NewStorageWithQuantization(s.dbPath, false, nil, false)
	if err != nil {
		return nil, fmt.Errorf("govector: 打开存储失败: %w", err)
	}

	// 创建持久化 Collection（加载全部向量到内存索引）
	col, err := gvcore.NewCollection(s.colName, s.dimension, gvcore.Cosine, storage, s.useHNSW)
	if err != nil {
		storage.Close()
		return nil, fmt.Errorf("govector: 打开集合失败: %w", err)
	}

	s.storage = storage
	s.collection = col
	return s, nil
}

// Upsert inserts or updates vectors in the store.
//
// Parameters:
//   - ctx: Context for cancellation
//   - vectors: The vectors to upsert
//
// Returns:
//   - error: Any error that occurred
func (s *Store) Upsert(ctx context.Context, vectors []*core.Vector) error {
	if len(vectors) == 0 {
		return nil
	}
	var points []gvcore.PointStruct
	for _, v := range vectors {
		payload := make(gvcore.Payload)
		for key, val := range v.Metadata {
			payload[key] = val
		}
		payload["chunk_id"] = v.ChunkID

		points = append(points, gvcore.PointStruct{
			ID:      v.ID,
			Vector:  v.Values,
			Payload: payload,
		})
	}

	if len(points) == 0 {
		return nil
	}

	return s.collection.Upsert(points)
}

// Search searches for vectors similar to the query vector.
//
// Parameters:
//   - ctx: Context for cancellation
//   - query: The query vector
//   - topK: The maximum number of results
//   - filters: Metadata filters
//
// Returns:
//   - []*core.Vector: The similar vectors
//   - []float32: The similarity scores
//   - error: Any error that occurred
func (s *Store) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	var gvFilter *gvcore.Filter

	if len(filters) > 0 {
		gvFilter = &gvcore.Filter{}
		for k, v := range filters {
			gvFilter.Must = append(gvFilter.Must, gvcore.Condition{
				Key:   k,
				Match: gvcore.MatchValue{Value: v},
			})
		}
	}

	if topK <= 0 {
		topK = 5
	}

	scoredPoints, err := s.collection.Search(query, gvFilter, topK)
	if err != nil {
		return nil, nil, err
	}

	var outVectors []*core.Vector
	var outScores []float32
	for _, pt := range scoredPoints {
		chunkID := ""
		if c, ok := pt.Payload["chunk_id"].(string); ok {
			chunkID = c
		}

		metadata := make(map[string]any)
		for k, v := range pt.Payload {
			if k != "chunk_id" {
				metadata[k] = v
			}
		}

		vec := &core.Vector{
			ID:       pt.ID,
			Values:   nil,
			ChunkID:  chunkID,
			Metadata: metadata,
		}
		outVectors = append(outVectors, vec)
		outScores = append(outScores, pt.Score)
	}

	return outVectors, outScores, nil
}

// Delete deletes a vector by ID or chunk_id.
//
// Parameters:
//   - ctx: Context for cancellation
//   - id: The vector ID (UUID format), or chunk_id (chunk_{docID}_{index}_{hash} format)
//
// Returns:
//   - error: Any error that occurred
func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}

	// 尝试按 vector UUID 删除
	deleted, err := s.collection.Delete([]string{id}, nil)
	if err != nil {
		return err
	}
	if deleted > 0 {
		return nil
	}

	// UUID 未命中，回退到按 chunk_id 元数据过滤删除
	filter := &gvcore.Filter{
		Must: []gvcore.Condition{{
			Key:   "chunk_id",
			Match: gvcore.MatchValue{Value: id},
		}},
	}
	deleted, err = s.collection.Delete(nil, filter)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return fmt.Errorf("vector with chunk_id %q not found", id)
	}
	return nil
}

// Clear removes all vectors from the store by dropping and recreating the collection.
func (s *Store) Clear(ctx context.Context) error {
	if err := s.storage.DropCollection(s.colName); err != nil {
		return fmt.Errorf("clear: drop collection failed: %w", err)
	}
	// 重新创建空的 Collection（替换旧的内存索引）
	col, err := gvcore.NewCollection(s.colName, s.dimension, gvcore.Cosine, s.storage, s.useHNSW)
	if err != nil {
		return fmt.Errorf("clear: recreate collection failed: %w", err)
	}
	s.collection = col
	return nil
}

// Count 返回向量总数（轻量操作，调用 col.Count()，复杂度 O(1)）。
func (s *Store) Count(ctx context.Context) (int, error) {
	return s.collection.Count(), nil
}

// Close 刷完写入缓冲并关闭持久化 bbolt 连接。
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns:
//   - error: Any error that occurred
func (s *Store) Close(ctx context.Context) error {
	return s.collection.Close()
}

// GetByDocID retrieves all vectors belonging to the same document by doc_id.
// Results are sorted by chunk_meta.index to enable document reconstruction.
//
// Parameters:
//   - ctx: Context for cancellation
//   - docID: The document ID to search for
//
// Returns:
//   - []*core.Vector: All vectors belonging to the document, sorted by chunk index
//   - error: Any error that occurred
func (s *Store) GetByDocID(ctx context.Context, docID string) ([]*core.Vector, error) {
	if docID == "" {
		return nil, fmt.Errorf("docID cannot be empty")
	}

	filter := &gvcore.Filter{
		Must: []gvcore.Condition{{
			Key:   "doc_id",
			Match: gvcore.MatchValue{Value: docID},
		}},
	}

	points, err := s.collection.GetPointsByFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get points by doc_id: %w", err)
	}

	vectors := make([]*core.Vector, 0, len(points))
	for _, pt := range points {
		chunkID := ""
		if c, ok := pt.Payload["chunk_id"].(string); ok {
			chunkID = c
		}

		metadata := make(map[string]any)
		for k, v := range pt.Payload {
			if k != "chunk_id" {
				metadata[k] = v
			}
		}

		vectors = append(vectors, &core.Vector{
			ID:       pt.ID,
			Values:   pt.Vector,
			ChunkID:  chunkID,
			Metadata: metadata,
		})
	}

	// Sort by chunk_meta.index for document reconstruction
	sort.Slice(vectors, func(i, j int) bool {
		return extractChunkIndex(vectors[i]) < extractChunkIndex(vectors[j])
	})

	return vectors, nil
}

// extractChunkIndex extracts the chunk index from a Vector's Metadata[chunkMetaKey].map["index"].
func extractChunkIndex(v *core.Vector) int {
	if v == nil || v.Metadata == nil {
		return 0
	}
	cm, ok := v.Metadata[chunkMetaKey].(map[string]any)
	if !ok {
		return 0
	}
	index, ok := cm["index"].(float64)
	if !ok {
		return 0
	}
	return int(index)
}

// List 分页获取向量，支持可选的元数据过滤条件。
// filters 为 nil 时返回全部，非 nil 时按条件过滤；多个 FilterCondition 之间为 AND 语义。
// 返回分页结果与过滤前总数。
func (s *Store) List(ctx context.Context, offset, limit int, filters []core.FilterCondition) ([]*core.Vector, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// 构建 govector 过滤器（filters 为 nil 或空时 GetPointsByFilter 返回全部）
	gvFilter := &gvcore.Filter{}
	for _, fc := range filters {
		cond := gvcore.Condition{
			Key:  fc.Key,
			Type: gvcore.ConditionType(fc.Type),
		}
		// JSON 数字以 float64 传入，但 protobuf 将 int 元数据存为 int64，直接比较会失败
		val := fc.Value
		if f64, ok := val.(float64); ok && f64 == float64(int64(f64)) {
			val = int64(f64)
		}
		switch fc.Type {
		case "exact":
			cond.Match = gvcore.MatchValue{Value: val}
		case "prefix":
			if s, ok := val.(string); ok {
				cond.Match = gvcore.MatchValue{Value: s}
			}
		}
		gvFilter.Must = append(gvFilter.Must, cond)
	}

	points, err := s.collection.GetPointsByFilter(gvFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list vectors: %w", err)
	}

	total := len(points)

	// 分页
	end := offset + limit
	if end > len(points) {
		end = len(points)
	}
	if offset >= len(points) {
		return []*core.Vector{}, total, nil
	}

	var vectors []*core.Vector
	for _, pt := range points[offset:end] {
		chunkID := ""
		if c, ok := pt.Payload["chunk_id"].(string); ok {
			chunkID = c
		}

		metadata := make(map[string]any)
		for k, v := range pt.Payload {
			if k != "chunk_id" {
				metadata[k] = v
			}
		}

		vectors = append(vectors, &core.Vector{
			ID:       pt.ID,
			Values:   pt.Vector,
			ChunkID:  chunkID,
			Metadata: metadata,
		})
	}

	return vectors, total, nil
}
