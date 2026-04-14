package structurizer

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func classifyNodeType(node *sitter.Node, lang string) string {
	nodeType := node.Type()

	switch lang {
	case "go":
		return classifyGoNode(nodeType)
	case "c", "cpp":
		return classifyCppNode(nodeType)
	case "java":
		return classifyJavaNode(nodeType)
	case "python":
		return classifyPythonNode(nodeType)
	case "javascript", "typescript", "tsx":
		return classifyJavaScriptNode(nodeType)
	case "rust":
		return classifyRustNode(nodeType)
	default:
		if isTopLevelDeclaration(nodeType, lang) {
			return "declaration"
		}
		return "statement"
	}
}

func extractTitle(node *sitter.Node, content []byte, lang string) string {
	switch node.Type() {
	case "function_declaration", "function_declaration_item",
		"method_declaration", "func_decl", "function_definition",
		"arrow_function", "function_expression",
		"function_item", "function_signature_item":

		if name := findChildByName(node, content); name != "" {
			receiver := findReceiver(node, content)
			if receiver != "" {
				return fmt.Sprintf("%s.%s()", receiver, name)
			}
			return name + "()"
		}
	case "class_declaration", "class_definition",
		"class_declaration_item",
		"struct_declaration", "struct_specifier",
		"type_declaration", "type_alias_declaration",
		"interface_type",
		"class_item", "struct_item", "enum_item":

		if name := findChildByName(node, content); name != "" {
			return name
		}
	case "impl_item", "trait_declaration",
		"enum_declaration":

		if name := findChildByName(node, content); name != "" {
			return name
		}
	case "const_declaration", "const_spec", "var_declaration":
		names := findAllNamesInDeclaration(node, content)
		if len(names) > 0 {
			return strings.Join(names, ", ")
		}
	}

	return ""
}

func extractLevel(node *sitter.Node, lang string) int {
	nodeType := node.Type()

	switch lang {
	case "go":
		switch nodeType {
		case "function_declaration", "method_declaration":
			return 1
		case "if_statement", "for_statement", "switch_statement",
			"select_statement", "case_clause":
			return 2
		}
	case "c", "cpp":
		switch nodeType {
		case "function_definition", "class_specifier",
			"struct_specifier", "enum_specifier":
			return 1
		case "if_statement", "for_statement", "while_statement",
			"switch_statement", "case_statement", "try_statement":
			return 2
		}
	case "java":
		switch nodeType {
		case "class_declaration", "interface_declaration",
			"enum_declaration", "method_declaration",
			"constructor_declaration":
			return 1
		case "if_statement", "for_statement", "while_statement",
			"switch_expression", "try_statement", "catch_clause":
			return 2
		}
	case "python":
		switch nodeType {
		case "function_definition", "class_definition":
			return 1
		case "if_statement", "for_statement", "while_statement",
			"with_statement", "try_statement":
			return 2
		}
	case "javascript", "typescript", "tsx":
		switch nodeType {
		case "function_declaration", "function_expression",
			"arrow_function", "class_declaration", "method_definition",
			"export_statement", "import_statement":
			return 1
		case "if_statement", "for_in_statement", "for_statement",
			"while_statement", "switch_statement", "try_statement":
			return 2
		}
	case "rust":
		switch nodeType {
		case "function_item", "impl_item", "trait_item",
			"enum_item", "struct_item", "mod_item":
			return 1
		case "if_expression", "for_expression", "loop_expression",
			"while_expression", "match_expression":
			return 2
		}
	}

	parent := node.Parent()
	if parent == nil {
		return 3
	}
	return extractLevel(parent, lang) + 1
}

func nodeContent(node *sitter.Node, content string) string {
	start := int(node.StartByte())
	end := int(node.EndByte())

	if start >= end || end > len(content) {
		return ""
	}

	text := content[start:end]

	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, strings.TrimRight(line, "\r"))
	}
	return strings.Join(result, "\n")
}

func isTopLevelDeclaration(nodeType string, lang string) bool {
	topLevelTypes := map[string][]string{
		"go": {
			"function_declaration", "method_declaration", "type_declaration",
			"const_declaration", "var_declaration", "import_declaration",
		},
		"c": {
			"function_definition", "declaration",
			"class_specifier", "struct_specifier",
			"enum_specifier", "namespace_definition",
		},
		"cpp": {
			"function_definition", "declaration",
			"class_specifier", "struct_specifier",
			"enum_specifier", "namespace_definition",
		},
		"java": {
			"class_declaration", "interface_declaration",
			"enum_declaration", "method_declaration",
			"constructor_declaration", "field_declaration",
		},
		"python": {
			"function_definition", "class_definition",
			"decorated_definition", "import_statement",
			"import_from_statement", "expression_statement",
		},
		"javascript": {
			"function_declaration", "class_declaration",
			"variable_declaration", "export_statement",
			"import_statement", "lexical_declaration",
		},
		"rust": {
			"function_item", "struct_item", "enum_item",
			"impl_item", "trait_item", "use_item",
			"mod_item", "const_item", "static_item",
		},
	}

	types, ok := topLevelTypes[lang]
	if !ok {
		return false
	}

	for _, t := range types {
		if nodeType == t {
			return true
		}
	}
	return false
}

func classifyGoNode(nodeType string) string {
	switch nodeType {
	case "function_declaration", "method_declaration":
		return "function"
	case "type_declaration", "struct_type", "interface_type":
		return "type"
	case "const_declaration", "var_declaration":
		return "variable"
	case "if_statement", "for_statement", "switch_statement",
		"select_statement", "range_clause":
		return "control_flow"
	case "import_declaration":
		return "import"
	default:
		return "statement"
	}
}

func classifyCppNode(nodeType string) string {
	switch nodeType {
	case "function_definition":
		return "function"
	case "class_specifier", "struct_specifier", "enum_specifier",
		"union_specifier", "template_declaration":
		return "type"
	case "declaration":
		return "variable"
	case "if_statement", "for_statement", "while_statement",
		"switch_statement", "case_statement",
		"do_statement", "preproc_ifdef":
		return "control_flow"
	case "preproc_include":
		return "import"
	default:
		return "statement"
	}
}

func classifyJavaNode(nodeType string) string {
	switch nodeType {
	case "method_declaration", "constructor_declaration":
		return "function"
	case "class_declaration", "interface_declaration",
		"enum_declaration", "record_declaration":
		return "type"
	case "field_declaration", "local_variable_declaration":
		return "variable"
	case "if_statement", "for_statement", "while_statement",
		"switch_expression", "try_statement", "catch_clause",
		"enhanced_for_statement", "do_statement":
		return "control_flow"
	case "import_declaration":
		return "import"
	default:
		return "statement"
	}
}

func classifyPythonNode(nodeType string) string {
	switch nodeType {
	case "function_definition":
		return "function"
	case "class_definition":
		return "class"
	case "import_statement", "import_from_statement":
		return "import"
	case "if_statement", "for_statement", "while_statement",
		"with_statement", "try_statement", "except_clause":
		return "control_flow"
	case "assignment", "augmented_assignment", "annotated_assignment":
		return "variable"
	default:
		return "statement"
	}
}

func classifyJavaScriptNode(nodeType string) string {
	switch nodeType {
	case "function_declaration", "function_expression",
		"arrow_function", "generator_function_declaration",
		"generator_function_expression", "method_definition":
		return "function"
	case "class_declaration", "class_expression":
		return "class"
	case "variable_declaration", "lexical_declaration":
		return "variable"
	case "import_statement", "export_statement":
		return "import"
	case "if_statement", "for_in_statement", "for_statement",
		"while_statement", "switch_statement", "try_statement",
		"catch_clause":
		return "control_flow"
	case "interface_declaration", "type_alias_declaration",
		"enum_declaration":
		return "type"
	default:
		return "statement"
	}
}

func classifyRustNode(nodeType string) string {
	switch nodeType {
	case "function_item":
		return "function"
	case "struct_item", "enum_item":
		return "type"
	case "impl_item", "trait_item":
		return "implementation"
	case "let_declaration", "static_item", "const_item":
		return "variable"
	case "use_item", "mod_item":
		return "module"
	case "if_expression", "match_expression", "for_expression",
		"loop_expression", "while_expression":
		return "control_flow"
	default:
		return "statement"
	}
}

func findChildByName(node *sitter.Node, content []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := child.Type()

		if childType == "identifier" ||
			childType == "name" ||
			childType == "type_identifier" ||
			childType == "plain_identifier" {

			return child.Content(content)
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}

		result := findChildByName(child, content)
		if result != "" {
			return result
		}
	}

	return ""
}

func findReceiver(node *sitter.Node, content []byte) string {
	receiverTypes := map[string]bool{
		"receiver":       true,
		"parameter_list": true,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || !receiverTypes[child.Type()] {
			continue
		}

		typeName := findTypeName(child, content)
		if typeName != "" {
			return typeName
		}
	}

	return ""
}

func findTypeName(node *sitter.Node, content []byte) string {
	typeNodes := map[string]bool{
		"type_identifier":   true,
		"user_type":         true,
		"pointer_type":      true,
		"sliced_type":       true,
		"qualified_type":    true,
		"generic_type":      true,
		"array_type":        true,
		"scoped_identifier": true,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		if typeNodes[child.Type()] && child.IsNamed() {
			childContent := child.Content(content)
			if childContent != "" {
				return childContent
			}
		}

		if child.IsNamed() {
			result := findTypeName(child, content)
			if result != "" {
				return result
			}
		}
	}

	return ""
}

func findAllNamesInDeclaration(node *sitter.Node, content []byte) []string {
	var names []string

	specTypes := map[string]bool{
		"declaration_list":     true,
		"variable_declarator":  true,
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}

		if specTypes[child.Type()] {
			names = append(names, findAllNamesInDeclaration(child, content)...)
		} else if child.Type() == "identifier" || child.Type() == "name" {
			content := child.Content(content)
			if content != "" {
				names = append(names, content)
			}
		}
	}

	return names
}
