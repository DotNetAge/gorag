package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/v2/chunker"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/utils"
)

// =====================================================================
// Refiller：基于预分块文本的实体回填
// =====================================================================

// Refiller 将所有 Chunk 序列化为 JSON 数组，结合 Schema 让 LLM 提取实体和关系，
// 最后将结果以 Node/Edge 形式回填到 ChunkResult 中。
//
// 设计要点：
//   - 输入为 chunker.ChunkResult，输出为补充了 Nodes/Edges 的 ChunkResult
//   - 不删除或修改已有的 Nodes/Edges，仅做追加
//   - 提取出的实体 Node 的 SourceChunkIDs 为空（纯图实体）
//   - SourceDocIDs 从 result.Chunks 中收集到的 DocID 去重后填充
type Refiller interface {
	Refill(ctx context.Context, result chunker.ChunkResult, schemas []EntitySchema) (chunker.ChunkResult, error)
}

// gochatRefiller 基于 gochat 的 Refiller 默认实现。
type gochatRefiller struct {
	config        Config
	client        chat.Client
	logger        logging.Logger
	usageRecorder UsageRecorder
}

// NewRefiller 创建基于 gochat 的 Refiller。
//
// 必传参数：
//   - cfg: LLM 配置（APIKey/BaseURL/Model 必填）
//   - logger: 日志实例（禁止为 nil）
func NewRefiller(cfg Config, logger logging.Logger) (Refiller, error) {
	if logger == nil {
		return nil, fmt.Errorf("llm.NewRefiller: logger 不能为空")
	}
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("llm.NewRefiller: %w", err)
	}

	client, err := newChatClient(cfg)
	if err != nil {
		return nil, err
	}
	return &gochatRefiller{
		config: cfg,
		client: client,
		logger: logger,
	}, nil
}

// SetUsageRecorder 设置 token 用量记录回调。
// 在 Refiller 每次成功调用 LLM 后自动记录用量信息。
func (r *gochatRefiller) SetUsageRecorder(recorder UsageRecorder) {
	r.usageRecorder = recorder
}

// Refill 调用 LLM 从预分块文本中提取实体和关系，并回填到 ChunkResult。
func (r *gochatRefiller) Refill(ctx context.Context, result chunker.ChunkResult, schemas []EntitySchema) (chunker.ChunkResult, error) {
	if len(result.Chunks) == 0 {
		return result, nil
	}

	docIDs := collectDocIDs(result.Chunks)
	content, err := serializeChunks(result.Chunks)
	if err != nil {
		return result, fmt.Errorf("llm.Refiller: 序列化分块失败: %w", err)
	}

	resp, err := timedChat(ctx, r.client, []chat.Message{
		chat.NewSystemMessage(r.buildSystemPrompt(schemas)),
		chat.NewUserMessage(r.buildUserPrompt(content)),
	}, r.logger, "Refiller", r.usageRecorder)
	if err != nil {
		return result, fmt.Errorf("llm.Refiller: LLM 调用失败: %w", err)
	}

	ext, err := parseRefillExtraction(resp.Content)
	if err != nil {
		return result, fmt.Errorf("llm.Refiller: 解析 LLM 响应失败: %w", err)
	}

	nodes, edges := buildNodesAndEdges(ext, docIDs)
	result.Nodes = append(result.Nodes, nodes...)
	result.Edges = append(result.Edges, edges...)
	return result, nil
}

// refillEntity / refillRelation / refillExtraction 是 LLM 返回的 JSON 结构。
type refillEntity struct {
	Name       string         `json:"name"`
	EntityType string         `json:"entity_type"`
	Properties map[string]any `json:"properties"`
}

type refillRelation struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

type refillExtraction struct {
	Entities  []refillEntity   `json:"entities"`
	Relations []refillRelation `json:"relations"`
}

// serializeChunks 将 Chunk 列表序列化为 JSON 数组字符串。
//
// 仅保留对实体提取有用的字段，避免 Base64 图片内容导致 prompt 过大。
func serializeChunks(chunks []core.Chunk) (string, error) {
	type item struct {
		ID       string         `json:"id"`
		Title    string         `json:"title"`
		Summary  string         `json:"summary"`
		Content  string         `json:"content"`
		Metadata map[string]any `json:"metadata"`
	}

	items := make([]item, 0, len(chunks))
	for _, c := range chunks {
		items = append(items, item{
			ID:       c.ID,
			Title:    c.Title,
			Summary:  c.Summary,
			Content:  c.Content,
			Metadata: c.Metadata,
		})
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// collectDocIDs 收集 Chunk 中所有不重复的 DocID。
func collectDocIDs(chunks []core.Chunk) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range chunks {
		if c.DocID == "" || seen[c.DocID] {
			continue
		}
		seen[c.DocID] = true
		out = append(out, c.DocID)
	}
	return out
}

// buildSystemPrompt 构建 Refiller 的系统提示词。
//
// 为每个实体类型生成完整的属性描述（含类型、枚举值、格式、必填约束），
// 确保小参数中文模型能按统一的预期结构输出。
func (r *gochatRefiller) buildSystemPrompt(schemas []EntitySchema) string {
	var sb strings.Builder
	sb.WriteString("你是一名精准的实体关系提取助手。\n")
	sb.WriteString("给定一个 JSON 数组形式的文本分块，请从中提取实体和关系。\n")
	sb.WriteString("输出严格的 JSON，仅包含两个字段：\n")
	sb.WriteString("- \"entities\": [{\"name\": string, \"entity_type\": string, \"properties\": {}}]\n")
	sb.WriteString("- \"relations\": [{\"subject\": string, \"predicate\": string, \"object\": string}]\n\n")

	if len(schemas) > 0 {
		sb.WriteString("### 实体类型\n\n")
		for _, s := range schemas {
			sb.WriteString("**")
			sb.WriteString(s.Type)
			sb.WriteString("**")
			if s.Description != "" {
				sb.WriteString(" — ")
				sb.WriteString(s.Description)
			}
			sb.WriteString("\n")

			// 必填字段
			if len(s.Required) > 0 {
				sb.WriteString("必填：")
				sb.WriteString(strings.Join(s.Required, ", "))
				sb.WriteString("\n")
			}

			// 属性详情
			if len(s.Properties) > 0 {
				sb.WriteString("属性：\n")
				for _, name := range sortedKeys(s.Properties) {
					prop := s.Properties[name]
					sb.WriteString("  - ")
					sb.WriteString(name)
					sb.WriteString(" (")
					sb.WriteString(formatPropType(prop))
					sb.WriteString(")")
					if prop.Description != "" {
						sb.WriteString(": ")
						sb.WriteString(prop.Description)
					}
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("规则：\n")
	sb.WriteString("- JSON 输出必须使用英文标点，禁止出现中文引号、中文逗号或中文冒号。\n")
	sb.WriteString("- 如果未找到实体或关系，返回 {\"entities\":[],\"relations\":[]}。\n")
	sb.WriteString("- 每个实体的 properties 必须严格按照对应实体类型的属性定义输出，必填字段不可省略，枚举值只能取定义范围内的值。\n")
	return sb.String()
}

// buildUserPrompt 构建 Refiller 的用户提示词。
func (r *gochatRefiller) buildUserPrompt(content string) string {
	var sb strings.Builder
	sb.WriteString("请从以下分块中提取实体和关系：\n\n")
	sb.WriteString(content)
	return sb.String()
}

// parseRefillExtraction 解析 Refiller 的 LLM 响应。
func parseRefillExtraction(resp string) (refillExtraction, error) {
	resp = normalizeLLMJSON(resp)
	var ext refillExtraction
	if err := json.Unmarshal([]byte(resp), &ext); err != nil {
		return refillExtraction{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return ext, nil
}

// buildNodesAndEdges 将 LLM 提取结果转换为 core.Node / core.Edge。
//
// 设计要点：
//   - Node.SourceChunkIDs 为空，表示纯图实体
//   - Node.SourceDocIDs 使用从 Chunk 收集到的 DocID
//   - Edge.SourceChunkIDs 为空；SourceDocIDs 与 Node 一致
func buildNodesAndEdges(ext refillExtraction, docIDs []string) ([]core.Node, []core.Edge) {
	nodes := make([]core.Node, 0, len(ext.Entities))
	nodeIDByName := map[string]string{}

	for _, e := range ext.Entities {
		if e.Name == "" || e.EntityType == "" {
			continue
		}
		props := map[string]any{
			"entity_type": e.EntityType,
		}
		for k, v := range e.Properties {
			props[k] = v
		}
		node := core.Node{
			ID:           utils.GenerateID([]byte(e.Name + ":" + e.EntityType)),
			Labels:       []string{e.EntityType},
			Name:         e.Name,
			Properties:   props,
			SourceDocIDs: docIDs,
		}
		nodes = append(nodes, node)
		nodeIDByName[e.Name] = node.ID
	}

	edges := make([]core.Edge, 0, len(ext.Relations))
	for _, rel := range ext.Relations {
		if rel.Subject == "" || rel.Predicate == "" || rel.Object == "" {
			continue
		}
		sourceID, sourceOK := nodeIDByName[rel.Subject]
		targetID, targetOK := nodeIDByName[rel.Object]
		if !sourceOK {
			continue
		}
		if !targetOK {
			continue
		}
		edges = append(edges, core.Edge{
			ID:           utils.GenerateID([]byte(rel.Subject + ":" + rel.Predicate + ":" + rel.Object)),
			Type:         rel.Predicate,
			Source:       sourceID,
			Target:       targetID,
			SourceDocIDs: docIDs,
		})
	}
	return nodes, edges
}

// sortedKeys 返回 map 的按键名排序后的键列表，确保 prompt 输出顺序稳定。
func sortedKeys(m map[string]SchemaProperty) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// formatPropType 格式化属性类型描述，包含数组元素类型、枚举值和格式信息。
func formatPropType(p SchemaProperty) string {
	switch {
	case p.Type == "array" && p.Items != nil:
		return "array[" + p.Items.Type + "]"
	case len(p.Enum) > 0:
		return "string, 枚举值 [" + strings.Join(p.Enum, ", ") + "]"
	case p.Format != "":
		return p.Type + ", " + p.Format
	default:
		return p.Type
	}
}
