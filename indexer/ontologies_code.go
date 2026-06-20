package indexer

import "strings"

// =============================================================================
// 代码域实体类型定义 — 内置固定全集，无需用户选择。
// 检测到代码后缀名时自动切换为该套定义。
// =============================================================================

// isCodeExt 根据文件扩展名判断是否为代码文件。
// 与 structurizer.isCodeExt 平行，这里做索引器自包含的独立判断。
func isCodeExt(ext string) bool {
	codeExts := map[string]bool{
		".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".go": true, ".java": true, ".c": true, ".cpp": true, ".h": true,
		".hpp": true, ".cs": true, ".rb": true, ".php": true, ".pl": true,
		".swift": true, ".kt": true, ".rs": true, ".scala": true,
		".sh": true, ".bash": true, ".zsh": true, ".ps1": true,
		".sql": true, ".r": true, ".lua": true, ".ex": true, ".exs": true,
		".erl": true, ".hrl": true, ".fs": true, ".fsx": true,
		".vb": true, ".vbs": true, ".dart": true,
		".groovy": true, ".gradle": true, ".makefile": true,
		".vue": true, ".svelte": true, ".graphql": true, ".gql": true,
	}
	return codeExts[strings.ToLower(ext)]
}

// codeEntityDefs 是代码文件专用的实体类型定义列表。
// 与文本域的 EntityDef 互斥：代码文件只加载此列表，忽略用户选择的标签。
// 每一项包含 Prompt（注入 ### Entity Types）和 Schema（注入 ### Entity Schema）。
var codeEntityDefs = []EntityDef{
	{
		Prompt: "**Interface** — interface, protocol, trait defining a contract of methods",
		Schema: `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "description": "interface/protocol/trait name"},
    "methods": {"type": "array", "items": {"type": "string"}, "description": "method signatures declared in the interface"},
    "extends": {"type": "array", "items": {"type": "string"}, "description": "parent interfaces or protocols extended"},
    "generics": {"type": "array", "items": {"type": "string"}, "description": "generic type parameters"}
  }
}`,
	},
	{
		Prompt: "**Struct** — struct, record, data class with named fields and optional methods",
		Schema: `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "description": "struct/record/data class name"},
    "fields": {"type": "array", "items": {"type": "string"}, "description": "field/property/attribute declarations with types"},
    "methods": {"type": "array", "items": {"type": "string"}, "description": "method signatures defined on this struct"},
    "generics": {"type": "array", "items": {"type": "string"}, "description": "generic type parameters"},
    "implements": {"type": "array", "items": {"type": "string"}, "description": "interfaces/traits/protocols implemented"},
    "modifiers": {"type": "array", "items": {"type": "string"}, "description": "modifiers (pub, private, packed, etc.)"}
  }
}`,
	},
	{
		Prompt: "**Class** — class with fields, methods, inheritance (OOP paradigm)",
		Schema: `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "description": "class name"},
    "methods": {"type": "array", "items": {"type": "string"}, "description": "method signatures defined on this class"},
    "fields": {"type": "array", "items": {"type": "string"}, "description": "field/property/attribute declarations"},
    "extends": {"type": "string", "description": "parent class or base struct"},
    "implements": {"type": "array", "items": {"type": "string"}, "description": "interfaces/traits/protocols implemented"},
    "generics": {"type": "array", "items": {"type": "string"}, "description": "generic type parameters"},
    "modifiers": {"type": "array", "items": {"type": "string"}, "description": "access or declaration modifiers (public, abstract, sealed, etc.)"}
  }
}`,
	},
	{
		Prompt: "**Function** — function, method, procedure, closure definition",
		Schema: `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "description": "function/method/procedure name"},
    "parameters": {"type": "array", "items": {"type": "string"}, "description": "parameter list in 'name: type' form"},
    "return_type": {"type": "string", "description": "return type annotation or void/nil"},
    "receiver": {"type": "string", "description": "receiver type for methods (e.g. Go, Rust impl blocks)"},
    "generics": {"type": "array", "items": {"type": "string"}, "description": "generic type parameters"},
    "modifiers": {"type": "array", "items": {"type": "string"}, "description": "modifiers (async, pub, private, static, etc.)"}
  }
}`,
	},
	{
		Prompt: "**Package** — package, module, namespace, library organizing code",
		Schema: `{
  "type": "object",
  "required": ["name", "path"],
  "properties": {
    "name": {"type": "string", "description": "package/module/namespace name"},
    "path": {"type": "string", "description": "import path or fully qualified name"},
    "exports": {"type": "array", "items": {"type": "string"}, "description": "publicly exported or accessible members"}
  }
}`,
	},
	{
		Prompt: "**Enum** — enumeration with named variants or constant values",
		Schema: `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "description": "enum type name"},
    "variants": {"type": "array", "items": {"type": "string"}, "description": "named members or variants of the enum"},
    "generics": {"type": "array", "items": {"type": "string"}, "description": "generic type parameters"}
  }
}`,
	},
	{
		Prompt: "**TypeAlias** — type alias, typedef, type definition creating an alternative name",
		Schema: `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "description": "alias or type definition name"},
    "underlying_type": {"type": "string", "description": "the original or underlying type"},
    "generics": {"type": "array", "items": {"type": "string"}, "description": "generic type parameters"}
  }
}`,
	},
	{
		Prompt: "**Variable** — variable, constant, global, config value declaration",
		Schema: `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": {"type": "string", "description": "variable or constant name"},
    "type": {"type": "string", "description": "type annotation if present in declaration"},
    "is_const": {"type": "boolean", "description": "whether this is a constant or immutable value"},
    "scope": {"type": "string", "description": "visibility scope: global, module, local, member"}
  }
}`,
	},
	{
		Prompt: "**Import** — import, include, require, using directive referencing external code",
		Schema: `{
  "type": "object",
  "required": ["source"],
  "properties": {
    "source": {"type": "string", "description": "import path, module name, file path being imported"},
    "alias": {"type": "string", "description": "local alias, rename, or qualifier if used"}
  }
}`,
	},
}

// codeRelationTypes 是代码域专属的关系类型定义。
// 与文本域的全局 globalRelationTypes 互斥。
const codeRelationTypes = `### Relation Types (Code Domain)
**Structural**: IMPLEMENTS, EXTENDS, CONTAINS, PARAMETER_OF
**Semantic**: CALLS, IMPORTS, DEFINES, RETURNS
**Metadata**: ANNOTATED_BY`
