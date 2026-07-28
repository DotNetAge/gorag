package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/logging"
)

// =====================================================================
// Summarizer：LLM 摘要/标题增强
// =====================================================================

// Summarizer 对一段文本内容进行摘要，返回标题和摘要。
//
// 设计要点：
//   - 每 chunk 一次调用，无批量语义
//   - 调用方（HyperIndexer）在 AddFile 中逐 chunk 调用
//   - 仅当 LLM 返回合法 title/summary 时才覆盖原值
//   - 失败时返回 error，由调用方决定是否跳过该 chunk
type Summarizer interface {
	Summarize(ctx context.Context, content string) (title string, summary string, err error)
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
func (s *gochatSummarizer) SetUsageRecorder(r UsageRecorder) {
	s.usageRecorder = r
}

// Summarize 对一段文本内容调用 LLM，返回标题和摘要。
func (s *gochatSummarizer) Summarize(ctx context.Context, content string) (string, string, error) {
	if strings.TrimSpace(content) == "" {
		return "", "", nil
	}

	resp, err := timedChat(ctx, s.client, []chat.Message{
		chat.NewSystemMessage(s.buildSystemPrompt()),
		chat.NewUserMessage(content),
	}, s.logger, "Sumarizer", s.usageRecorder)
	if err != nil {
		return "", "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	res, err := parseSummarizeResult(resp.Content)
	if err != nil {
		return "", "", fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	return res.Title, res.Summary, nil
}

// summarizeResult 是 LLM 返回的 JSON 结构。
type summarizeResult struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags,omitempty"`
}

// buildSystemPrompt 构建摘要的系统提示词。
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
	sb.WriteString("- \"tags\": 3-5 个关键词标签，用于分类和检索（字符串数组，可选，没有则返回空数组）\n\n")
	sb.WriteString("格式规则：\n")
	sb.WriteString("- JSON 输出必须使用英文标点，禁止出现中文引号、中文逗号或中文冒号\n")
	sb.WriteString("- 输出字段的内容必须是中文，禁止使用英文\n")
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
