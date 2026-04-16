package query

import (
	"context"
	"fmt"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/core"
)

// SemanticQuery 语义查询
type SemanticQuery struct {
	BaseQuery
	Embedder core.Embedder // 向量编码器，用于语义相似度计算
	rewrited string        // 重写后的查询
}

// Vector returns the vector representations of the query.
func (q *SemanticQuery) Vector() *core.Vector {
	text := q.normalized
	if q.rewrited != "" {
		text = q.rewrited
	}

	vector, err := q.Embedder.CalcText(text)
	if err != nil {
		return nil
	}
	return vector
}

// Expansion 查询扩展：生成多个相关查询变体，扩大检索范围
// 适用于查询词过于狭窄或模糊的场景
func (q *SemanticQuery) Expansion(client chat.Client) error {
	ctx := context.Background()

	prompt := `You are a professional query expansion assistant. Generate 3-5 related query variations based on the user's original query.

IMPORTANT: You MUST output in the same language as the user query.

Requirements:
1. Maintain the core intent of the query
2. Use synonyms, related terms, or different expressions
3. Cover different angles and aspects of the query
4. One variation per line, no numbering or formatting

Original query: ` + q.raw + `

Output the query variations directly, one per line:`

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("query expansion failed: %w", err)
	}

	q.rewrited = response.Content
	return nil
}

// HyDe 假设式文档查询：生成一个假设性的文档来回答查询
// 适用于需要语义匹配但缺乏精确关键词的场景
func (q *SemanticQuery) HyDe(client chat.Client) error {
	ctx := context.Background()

	prompt := `You are a professional document generation assistant. Generate a hypothetical document paragraph that would answer the user's query well.

IMPORTANT: You MUST output in the same language as the user query.

Requirements:
1. The document content should directly answer the query
2. Use professional and clear language
3. Include relevant details and information
4. Length should be 100-300 words (or 150-450 characters for Chinese)
5. Do not add "hypothetical document" or similar labels

User query: ` + q.raw + `

Generate the document content directly:`

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("HyDe generation failed: %w", err)
	}

	q.rewrited = response.Content
	return nil
}

// Rewriting 查询重写：将模糊或不明确的查询改写为更清晰的形式
// 适用于口语化、模糊或不完整的查询
func (q *SemanticQuery) Rewriting(client chat.Client) error {
	ctx := context.Background()

	prompt := `You are a professional query optimization assistant. Rewrite the user's original query into a clearer, more specific, and more retrieval-friendly form.

IMPORTANT: You MUST output in the same language as the user query.

Requirements:
1. Maintain the core intent of the original query
2. Convert colloquial expressions to formal expressions
3. Add necessary context or details
4. Use more accurate terminology and keywords
5. Only output the rewritten query, no explanations

Examples:
Chinese:
Original: "那个手机好用吗"
Rewritten: "智能手机推荐 性能评价 用户评测对比"

Original: "昨天那个会议内容"
Rewritten: "2024年产品评审会议记录 议程内容"

Original: "怎么处理退款"
Rewritten: "退款申请流程 操作步骤 审核时间"

English:
Original: "is that phone good"
Rewritten: "smartphone recommendations performance reviews user comparisons"

Original: "yesterday's meeting content"
Rewritten: "2024 product review meeting minutes agenda"

Original: "how to handle refund"
Rewritten: "refund application process steps approval time"

User original query: ` + q.raw + `

Output the rewritten query:`

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return fmt.Errorf("query rewriting failed: %w", err)
	}

	q.rewrited = response.Content
	return nil
}

// NewSemanticQuery creates a new semantic query from the given terms.
func NewSemanticQuery(terms string, embedder core.Embedder) core.Query {
	return &SemanticQuery{
		BaseQuery: BaseQuery{
			raw:        terms,
			embedder:   embedder,
			normalized: core.CleanText(terms),
			filters:    make(map[string]any),
		},
		Embedder: embedder,
	}
}
