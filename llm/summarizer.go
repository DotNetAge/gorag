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
	config Config
	client chat.Client
	logger logging.Logger
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

// Summarize 调用 LLM 为每个 Chunk 生成更语义化的 Title 和 Summary。
func (s *gochatSummarizer) Summarize(ctx context.Context, chunks []core.Chunk) ([]core.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	for i := range chunks {
		if strings.TrimSpace(chunks[i].Content) == "" {
			continue
		}
		title, summary, err := s.summarizeOne(ctx, chunks[i].Content)
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
	}
	return chunks, nil
}

// summarizeResult 是 LLM 返回的 JSON 结构。
type summarizeResult struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// summarizeOne 对单段内容调用 LLM，返回 (title, summary, error)。
func (s *gochatSummarizer) summarizeOne(ctx context.Context, content string) (string, string, error) {
	resp, err := s.client.Chat(ctx, []chat.Message{
		chat.NewSystemMessage(s.buildSystemPrompt()),
		chat.NewUserMessage(content),
	})
	if err != nil {
		return "", "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	res, err := parseSummarizeResult(resp.Content)
	if err != nil {
		return "", "", fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	return res.Title, res.Summary, nil
}

// buildSystemPrompt 构建 Summarizer 的系统提示词。
func (s *gochatSummarizer) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("你是一名精准的文档摘要助手。\n")
	sb.WriteString("请对给定文本片段生成严格的 JSON，仅包含两个字段：\n")
	sb.WriteString("- \"title\": 简洁、利于搜索的标题（最多 10 个词）\n")
	sb.WriteString("- \"summary\": 1-3 句话，准确概括关键语义\n\n")
	sb.WriteString("规则：\n")
	sb.WriteString("- JSON 输出必须使用英文标点，禁止出现中文引号、中文逗号或中文冒号。\n")
	sb.WriteString("- 标题和摘要使用 ")
	sb.WriteString(s.config.Language)
	sb.WriteString(" 表达。\n")
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
