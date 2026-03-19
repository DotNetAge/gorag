package core

import (
	"context"

	chat "github.com/DotNetAge/gochat/pkg/core"
)

// State 贯穿整个管线的状态对象
type State struct {
	Ctx      context.Context `json:"-"`
	FilePath string          `json:"file_path,omitempty"`

	// 索引阶段字段
	Documents <-chan *Document `json:"-"`
	Chunks    <-chan *Chunk    `json:"-"`
	Vectors   []*Vector        `json:"vectors,omitempty"`
	Metadata  Metadata         `json:"metadata,omitempty"`

	// 检索/生成阶段字段
	OriginalQuery   string              `json:"original_query"`
	Query           *Query              `json:"query"`
	RetrievedChunks [][]*Chunk          `json:"retrieved_chunks"` // 支持多路召回与多阶段检索
	ParallelResults map[string][]*Chunk `json:"parallel_results"` // 命名的多路结果
	RerankScores    []float32           `json:"rerank_scores,omitempty"`
	Filters         map[string]any      `json:"filters,omitempty"`

	// 生成结果
	Answer           *Result `json:"answer"`
	GenerationPrompt string  `json:"generation_prompt,omitempty"`

	// Self-RAG / Agentic 字段
	Agentic       *AgenticState `json:"agentic,omitempty"`
	SelfRagScore  float32       `json:"self_rag_score,omitempty"`
	SelfRagReason string        `json:"self_rag_reason,omitempty"`

	// 指标
	TotalChunks int `json:"total_chunks,omitempty"`
}

// Metadata 文件元数据
type Metadata struct {
	Source   string `json:"source"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	ModTime  any    `json:"mod_time,omitempty"`
}

// AgenticState 代理状态
type AgenticState struct {
	NextStep             string         `json:"next_step,omitempty"`
	Intent               string         `json:"intent,omitempty"`
	History              []chat.Message `json:"history,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	SubQueries           []string       `json:"sub_queries,omitempty"`
	HypotheticalDocument string         `json:"hypothetical_document,omitempty"`
	HydeApplied          bool           `json:"hyde_applied,omitempty"`
	CacheHit             bool           `json:"cache_hit,omitempty"`
	EntityIDs            []string       `json:"entity_ids,omitempty"`
	StepBackQuery        string         `json:"step_back_query,omitempty"`
	Filters              map[string]any `json:"filters,omitempty"`
	Custom               map[string]any `json:"custom,omitempty"`
}

// Result 统一的结果对象
type Result struct {
	Answer string  `json:"answer"`
	Score  float32 `json:"score"`
}

// NewResult 创建结果
func NewResult(answer string, score float32) *Result {
	return &Result{Answer: answer, Score: score}
}

// DefaultState 创建默认状态
func DefaultState(ctx context.Context, filePath string) *State {
	return &State{
		Ctx:             ctx,
		FilePath:        filePath,
		Metadata:        Metadata{Source: filePath, FileName: filePath},
		RetrievedChunks: make([][]*Chunk, 0),
		ParallelResults: make(map[string][]*Chunk),
		Filters:         make(map[string]any),
		Agentic: &AgenticState{
			Metadata: make(map[string]any),
			Custom:   make(map[string]any),
		},
		Answer: &Result{},
	}
}

// NewAgenticState 初始化AgenticState
func NewAgenticState() *AgenticState {
	return &AgenticState{
		Metadata: make(map[string]any),
		Custom:   make(map[string]any),
	}
}
