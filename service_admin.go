package gorag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/store/meta"
)

// RAGInfo RAG 库信息
type RAGInfo struct {
	Config      *Config
	ConfigYAML  string
	AbsPath     string
	Sizes       map[string]int64
	VectorCount int
	GraphNodes  int64
	GraphEdges  int64
}

// CheckResult 诊断项结果
type CheckResult struct {
	Name string
	OK   bool
	Hint string
}

// SourceTreeNode 目录树节点，对应一个文件或目录。
type SourceTreeNode struct {
	Name     string            // 文件/目录名
	Path     string            // 完整路径
	Size     int64             // 文件内容总大小（所有 Chunk Content 长度之和）
	IsDir    bool              // 是否为目录
	Summary  string            // 目录摘要（来自 README.md）
	Chunks   []SourceChunkNode // 该文件下的顶层 Chunk（ParentID==""）
	Children []*SourceTreeNode // 子目录
}

// SourceChunkNode Chunk 树节点。
type SourceChunkNode struct {
	Type      string            // 数据|文档|图片|代码
	Title     string            // Chunk 标题
	Summary   string            // Chunk 摘要
	StartLine int               // 在源文件中的起始行号
	EndLine   int               // 在源文件中的结束行号
	Children  []SourceChunkNode // 通过 ParentID 连结的子块
}

// maxTreeChunks 是 tree 命令单次最多加载的 Chunk 数量，防止大库内存爆炸。
const maxTreeChunks = 10000

// AdminService 负责库信息、诊断、日志、目录树。
type AdminService struct {
	svc *IndexingService
}

// Info 获取 RAG 库的完整信息。
func (s *AdminService) Info() (*RAGInfo, error) {
	info := &RAGInfo{AbsPath: s.svc.dataDir}

	// 1. 读取配置
	cfg, raw, err := loadConfigRaw(s.svc.dataDir)
	if err != nil {
		return nil, err
	}
	info.Config = cfg
	info.ConfigYAML = raw

	// 2. 目录大小
	info.Sizes = calcDirSizes(s.svc.dataDir)

	// 3. 向量索引统计
	name := filepath.Base(s.svc.dataDir)
	vecDB := filepath.Join(s.svc.dataDir, "vectors", name+".db")
	if _, err := os.Stat(vecDB); err == nil {
		if cnt := getVectorCount(vecDB); cnt >= 0 {
			info.VectorCount = cnt
		}
	}

	// 4. 图索引统计
	graphDB := filepath.Join(s.svc.dataDir, "graphs", name+".db")
	if _, err := os.Stat(graphDB); err == nil {
		nodes, edges := getGraphCount(graphDB)
		if nodes >= 0 {
			info.GraphNodes = nodes
		}
		if edges >= 0 {
			info.GraphEdges = edges
		}
	}

	return info, nil
}

// Doctor 诊断 RAG 库的配置完整性。
func (s *AdminService) Doctor() []CheckResult {
	cfg, err := loadConfig(s.svc.dataDir)
	if err != nil {
		return []CheckResult{{Name: "config.yml", OK: false, Hint: "读取失败: " + err.Error()}}
	}

	return []CheckResult{
		{Name: "config.yml", OK: true},
		{Name: "embedding.model_file", OK: cfg.Embedding.ModelFile != "", Hint: "运行：grag config embedder <path>"},
		{Name: "向量库目录", OK: dirExists(filepath.Join(s.svc.dataDir, "vectors"))},
		{Name: "图库目录", OK: dirExists(filepath.Join(s.svc.dataDir, "graphs"))},
		{Name: "meta.db", OK: fileExists(filepath.Join(s.svc.dataDir, "meta.db"))},
	}
}

// FileStatuses 查询所有文件的索引与 LLM 处理进度。
// status 为空时返回全部状态；filterPath 非空时按绝对路径前缀过滤。
func (s *AdminService) FileStatuses(ctx context.Context, status, filterPath string) ([]*meta.DocumentProgress, error) {
	var absFilter string
	if filterPath != "" {
		var err error
		absFilter, err = filepath.Abs(filterPath)
		if err != nil {
			absFilter = filterPath
		}
	}
	return s.svc.metaStore.ListDocumentsWithProgress(status, absFilter)
}

// StatusSummary 返回各索引状态的文档数量统计。
func (s *AdminService) StatusSummary(ctx context.Context) (map[string]int, error) {
	return s.svc.metaStore.CountDocumentsByStatus()
}

// Logs 返回 RAG 库的日志内容。
func (s *AdminService) Logs() (string, error) {
	logFile := filepath.Join(s.svc.dataDir, "logs", "gorag.log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		return "", fmt.Errorf("读取日志失败: %w", err)
	}
	return string(data), nil
}

// Tree 基于所有 Chunk 的 Source 属性重建文件目录树。
//
// 获取全部已索引的 Chunk，按 Source 分组后重建目录层级。
// 每个文件节点下挂载该文件的顶层 Chunk（ParentID=""），
// 再通过 ParentID 连结子块形成块子树。
func (s *AdminService) Tree(ctx context.Context) (*SourceTreeNode, error) {
	admin, ok := s.svc.indexer.(indexer.IndexerAdmin)
	if !ok {
		return nil, fmt.Errorf("索引器不支持列表查询")
	}

	total, err := admin.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 Chunk 总数失败: %w", err)
	}

	if total == 0 {
		return &SourceTreeNode{Name: ".", IsDir: true}, nil
	}
	if total > maxTreeChunks {
		return nil, fmt.Errorf("Chunk 数量 %d 超过 tree 命令上限 %d，请使用 query/cypher 命令", total, maxTreeChunks)
	}

	allChunks, _, err := admin.List(ctx, 0, total, nil)
	if err != nil {
		return nil, fmt.Errorf("获取 Chunk 列表失败: %w", err)
	}

	// 按 ChunkID 去重（List 可能返回多个维度的向量对应的 Chunk）
	seen := make(map[string]bool, len(allChunks))
	deduped := make([]core.Chunk, 0, len(allChunks))
	for _, c := range allChunks {
		if !seen[c.ID] {
			seen[c.ID] = true
			deduped = append(deduped, c)
		}
	}

	if len(deduped) == 0 {
		return &SourceTreeNode{Name: ".", IsDir: true}, nil
	}

	// 按 Source 分组
	sourceChunks := make(map[string][]core.Chunk) // source -> 顶层 Chunk（ParentID==""）
	allBySource := make(map[string][]core.Chunk)  // source -> 全部 Chunk
	for _, chunk := range deduped {
		src := chunk.Source
		if src == "" {
			continue
		}
		allBySource[src] = append(allBySource[src], chunk)
		if chunk.ParentID == "" {
			sourceChunks[src] = append(sourceChunks[src], chunk)
		}
	}

	// 构建目录树
	root := &SourceTreeNode{Name: ".", IsDir: true}
	for source, topChunks := range sourceChunks {
		allForSource := allBySource[source]

		var fileSize int64
		for _, c := range allForSource {
			fileSize += int64(len(c.Content))
		}

		fileNode := &SourceTreeNode{
			Name:   filepath.Base(source),
			Path:   source,
			Size:   fileSize,
			Chunks: buildChunkTree(topChunks, allBySource[source]),
		}

		insertIntoTree(root, source, fileNode)
	}

	// 将 README.md 文件节点的摘要折叠到父目录节点
	foldReadmeIntoDirectories(root)

	return root, nil
}

// buildChunkTree 为单个文件构建 Chunk 子树。
func buildChunkTree(parentChunks []core.Chunk, allChunks []core.Chunk) []SourceChunkNode {
	childMap := make(map[string][]core.Chunk)
	for _, c := range allChunks {
		if c.ParentID != "" {
			childMap[c.ParentID] = append(childMap[c.ParentID], c)
		}
	}

	nodes := make([]SourceChunkNode, 0, len(parentChunks))
	for _, pc := range parentChunks {
		// 每个顶层 Chunk 使用独立的访问集合，避免跨子树误判
		nodes = append(nodes, chunkToNode(pc, childMap, make(map[string]bool)))
	}
	return nodes
}

// chunkToNode 递归构建单个 Chunk 节点及其子块。
// visited 用于检测 ParentID 循环引用，防止异常数据导致无限递归。
func chunkToNode(chunk core.Chunk, childMap map[string][]core.Chunk, visited map[string]bool) SourceChunkNode {
	node := SourceChunkNode{
		Type:      chunkTypeFromSource(chunk.Source, chunk.Language),
		Title:     chunk.Title,
		Summary:   chunk.Summary,
		StartLine: chunk.StartLine,
		EndLine:   chunk.EndLine,
	}

	visited[chunk.ID] = true
	for _, child := range childMap[chunk.ID] {
		if visited[child.ID] {
			continue
		}
		node.Children = append(node.Children, chunkToNode(child, childMap, visited))
	}
	return node
}

// chunkTypeFromSource 根据 Source 路径和 Language 判断 Chunk 显示类型。
func chunkTypeFromSource(source, language string) string {
	if language != "" {
		return "代码"
	}
	ext := strings.ToLower(filepath.Ext(source))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".tif":
		return "图片"
	case ".csv", ".xlsx", ".json", ".yaml", ".yml", ".xml", ".toml", ".eml", ".msg", ".log":
		return "数据"
	default:
		return "文档"
	}
}

// insertIntoTree 将文件节点按 Source 路径插入到目录树中。
func insertIntoTree(root *SourceTreeNode, source string, fileNode *SourceTreeNode) {
	dir := filepath.Dir(source)
	if dir == "." || dir == "/" || dir == "" {
		root.Children = append(root.Children, fileNode)
		return
	}

	parts := strings.Split(dir, string(filepath.Separator))
	current := root
	for _, part := range parts {
		if part == "" {
			continue
		}
		found := false
		for _, child := range current.Children {
			if child.IsDir && child.Name == part {
				current = child
				found = true
				break
			}
		}
		if !found {
			dirNode := &SourceTreeNode{Name: part, IsDir: true}
			current.Children = append(current.Children, dirNode)
			current = dirNode
		}
	}
	current.Children = append(current.Children, fileNode)
}

// foldReadmeIntoDirectories 递归将 README.md 文件节点的摘要折叠到父目录节点，
// 并移除 README.md 文件节点，使其不在 tree 中单独显示。
func foldReadmeIntoDirectories(node *SourceTreeNode) {
	if node == nil {
		return
	}

	var filtered []*SourceTreeNode
	for _, child := range node.Children {
		if child.IsDir {
			foldReadmeIntoDirectories(child)
			filtered = append(filtered, child)
			continue
		}
		if strings.EqualFold(child.Name, "README.md") {
			node.Summary = collectReadmeSummary(child)
			continue
		}
		filtered = append(filtered, child)
	}
	node.Children = filtered
}

// collectReadmeSummary 收集 README.md 文件节点下顶层 Chunk 的摘要，
// 去重后拼接并截断到 200 字符。
func collectReadmeSummary(readmeNode *SourceTreeNode) string {
	if readmeNode == nil || len(readmeNode.Chunks) == 0 {
		return ""
	}

	var summaries []string
	for _, chunk := range readmeNode.Chunks {
		if chunk.Summary == "" {
			continue
		}
		if !contains(summaries, chunk.Summary) {
			summaries = append(summaries, chunk.Summary)
		}
	}

	if len(summaries) == 0 {
		return ""
	}

	summary := strings.Join(summaries, "；")
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	return summary
}
