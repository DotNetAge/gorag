package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
)

// =====================================================================
// Summarizer：LLM 摘要/标题增强
// =====================================================================

// Summarizer 对文本型 Chunk 的 Title 和 Summary 进行 LLM 增强。
//
// 设计要点：
//   - 输入为 []core.Chunk，输出为增强后的 []core.Chunk（原地修改并返回）
//   - 每个 Chunk 独立调用 LLM，便于控制上下文和成本
//   - 仅当 LLM 返回合法 title/summary 时才覆盖原值
//   - 失败时记录日志并保留原 Chunk，不中断整个流程
type Summarizer interface {
	Summarize(ctx context.Context, chunks []core.Chunk) ([]core.Chunk, error)
}

// gochatSummarizer 基于 gochat 的 Summarizer 默认实现。
type gochatSummarizer struct {
	config        Config
	client        chat.Client
	logger        logging.Logger
	usageRecorder UsageRecorder
}

// NewSummarizer 创建基于 gochat 的 Summarizer。
//
// 必传参数：
//   - cfg: LLM 配置（APIKey/BaseURL/Model 必填）
//   - logger: 日志实例（禁止为 nil）
func NewSummarizer(cfg Config, logger logging.Logger) (Summarizer, error) {
	if logger == nil {
		return nil, fmt.Errorf("llm.NewSummarizer: logger 不能为空")
	}
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("llm.NewSummarizer: %w", err)
	}

	client, err := newChatClient(cfg)
	if err != nil {
		return nil, err
	}
	return &gochatSummarizer{
		config: cfg,
		client: client,
		logger: logger,
	}, nil
}

// SetUsageRecorder 设置 token 用量记录回调。
// 在 Summarizer 每次成功调用 LLM 后自动记录用量信息。
func (s *gochatSummarizer) SetUsageRecorder(r UsageRecorder) {
	s.usageRecorder = r
}

// Summarize 调用 LLM 为每个 Chunk 生成更语义化的 Title、Summary 和 Tags。
func (s *gochatSummarizer) Summarize(ctx context.Context, chunks []core.Chunk) ([]core.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	for i := range chunks {
		if strings.TrimSpace(chunks[i].Content) == "" {
			continue
		}
		title, summary, tags, err := s.summarizeOne(ctx, chunks[i].Content)
		if err != nil {
			s.logger.Warn("Summarizer 处理分块失败",
				"chunkID", chunks[i].ID,
				"error", err,
			)
			continue
		}
		if title != "" {
			chunks[i].Title = title
		}
		if summary != "" {
			chunks[i].Summary = summary
		}
		if len(tags) > 0 {
			chunks[i].Tags = tags
		}
	}
	return chunks, nil
}

// summarizeResult 是 LLM 返回的 JSON 结构。
type summarizeResult struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// summarizeOne 对单段内容调用 LLM，返回 (title, summary, tags, error)。
func (s *gochatSummarizer) summarizeOne(ctx context.Context, content string) (string, string, []string, error) {
	resp, err := timedChat(ctx, s.client, []chat.Message{
		chat.NewSystemMessage(s.buildSystemPrompt()),
		chat.NewUserMessage(content),
	}, s.logger, "Summarizer(单条)", s.usageRecorder)
	if err != nil {
		return "", "", nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	res, err := parseSummarizeResult(resp.Content)
	if err != nil {
		return "", "", nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	return res.Title, res.Summary, res.Tags, nil
}

// buildSystemPrompt 构建 Summarizer 的系统提示词。
func (s *gochatSummarizer) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是一名精准的文档摘要助手。\n")
	sb.WriteString("请对给定文本片段生成严格的 JSON，仅包含三个字段：\n")
	sb.WriteString("- \"title\": 简洁、利于搜索的标题（最多 10 个词）\n")
	sb.WriteString("- \"summary\": 准确概括关键语义（禁止使用一切标记，包括 Markdown、XML、HTML、Emoji 等）\n")
	sb.WriteString("- \"tags\": 3-5 个关键词标签，用于分类和检索（字符串数组）\n\n")
	sb.WriteString("规则：\n")
	sb.WriteString("- JSON 输出必须使用英文标点，禁止出现中文引号、中文逗号或中文冒号。\n")
	sb.WriteString("- 标题、摘要和标签使用 ")
	sb.WriteString(s.config.Language)
	sb.WriteString(" 表达。\n")
	sb.WriteString("- 标签应简洁，避免与标题重复。\n")
	sb.WriteString("- 如果内容很短，标题和摘要也应简短但有意义。\n")
	sb.WriteString("- 如果输入是代码，标题优先使用函数/类名，摘要描述其行为。\n")
	return sb.String()
}

// parseSummarizeResult 解析 Summarizer 的 LLM 响应。
func parseSummarizeResult(resp string) (summarizeResult, error) {
	resp = normalizeLLMJSON(resp)
	var res summarizeResult
	if err := json.Unmarshal([]byte(resp), &res); err != nil {
		return summarizeResult{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return res, nil
}

// =====================================================================
// 批量摘要（在 gochatSummarizer 上增加方法，不增加新类型）
// =====================================================================

// batchSummarizeEntry 是批量摘要的 JSON 序列化结构。
// 同时用于 LLM 输入（title/summary/tags 为空）和输出（LLM 填充 title/summary/tags）。
type batchSummarizeEntry struct {
	ChunkID string   `json:"chunk_id"`
	Content string   `json:"content"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// SummarizeBatch 一次 LLM 请求处理所有分片（批量模式）。
// 与 Summarize 共存于 gochatSummarizer，调用方通过类型断言访问：
//
//	if bs, ok := s.(interface{ SummarizeBatch(context.Context, []core.Chunk) ([]core.Chunk, error) }); ok {
//	    bs.SummarizeBatch(ctx, chunks)
//	}
func (s *gochatSummarizer) SummarizeBatch(ctx context.Context, chunks []core.Chunk) ([]core.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}
	// 1. 序列化为 JSON 数组
	entries := make([]batchSummarizeEntry, 0, len(chunks))
	for _, c := range chunks {
		entries = append(entries, batchSummarizeEntry{
			ChunkID: c.ID,
			Content: c.Content,
		})
	}
	input, err := json.Marshal(entries)
	if err != nil {
		return chunks, fmt.Errorf("SummarizeBatch: 序列化分块失败: %w", err)
	}
	// 2. 一次 LLM 调用（带耗时统计）
	resp, err := timedChat(ctx, s.client, []chat.Message{
		chat.NewSystemMessage(s.buildBatchSystemPrompt()),
		chat.NewUserMessage(string(input)),
	}, s.logger, "Summarizer(批量)", s.usageRecorder)
	if err != nil {
		return chunks, fmt.Errorf("SummarizeBatch: LLM 调用失败: %w", err)
	}
	// 3. 解析结果
	result, err := parseBatchSummarizeResult(resp.Content)
	if err != nil {
		s.logger.Warn("SummarizeBatch: 解析 LLM 响应失败，保留原始分片", "error", err)
		return chunks, nil
	}
	// 4. 按 chunk_id 匹配回写
	resultByID := make(map[string]batchSummarizeEntry, len(result))
	for _, r := range result {
		if r.ChunkID != "" {
			resultByID[r.ChunkID] = r
		}
	}
	for i := range chunks {
		if r, ok := resultByID[chunks[i].ID]; ok {
			if r.Title != "" {
				chunks[i].Title = r.Title
			}
			if r.Summary != "" {
				chunks[i].Summary = r.Summary
			}
			if len(r.Tags) > 0 {
				chunks[i].Tags = r.Tags
			}
		}
	}
	return chunks, nil
}

// buildBatchSystemPrompt 构建批量模式的系统提示词。
func (s *gochatSummarizer) buildBatchSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是一名精准的文档摘要助手。\n")
	sb.WriteString("给定一个 JSON 数组形式的文档分块列表，请为每条记录生成 title、summary 和 tags。\n")
	sb.WriteString("输入格式：\n")
	sb.WriteString("[{\"chunk_id\": \"...\", \"content\": \"...\"}]\n\n")
	sb.WriteString("输出格式（严格的 JSON 数组，内容和输入一一对应）：\n")
	sb.WriteString("[{\"chunk_id\": \"...\", \"title\": \"...\", \"summary\": \"...\", \"tags\": [\"...\"]}]\n\n")
	sb.WriteString("字段说明：\n")
	sb.WriteString("- \"title\": 简洁、利于搜索的标题（最多 10 个词）\n")
	sb.WriteString("- \"summary\": 准确概括关键语义（禁止使用一切标记，包括 Markdown、XML、HTML、Emoji 等）\n")
	sb.WriteString("- \"tags\": 3-5 个关键词标签，用于分类和检索（字符串数组）\n\n")
	sb.WriteString("规则：\n")
	sb.WriteString("- JSON 输出必须使用英文标点，禁止出现中文引号、中文逗号或中文冒号。\n")
	sb.WriteString("- 标题、摘要和标签使用 ")
	sb.WriteString(s.config.Language)
	sb.WriteString(" 表达。\n")
	sb.WriteString("- 标签应简洁，避免与标题重复。\n")
	sb.WriteString("- 确保输出数组的长度与输入数组一致，chunk_id 一一对应。\n")
	sb.WriteString("- 如果某个分块内容很短，标题和摘要也应简短但有意义。\n")
	sb.WriteString("- 如果输入是代码段，标题优先使用函数/类名，摘要描述其行为。\n")
	return sb.String()
}

// parseBatchSummarizeResult 解析批量 Summarizer 的 LLM 响应。
func parseBatchSummarizeResult(resp string) ([]batchSummarizeEntry, error) {
	resp = normalizeLLMJSON(resp)
	var res []batchSummarizeEntry
	if err := json.Unmarshal([]byte(resp), &res); err != nil {
		return nil, fmt.Errorf("批量 JSON 解析失败: %w", err)
	}
	return res, nil
}
