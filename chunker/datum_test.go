package chunker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
)

// writeTempDataFile 创建临时数据文件并写入内容，返回绝对路径。
func writeTempDataFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时文件 %s 失败: %v", path, err)
	}
	return path
}

func TestDatumChunker_NestedObjects_AreSplitIntoSubchunks(t *testing.T) {
	content := `{
  "users": [
    {"id": 1, "name": "Alice"},
    {"id": 2, "name": "Bob"}
  ],
  "config": {
    "debug": true,
    "version": "1.0.0"
  },
  "count": 42,
  "name": "test"
}`

	path := writeTempDataFile(t, "data.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}
	chunks := result.Chunks

	// 期望至少有：users、config、其余字段 三类 chunk
	titles := map[string]bool{}
	for _, c := range chunks {
		titles[c.Title] = true
		t.Logf("chunk: title=%q start=%d end=%d body=%q", c.Title, c.StartPos, c.EndPos, truncate(c.Content, 80))
	}

	for _, expected := range []string{"JSON.users", "JSON.config"} {
		if !titles[expected] {
			t.Errorf("期望包含 %q，实际: %v", expected, titles)
		}
	}

	// 验证「其余字段」块存在
	foundMisc := false
	for title := range titles {
		if strings.HasPrefix(title, "JSON.其余字段") {
			foundMisc = true
			break
		}
	}
	if !foundMisc {
		t.Errorf("期望包含 JSON.其余字段 块，实际: %v", titles)
	}
}

func TestDatumChunker_DeepNesting_RecursesByPath(t *testing.T) {
	content := `{
  "level1": {
    "level2": {
      "level3": {
        "level4": {
          "level5": {
            "deep_key": "deep_value"
          }
        }
      }
    }
  }
}`

	path := writeTempDataFile(t, "deep.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	t.Logf("chunks 数量: %d", len(result.Chunks))
	for _, c := range result.Chunks {
		t.Logf("chunk: title=%q body=%q", c.Title, truncate(c.Content, 80))
	}

	if len(result.Chunks) == 0 {
		t.Fatalf("期望至少 1 个 chunk，实际 0")
	}
}

// TestDatumChunker_ParentIDs 验证数据分块按路径前缀正确建立 ParentID 链。
func TestDatumChunker_ParentIDs(t *testing.T) {
	content := `{
  "users": [
    {"id": 1, "name": "Alice", "profile": {"age": 30, "city": "Beijing"}},
    {"id": 2, "name": "Bob"}
  ],
  "config": {
    "debug": true,
    "database": {"host": "localhost", "port": 5432}
  }
}`

	path := writeTempDataFile(t, "nested.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	chunkByTitle := map[string]core.Chunk{}
	for _, c := range result.Chunks {
		chunkByTitle[c.Title] = c
	}

	// JSON.users[0].profile.age / JSON.users[0].profile.city 的父级是 JSON.users[0].profile
	if profile, ok := chunkByTitle["JSON.users[0].profile"]; ok {
		if age, ok := chunkByTitle["JSON.users[0].profile.age"]; ok && age.ParentID != profile.ID {
			t.Errorf("JSON.users[0].profile.age ParentID 期望 %q，实际 %q", profile.ID, age.ParentID)
		}
		if city, ok := chunkByTitle["JSON.users[0].profile.city"]; ok && city.ParentID != profile.ID {
			t.Errorf("JSON.users[0].profile.city ParentID 期望 %q，实际 %q", profile.ID, city.ParentID)
		}
	}

	// JSON.config.database.host / JSON.config.database.port 的父级是 JSON.config.database
	if db, ok := chunkByTitle["JSON.config.database"]; ok {
		if host, ok := chunkByTitle["JSON.config.database.host"]; ok && host.ParentID != db.ID {
			t.Errorf("JSON.config.database.host ParentID 期望 %q，实际 %q", db.ID, host.ParentID)
		}
		if port, ok := chunkByTitle["JSON.config.database.port"]; ok && port.ParentID != db.ID {
			t.Errorf("JSON.config.database.port ParentID 期望 %q，实际 %q", db.ID, port.ParentID)
		}
	}

	// JSON.config.database 的父级是 JSON.config
	if config, ok := chunkByTitle["JSON.config"]; ok {
		if db, ok := chunkByTitle["JSON.config.database"]; ok && db.ParentID != config.ID {
			t.Errorf("JSON.config.database ParentID 期望 %q，实际 %q", config.ID, db.ParentID)
		}
	}
}

func TestDatumChunker_LogFile_GoesByLines(t *testing.T) {
	content := `2024-01-01 10:00:00 INFO 启动服务
2024-01-01 10:00:05 INFO 加载配置完成
2024-01-01 10:01:00 WARN 磁盘空间不足
2024-01-01 10:02:00 ERROR 服务崩溃`

	path := writeTempDataFile(t, "app.log", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	if len(result.Chunks) == 0 {
		t.Fatalf("期望至少 1 个 chunk，实际 0")
	}
	for _, c := range result.Chunks {
		if !strings.HasPrefix(c.Title, "Log 行") {
			t.Errorf("期望 Title 以 'Log 行' 开头，实际 %q", c.Title)
		}
		t.Logf("chunk: title=%q", c.Title)
	}
}

func TestDatumChunker_TopLevelArray_ElementsAreSplit(t *testing.T) {
	content := `[
  {"id": 1, "name": "Alice"},
  {"id": 2, "name": "Bob"},
  {"id": 3, "name": "Charlie"}
]`

	path := writeTempDataFile(t, "users.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	t.Logf("chunks 数量: %d", len(result.Chunks))
	titles := map[string]bool{}
	for _, c := range result.Chunks {
		titles[c.Title] = true
		t.Logf("chunk: title=%q body=%q", c.Title, truncate(c.Content, 80))
	}

	// 期望三个数组元素各作为一个 chunk
	for _, expected := range []string{"JSON.[0]", "JSON.[1]", "JSON.[2]"} {
		if !titles[expected] {
			t.Errorf("期望包含 %q，实际: %v", expected, titles)
		}
	}
}

func TestDatumChunker_ScalarOnly_YieldsSingleMiscChunk(t *testing.T) {
	content := `{"a": 1, "b": "hello", "c": true}`

	path := writeTempDataFile(t, "scalar.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	if len(result.Chunks) == 0 {
		t.Fatalf("期望至少 1 个 chunk，实际 0")
	}
	// 所有标量合并到一个「其余字段」块
	for _, c := range result.Chunks {
		t.Logf("chunk: title=%q body=%q", c.Title, c.Content)
		if !strings.HasPrefix(c.Title, "JSON.其余字段") {
			t.Errorf("期望 Title 以 'JSON.其余字段' 开头，实际 %q", c.Title)
		}
	}
}

func TestDatumChunker_YAML_NativeStyle(t *testing.T) {
	// 验证 yaml block_* 风格也能切分（如果 document 包未归一化）
	content := `users:
  - id: 1
    name: Alice
  - id: 2
    name: Bob
config:
  debug: true
  version: 1.0.0
count: 42
`

	path := writeTempDataFile(t, "config.yaml", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	t.Logf("chunks 数量: %d", len(result.Chunks))
	for _, c := range result.Chunks {
		t.Logf("chunk: title=%q body=%q", c.Title, truncate(c.Content, 80))
	}
	if len(result.Chunks) == 0 {
		t.Fatalf("期望至少 1 个 chunk，实际 0")
	}
}

func TestDatumChunker_Empty(t *testing.T) {
	// document.Open 拒绝空 JSON 文件，这里直接用 document.New 构造空内容
	doc := document.New("", document.RawDocData)
	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Fatalf("期望 0 个 chunk，实际 %d", len(result.Chunks))
	}
}

// TestDatumChunker_GraphStructure 验证 JSON 数据结构生成 Nodes/Edges。
func TestDatumChunker_GraphStructure(t *testing.T) {
	content := `{
  "users": [
    {"id": 1, "name": "Alice"},
    {"id": 2, "name": "Bob"}
  ],
  "config": {
    "debug": true
  }
}`

	path := writeTempDataFile(t, "graph.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	// 期望至少包含 JSON Document 节点
	if len(result.Nodes) == 0 {
		t.Fatalf("期望至少 1 个 Node，实际 0")
	}
	// 期望至少包含 CONTAINS 边
	if len(result.Edges) == 0 {
		t.Fatalf("期望至少 1 条 Edge，实际 0")
	}

	// 验证 JSON 根节点包含 JSON.users
	rootID := ""
	for _, n := range result.Nodes {
		if n.Name == "JSON" {
			rootID = n.ID
			break
		}
	}
	if rootID == "" {
		t.Fatalf("未找到 JSON 根节点")
	}
	foundUsers := false
	for _, e := range result.Edges {
		if e.Source == rootID && e.Type == "CONTAINS" {
			for _, n := range result.Nodes {
				if n.ID == e.Target && strings.HasSuffix(n.Name, "users") {
					foundUsers = true
					break
				}
			}
		}
	}
	if !foundUsers {
		t.Errorf("未找到 JSON CONTAINS JSON.users 的边")
	}
}

// TestDatumChunker_DuplicateContent_HasUniqueIDs 验证相同内容但不同路径的数据记录生成不同 Chunk.ID。
func TestDatumChunker_DuplicateContent_HasUniqueIDs(t *testing.T) {
	content := `{
  "items": [
    {"status": "ok"},
    {"status": "ok"},
    {"status": "ok"}
  ]
}`

	path := writeTempDataFile(t, "duplicate.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}

	ids := map[string]string{}
	for _, c := range result.Chunks {
		if prev, ok := ids[c.ID]; ok {
			t.Errorf("发现重复 Chunk.ID %q：%s 与 %s", c.ID, prev, c.Title)
		}
		ids[c.ID] = c.Title
	}
}

// TestDatumChunker_SummaryFilled 验证数据分块统一填充 Summary。
func TestDatumChunker_SummaryFilled(t *testing.T) {
	content := `{
  "users": [{"id": 1, "name": "Alice"}],
  "config": {"debug": true}
}`

	path := writeTempDataFile(t, "summary.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
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

// TestDatumChunker_LogSummaryFilled 验证 Log 按行切分后也填充 Summary。
func TestDatumChunker_LogSummaryFilled(t *testing.T) {
	content := `2024-01-01 10:00:00 INFO 启动服务
2024-01-01 10:00:05 INFO 加载配置完成`

	path := writeTempDataFile(t, "summary.log", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}
	for _, c := range result.Chunks {
		if c.Summary == "" {
			t.Errorf("log chunk %q 的 Summary 不应为空", c.Title)
		}
	}
}

// TestDatumChunker_DirectoryMetadata 验证数据 Chunk 元数据包含文件所在目录。
func TestDatumChunker_DirectoryMetadata(t *testing.T) {
	content := `{"key": "value"}`
	path := writeTempDataFile(t, "dir.json", content)
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("document.Open 失败: %v", err)
	}

	result, err := NewDatumChunker().Chunk(doc)
	if err != nil {
		t.Fatalf("DatumChunker 返回错误: %v", err)
	}
	expectedDir := filepath.Dir(path)
	for _, c := range result.Chunks {
		if c.Metadata[core.MetaDirectory] != expectedDir {
			t.Errorf("chunk %q directory 期望 %q，实际 %v", c.Title, expectedDir, c.Metadata[core.MetaDirectory])
		}
	}
}

// truncate 截断字符串用于日志输出。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
