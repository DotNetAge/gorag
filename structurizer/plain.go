package structurizer

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/DotNetAge/gorag/core"
)

// ClassificationConfig 分类规则配置
type ClassificationConfig struct {
	// 标题识别
	HeadingMinLength    int     // 标题最小长度
	HeadingMaxLength    int     // 标题最大长度
	HeadingConfidence   float64 // 标题分类置信度阈值

	// 代码块识别
	CodeIndentThreshold    float64 // 缩进特征阈值（有缩进行的比例）
	CodeCharThreshold      float64 // 代码字符特征阈值
	CodeMinLines           int     // 代码块最小行数

	// 列表识别
	ListMarkerPatterns []string // 列表标记模式

	// 引用识别
	QuoteMarker string // 引用标记
}

// DefaultConfig 返回默认配置
func DefaultConfig() *ClassificationConfig {
	return &ClassificationConfig{
		HeadingMinLength:    1,
		HeadingMaxLength:    80,
		HeadingConfidence:   0.7,

		CodeIndentThreshold:    0.5,
		CodeCharThreshold:      0.3,
		CodeMinLines:           2,

		ListMarkerPatterns:  []string{"-", "*", "+", "•"},
		QuoteMarker:         ">",
	}
}

// PlainTextStructurizer 纯文本结构化分析器
// 使用启发式规则将纯文本分割为标题、段落、列表、代码块等结构
type PlainTextStructurizer struct {
	config *ClassificationConfig
}

// NewPlainTextStructurizer 创建带默认配置的纯文本结构化分析器
func NewPlainTextStructurizer() *PlainTextStructurizer {
	return &PlainTextStructurizer{
		config: DefaultConfig(),
	}
}

// NewPlainTextStructurizerWithConfig 创建带自定义配置的纯文本结构化分析器
func NewPlainTextStructurizerWithConfig(config *ClassificationConfig) *PlainTextStructurizer {
	return &PlainTextStructurizer{
		config: config,
	}
}

// Parse 实现 Structurizer 接口
func (p *PlainTextStructurizer) Parse(raw core.Document) (*core.StructuredDocument, error) {
	text := strings.TrimSpace(raw.GetContent())
	if text == "" {
		sd := &core.StructuredDocument{
			RawDoc: raw,
			Title:  raw.GetSource(),
			Root:   emptyRoot(),
		}
		sd.SetValue("file", raw.GetSource())
		return sd, nil
	}

	// 1. 按空行分割成逻辑块
	blocks := p.splitIntoBlocks(text)

	// 2. 分类每个块（带上下文感知）
	classified := p.classifyBlocks(blocks)

	// 3. 构建层级树（自动推导标题层级）
	root := p.buildTree(classified)

	// 4. 提取文档标题（第一个标题或使用文件名）
	title := extractTitleFromRoot(root, raw.GetSource())

	sd := &core.StructuredDocument{
		RawDoc: raw,
		Title:  title,
		Root:   root,
	}
	sd.SetValue("file", raw.GetSource())
	sd.SetValue("total_lines", strings.Count(raw.GetContent(), "\n")+1)
	sd.SetValue("block_count", len(classified))
	return sd, nil
}

// splitIntoBlocks 按空行分割逻辑块
func (p *PlainTextStructurizer) splitIntoBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var blocks []string
	var currentBlock []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(currentBlock) > 0 {
				blocks = append(blocks, strings.Join(currentBlock, "\n"))
				currentBlock = nil
			}
		} else {
			currentBlock = append(currentBlock, line)
		}
	}

	if len(currentBlock) > 0 {
		blocks = append(blocks, strings.Join(currentBlock, "\n"))
	}
	return blocks
}

// Block 表示文本块的分类结果（带置信度）
type Block struct {
	Text        string
	Lines       []string
	LineCount   int
	Type        string  // heading, paragraph, list, code_block, quote, table, definition_list
	Level       int     // 仅对 heading 有效
	Confidence  float64 // 分类置信度 (0.0 - 1.0)
	StartLine   int     // 起始行号
	EndLine     int     // 结束行号

	// 列表特定属性
	IsOrdered   bool    // 是否有序列表
	ListItems   []string // 列表项

	// 表格特定属性
	HasTable    bool
	TableRows   [][]string

	// 任务列表
	IsTaskList  bool
	TaskItems   []TaskItem
}

// TaskItem 任务列表项
type TaskItem struct {
	Text     string
	Checked  bool
}

// 预编译正则表达式（性能优化）
var (
	// 标题匹配
	headingUnderlineRegex = regexp.MustCompile(`^[=]+$|^[-]+$`)
	sectionNumberRegex    = regexp.MustCompile(`^第[一二三四五六七八九十百千万零]+[章节条款]|^[0-9]+(\.[0-9]+)*\s+|^[一二三四五六七八九十]+[、.]\s+`)
	chapterNumberRegex    = regexp.MustCompile(`^[一二三四五六七八九十]+`)

	// 列表匹配
	orderedListRegex = regexp.MustCompile(`^\s*[0-9]+[.)]\s+`)

	// 表格匹配
	tableRowRegex = regexp.MustCompile(`^\|.*\|$|^[^\|]+\|[^\|]+`)

	// 代码特征
	codeCharsRegex = regexp.MustCompile(`[{}\(\)\[\];=<>!&|+\-*/%]`)

	// 定义列表
	definitionTermRegex = regexp.MustCompile(`^\s*[^:]+:\s*$|^\s*[^:]+：\s*$`)

	// 任务列表
	taskListRegex = regexp.MustCompile(`^\s*[-*+]\s+\[([ xX])\]\s+`)

	// 引用
	quoteRegex = regexp.MustCompile(`^\s*>\s*`)
)

// classifyBlocks 分类所有块（带上下文感知）
func (p *PlainTextStructurizer) classifyBlocks(blocks []string) []*Block {
	result := make([]*Block, 0, len(blocks))
	lineNum := 1

	for i, block := range blocks {
		block = strings.TrimSpace(block)
		lines := strings.Split(block, "\n")
		lineCount := len(lines)

		classified := p.classifyBlock(block, lines, lineCount, i, blocks)
		classified.StartLine = lineNum
		classified.EndLine = lineNum + lineCount - 1
		classified.Lines = lines

		result = append(result, classified)
		lineNum += lineCount + 1 // +1 for empty line
	}

	return result
}

// classifyBlock 分类单个块（核心分类逻辑）
func (p *PlainTextStructurizer) classifyBlock(block string, lines []string, lineCount, blockIndex int, allBlocks []string) *Block {
	firstLine := strings.TrimSpace(lines[0])
	lastLine := strings.TrimSpace(lines[lineCount-1])

	result := &Block{
		Text:       block,
		LineCount:  lineCount,
		Type:       "paragraph",
		Confidence: 0.5,
	}

	// === 1. 检测表格 ===
	if p.isTable(lines) {
		result.Type = "table"
		result.Confidence = 0.9
		result.HasTable = true
		result.TableRows = p.parseTableRows(lines)
		return result
	}

	// === 2. 检测任务列表 ===
	if isTaskList, items := p.detectTaskList(lines); isTaskList {
		result.Type = "list"
		result.Confidence = 0.95
		result.IsTaskList = true
		result.TaskItems = items
		return result
	}

	// === 3. 检测普通列表 ===
	if isList, isOrdered, items := p.detectList(lines); isList {
		result.Type = "list"
		result.Confidence = 0.85
		result.IsOrdered = isOrdered
		result.ListItems = items
		return result
	}

	// === 4. 检测定义列表 ===
	if p.isDefinitionList(lines) {
		result.Type = "definition_list"
		result.Confidence = 0.8
		return result
	}

	// === 5. 检测引用 ===
	if p.isQuote(lines) {
		result.Type = "quote"
		result.Confidence = 0.85
		return result
	}

	// === 6. 检测标题（优先级高） ===
	if headingType, level, confidence := p.detectHeading(blockIndex, allBlocks, firstLine, lastLine, lineCount); headingType != "" {
		result.Type = "heading"
		result.Level = level
		result.Confidence = confidence
		return result
	}

	// === 7. 检测代码块 ===
	if p.isCodeBlock(lines, lineCount) {
		result.Type = "code_block"
		result.Confidence = 0.8
		return result
	}

	// === 8. 默认：段落 ===
	result.Type = "paragraph"
	result.Confidence = 0.6

	// 上下文感知：如果前后都是代码块，这个段落可能是代码注释
	if blockIndex > 0 && blockIndex < len(allBlocks)-1 {
		prevType := p.quickClassify(strings.TrimSpace(allBlocks[blockIndex-1]))
		nextType := p.quickClassify(strings.TrimSpace(allBlocks[blockIndex+1]))
		if prevType == "code_block" && nextType == "code_block" {
			result.Type = "code_block"
			result.Confidence = 0.7
		}
	}

	return result
}

// quickClassify 快速分类（用于上下文感知）
func (p *PlainTextStructurizer) quickClassify(block string) string {
	lines := strings.Split(block, "\n")
	if len(lines) >= 2 {
		codeScore := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
				codeScore++
			}
		}
		if float64(codeScore)/float64(len(lines)) >= p.config.CodeIndentThreshold {
			return "code_block"
		}
	}
	return "paragraph"
}

// detectHeading 检测标题
func (p *PlainTextStructurizer) detectHeading(blockIndex int, allBlocks []string, firstLine, lastLine string, lineCount int) (string, int, float64) {
	// Setext 风格标题（当前块是标题文本，下一块是 === 或 ---）
	if blockIndex+1 < len(allBlocks) {
		nextBlock := strings.TrimSpace(allBlocks[blockIndex+1])
		if headingUnderlineRegex.MatchString(nextBlock) {
			level := 2
			if strings.HasPrefix(nextBlock, "=") {
				level = 1
			}
			return "heading", level, 0.95
		}
	}

	// 带章节编号的标题
	if sectionNumberRegex.MatchString(firstLine) {
		level := p.extractHeadingLevel(firstLine)
		return "heading", level, 0.9
	}

	// 单行短文本 + 全大写 或 首字母大写其余小写（标题风格）
	if lineCount == 1 && len(firstLine) >= p.config.HeadingMinLength && len(firstLine) <= p.config.HeadingMaxLength {
		// 全大写
		if isAllUpper(firstLine) && hasLetters(firstLine) {
			return "heading", 3, 0.75
		}
		// 标题风格（每个单词首字母大写）
		if isTitleCase(firstLine) && hasLetters(firstLine) {
			return "heading", 2, 0.7
		}
	}

	// 单行 + 较短 + 不以标点结尾
	if lineCount == 1 && len(firstLine) < 50 && !endsWithPunctuation(firstLine) && hasLetters(firstLine) {
		return "heading", 4, 0.6
	}

	return "", 0, 0
}

// extractHeadingLevel 从编号提取标题层级
func (p *PlainTextStructurizer) extractHeadingLevel(text string) int {
	numStr := sectionNumberRegex.FindString(text)

	// 检测中文章节编号
	if strings.HasPrefix(numStr, "第") {
		if strings.Contains(numStr, "章") {
			return 1
		} else if strings.Contains(numStr, "节") {
			return 2
		} else if strings.Contains(numStr, "条") {
			return 3
		}
		return 2
	}

	// 检测数字编号 (1.1.1)
	dots := strings.Count(numStr, ".")
	level := dots + 1
	if level > 6 {
		level = 6
	}

	// 中文数字编号（一、二、三）
	if chapterNumberRegex.MatchString(numStr) {
		return 2
	}

	return level
}

// isTable 检测表格
func (p *PlainTextStructurizer) isTable(lines []string) bool {
	if len(lines) < 2 {
		return false
	}

	tableLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if tableRowRegex.MatchString(trimmed) {
			tableLines++
		}
	}

	return float64(tableLines)/float64(len(lines)) >= 0.8
}

// parseTableRows 解析表格行
func (p *PlainTextStructurizer) parseTableRows(lines []string) [][]string {
	var rows [][]string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !tableRowRegex.MatchString(trimmed) {
			continue
		}

		// 跳过分隔行
		if isTableSeparator(trimmed) {
			continue
		}

		cells := parseTableRow(trimmed)
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	return rows
}

// isTableSeparator 检测表格分隔行
func isTableSeparator(line string) bool {
	line = strings.Trim(line, "| ")
	for _, c := range line {
		if c != '-' && c != ':' && c != ' ' {
			return false
		}
	}
	return len(line) > 0
}

// parseTableRow 解析表格行
func parseTableRow(line string) []string {
	line = strings.Trim(line, "| ")
	parts := strings.Split(line, "|")
	var cells []string
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// detectTaskList 检测任务列表
func (p *PlainTextStructurizer) detectTaskList(lines []string) (bool, []TaskItem) {
	var items []TaskItem
	taskCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		matches := taskListRegex.FindStringSubmatch(trimmed)
		if matches != nil {
			taskCount++
			checked := strings.ToLower(matches[1]) == "x"
			text := taskListRegex.ReplaceAllString(trimmed, "")
			items = append(items, TaskItem{
				Text:    strings.TrimSpace(text),
				Checked: checked,
			})
		}
	}

	// 至少有一半是任务项
	if taskCount > 0 && float64(taskCount)/float64(len(lines)) >= 0.5 {
		return true, items
	}
	return false, nil
}

// detectList 检测列表
func (p *PlainTextStructurizer) detectList(lines []string) (bool, bool, []string) {
	orderedCount := 0
	unorderedCount := 0
	var items []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 有序列表
		if orderedListRegex.MatchString(trimmed) {
			orderedCount++
			items = append(items, orderedListRegex.ReplaceAllString(trimmed, ""))
			continue
		}

		// 无序列表（性能优化：使用 HasPrefix）
		for _, marker := range p.config.ListMarkerPatterns {
			if strings.HasPrefix(trimmed, marker+" ") || strings.HasPrefix(trimmed, marker+"\t") {
				unorderedCount++
				text := strings.TrimPrefix(trimmed, marker+" ")
				text = strings.TrimPrefix(text, marker+"\t")
				items = append(items, strings.TrimSpace(text))
				break
			}
		}
	}

	total := orderedCount + unorderedCount
	if total == 0 {
		return false, false, nil
	}

	// 至少有一半是列表项
	if float64(total)/float64(len(lines)) >= 0.5 {
		return true, orderedCount > unorderedCount, items
	}

	return false, false, nil
}

// isDefinitionList 检测定义列表
func (p *PlainTextStructurizer) isDefinitionList(lines []string) bool {
	defCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if definitionTermRegex.MatchString(trimmed) {
			defCount++
		}
	}
	return defCount >= 2
}

// isQuote 检测引用
func (p *PlainTextStructurizer) isQuote(lines []string) bool {
	quoteCount := 0
	for _, line := range lines {
		if quoteRegex.MatchString(line) {
			quoteCount++
		}
	}
	return float64(quoteCount)/float64(len(lines)) >= 0.7
}

// isCodeBlock 检测代码块
func (p *PlainTextStructurizer) isCodeBlock(lines []string, lineCount int) bool {
	if lineCount < p.config.CodeMinLines {
		return false
	}

	indentScore := 0
	charScore := 0

	for _, line := range lines {
		// 缩进特征
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			indentScore++
		}
		// 代码字符特征
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && codeCharsRegex.MatchString(trimmed) {
			charScore++
		}
	}

	indentRatio := float64(indentScore) / float64(lineCount)
	charRatio := float64(charScore) / float64(lineCount)

	return indentRatio >= p.config.CodeIndentThreshold || charRatio >= p.config.CodeCharThreshold
}

// buildTree 构建层级树
func (p *PlainTextStructurizer) buildTree(blocks []*Block) *core.StructureNode {
	root := emptyRoot()
	var stack []*core.StructureNode

	for _, block := range blocks {
		node := &core.StructureNode{
			NodeType: block.Type,
			Text:     block.Text,
			Level:    block.Level,
		}

		if block.Type == "heading" {
			// 弹出所有大于等于当前标题层级的节点
			for len(stack) > 0 && stack[len(stack)-1].Level >= block.Level {
				stack = stack[:len(stack)-1]
			}

			// 设置标题文本
			node.Title = extractHeadingTitle(block.Text)

			// 添加到树中
			if len(stack) == 0 {
				root.Children = append(root.Children, node)
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			}

			// 当前标题入栈
			stack = append(stack, node)
		} else {
			// 非标题节点：添加到当前最近的标题下
			if len(stack) == 0 {
				root.Children = append(root.Children, node)
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			}
		}
	}

	root.Clean()
	return root
}

// extractHeadingTitle 从标题文本中提取纯标题
func extractHeadingTitle(text string) string {
	// 去除章节编号
	title := sectionNumberRegex.ReplaceAllString(text, "")
	title = strings.TrimSpace(title)
	if title == "" {
		return text
	}
	return title
}

// extractTitleFromRoot 从根节点提取文档标题
func extractTitleFromRoot(root *core.StructureNode, source string) string {
	for _, child := range root.Children {
		if child.NodeType == "heading" && child.Title != "" {
			return child.Title
		}
	}
	return source
}

// emptyRoot 创建空的根节点
func emptyRoot() *core.StructureNode {
	return &core.StructureNode{
		NodeType: "document",
		Title:    "Document",
		Children: []*core.StructureNode{},
	}
}

// === 辅助函数 ===

func isAllUpper(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func isTitleCase(s string) bool {
	words := strings.Fields(s)
	titleWords := 0
	for _, word := range words {
		if len(word) == 0 {
			continue
		}
		runes := []rune(word)
		if unicode.IsUpper(runes[0]) {
			// 检查其余是否小写
			allLower := true
			for _, r := range runes[1:] {
				if unicode.IsLetter(r) && unicode.IsUpper(r) {
					allLower = false
					break
				}
			}
			if allLower {
				titleWords++
			}
		}
	}
	return titleWords > 0 && float64(titleWords)/float64(len(words)) >= 0.6
}

func hasLetters(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func endsWithPunctuation(s string) bool {
	if len(s) == 0 {
		return false
	}
	last := rune(s[len(s)-1])
	return last == '.' || last == '。' || last == '!' || last == '！' ||
		last == '?' || last == '？' || last == ',' || last == '，' ||
		last == ';' || last == '；' || last == ':'
}
