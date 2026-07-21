package chunker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
)

// writeTempFile 创建临时文件并写入内容，返回绝对路径。
// 测试结束后由 t.Cleanup 统一清理。
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时文件 %s 失败: %v", path, err)
	}
	return path
}

func TestMarkdownChunker_ATXHeadings(t *testing.T) {
	content := `# 标题一

这是第一段内容。

## 子标题 1.1

子标题内容。

## 子标题 1.2

另一段内容。

# 标题二

第二段顶层内容。`

	path := writeTempFile(t, "test.md", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewMarkdownChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker 返回错误: %v", err)
	}
	chunks := result.Chunks
	if len(chunks) != 4 {
		t.Fatalf("期望 4 个 chunk（标题一/子1.1/子1.2/标题二），实际 %d", len(chunks))
	}
	expectTitles := []string{"标题一", "子标题 1.1", "子标题 1.2", "标题二"}
	for i, c := range chunks {
		if c.Title != expectTitles[i] {
			t.Errorf("chunk[%d].Title 期望 %q, 实际 %q", i, expectTitles[i], c.Title)
		}
	}
	if level, _ := chunks[0].Metadata[core.MetaHeadingLevel].(int); level != 1 {
		t.Errorf("chunks[0].Metadata[heading_level] 期望 1, 实际 %v", chunks[0].Metadata[core.MetaHeadingLevel])
	}
	if level, _ := chunks[1].Metadata[core.MetaHeadingLevel].(int); level != 2 {
		t.Errorf("chunks[1].Metadata[heading_level] 期望 2, 实际 %v", chunks[1].Metadata[core.MetaHeadingLevel])
	}
	// 验证 ParentID：一级标题为根，二级标题指向其所属一级标题
	if chunks[0].ParentID != "" {
		t.Errorf("chunks[0]（一级标题）ParentID 应为空，实际 %q", chunks[0].ParentID)
	}
	if chunks[3].ParentID != "" {
		t.Errorf("chunks[3]（一级标题）ParentID 应为空，实际 %q", chunks[3].ParentID)
	}
	if chunks[1].ParentID != chunks[0].ID {
		t.Errorf("chunks[1] ParentID 期望 %q，实际 %q", chunks[0].ID, chunks[1].ParentID)
	}
	if chunks[2].ParentID != chunks[0].ID {
		t.Errorf("chunks[2] ParentID 期望 %q，实际 %q", chunks[0].ID, chunks[2].ParentID)
	}
	// 验证 Markdown 分块同时产出 Section 节点与 CONTAINS 边
	if len(result.Nodes) == 0 {
		t.Errorf("期望 MarkdownChunker 产出 Nodes，实际 0")
	}
	if len(result.Edges) == 0 {
		t.Errorf("期望 MarkdownChunker 产出 Edges，实际 0")
	}
}

func TestMarkdownChunker_NoHeading(t *testing.T) {
	content := "这是一段纯文本，没有任何 heading。"
	path := writeTempFile(t, "plain.md", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewMarkdownChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker 返回错误: %v", err)
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("期望 1 个 chunk，实际 %d", len(result.Chunks))
	}
	if result.Chunks[0].Title != "plain" {
		t.Errorf("Title 期望 'plain'，实际 %q", result.Chunks[0].Title)
	}
}

func TestMarkdownChunker_SetextHeading(t *testing.T) {
	content := `Title One
=========

正文。

Subtitle
--------

副标题正文。`

	path := writeTempFile(t, "setext.md", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewMarkdownChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker 返回错误: %v", err)
	}
	if len(result.Chunks) != 2 {
		t.Fatalf("期望 2 个 chunk，实际 %d", len(result.Chunks))
	}
	if result.Chunks[0].Title != "Title One" {
		t.Errorf("chunk[0].Title 期望 'Title One', 实际 %q", result.Chunks[0].Title)
	}
	if result.Chunks[1].Title != "Subtitle" {
		t.Errorf("chunk[1].Title 期望 'Subtitle', 实际 %q", result.Chunks[1].Title)
	}
}

func TestCodeChunker_Go(t *testing.T) {
	content := `package main

import "fmt"

// hello prints hello message
func hello() {
	fmt.Println("hello")
}

// add returns a + b
func add(a, b int) int {
	return a + b
}

type Person struct {
	Name string
}

func (p Person) Greet() string {
	return "hi " + p.Name
}
`
	path := writeTempFile(t, "test.go", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}
	chunks := result.Chunks
	if len(chunks) < 4 {
		t.Fatalf("期望至少 4 个 chunk（header + hello + add + Person + Greet），实际 %d", len(chunks))
	}

	if chunks[0].Title != "test (header)" {
		t.Errorf("chunks[0].Title 期望 'test (header)', 实际 %q", chunks[0].Title)
	}

	titles := map[string]bool{}
	for _, c := range chunks {
		titles[c.Title] = true
	}
	for _, expected := range []string{"func hello()", "func add(a, b int) int", "Person struct", "func (p Person) Greet() string"} {
		if !titles[expected] {
			t.Errorf("期望包含符号 %q，实际 chunks 标题集合: %v", expected, titles)
		}
	}
	// 验证 Summary 从注释中提取
	summaryByTitle := map[string]string{}
	for _, c := range chunks {
		summaryByTitle[c.Title] = c.Summary
	}
	if summaryByTitle["func hello()"] != "hello prints hello message" {
		t.Errorf("hello 的 Summary 期望 %q，实际 %q", "hello prints hello message", summaryByTitle["func hello()"])
	}
	if summaryByTitle["func add(a, b int) int"] != "add returns a + b" {
		t.Errorf("add 的 Summary 期望 %q，实际 %q", "add returns a + b", summaryByTitle["func add(a, b int) int"])
	}
	// 验证代码分块同时产出符号 Node/Edge
	if len(result.Nodes) == 0 {
		t.Errorf("期望 CodeChunker 产出 Nodes，实际 0")
	}
	if len(result.Edges) == 0 {
		t.Errorf("期望 CodeChunker 产出 Edges，实际 0")
	}
	// 验证 Go 方法 receiver 生成 BELONGS_TO 边：Person.Greet --BELONGS_TO--> Person
	// 注意：方法 Node 已使用 qualifiedName（Person.Greet），类 Node 名为 Person
	nodeIDByName := map[string]string{}
	nodeByName := map[string]core.Node{}
	for _, n := range result.Nodes {
		nodeIDByName[n.Name] = n.ID
		nodeByName[n.Name] = n
	}
	found := false
	for _, e := range result.Edges {
		if e.Type == "BELONGS_TO" && e.Source == nodeIDByName["Person.Greet"] && e.Target == nodeIDByName["Person"] {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("期望存在 Person.Greet --BELONGS_TO--> Person 的边，实际未找到")
	}
	// 验证 Chunk 第一层字段（language/StartLine/EndLine）与 Metadata 第二层属性（signature/visibility）
	for _, c := range chunks {
		if c.Language != "go" {
			t.Errorf("%s 的 language 期望 go，实际 %v", c.Title, c.Language)
		}
		if c.StartLine <= 0 {
			t.Errorf("%s 的 start_line 应大于 0，实际 %v", c.Title, c.StartLine)
		}
		if c.EndLine < c.StartLine {
			t.Errorf("%s 的 end_line 不应小于 start_line", c.Title)
		}
		if c.Metadata[core.MetaSignature] == "" {
			t.Errorf("%s 的 signature 不应为空", c.Title)
		}
		if c.Metadata[core.MetaVisibility] == "" {
			t.Errorf("%s 的 visibility 不应为空", c.Title)
		}
	}
	// 验证 Node.Properties 包含 signature/visibility/receiver（使用 core.Prop* 常量）
	if greetNode, ok := nodeByName["Person.Greet"]; ok {
		if greetNode.Properties[core.PropSignature] != "func (p Person) Greet() string" {
			t.Errorf("Person.Greet Node signature 期望 %q，实际 %q", "func (p Person) Greet() string", greetNode.Properties[core.PropSignature])
		}
		if greetNode.Properties[core.PropVisibility] != "exported" {
			t.Errorf("Person.Greet Node visibility 期望 exported，实际 %v", greetNode.Properties[core.PropVisibility])
		}
		if greetNode.Properties[core.PropReceiver] != "Person" {
			t.Errorf("Person.Greet Node receiver 期望 Person，实际 %v", greetNode.Properties[core.PropReceiver])
		}
	} else {
		t.Errorf("未找到 Person.Greet Node")
	}
	// 验证 Document Node 包含 package（使用 core.PropPackage 常量）
	docNode := result.Nodes[0]
	if docNode.Properties[core.PropPackage] != "main" {
		t.Errorf("Document Node package 期望 main，实际 %v", docNode.Properties[core.PropPackage])
	}
	// 验证 ParentID：Go 方法不在类型字节范围内，属于顶层符号，ParentID 为空
	chunkByTitle := map[string]core.Chunk{}
	for _, c := range chunks {
		chunkByTitle[c.Title] = c
	}
	if chunkByTitle["hello"].ParentID != "" {
		t.Errorf("顶层函数 hello 的 ParentID 应为空，实际 %q", chunkByTitle["hello"].ParentID)
	}
	if chunkByTitle["Person"].ParentID != "" {
		t.Errorf("顶层类型 Person 的 ParentID 应为空，实际 %q", chunkByTitle["Person"].ParentID)
	}
}

func TestCodeChunker_Python(t *testing.T) {
	content := `def greet(name):
    """Say hello to name."""
    print(f"hello {name}")


class Animal:
    """Animal base class."""
    def __init__(self, name):
        self.name = name

    def speak(self):
        raise NotImplementedError
`
	path := writeTempFile(t, "test.py", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}
	if len(result.Chunks) < 3 {
		t.Fatalf("期望至少 3 个 chunk，实际 %d", len(result.Chunks))
	}
	titles := map[string]bool{}
	for _, c := range result.Chunks {
		titles[c.Title] = true
	}
	for _, expected := range []string{"def greet(name):", "class Animal:"} {
		if !titles[expected] {
			t.Errorf("期望包含符号 %q，实际 chunks 标题集合: %v", expected, titles)
		}
	}
	// 验证 Python docstring 被提取为 Summary
	summaryByTitle := map[string]string{}
	chunkByTitle := map[string]core.Chunk{}
	for _, c := range result.Chunks {
		summaryByTitle[c.Title] = c.Summary
		chunkByTitle[c.Title] = c
	}
	if !strings.Contains(summaryByTitle["def greet(name):"], "hello") {
		t.Errorf("greet 的 Summary 期望包含 hello，实际 %q", summaryByTitle["def greet(name):"])
	}
	// 验证 ParentID：类方法在类的字节范围内，ParentID 指向类
	if chunkByTitle["class Animal:"].ParentID != "" {
		t.Errorf("顶层类 Animal 的 ParentID 应为空，实际 %q", chunkByTitle["class Animal:"].ParentID)
	}
	if chunkByTitle["def __init__(self, name):"].ParentID != chunkByTitle["class Animal:"].ID {
		t.Errorf("__init__ ParentID 期望 %q，实际 %q", chunkByTitle["class Animal:"].ID, chunkByTitle["def __init__(self, name):"].ParentID)
	}
	if chunkByTitle["def speak(self):"].ParentID != chunkByTitle["class Animal:"].ID {
		t.Errorf("speak ParentID 期望 %q，实际 %q", chunkByTitle["class Animal:"].ID, chunkByTitle["def speak(self):"].ParentID)
	}
}

func TestCodeChunker_UnsupportedExt_FallsBackToPlainText(t *testing.T) {
	content := `第一段。

第二段。

第三段，并且这是一段非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常非常长的文本以确保触发切分逻辑。`

	path := writeTempFile(t, "notes.markdownxyz", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatalf("期望至少 1 个 chunk，实际 0")
	}
}

// TestMarkdownChunker_GraphStructure 验证 heading 层级正确生成 Nodes/Edges。
func TestMarkdownChunker_GraphStructure(t *testing.T) {
	content := `# 标题一

正文一。

## 子标题

子正文。

# 标题二

正文二。`

	path := writeTempFile(t, "graph.md", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewMarkdownChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker 返回错误: %v", err)
	}

	// 期望节点：Document + 3 个 Section
	if len(result.Nodes) != 4 {
		t.Fatalf("期望 4 个 Node（Document + 3 Section），实际 %d", len(result.Nodes))
	}
	// 期望边：doc->标题一, 标题一->子标题, doc->标题二
	if len(result.Edges) != 3 {
		t.Fatalf("期望 3 条 CONTAINS 边，实际 %d", len(result.Edges))
	}
	for _, e := range result.Edges {
		if e.Type != "CONTAINS" {
			t.Errorf("期望 Edge.Type=CONTAINS，实际 %q", e.Type)
		}
	}
}

// TestCodeChunker_Python_GraphStructure 验证类包含方法的结构关系。
func TestCodeChunker_Python_GraphStructure(t *testing.T) {
	content := `class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        raise NotImplementedError
`
	path := writeTempFile(t, "animal.py", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}

	// 期望节点：Document + Animal + __init__ + speak
	if len(result.Nodes) != 4 {
		t.Fatalf("期望 4 个 Node，实际 %d", len(result.Nodes))
	}
	// 期望边：doc->Animal, Animal->__init__, Animal->speak
	if len(result.Edges) != 3 {
		t.Fatalf("期望 3 条边，实际 %d", len(result.Edges))
	}

	// 查找 Animal 节点到 speak 方法的边
	animalID := ""
	for _, n := range result.Nodes {
		if n.Name == "Animal" {
			animalID = n.ID
			break
		}
	}
	if animalID == "" {
		t.Fatalf("未找到 Animal 节点")
	}
	foundSpeak := false
	for _, e := range result.Edges {
		if e.Source == animalID && e.Type == "CONTAINS" {
			targetName := ""
			for _, n := range result.Nodes {
				if n.ID == e.Target {
					targetName = n.Name
					break
				}
			}
			if targetName == "Animal.speak" {
				foundSpeak = true
			}
		}
	}
	if !foundSpeak {
		t.Errorf("未找到 Animal CONTAINS speak 的边")
	}
}

// hasEdge 在 edges 中查找指定类型与端点的边。
func hasEdge(t *testing.T, edges []core.Edge, edgeType, sourceID, targetID string) bool {
	t.Helper()
	for _, e := range edges {
		if e.Type == edgeType && e.Source == sourceID && e.Target == targetID {
			return true
		}
	}
	return false
}

func TestCodeChunker_Go_Calls(t *testing.T) {
	content := `package main

import "fmt"

func add(a, b int) int {
	return a + b
}

type Person struct {
	Name string
}

func (p Person) Greet() string {
	fmt.Println("hi")
	return "hi " + p.Name
}

func main() {
	fmt.Println(add(1, 2))
	p := Person{Name: "x"}
	fmt.Println(p.Greet())
}
`
	path := writeTempFile(t, "calls.go", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}

	nodeIDByName := map[string]string{}
	for _, n := range result.Nodes {
		nodeIDByName[n.Name] = n.ID
	}

	if !hasEdge(t, result.Edges, "CALLS", nodeIDByName["main"], nodeIDByName["add"]) {
		t.Errorf("期望存在 main --CALLS--> add 的边")
	}
	if !hasEdge(t, result.Edges, "CALLS", nodeIDByName["main"], nodeIDByName["Person.Greet"]) {
		t.Errorf("期望存在 main --CALLS--> Person.Greet 的边")
	}
	if hasEdge(t, result.Edges, "CALLS", nodeIDByName["Person.Greet"], nodeIDByName["Println"]) {
		t.Errorf("不应为外部符号 fmt.Println 生成 CALLS 边")
	}
}

func TestCodeChunker_Python_Calls(t *testing.T) {
	content := `class Animal:
    def __init__(self, name):
        self.name = name
        self.speak()

    def speak(self):
        print("sound")

def greet(name):
    print(f"hello {name}")
    a = Animal()
    a.speak()

greet("world")
`
	path := writeTempFile(t, "calls.py", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}

	nodeIDByName := map[string]string{}
	for _, n := range result.Nodes {
		nodeIDByName[n.Name] = n.ID
	}

	if !hasEdge(t, result.Edges, "CALLS", nodeIDByName["Animal.__init__"], nodeIDByName["Animal.speak"]) {
		t.Errorf("期望存在 Animal.__init__ --CALLS--> Animal.speak 的边")
	}
	if !hasEdge(t, result.Edges, "CALLS", nodeIDByName["greet"], nodeIDByName["Animal.speak"]) {
		t.Errorf("期望存在 greet --CALLS--> Animal.speak 的边")
	}
	if hasEdge(t, result.Edges, "CALLS", nodeIDByName["greet"], nodeIDByName["print"]) {
		t.Errorf("不应为外部符号 print 生成 CALLS 边")
	}
	if hasEdge(t, result.Edges, "CALLS", nodeIDByName["greet"], nodeIDByName["Animal"]) {
		t.Errorf("不应把类实例化 Animal() 当作函数调用")
	}
}

func TestCodeChunker_Java_Inheritance(t *testing.T) {
	content := `package app;

class Animal {
    void speak() {}
}

class Dog extends Animal {
    void speak() {}
}

interface Flyable {
    void fly();
}

class Bird implements Flyable {
    public void fly() {}
}
`
	path := writeTempFile(t, "inheritance.java", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}

	nodeIDByName := map[string]string{}
	for _, n := range result.Nodes {
		nodeIDByName[n.Name] = n.ID
	}

	if !hasEdge(t, result.Edges, "INHERITS", nodeIDByName["Dog"], nodeIDByName["Animal"]) {
		t.Errorf("期望存在 Dog --INHERITS--> Animal 的边")
	}
	if !hasEdge(t, result.Edges, "IMPLEMENTS", nodeIDByName["Bird"], nodeIDByName["Flyable"]) {
		t.Errorf("期望存在 Bird --IMPLEMENTS--> Flyable 的边")
	}
}

func TestCodeChunker_TypeScript_Inheritance(t *testing.T) {
	content := `class Animal {
    speak() {}
}

class Dog extends Animal {
    speak() {}
}

interface Flyable {
    fly(): void;
}

class Bird implements Flyable {
    fly() {}
}
`
	path := writeTempFile(t, "inheritance.ts", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}

	nodeIDByName := map[string]string{}
	for _, n := range result.Nodes {
		nodeIDByName[n.Name] = n.ID
	}

	if !hasEdge(t, result.Edges, "INHERITS", nodeIDByName["Dog"], nodeIDByName["Animal"]) {
		t.Errorf("期望存在 Dog --INHERITS--> Animal 的边")
	}
	if !hasEdge(t, result.Edges, "IMPLEMENTS", nodeIDByName["Bird"], nodeIDByName["Flyable"]) {
		t.Errorf("期望存在 Bird --IMPLEMENTS--> Flyable 的边")
	}
}

// TestMarkdownChunker_SummaryFilled 验证 Markdown 分块统一填充 Summary。
func TestMarkdownChunker_SummaryFilled(t *testing.T) {
	content := `# 标题一

这是第一段内容。这是第二段内容。

## 子标题

子标题内容。`

	path := writeTempFile(t, "summary.md", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewMarkdownChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker 返回错误: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatalf("期望至少 1 个 chunk，实际 0")
	}
	for _, c := range result.Chunks {
		if c.Summary == "" {
			t.Errorf("chunk %q 的 Summary 不应为空", c.Title)
		}
	}
}

// TestCodeChunker_SummaryFallback 验证无注释时代码符号 fallback 到内容摘要。
func TestCodeChunker_SummaryFallback(t *testing.T) {
	content := `package main

func noComment() string {
	return "no comment"
}
`
	path := writeTempFile(t, "fallback.go", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewCodeChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("CodeChunker 返回错误: %v", err)
	}
	for _, c := range result.Chunks {
		if c.Title == "noComment" && c.Summary == "" {
			t.Errorf("noComment 的 Summary 不应为空")
		}
	}
}

// TestDocumentNode_SourceChunkIDsEmpty 验证 Document 根节点 SourceChunkIDs 为空（纯图实体）。
func TestDocumentNode_SourceChunkIDsEmpty(t *testing.T) {
	content := `# 标题一

正文。`
	path := writeTempFile(t, "pure_graph.md", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewMarkdownChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker 返回错误: %v", err)
	}
	if len(result.Nodes) == 0 {
		t.Fatalf("期望至少 1 个 Node，实际 0")
	}

	docNode := result.Nodes[0]
	if len(docNode.SourceChunkIDs) != 0 {
		t.Errorf("Document 节点的 SourceChunkIDs 应为空，实际 %v", docNode.SourceChunkIDs)
	}

	// 非文档级节点（Section）应绑定到对应 Chunk
	foundSection := false
	for _, n := range result.Nodes[1:] {
		if len(n.SourceChunkIDs) > 0 {
			foundSection = true
			break
		}
	}
	if !foundSection {
		t.Errorf("期望至少存在一个非文档级节点绑定 SourceChunkIDs")
	}
}

// TestChunkMetadata_Directory 验证 Chunk 元数据包含文件所在目录。
func TestChunkMetadata_Directory(t *testing.T) {
	content := `# 标题

正文。`
	path := writeTempFile(t, "dir.md", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewMarkdownChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("MarkdownChunker 返回错误: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatalf("期望至少 1 个 chunk，实际 0")
	}

	expectedDir := filepath.Dir(path)
	for _, c := range result.Chunks {
		if c.Metadata[core.MetaDirectory] != expectedDir {
			t.Errorf("chunk %q directory 期望 %q，实际 %v", c.Title, expectedDir, c.Metadata[core.MetaDirectory])
		}
	}
}

// _ 确保 core 包被使用（测试中可能引用 core.Chunk 字段）
var _ = core.Chunk{}
