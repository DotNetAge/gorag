package core

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// IndexingContext 专门用于文档索引管线的上下文
type IndexingContext struct {
	Ctx      context.Context `json:"-"`
	FilePath string          `json:"file_path,omitempty"`
	Metadata Metadata        `json:"metadata,omitempty"`

	// 可观测性
	Tracer observability.Tracer `json:"-"`
	Span   observability.Span   `json:"-"`

	// 流式处理通道
	Documents <-chan *Document `json:"-"`
	Chunks    <-chan *Chunk    `json:"-"`

	// 产出产物
	Vectors []*Vector `json:"vectors,omitempty"`
	Nodes   []*Node   `json:"nodes,omitempty"`
	Edges   []*Edge   `json:"edges,omitempty"`

	// 统计与指标
	TotalChunks int            `json:"total_chunks,omitempty"`
	Custom      map[string]any `json:"custom,omitempty"`
}

// NewIndexingContext 创建索引上下文的标准方式
func NewIndexingContext(ctx context.Context, filePath string) *IndexingContext {
	return &IndexingContext{
		Ctx:      ctx,
		FilePath: filePath,
		Metadata: Metadata{Source: filePath, FileName: filePath},
		Tracer:   observability.DefaultNoopTracer(),
		Custom:   make(map[string]any),
	}
}

// RetrievalContext 专门用于检索与生成管线的上下文
type RetrievalContext struct {
	Ctx context.Context `json:"-"`

	// 输入查询
	OriginalQuery string `json:"original_query"`
	Query         *Query `json:"query"` // 当前正在处理的查询（可能是重写后的）

	// 可观测性
	Tracer observability.Tracer `json:"-"`
	Span   observability.Span   `json:"-"`

	// 检索中间产物
	RetrievedChunks [][]*Chunk          `json:"retrieved_chunks"` // 支持多路召回
	ParallelResults map[string][]*Chunk `json:"parallel_results"` // 命名的中间结果
	RerankScores    []float32           `json:"rerank_scores,omitempty"`
	Filters         map[string]any      `json:"filters,omitempty"`

	// 代理与高级 RAG 状态
	Agentic *AgenticContext `json:"agentic,omitempty"`

	// 最终产出
	Answer *Result `json:"answer"`

	// 扩展字段
	Custom map[string]any `json:"custom,omitempty"`

	// 指标
	Metrics map[string]any `json:"metrics,omitempty"`
}

// NewRetrievalContext 创建检索上下文的标准方式
func NewRetrievalContext(ctx context.Context, queryText string) *RetrievalContext {
	return &RetrievalContext{
		Ctx:             ctx,
		OriginalQuery:   queryText,
		Query:           &Query{Text: queryText},
		Tracer:          observability.DefaultNoopTracer(),
		RetrievedChunks: make([][]*Chunk, 0),
		ParallelResults: make(map[string][]*Chunk),
		Filters:         make(map[string]any),
		Agentic:         NewAgenticState(),
		Answer:          &Result{},
		Custom:          make(map[string]any),
		Metrics:         make(map[string]any),
	}
}

// AgenticContext 存储代理推理、HyDE 等高级过程的中间状态
type AgenticContext struct {
	NextStep             string         `json:"next_step,omitempty"`
	Intent               string         `json:"intent,omitempty"`
	History              []chat.Message `json:"history,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	SubQueries           []string       `json:"sub_queries,omitempty"`
	HypotheticalDocument string         `json:"hypothetical_document,omitempty"`
	HydeApplied          bool           `json:"hyde_applied,omitempty"`
	StepBackQuery        string         `json:"step_back_query,omitempty"`
	CacheHit             bool           `json:"cache_hit,omitempty"`
	Filters              map[string]any `json:"filters,omitempty"`
	Custom               map[string]any `json:"custom,omitempty"`
}

// NewAgenticState 创建新的代理状态
func NewAgenticState() *AgenticContext {
	return &AgenticContext{
		Metadata:   make(map[string]any),
		SubQueries: make([]string, 0),
		Custom:     make(map[string]any),
	}
}

// Metadata 文件元数据
type Metadata struct {
	Source   string `json:"source"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	ModTime  any    `json:"mod_time,omitempty"`
}

// Result 统一的结果对象
type Result struct {
	Answer string  `json:"answer"`
	Score  float32 `json:"score"`
}
