package chunker

import (
	"maps"

	"github.com/DotNetAge/gorag/core"
)

// CodeChunker 代码分块器
// 基于 StructureNode（由 CodeStructurizer 生成的 AST 结构）进行分块
type CodeChunker struct {
	minLinesPerChunk int     // 最小行数
	maxLinesPerChunk int     // 最大行数
	options          Options // 可选配置
}

// NewCodeChunker 创建代码分块器
func NewCodeChunker(opts ...Option) *CodeChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &CodeChunker{
		minLinesPerChunk: 5,   // 默认最小 5 行
		maxLinesPerChunk: 100, // 默认最大 100 行
		options:          options,
	}
}

// Chunk 实现分块接口
func (c *CodeChunker) Chunk(
	structured *core.StructuredDocument,
	entities []*core.Entity,
) ([]*core.Chunk, error) {
	if structured == nil || structured.RawDoc == nil {
		return []*core.Chunk{}, nil
	}

	text := structured.RawDoc.GetContent()
	if text == "" {
		return []*core.Chunk{}, nil
	}

	// 如果没有结构化信息，降级为固定大小分块
	if structured.Root == nil {
		return NewFixedSizeChunker(WithChunkSize(c.options.ChunkSize)).Chunk(structured, entities)
	}

	// 遍历结构树，提取代码块
	var chunks []*core.Chunk
	c.extractCodeBlocks(text, structured.Root, &chunks, []string{})

	// 补充元数据
	for i, chunk := range chunks {
		chunk.ID = GenerateChunkID(structured.RawDoc.GetID(), i, chunk.Content)
		chunk.DocID = structured.RawDoc.GetID()
		chunk.MIMEType = structured.RawDoc.GetMimeType()
		chunk.Metadata = c.mergeMetadata(structured.RawDoc.GetMeta(), chunk.Metadata)
		chunk.ChunkMeta.Index = i
	}

	return chunks, nil
}

// GetStrategy 返回策略类型
func (c *CodeChunker) GetStrategy() core.ChunkStrategy {
	return StrategyCode
}

// extractCodeBlocks 递归提取代码块
func (c *CodeChunker) extractCodeBlocks(
	content string,
	node *core.StructureNode,
	chunks *[]*core.Chunk,
	headingPath []string,
) {
	if node == nil {
		return
	}

	// 判断是否为关键代码结构节点
	if c.isKeyCodeNode(node) {
		// 提取代码块
		codeContent := content[node.StartPos:node.EndPos]
		lines := CountLines(codeContent)

		// 根据行数决定是否需要进一步分割
		if lines > c.maxLinesPerChunk {
			// 超过最大行数，尝试按子节点分割
			if len(node.Children) > 0 {
				for _, child := range node.Children {
					c.extractCodeBlocks(content, child, chunks, headingPath)
				}
				return
			}
		}

		// 创建代码块
		chunk := &core.Chunk{
			Content: codeContent,
			ChunkMeta: core.ChunkMeta{
				StartPos:     node.StartPos,
				EndPos:       node.EndPos,
				HeadingLevel: node.Level,
				HeadingPath:  append([]string{}, headingPath...),
			},
			Metadata: map[string]any{
				"node_id":   node.ID(), // 建立与 StructureNode 的关联
				"node_type": node.NodeType,
				"title":     node.Title,
				"lines":     lines,
			},
		}

		// If too small, try to merge with previous block
		if lines < c.minLinesPerChunk && len(*chunks) > 0 {
			lastChunk := (*chunks)[len(*chunks)-1]
			if lastChunk.Metadata == nil {
				lastChunk.Metadata = make(map[string]any)
			}
			lastLines, ok := lastChunk.Metadata["lines"].(int)

			// Merge only if lines count is available and won't exceed max
			if ok && lastLines+lines <= c.maxLinesPerChunk {
				mergedContent := lastChunk.Content + "\n\n" + codeContent
				lastChunk.Content = mergedContent
				lastChunk.ChunkMeta.EndPos = node.EndPos
				lastChunk.Metadata["lines"] = lastLines + lines
				return
			}
		}

		*chunks = append(*chunks, chunk)
		return
	}

	// 非关键节点，继续递归子节点
	newHeadingPath := headingPath
	if node.NodeType == "heading" && node.Title != "" {
		newHeadingPath = append(headingPath, node.Title)
	}

	for _, child := range node.Children {
		c.extractCodeBlocks(content, child, chunks, newHeadingPath)
	}
}

// isKeyCodeNode checks if a node is a key code structure node
func (c *CodeChunker) isKeyCodeNode(node *core.StructureNode) bool {
	if node == nil {
		return false
	}

	// Key code structure types
	keyTypes := map[string]bool{
		"function":       true, // 函数
		"method":         true, // 方法
		"class":          true, // 类
		"type":           true, // 类型定义
		"interface":      true, // 接口
		"struct":         true, // 结构体
		"enum":           true, // 枚举
		"implementation": true, // 实现
		"module":         true, // 模块
	}

	return keyTypes[node.NodeType]
}

// mergeMetadata merges document metadata with chunk metadata
func (c *CodeChunker) mergeMetadata(docMeta, chunkMeta map[string]any) map[string]any {
	result := make(map[string]any)

	// Copy document metadata
	maps.Copy(result, docMeta)

	// Copy chunk metadata
	maps.Copy(result, chunkMeta)

	return result
}
