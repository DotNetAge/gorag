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

// buildSystemPrompt 构建单条摘要的系统提示词。
//
// 设计要点：
//   - 先识别内容类型（代码/文档/数据），再按类型执行不同的摘要策略
//   - 明确禁止复制原文，要求对内容进行提炼、概括和重组
//   - summary 必须比原文短且更具概括性
func (s *gochatSummarizer) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是一名精准的文档分块摘要助手。\n\n")
	sb.WriteString("用户提供的是一个从完整文档中切分出的片段。请你先判断这个片段属于哪种类型")
	sb.WriteString("（代码片段/自然语言文档/结构化数据/其他），然后按以下规则处理：\n\n")
	sb.WriteString("输出字段：\n")
	sb.WriteString("- \"title\": 该片段的核心标识（最多 10 个词）。\n")
	sb.WriteString("  · 代码：优先使用函数名、类名、方法名\n")
	sb.WriteString("  · 文档：使用该片段讨论的核心主题\n")
	sb.WriteString("  · 数据：使用表格/记录的语义描述\n")
	sb.WriteString("- \"summary\": 对该片段关键语义的**提炼概括**，而非复制。\n")
	sb.WriteString("  · 必须是对原文的浓缩、重述，禁止直接复制或基本照搬原文句子\n")
	sb.WriteString("  · 长度应在 1-3 句话内，比原文短得多\n")
	sb.WriteString("  · 禁止使用一切标记（Markdown、XML、HTML、Emoji 等）\n")
	sb.WriteString("  · 如果原文是代码：用自然语言描述该代码段的功能和行为\n")
	sb.WriteString("  · 如果原文是文档：提取核心观点或事实\n")
	sb.WriteString("  · 如果原文是数据：描述数据的内容、范围或统计特征\n")
	sb.WriteString("- \"tags\": 3-5 个关键词标签，用于分类和检索（字符串数组）\n\n")
	sb.WriteString("格式规则：\n")
	sb.WriteString("- JSON 输出必须使用英文标点，禁止出现中文引号、中文逗号或中文冒号\n")
	sb.WriteString("- 标签应简洁，避免与标题重复\n")
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
		s.logger.Warn("SummarizeBatch: 解析 LLM 响应失败",
			"error", err,
			"raw_response", resp.Content,
		)
		return nil, fmt.Errorf("批量 JSON 解析失败: %w", err)
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
//
// 与单条模式相同的摘要策略，区别在于输入输出为 JSON 数组格式。
// 注意保持 prompt 尽可能简短，避免 LLM 输出 token 超限导致 JSON 截断。
func (s *gochatSummarizer) buildBatchSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是一名精准的文档分块摘要助手。\n")
	sb.WriteString("输入是 JSON 数组，每个元素是一个文档片段。\n")
	sb.WriteString("请为每条记录输出 title, summary, tags。\n\n")
	sb.WriteString("输入: [{\"chunk_id\":\"...\",\"content\":\"...\"}]\n")
	sb.WriteString("输出: [{\"chunk_id\":\"...\",\"title\":\"...\",\"summary\":\"...\",\"tags\":[...]}]\n\n")
	sb.WriteString("规则：\n")
	sb.WriteString("- title: 核心标识，代码用函数/类名，文档用主题（最多10词）\n")
	sb.WriteString("- summary: 提炼概括，禁止复制原文，1-3句话，禁止使用任何标记\n")
	sb.WriteString("  · 代码：自然语言描述功能\n")
	sb.WriteString("  · 文档：提取核心观点\n")
	sb.WriteString("  · 数据：描述内容/范围/特征\n")
	sb.WriteString("- tags: 3-5个关键词\n")
	sb.WriteString("- chunk_id 必须原样返回\n")
	sb.WriteString("- 输出必须是合法 JSON 数组，长度与输入一致\n")
	return sb.String()
}

// parseBatchSummarizeResult 解析批量 Summarizer 的 LLM 响应。
func parseBatchSummarizeResult(resp string) ([]batchSummarizeEntry, error) {
	resp = normalizeLLMJSON(resp)
	var res []batchSummarizeEntry
	if err := json.Unmarshal([]byte(resp), &res); err != nil {
		return nil, fmt.Errorf("批量 JSON 解析失败: %w\n原始响应: %s", err, resp)
	}
	return res, nil
}
