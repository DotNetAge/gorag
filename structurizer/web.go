package structurizer

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"golang.org/x/net/html"
)

// WebStructurizer HTML/XML 结构化分析器
// 支持解析 HTML 和 XML 文档，提取文档结构树
type WebStructurizer struct {
	// SkipTags 跳过处理的标签列表（如 script、style 等）
	SkipTags map[string]bool
	// InlineTags 内联标签列表（不作为独立节点）
	InlineTags map[string]bool
	// ExtractAttributes 需要提取的属性列表
	ExtractAttributes []string
}

// NewWebStructurizer 创建默认配置的 Web 结构化分析器
func NewWebStructurizer() *WebStructurizer {
	return &WebStructurizer{
		SkipTags: map[string]bool{
			"script": true, "style": true, "noscript": true,
			"iframe": true, "object": true, "embed": true,
			"svg": true, "math": true, "template": true,
		},
		InlineTags: map[string]bool{
			"span": true, "a": true, "strong": true, "em": true,
			"b": true, "i": true, "u": true, "s": true,
			"code": true, "kbd": true, "samp": true, "var": true,
			"abbr": true, "cite": true, "dfn": true, "mark": true,
			"q": true, "small": true, "sub": true, "sup": true,
			"time": true, "wbr": true,
		},
		ExtractAttributes: []string{"id", "class", "href", "src", "alt", "title", "name", "type", "value"},
	}
}

// Parse 实现 Structurizer 接口
func (w *WebStructurizer) Parse(doc core.Document) (*core.StructuredDocument, error) {
	content := doc.GetContent()
	ext := strings.ToLower(doc.GetExt())

	var root *core.StructureNode
	var err error

	switch ext {
	case "html", "htm":
		root, err = w.parseHTML(content)
	case "xml":
		root, err = w.parseXML(content)
	default:
		// 尝试自动检测
		if strings.HasPrefix(strings.TrimSpace(content), "<!DOCTYPE html") ||
			strings.HasPrefix(strings.TrimSpace(content), "<html") {
			root, err = w.parseHTML(content)
		} else {
			root, err = w.parseXML(content)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("parse %s failed: %w", ext, err)
	}

	// 提取文档标题
	title := w.extractTitle(root, doc.GetSource())

	// 统计节点数量
	nodeCount := countStructureNodes(root)

	// 清洗节点文本
	root.Clean()

	sd := &core.StructuredDocument{
		RawDoc: doc,
		Title:  title,
		Root:   root,
	}
	sd.SetValue("file", doc.GetSource())
	sd.SetValue("format", ext)
	sd.SetValue("node_count", nodeCount)
	return sd, nil
}

// parseHTML 解析 HTML 内容
func (w *WebStructurizer) parseHTML(content string) (*core.StructureNode, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("invalid HTML: %w", err)
	}

	root := &core.StructureNode{
		NodeType: "document",
		Title:    "HTML Document",
		Children: []*core.StructureNode{},
	}

	w.buildHTMLNode(root, doc, 0)
	return root, nil
}

// buildHTMLNode 递归构建 HTML 节点树
func (w *WebStructurizer) buildHTMLNode(parent *core.StructureNode, node *html.Node, depth int) {
	if node == nil {
		return
	}

	switch node.Type {
	case html.DocumentNode:
		// 文档节点，递归处理子节点
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			w.buildHTMLNode(parent, child, depth)
		}

	case html.ElementNode:
		tagName := strings.ToLower(node.Data)

		// 跳过特定标签
		if w.SkipTags[tagName] {
			return
		}

		// 创建结构节点
		structNode := w.createHTMLStructureNode(node, depth)

		// 递归处理子节点
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			w.buildHTMLNode(structNode, child, depth+1)
		}

		// 内联标签不作为独立节点，内容合并到父节点
		if w.InlineTags[tagName] && parent != nil {
			// 提取文本内容追加到父节点
			if len(structNode.Children) > 0 {
				parent.Children = append(parent.Children, structNode.Children...)
			}
		} else {
			parent.Children = append(parent.Children, structNode)
		}

	case html.TextNode:
		text := strings.TrimSpace(node.Data)
		if text != "" && parent != nil {
			// 文本节点追加到父节点
			if parent.Text != "" {
				parent.Text += " " + text
			} else {
				parent.Text = text
			}
		}

	case html.CommentNode:
		// 忽略注释节点
	}
}

// createHTMLStructureNode 创建 HTML 结构节点
func (w *WebStructurizer) createHTMLStructureNode(node *html.Node, depth int) *core.StructureNode {
	tagName := strings.ToLower(node.Data)
	attrs := w.extractHTMLAttributes(node)

	structNode := &core.StructureNode{
		NodeType: w.mapHTMLTagToNodeType(tagName),
		Level:    depth,
		Children: []*core.StructureNode{},
	}

	// 设置标题/文本
	switch tagName {
	case "h1":
		structNode.Title = w.extractTextContent(node)
		structNode.Level = 1
	case "h2":
		structNode.Title = w.extractTextContent(node)
		structNode.Level = 2
	case "h3":
		structNode.Title = w.extractTextContent(node)
		structNode.Level = 3
	case "h4":
		structNode.Title = w.extractTextContent(node)
		structNode.Level = 4
	case "h5":
		structNode.Title = w.extractTextContent(node)
		structNode.Level = 5
	case "h6":
		structNode.Title = w.extractTextContent(node)
		structNode.Level = 6
	case "title":
		structNode.Title = w.extractTextContent(node)
		structNode.Text = structNode.Title
	case "img":
		structNode.Title = attrs["alt"]
		if structNode.Title == "" {
			structNode.Title = attrs["src"]
		}
		structNode.Text = fmt.Sprintf("[Image: %s]", attrs["src"])
	case "a":
		structNode.Title = w.extractTextContent(node)
		structNode.Text = attrs["href"]
	case "input":
		structNode.Title = attrs["name"]
		if structNode.Title == "" {
			structNode.Title = attrs["id"]
		}
		structNode.Text = fmt.Sprintf("%s: %s", attrs["type"], attrs["value"])
	default:
		// 其他标签使用 id 或 class 作为标题
		if id := attrs["id"]; id != "" {
			structNode.Title = fmt.Sprintf("#%s", id)
		} else if class := attrs["class"]; class != "" {
			structNode.Title = fmt.Sprintf(".%s", strings.Fields(class)[0])
		}
	}

	// 将关键属性存入 Title 后缀
	if id := attrs["id"]; id != "" && structNode.Title == "" {
		structNode.Title = id
	}

	return structNode
}

// mapHTMLTagToNodeType 映射 HTML 标签到节点类型
func (w *WebStructurizer) mapHTMLTagToNodeType(tagName string) string {
	switch tagName {
	case "html":
		return "document"
	case "head":
		return "head"
	case "body":
		return "body"
	case "header", "footer", "nav", "main", "aside", "article", "section":
		return "section"
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return "heading"
	case "p":
		return "paragraph"
	case "ul", "ol", "dl":
		return "list"
	case "li", "dt", "dd":
		return "list_item"
	case "table":
		return "table"
	case "thead", "tbody", "tfoot":
		return "table_section"
	case "tr":
		return "table_row"
	case "td", "th":
		return "table_cell"
	case "form":
		return "form"
	case "input", "select", "textarea", "button":
		return "input"
	case "img", "video", "audio", "picture", "figure":
		return "media"
	case "a":
		return "link"
	case "div", "span":
		return "container"
	case "pre", "code":
		return "code_block"
	case "blockquote":
		return "quote"
	default:
		return "element"
	}
}

// extractHTMLAttributes 提取 HTML 属性
func (w *WebStructurizer) extractHTMLAttributes(node *html.Node) map[string]string {
	attrs := make(map[string]string)
	for _, attr := range node.Attr {
		// 只提取预定义的属性
		for _, extractAttr := range w.ExtractAttributes {
			if strings.EqualFold(attr.Key, extractAttr) {
				attrs[extractAttr] = attr.Val
				break
			}
		}
		// id 和 class 始终提取
		if strings.EqualFold(attr.Key, "id") || strings.EqualFold(attr.Key, "class") {
			attrs[strings.ToLower(attr.Key)] = attr.Val
		}
	}
	return attrs
}

// extractTextContent 提取节点的纯文本内容
func (w *WebStructurizer) extractTextContent(node *html.Node) string {
	var texts []string
	var extract func(n *html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			if text := strings.TrimSpace(n.Data); text != "" {
				texts = append(texts, text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			extract(child)
		}
	}
	extract(node)
	return strings.Join(texts, " ")
}

// parseXML 解析 XML 内容
func (w *WebStructurizer) parseXML(content string) (*core.StructureNode, error) {
	decoder := xml.NewDecoder(strings.NewReader(content))
	decoder.Strict = false // 宽容模式

	root := &core.StructureNode{
		NodeType: "document",
		Title:    "XML Document",
		Children: []*core.StructureNode{},
	}

	// 使用通用 XML 解析
	err := w.buildXMLNode(root, decoder, 0)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("invalid XML: %w", err)
	}

	return root, nil
}

// buildXMLNode 递归构建 XML 节点树
func (w *WebStructurizer) buildXMLNode(parent *core.StructureNode, decoder *xml.Decoder, depth int) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// 创建元素节点
			structNode := &core.StructureNode{
				NodeType: "element",
				Title:    t.Name.Local,
				Level:    depth,
				Children: []*core.StructureNode{},
			}

			// 提取属性
			for _, attr := range t.Attr {
				if structNode.Text != "" {
					structNode.Text += " "
				}
				structNode.Text += fmt.Sprintf("%s=\"%s\"", attr.Name.Local, attr.Value)
			}

			// 递归处理子节点
			err := w.buildXMLNode(structNode, decoder, depth+1)
			if err != nil && err != io.EOF {
				return err
			}

			parent.Children = append(parent.Children, structNode)

		case xml.EndElement:
			// 结束当前元素
			return nil

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				if parent.Text != "" {
					parent.Text += " " + text
				} else {
					parent.Text = text
				}
			}

		case xml.Comment:
			// 忽略注释

		case xml.ProcInst:
			// 处理指令（如 <?xml version="1.0"?>）
			if t.Target == "xml" {
				parent.Title = "XML Document"
			}

		case xml.Directive:
			// 忽略指令（如 <!DOCTYPE>）
		}
	}
}

// extractTitle 提取文档标题
func (w *WebStructurizer) extractTitle(root *core.StructureNode, source string) string {
	// 从 <title> 标签提取
	var findTitle func(node *core.StructureNode) string
	findTitle = func(node *core.StructureNode) string {
		if node.NodeType == "element" && node.Title == "title" && node.Text != "" {
			return node.Text
		}
		for _, child := range node.Children {
			if title := findTitle(child); title != "" {
				return title
			}
		}
		return ""
	}

	if title := findTitle(root); title != "" {
		return title
	}

	// 从第一个 h1 标签提取
	var findH1 func(node *core.StructureNode) string
	findH1 = func(node *core.StructureNode) string {
		if node.NodeType == "heading" && node.Level == 1 && node.Title != "" {
			return node.Title
		}
		for _, child := range node.Children {
			if title := findH1(child); title != "" {
				return title
			}
		}
		return ""
	}

	if title := findH1(root); title != "" {
		return title
	}

	// 默认使用文件名
	return source
}
