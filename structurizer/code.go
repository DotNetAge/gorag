package structurizer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/core"
	sitter "github.com/smacker/go-tree-sitter"
	clang "github.com/smacker/go-tree-sitter/c"
	cpp "github.com/smacker/go-tree-sitter/cpp"
	golang "github.com/smacker/go-tree-sitter/golang"
	javal "github.com/smacker/go-tree-sitter/java"
	javascript "github.com/smacker/go-tree-sitter/javascript"
	python "github.com/smacker/go-tree-sitter/python"
	rust "github.com/smacker/go-tree-sitter/rust"
	tsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

type CodeStructurizer struct {
	languageParsers map[string]*sitter.Parser
}

func NewCodeStructurizer() *CodeStructurizer {
	cs := &CodeStructurizer{
		languageParsers: make(map[string]*sitter.Parser),
	}

	cs.registerParser("go", golang.GetLanguage())
	cs.registerParser("javascript", javascript.GetLanguage())
	cs.registerParser("js", javascript.GetLanguage())
	cs.registerParser("python", python.GetLanguage())
	cs.registerParser("py", python.GetLanguage())
	cs.registerParser("typescript", typescript.GetLanguage())
	cs.registerParser("ts", typescript.GetLanguage())
	cs.registerParser("tsx", tsx.GetLanguage())
	cs.registerParser("rust", rust.GetLanguage())
	cs.registerParser("c", clang.GetLanguage())
	cs.registerParser("cpp", cpp.GetLanguage())
	cs.registerParser("java", javal.GetLanguage())

	return cs
}

func (cs *CodeStructurizer) registerParser(lang string, langType *sitter.Language) {
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(langType)
	cs.languageParsers[lang] = parser
}

func (cs *CodeStructurizer) Parse(raw core.Document) (*core.StructuredDocument, error) {
	ext := raw.GetExt()

	var lang string
	switch ext {
	case ".go":
		lang = "go"
	case ".js", ".mjs", ".cjs":
		lang = "javascript"
	case ".ts":
		lang = "typescript"
	case ".tsx":
		lang = "tsx"
	case ".py":
		lang = "python"
	case ".rs":
		lang = "rust"
	case ".c", ".h":
		lang = "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		lang = "cpp"
	case ".java":
		lang = "java"
	default:
		return nil, fmt.Errorf("unsupported source file extension: %s", ext)
	}

	parser, ok := cs.languageParsers[lang]
	if !ok {
		return nil, fmt.Errorf("no parser available for language: %s", lang)
	}

	tree, err := parser.ParseCtx(context.Background(), nil, []byte(raw.GetContent()))
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()

	root := cs.buildStructureTree(raw.GetContent(), rootNode, lang)

	title := filepath.Base(raw.GetSource())
	if t, ok := raw.GetMeta()["title"].(string); ok && t != "" {
		title = t
	}

	doc := &core.StructuredDocument{
		RawDoc: raw,
		Title:  title,
		Root:   root,
	}

	return doc.SetValue("language", lang).
		SetValue("total_lines", strings.Count(raw.GetContent(), "\n")+1).
		SetValue("node_count", countNodes(root)), nil
}

func (cs *CodeStructurizer) buildStructureTree(content string, node *sitter.Node, lang string) *core.StructureNode {
	if node == nil || !node.IsNamed() || node.Type() == "program" || node.Type() == "source_file" {
		return nil
	}

	text := nodeContent(node, content)

	nodeType := classifyNodeType(node, lang)
	title := extractTitle(node, []byte(content), lang)
	level := extractLevel(node, lang)

	children := make([]*core.StructureNode, 0)

	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if childNode := cs.buildStructureTree(content, child, lang); childNode != nil {
			children = append(children, childNode)
		}
	}

	startPos := int(node.StartByte())
	endPos := int(node.EndByte())

	result := &core.StructureNode{
		NodeType: nodeType,
		Title:    title,
		Level:    level,
		Text:     text,
		StartPos: startPos,
		EndPos:   endPos,
		Children: children,
	}

	result.Clean()
	return result
}

func countNodes(node *core.StructureNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countNodes(child)
	}
	return count
}
