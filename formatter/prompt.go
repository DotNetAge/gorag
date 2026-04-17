package formatter

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/DotNetAge/gorag/core"
)

// PromptConfig Prompt 格式化配置
type PromptConfig struct {
	// 系统提示词模板
	SystemPrompt string
	// 上下文模板
	ContextTemplate string
	// 单个文档模板
	DocumentTemplate string
	// 是否包含分数
	IncludeScore bool
	// 是否包含来源
	IncludeSource bool
	// 内容最大长度
	ContentMax int
	// 最大文档数
	MaxDocuments int
	// 分隔符
	Separator string
}

// DefaultPromptConfig 默认 Prompt 配置
func DefaultPromptConfig() *PromptConfig {
	return &PromptConfig{
		SystemPrompt: `You are a knowledgeable assistant. Answer the user's question based strictly on the reference documents provided below.

Requirements:
1. Your answer must be grounded entirely in the provided reference documents.
2. If the reference documents do not contain relevant information, state clearly: "Based on the provided documents, I cannot answer this question."
3. Do not fabricate or infer information not present in the documents.
4. You may cite document numbers to support your answer.
5. Respond in the same language as the user's question. If the user asks in Chinese, respond in Chinese; if in English, respond in English, and so on.`,
		ContextTemplate: `Here are the relevant reference documents:

{{range $i, $doc := .Documents}}
{{$doc}}
{{end}}

Please answer the following question based on the reference documents above: {{.Query}}`,
		DocumentTemplate: `[Document {{.Index}}]{{if .Score}} (relevance: {{printf "%.2f" .Score}}){{end}}
{{.Content}}`,
		IncludeScore:  true,
		IncludeSource: true,
		ContentMax:    1000,
		MaxDocuments:  10,
		Separator:     "\n\n---\n\n",
	}
}

// PromptFormatter LLM Prompt 格式化器
// 用于生成抑制幻觉的提示词
type PromptFormatter struct {
	core.BaseFormatter
	config *PromptConfig
}

// NewPromptFormatter 创建 Prompt 格式化器
func NewPromptFormatter(opts ...func(*PromptConfig)) *PromptFormatter {
	cfg := DefaultPromptConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &PromptFormatter{config: cfg}
}

// WithSystemPrompt 设置系统提示词
func WithSystemPrompt(prompt string) func(*PromptConfig) {
	return func(c *PromptConfig) {
		c.SystemPrompt = prompt
	}
}

// WithContextTemplate 设置上下文模板
func WithContextTemplate(tmpl string) func(*PromptConfig) {
	return func(c *PromptConfig) {
		c.ContextTemplate = tmpl
	}
}

// WithDocumentTemplate 设置文档模板
func WithDocumentTemplate(tmpl string) func(*PromptConfig) {
	return func(c *PromptConfig) {
		c.DocumentTemplate = tmpl
	}
}

// WithIncludeScore 设置是否包含分数
func WithIncludeScore(include bool) func(*PromptConfig) {
	return func(c *PromptConfig) {
		c.IncludeScore = include
	}
}

// WithIncludeSource 设置是否包含来源
func WithIncludeSource(include bool) func(*PromptConfig) {
	return func(c *PromptConfig) {
		c.IncludeSource = include
	}
}

// WithContentMaxPrompt 设置内容最大长度
func WithContentMaxPrompt(max int) func(*PromptConfig) {
	return func(c *PromptConfig) {
		c.ContentMax = max
	}
}

// WithMaxDocuments 设置最大文档数
func WithMaxDocuments(max int) func(*PromptConfig) {
	return func(c *PromptConfig) {
		c.MaxDocuments = max
	}
}

// promptData 完整提示词数据
type promptData struct {
	Documents []string
	Query     string
}

// Format 格式化单个 Hit
func (f *PromptFormatter) Format(hit *core.Hit) string {
	return f.formatDocument(1, hit)
}

// formatDocument 格式化单个文档
func (f *PromptFormatter) formatDocument(index int, hit *core.Hit) string {
	content := hit.Content
	if f.config.ContentMax > 0 && len(content) > f.config.ContentMax {
		content = content[:f.config.ContentMax] + "..."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[Document %d]", index)

	if f.config.IncludeScore && hit.Score > 0 {
		fmt.Fprintf(&sb, " (relevance: %.4f)", hit.Score)
	}

	if f.config.IncludeSource && hit.DocID != "" {
		fmt.Fprintf(&sb, " [source: %s]", hit.DocID)
	}

	sb.WriteString("\n")
	sb.WriteString(content)

	return sb.String()
}

// FormatAll 格式化多个 Hit（不包含查询）
func (f *PromptFormatter) FormatAll(hits []core.Hit) string {
	return f.FormatWithContext(hits, "")
}

// FormatWithContext 格式化为完整 Prompt
func (f *PromptFormatter) FormatWithContext(hits []core.Hit, query string) string {
	var sb strings.Builder

	// System prompt
	if f.config.SystemPrompt != "" {
		sb.WriteString("## System Instructions\n\n")
		sb.WriteString(f.config.SystemPrompt)
		sb.WriteString("\n\n")
	}

	// Reference documents
	sb.WriteString("## Reference Documents\n\n")

	maxDocs := len(hits)
	if f.config.MaxDocuments > 0 && maxDocs > f.config.MaxDocuments {
		maxDocs = f.config.MaxDocuments
	}

	for i := 0; i < maxDocs; i++ {
		sb.WriteString(f.formatDocument(i+1, &hits[i]))
		if i < maxDocs-1 {
			sb.WriteString(f.config.Separator)
		}
	}

	// User query
	if query != "" {
		sb.WriteString("\n\n## User Question\n\n")
		sb.WriteString(query)
	}

	return sb.String()
}

// FormatWithTemplate 使用自定义模板格式化
func (f *PromptFormatter) FormatWithTemplate(hits []core.Hit, query string) (string, error) {
	maxDocs := len(hits)
	if f.config.MaxDocuments > 0 && maxDocs > f.config.MaxDocuments {
		maxDocs = f.config.MaxDocuments
	}

	docs := make([]string, maxDocs)
	for i := 0; i < maxDocs; i++ {
		docs[i] = f.formatDocument(i+1, &hits[i])
	}

	data := promptData{
		Documents: docs,
		Query:     query,
	}

	tmpl, err := template.New("context").Parse(f.config.ContextTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse context template: %w", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return sb.String(), nil
}

// Write 格式化并写入输出流
func (f *PromptFormatter) Write(w io.Writer, hits []core.Hit) error {
	_, err := w.Write([]byte(f.FormatAll(hits)))
	return err
}

// WriteWithContext 格式化完整 Prompt 并写入输出流
func (f *PromptFormatter) WriteWithContext(w io.Writer, hits []core.Hit, query string) error {
	_, err := w.Write([]byte(f.FormatWithContext(hits, query)))
	return err
}

// FormatForRAG 生成标准的 RAG Prompt
// 包含系统提示、文档上下文和用户查询
func (f *PromptFormatter) FormatForRAG(hits []core.Hit, query string) string {
	return f.FormatWithContext(hits, query)
}

// FormatMessages 生成对话格式的消息列表
// 返回 [system, user] 两条消息
func (f *PromptFormatter) FormatMessages(hits []core.Hit, query string) []Message {
	return []Message{
		{Role: "system", Content: f.config.SystemPrompt},
		{Role: "user", Content: f.FormatWithContext(hits, query)},
	}
}

// Message 对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// JSONFormatter JSON 格式化器
type JSONFormatter struct {
	core.BaseFormatter
}

// NewJSONFormatter 创建 JSON 格式化器
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// FormatAll 格式化为 JSON
func (f *JSONFormatter) FormatAll(hits []core.Hit) string {
	var sb strings.Builder
	sb.WriteString("[\n")
	for i, hit := range hits {
		sb.WriteString("  {\n")
		fmt.Fprintf(&sb, "    \"id\": \"%s\",\n", hit.ID)
		fmt.Fprintf(&sb, "    \"score\": %.4f,\n", hit.Score)
		fmt.Fprintf(&sb, "    \"doc_id\": \"%s\",\n", hit.DocID)
		// 转义内容中的特殊字符
		content := strings.ReplaceAll(hit.Content, "\n", "\\n")
		content = strings.ReplaceAll(content, "\"", "\\\"")
		fmt.Fprintf(&sb, "    \"content\": \"%s\"\n", content)
		sb.WriteString("  }")
		if i < len(hits)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("]")
	return sb.String()
}
