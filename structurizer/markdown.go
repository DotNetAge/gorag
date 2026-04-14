package structurizer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/core"
	sitter "github.com/smacker/go-tree-sitter"
	markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
)

type MarkdownStructurizer struct {
	parser *sitter.Parser
}

func NewMarkdownStructurizer() *MarkdownStructurizer {
	parser := sitter.NewParser()
	parser.SetLanguage(markdown.GetLanguage())

	return &MarkdownStructurizer{parser: parser}
}

func (ms *MarkdownStructurizer) Close() {
	if ms.parser != nil {
		ms.parser.Close()
	}
}

func (ms *MarkdownStructurizer) Parse(raw core.Document) (*core.StructuredDocument, error) {
	tree, err := ms.parser.ParseCtx(context.Background(), nil, []byte(raw.GetContent()))
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	content := []byte(raw.GetContent())

	root := ms.buildStructureTree(content, rootNode)

	title := filepath.Base(raw.GetSource())
	if t, ok := raw.GetMeta()["title"].(string); ok && t != "" {
		title = t
	}

	// metadata := map[string]any{
	// 	"totalLines": strings.Count(raw.Content, "\n") + 1,
	// 	"nodeCount":  countNodes(root),
	// }
	sd := &core.StructuredDocument{
		Title: title,
		Root:  root,
	}

	return sd.SetValue("total_lines", strings.Count(raw.GetContent(), "\n")+1).
		SetValue("node_count", countNodes(root)), nil
}

func (ms *MarkdownStructurizer) buildStructureTree(content []byte, node *sitter.Node) *core.StructureNode {
	if node == nil || !node.IsNamed() {
		return nil
	}

	nodeType := node.Type()

	// Skip document root
	if nodeType == "document" {
		children := ms.collectChildren(content, node)
		if len(children) == 0 {
			return nil
		}
		// Return first significant child or wrap in a document node
		if len(children) == 1 {
			return children[0]
		}
		return &core.StructureNode{
			NodeType: "document",
			Title:    "Document",
			Children: children,
		}
	}

	structureNode := ms.parseNode(content, node)
	if structureNode == nil {
		return nil
	}

	// Collect children
	structureNode.Children = ms.collectChildren(content, node)
	structureNode.Clean()

	return structureNode
}

func (ms *MarkdownStructurizer) collectChildren(content []byte, node *sitter.Node) []*core.StructureNode {
	var children []*core.StructureNode

	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if childNode := ms.buildStructureTree(content, child); childNode != nil {
			children = append(children, childNode)
		}
	}

	return children
}

func (ms *MarkdownStructurizer) parseNode(content []byte, node *sitter.Node) *core.StructureNode {
	nodeType := node.Type()

	startPos := int(node.StartByte())
	endPos := int(node.EndByte())

	if startPos >= endPos || endPos > len(content) {
		return nil
	}

	text := string(content[startPos:endPos])

	switch nodeType {
	case "atx_heading", "setext_heading":
		return ms.parseHeading(content, node, text)

	case "paragraph":
		return &core.StructureNode{
			NodeType: "paragraph",
			Text:     strings.TrimSpace(text),
			StartPos: startPos,
			EndPos:   endPos,
		}

	case "list":
		return ms.parseList(content, node, text)

	case "list_item":
		return &core.StructureNode{
			NodeType: "list_item",
			Text:     strings.TrimSpace(text),
			StartPos: startPos,
			EndPos:   endPos,
		}

	case "fenced_code_block", "indented_code_block":
		return ms.parseCodeBlock(content, node, text)

	case "table":
		return ms.parseTable(content, node, text)

	case "image":
		return ms.parseImage(content, node, text)

	case "block_quote":
		return &core.StructureNode{
			NodeType: "block_quote",
			Text:     strings.TrimSpace(text),
			StartPos: startPos,
			EndPos:   endPos,
		}

	case "thematic_break":
		return &core.StructureNode{
			NodeType: "thematic_break",
			Text:     text,
			StartPos: startPos,
			EndPos:   endPos,
		}

	case "link":
		return ms.parseLink(content, node, text)

	default:
		// Return a generic node for other types
		return &core.StructureNode{
			NodeType: nodeType,
			Text:     strings.TrimSpace(text),
			StartPos: startPos,
			EndPos:   endPos,
		}
	}
}

func (ms *MarkdownStructurizer) parseHeading(content []byte, node *sitter.Node, text string) *core.StructureNode {
	level := ms.extractHeadingLevel(node, content)

	// Extract heading text (remove # symbols)
	title := ms.extractHeadingText(node, content)

	return &core.StructureNode{
		NodeType: "heading",
		Title:    title,
		Level:    level,
		Text:     strings.TrimSpace(text),
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}
}

func (ms *MarkdownStructurizer) extractHeadingLevel(node *sitter.Node, content []byte) int {
	// ATX headings: # heading, ## heading, etc.
	// Setext headings: underlined with = (level 1) or - (level 2)

	nodeType := node.Type()

	if nodeType == "setext_heading" {
		// Check the underline character
		childCount := int(node.ChildCount())
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child != nil && child.Type() == "setext_h1_underline" {
				return 1
			}
			if child != nil && child.Type() == "setext_h2_underline" {
				return 2
			}
		}
		return 1 // default
	}

	// ATX heading - check marker type
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "atx_h1_marker":
			return 1
		case "atx_h2_marker":
			return 2
		case "atx_h3_marker":
			return 3
		case "atx_h4_marker":
			return 4
		case "atx_h5_marker":
			return 5
		case "atx_h6_marker":
			return 6
		}
	}

	// Fallback: count # from content
	headingContent := string(content[node.StartByte():node.EndByte()])
	count := 0
	for _, c := range headingContent {
		if c == '#' {
			count++
		} else {
			break
		}
	}
	if count > 0 && count <= 6 {
		return count
	}

	return 1
}

func (ms *MarkdownStructurizer) extractHeadingText(node *sitter.Node, content []byte) string {
	// Find the heading_content child
	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type() == "heading_content" {
			return strings.TrimSpace(child.Content(content))
		}
	}

	// Fallback: get text content and strip # markers
	text := string(content[node.StartByte():node.EndByte()])
	text = strings.TrimLeft(text, "#")
	text = strings.TrimSpace(text)
	return text
}

func (ms *MarkdownStructurizer) parseList(content []byte, node *sitter.Node, text string) *core.StructureNode {
	// Determine if it's ordered or unordered
	isOrdered := false
	isTaskList := false

	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "list_marker_dot" || childType == "list_marker_parenthesis" {
			isOrdered = true
		}
		if childType == "task_list_marker_unchecked" || childType == "task_list_marker_checked" {
			isTaskList = true
		}
	}

	nodeTypeStr := "list"
	if isTaskList {
		nodeTypeStr = "task_list"
	} else if isOrdered {
		nodeTypeStr = "ordered_list"
	} else {
		nodeTypeStr = "unordered_list"
	}

	return &core.StructureNode{
		NodeType: nodeTypeStr,
		Text:     strings.TrimSpace(text),
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}
}

func (ms *MarkdownStructurizer) parseCodeBlock(content []byte, node *sitter.Node, text string) *core.StructureNode {
	// Extract language info for fenced code blocks
	language := ms.extractCodeLanguage(node, content)

	// Extract the actual code content
	codeContent := ms.extractCodeContent(node, content)

	return &core.StructureNode{
		NodeType: "code_block",
		Text:     language + "\n" + codeContent,
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}
}

func (ms *MarkdownStructurizer) extractCodeLanguage(node *sitter.Node, content []byte) string {
	// Look for info_string child
	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type() == "info_string" {
			return strings.TrimSpace(child.Content(content))
		}
	}
	return ""
}

func (ms *MarkdownStructurizer) extractCodeContent(node *sitter.Node, content []byte) string {
	var lines []string
	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "code_fence_content" || child.Type() == "code_block_content" {
			lines = append(lines, child.Content(content))
		}
	}

	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	// Fallback: return raw text
	return string(content[node.StartByte():node.EndByte()])
}

func (ms *MarkdownStructurizer) parseTable(content []byte, node *sitter.Node, text string) *core.StructureNode {
	return &core.StructureNode{
		NodeType: "table",
		Text:     strings.TrimSpace(text),
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}
}

func (ms *MarkdownStructurizer) parseImage(content []byte, node *sitter.Node, text string) *core.StructureNode {
	altText := ""
	url := ""

	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "image_description", "link_text":
			altText = strings.TrimSpace(child.Content(content))
		case "link_destination":
			url = strings.TrimSpace(child.Content(content))
		}
	}

	if altText == "" && url == "" {
		altText = strings.TrimSpace(text)
	}

	result := &core.StructureNode{
		NodeType: "image",
		Text:     altText,
		Title:    altText,
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// URL stored in Title for now since no Metadata field
	if url != "" && altText == "" {
		result.Title = url
	}

	return result
}

func (ms *MarkdownStructurizer) parseLink(content []byte, node *sitter.Node, text string) *core.StructureNode {
	linkText := ""
	url := ""

	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "link_text", "link_title":
			linkText = strings.TrimSpace(child.Content(content))
		case "link_destination":
			url = strings.TrimSpace(child.Content(content))
		}
	}

	if linkText == "" && url == "" {
		linkText = strings.TrimSpace(text)
	}

	result := &core.StructureNode{
		NodeType: "link",
		Text:     linkText,
		Title:    linkText,
		StartPos: int(node.StartByte()),
		EndPos:   int(node.EndByte()),
	}

	// URL stored in Title for now since no Metadata field
	if url != "" && linkText == "" {
		result.Title = url
	}

	return result
}
