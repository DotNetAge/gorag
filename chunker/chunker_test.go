package chunker

import (
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/document"
)

func TestFixedSizeChunker(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		chunkSize int
		overlap   int
		wantCount int
	}{
		{
			name:      "short text",
			content:   "Hello world",
			chunkSize: 100,
			overlap:   0,
			wantCount: 1,
		},
		{
			name:      "exact fit",
			content:   strings.Repeat("a", 100),
			chunkSize: 100,
			overlap:   0,
			wantCount: 1,
		},
		{
			name:      "multiple chunks",
			content:   strings.Repeat("a", 250),
			chunkSize: 100,
			overlap:   0,
			wantCount: 3,
		},
		{
			name:      "with overlap",
			content:   strings.Repeat("a", 200),
			chunkSize: 100,
			overlap:   20,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFixedSizeChunker(
				WithChunkSize(tt.chunkSize),
				WithOverlap(tt.overlap),
			)

			doc := document.New(tt.content, "text/plain")
			structured := &core.StructuredDocument{
				RawDoc: doc,
				Title:  "test",
				Root:   nil,
			}

			chunks, err := chunker.Chunk(structured)
			if err != nil {
				t.Fatalf("Chunk() error = %v", err)
			}

			if len(chunks) != tt.wantCount {
				t.Errorf("Chunk() got %d chunks, want %d", len(chunks), tt.wantCount)
			}

			// 验证块大小
			for i, chunk := range chunks {
				if len(chunk.Content) > tt.chunkSize {
					t.Errorf("Chunk[%d] size %d exceeds chunkSize %d", i, len(chunk.Content), tt.chunkSize)
				}
				if chunk.ChunkMeta.Index != i {
					t.Errorf("Chunk[%d] has wrong index %d", i, chunk.ChunkMeta.Index)
				}
			}
		})
	}
}

func TestRecursiveChunker(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		chunkSize int
		wantCount int // approximate
	}{
		{
			name:      "short text",
			content:   "Hello world",
			chunkSize: 100,
			wantCount: 1,
		},
		{
			name:      "multiple paragraphs",
			content:   "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.",
			chunkSize: 50,
			wantCount: 3,
		},
		{
			name:      "long text",
			content:   strings.Repeat("This is a sentence. ", 20),
			chunkSize: 100,
			wantCount: 4, // approximate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewRecursiveChunker(WithChunkSize(tt.chunkSize))

			doc := document.New(tt.content, "text/plain")
			structured := &core.StructuredDocument{
				RawDoc: doc,
				Title:  "test",
				Root:   nil,
			}

			chunks, err := chunker.Chunk(structured)
			if err != nil {
				t.Fatalf("Chunk() error = %v", err)
			}

			// 验证块大小不超过限制
			for i, chunk := range chunks {
				if len(chunk.Content) > tt.chunkSize*2 { // 允许一定的灵活度
					t.Errorf("Chunk[%d] size %d is too large", i, len(chunk.Content))
				}
			}
		})
	}
}

func TestSentenceChunker(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		maxSentences int
		wantCount    int
	}{
		{
			name:         "single sentence",
			content:      "Hello world.",
			maxSentences: 5,
			wantCount:    1,
		},
		{
			name:         "multiple sentences",
			content:      "First sentence. Second sentence. Third sentence. Fourth sentence. Fifth sentence.",
			maxSentences: 3,
			wantCount:    2,
		},
		{
			name:         "chinese sentences",
			content:      "第一句话。第二句话。第三句话。",
			maxSentences: 2,
			wantCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewSentenceChunker(WithMaxSentences(tt.maxSentences))

			doc := document.New(tt.content, "text/plain")
			structured := &core.StructuredDocument{
				RawDoc: doc,
				Title:  "test",
				Root:   nil,
			}

			chunks, err := chunker.Chunk(structured)
			if err != nil {
				t.Fatalf("Chunk() error = %v", err)
			}

			if len(chunks) != tt.wantCount {
				t.Errorf("Chunk() got %d chunks, want %d", len(chunks), tt.wantCount)
			}
		})
	}
}

func TestParagraphChunker(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		maxParagraphs int
		wantCount     int
	}{
		{
			name:          "single paragraph",
			content:       "Single paragraph content.",
			maxParagraphs: 3,
			wantCount:     1,
		},
		{
			name:          "multiple paragraphs",
			content:       "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.\n\nFourth paragraph.",
			maxParagraphs: 2,
			wantCount:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewParagraphChunker(WithMaxParagraphs(tt.maxParagraphs))

			doc := document.New(tt.content, "text/plain")
			structured := &core.StructuredDocument{
				RawDoc: doc,
				Title:  "test",
				Root:   nil,
			}

			chunks, err := chunker.Chunk(structured)
			if err != nil {
				t.Fatalf("Chunk() error = %v", err)
			}

			if len(chunks) != tt.wantCount {
				t.Errorf("Chunk() got %d chunks, want %d", len(chunks), tt.wantCount)
			}
		})
	}
}

func TestCodeChunker(t *testing.T) {
	codeContent := `package main

func main() {
    fmt.Println("Hello, World!")
}

func helper() int {
    return 42
}
`

	rawDoc := document.New(codeContent, "text/x-go")

	structured := &core.StructuredDocument{
		RawDoc: rawDoc,
		Title:  "main.go",
		Root: &core.StructureNode{
			NodeType: "program",
			Children: []*core.StructureNode{
				{
					NodeType: "function",
					Title:    "main()",
					Text:     "func main() {\n    fmt.Println(\"Hello, World!\")\n}",
					StartPos: 0,
					EndPos:   50,
					Level:    1,
				},
				{
					NodeType: "function",
					Title:    "helper()",
					Text:     "func helper() int {\n    return 42\n}",
					StartPos: 52,
					EndPos:   85,
					Level:    1,
				},
			},
		},
	}

	chunker := NewCodeChunker()
	chunks, err := chunker.Chunk(structured)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// 验证元数据
	for _, chunk := range chunks {
		if chunk.MIMEType != "code" {
			t.Errorf("Expected ContentType 'code', got '%s'", chunk.MIMEType)
		}
		if _, ok := chunk.Metadata["node_type"]; !ok {
			t.Error("Missing node_type in metadata")
		}
	}
}

func TestParentDocChunker(t *testing.T) {
	content := "First paragraph with multiple sentences. Second sentence. Third sentence.\n\nSecond paragraph. Another sentence."

	doc := document.New(content, "text/plain")
	structured := &core.StructuredDocument{
		RawDoc: doc,
		Title:  "test",
		Root:   nil,
	}

	chunker := NewParentDocChunker(
		WithParentSize(150),
		WithChildSize(50),
	)

	chunks, err := chunker.Chunk(structured)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// 调试输出
	t.Logf("Total chunks: %d", len(chunks))
	for i, chunk := range chunks {
		isParent := false
		if chunk.Metadata != nil {
			isParent, _ = chunk.Metadata["is_parent"].(bool)
		}
		t.Logf("Chunk[%d]: isParent=%v, StartPos=%d, EndPos=%d, Content=%q",
			i, isParent, chunk.ChunkMeta.StartPos, chunk.ChunkMeta.EndPos, chunk.Content)
	}

	// 查找父块和子块
	var parents, children []*core.Chunk
	for _, chunk := range chunks {
		isParent := false
		if chunk.Metadata != nil {
			isParent, _ = chunk.Metadata["is_parent"].(bool)
		}
		if isParent {
			parents = append(parents, chunk)
		} else {
			children = append(children, chunk)
		}
	}

	if len(parents) == 0 {
		t.Error("Expected at least one parent chunk")
	}

	if len(children) == 0 {
		t.Error("Expected at least one child chunk")
	}

	// 验证父子关系
	for _, child := range children {
		if child.ParentID == "" {
			t.Errorf("Child chunk (StartPos=%d, EndPos=%d) has no parent",
				child.ChunkMeta.StartPos, child.ChunkMeta.EndPos)
		}
	}
}

func TestChunkingFactory(t *testing.T) {
	factory := NewChunkingFactory()

	// 测试创建各种策略的分块器
	strategies := []core.ChunkStrategy{
		StrategyFixedSize,
		StrategyRecursive,
		StrategySentence,
		StrategyParagraph,
		StrategyCode,
		StrategyParentDoc,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			chunker, err := factory.CreateChunker(strategy)
			if err != nil {
				t.Fatalf("CreateChunker(%s) error = %v", strategy, err)
			}

			if chunker == nil {
				t.Errorf("CreateChunker(%s) returned nil", strategy)
			}

			if chunker.GetStrategy() != strategy {
				t.Errorf("GetStrategy() = %s, want %s", chunker.GetStrategy(), strategy)
			}
		})
	}

	// 测试不支持的策略
	_, err := factory.CreateChunker("unsupported")
	if err == nil {
		t.Error("Expected error for unsupported strategy")
	}
}

func TestChunkValidator(t *testing.T) {
	chunks := []*core.Chunk{
		{
			ID:      "chunk-1",
			Content: strings.Repeat("a", 100),
		},
		{
			ID:      "chunk-2",
			Content: strings.Repeat("b", 200),
		},
		{
			ID:      "chunk-3",
			Content: "", // 空块
		},
	}

	validator := NewChunkValidator()
	report := validator.Validate(chunks)

	if report.TotalChunks != 3 {
		t.Errorf("TotalChunks = %d, want 3", report.TotalChunks)
	}

	if report.InvalidChunks != 1 {
		t.Errorf("InvalidChunks = %d, want 1", report.InvalidChunks)
	}

	if len(report.Errors) != 1 {
		t.Errorf("Errors count = %d, want 1", len(report.Errors))
	}

	if report.Errors[0].ErrorType != "empty_chunk" {
		t.Errorf("Error type = %s, want empty_chunk", report.Errors[0].ErrorType)
	}
}
