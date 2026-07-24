package gorag

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/v2/core"
)

// TestSourceHasPrefix 验证 source 路径前缀过滤。
func TestSourceHasPrefix(t *testing.T) {
	if sourceHasPrefix("/home/user/docs/file.md", "./docs") {
		t.Errorf("相对路径 filter 不应匹配绝对路径 source")
	}
	if !sourceHasPrefix("/home/user/docs/file.md", "/home/user/docs") {
		t.Errorf("绝对路径前缀应匹配")
	}
	if sourceHasPrefix("/home/user/docs/file.md", "/home/user/other") {
		t.Errorf("非前缀路径不应匹配")
	}
	if sourceHasPrefix("", "/home/user") {
		t.Errorf("空 source 不应匹配")
	}
	// 边界检查：/home/user/doc 不应匹配 /home/user/docs/file.md
	if sourceHasPrefix("/home/user/docs/file.md", "/home/user/doc") {
		t.Errorf("部分目录名前缀不应匹配")
	}
	// 精确目录边界：/home/user/docs/ 应匹配 /home/user/docs/file.md
	if !sourceHasPrefix("/home/user/docs/file.md", "/home/user/docs/") {
		t.Errorf("已带分隔符的目录前缀应匹配")
	}
}

// TestFilterHitBySourcePrefix 验证命中结果按 source 前缀过滤。
func TestFilterHitBySourcePrefix(t *testing.T) {
	hit := &core.Hit{
		Chunks: []core.ChunkHit{
			{Chunk: &core.Chunk{ID: "a", Source: "/home/user/docs/a.md"}, Score: 0.9},
			{Chunk: &core.Chunk{ID: "b", Source: "/home/user/other/b.md"}, Score: 0.8},
		},
	}
	filtered := filterHitBySourcePrefix(hit, "/home/user/docs")
	if len(filtered.Chunks) != 1 || filtered.Chunks[0].ID != "a" {
		t.Errorf("过滤后期望保留 a，实际 %v", filtered.Chunks)
	}
}

// TestTopChunkScore 验证最高分提取。
func TestTopChunkScore(t *testing.T) {
	hits := []core.ChunkHit{
		{Score: 0.5},
		{Score: 0.9},
		{Score: 0.7},
	}
	if got := topChunkScore(hits); got != 0.9 {
		t.Errorf("最高分期望 0.9，实际 %v", got)
	}
	if got := topChunkScore(nil); got != 0 {
		t.Errorf("空切片期望 0，实际 %v", got)
	}
}

// ── computeChunkContentHash ──────────────────────────────────────

func TestComputeChunkContentHash(t *testing.T) {
	t.Run("非空内容产生非空哈希", func(t *testing.T) {
		h := computeChunkContentHash("hello world")
		if h == "" {
			t.Fatal("期望非空哈希")
		}
		if len(h) != 16 {
			t.Fatalf("哈希长度应为 16，实际: %d", len(h))
		}
	})

	t.Run("空字符串返回空哈希", func(t *testing.T) {
		if h := computeChunkContentHash(""); h != "" {
			t.Fatalf("空字符串应返回空哈希，实际: %s", h)
		}
	})

	t.Run("相同内容产生相同哈希", func(t *testing.T) {
		a := computeChunkContentHash("consistent")
		b := computeChunkContentHash("consistent")
		if a != b {
			t.Fatalf("相同内容应产生相同哈希: %s != %s", a, b)
		}
	})

	t.Run("不同内容产生不同哈希", func(t *testing.T) {
		a := computeChunkContentHash("foo")
		b := computeChunkContentHash("bar")
		if a == b {
			t.Fatal("不同内容不应产生相同哈希")
		}
	})
}

// ── abs ────────────────────────────────────────────────────────

func TestAbs(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{5, 5},
		{-5, 5},
		{0, 0},
		{-1, 1},
		{100, 100},
		{-100, 100},
	}
	for _, tc := range tests {
		got := abs(tc.input)
		if got != tc.expected {
			t.Errorf("abs(%d) = %d, 期望 %d", tc.input, got, tc.expected)
		}
	}
}

// ── timePtr ─────────────────────────────────────────────────────

func TestTimePtr(t *testing.T) {
	now := time.Now()
	p := timePtr(now)
	if p == nil {
		t.Fatal("timePtr 不应返回 nil")
	}
	if !p.Equal(now) {
		t.Fatalf("timePtr 返回的时间不匹配: %v != %v", *p, now)
	}
}

// ── isTextFile ─────────────────────────────────────────────────

func TestIsTextFile(t *testing.T) {
	textCases := []string{
		"readme.md", "main.go", "app.py",
		"config.json", "data.yaml", "index.html",
	}
	for _, f := range textCases {
		if !isTextFile(f) {
			t.Errorf("%s 应被识别为文本文件", f)
		}
	}

	nonTextCases := []string{
		"photo.jpg", "archive.zip", "movie.mp4", "data.pdf", "font.ttf",
	}
	for _, f := range nonTextCases {
		if isTextFile(f) {
			t.Errorf("%s 不应被识别为文本文件", f)
		}
	}

	t.Run("大小写不敏感", func(t *testing.T) {
		if !isTextFile("README.MD") {
			t.Error(".MD 扩展名应匹配 .md")
		}
	})
}

// ── matchRagignoreDir ────────────────────────────────────────────

func TestMatchRagignoreDir(t *testing.T) {
	sep := string(filepath.Separator)
	root := sep + "tmp" + sep + "project"

	t.Run("精确目录名匹配", func(t *testing.T) {
		patterns := []string{"node_modules/"}
		if !matchRagignoreDir(root+sep+"node_modules", root, patterns) {
			t.Fatal("node_modules 应被忽略")
		}
	})

	t.Run("不匹配的目录", func(t *testing.T) {
		patterns := []string{"node_modules/"}
		if matchRagignoreDir(root+sep+"src", root, patterns) {
			t.Fatal("src 不应被忽略")
		}
	})

	t.Run("**.pyc 通配符匹配", func(t *testing.T) {
		patterns := []string{"**.pyc"}
		if !matchRagignoreDir(root+sep+"subdir.pyc", root, patterns) {
			t.Fatal("subdir.pyc 应被 **.pyc 规则忽略")
		}
	})

	t.Run("* 通配符匹配", func(t *testing.T) {
		patterns := []string{"*.swp"}
		if !matchRagignoreDir(root+sep+".swp", root, patterns) {
			t.Fatal(".swp 应被 *.swp 规则忽略")
		}
	})

	t.Run("嵌套路径包含模式匹配", func(t *testing.T) {
		patterns := []string{"dist"}
		if !matchRagignoreDir(root+sep+"a"+sep+"dist"+sep+"b", root, patterns) {
			t.Fatal("嵌套路径中的 dist 应被匹配")
		}
	})

	t.Run("空模式列表不匹配任何目录", func(t *testing.T) {
		if matchRagignoreDir(root+sep+"anything", root, nil) {
			t.Fatal("空模式列表不应匹配任何目录")
		}
	})
}

// ── loadRagignore ────────────────────────────────────────────────

func TestLoadRagignore(t *testing.T) {
	t.Run("文件不存在返回 nil", func(t *testing.T) {
		patterns := loadRagignore(t.TempDir())
		if patterns != nil {
			t.Fatal("不存在的 .ragignore 应返回 nil")
		}
	})

	t.Run("正常加载规则", func(t *testing.T) {
		dir := t.TempDir()
		content := "# 注释行应被跳过\nnode_modules/\ndist/\n.git/"
		if err := os.WriteFile(filepath.Join(dir, ".ragignore"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		patterns := loadRagignore(dir)
		if len(patterns) != 3 {
			t.Fatalf("应加载 3 条规则，实际: %d", len(patterns))
		}
		expected := []string{"node_modules/", "dist/", ".git/"}
		for i, exp := range expected {
			if patterns[i] != exp {
				t.Errorf("规则[%d] 期望 %q，实际 %q", i, exp, patterns[i])
			}
		}
	})

	t.Run("全部注释应返回 nil", func(t *testing.T) {
		dir := t.TempDir()
		content := "# 全部是注释\n# 另一行注释\n"
		if err := os.WriteFile(filepath.Join(dir, ".ragignore"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		patterns := loadRagignore(dir)
		if patterns != nil {
			t.Fatalf("全部注释时应返回 nil，实际: %v", patterns)
		}
	})
}

// ── computeFileHash ──────────────────────────────────────────────

func TestComputeFileHash(t *testing.T) {
	t.Run("正常文件计算哈希", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}

		hash, err := computeFileHash(f)
		if err != nil {
			t.Fatalf("计算哈希失败: %v", err)
		}
		if hash == "" {
			t.Fatal("哈希不应为空")
		}
	})

	t.Run("相同内容产生相同哈希", func(t *testing.T) {
		dir := t.TempDir()
		f1 := filepath.Join(dir, "a.txt")
		f2 := filepath.Join(dir, "b.txt")
		content := []byte("identical content")
		os.WriteFile(f1, content, 0644)
		os.WriteFile(f2, content, 0644)

		h1, _ := computeFileHash(f1)
		h2, _ := computeFileHash(f2)
		if h1 != h2 {
			t.Fatal("相同内容应产生相同哈希")
		}
	})

	t.Run("不存在的文件", func(t *testing.T) {
		_, err := computeFileHash("/nonexistent/file.txt")
		if err == nil {
			t.Fatal("不存在的文件应返回错误")
		}
	})
}
