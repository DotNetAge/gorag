package enhancement

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

func TestEnhance_Success(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(1),
	)

	fullDoc := "这是第一句话。这是第二句话。这是第三句话。"
	chunk := &core.Chunk{
		Content: "这是第二句话。",
		Metadata: map[string]any{
			"full_document": fullDoc,
		},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk})

	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Contains(t, chunks[0].Content, "第一句话")
	assert.Contains(t, chunks[0].Content, "第二句话")
	assert.Contains(t, chunks[0].Content, "第三句话")
}

func TestEnhance_NilChunks(t *testing.T) {
	expander := NewSentenceWindowExpander()

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, nil)

	assert.NoError(t, err)
	assert.Nil(t, chunks)
}

func TestEnhance_EmptyChunks(t *testing.T) {
	expander := NewSentenceWindowExpander()

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{})

	assert.NoError(t, err)
	assert.Len(t, chunks, 0)
}

func TestEnhance_NoFullDocument(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(2),
	)

	chunk := &core.Chunk{
		Content: "这是一段内容。",
		Metadata: map[string]any{},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk})

	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "这是一段内容。", chunks[0].Content)
}

func TestEnhance_WindowBoundary(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(1),
	)

	fullDoc := "第一句。第二句。第三句。第四句。第五句。"
	chunk := &core.Chunk{
		Content: "第一句。",
		Metadata: map[string]any{
			"full_document": fullDoc,
		},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk})

	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "第一句。 第二句。", chunks[0].Content)
}

func TestEnhance_WindowBoundary_EndOfDocument(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(2),
	)

	fullDoc := "第一句。第二句。第三句。"
	chunk := &core.Chunk{
		Content: "第三句。",
		Metadata: map[string]any{
			"full_document": fullDoc,
		},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk})

	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "第一句。 第二句。 第三句。", chunks[0].Content)
}

func TestEnhance_MaxChars(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(10),
		WithMaxChars(10),
	)

	fullDoc := "A。B。"
	chunk := &core.Chunk{
		Content: "B。",
		Metadata: map[string]any{
			"full_document": fullDoc,
		},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk})

	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.LessOrEqual(t, len(chunks[0].Content), 10)
}

func TestEnhance_MergeMetadata(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(1),
	)

	fullDoc := "第一句。第二句。第三句。"
	chunk := &core.Chunk{
		Content: "第二句。",
		Metadata: map[string]any{
			"full_document": fullDoc,
			"source":        "test.pdf",
			"page":          5,
		},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk})

	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "test.pdf", chunks[0].Metadata["source"])
	assert.Equal(t, 5, chunks[0].Metadata["page"])
}

func TestEnhance_MultipleChunks(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(1),
	)

	fullDoc := "第一句。第二句。第三句。"

	chunk1 := &core.Chunk{
		Content: "第一句。",
		Metadata: map[string]any{
			"full_document": fullDoc,
		},
	}

	chunk2 := &core.Chunk{
		Content: "第三句。",
		Metadata: map[string]any{
			"full_document": fullDoc,
		},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk1, chunk2})

	assert.NoError(t, err)
	assert.Len(t, chunks, 2)
}

func TestEnhance_ChunkNotFoundInDocument(t *testing.T) {
	expander := NewSentenceWindowExpander(
		WithWindowSize(2),
	)

	fullDoc := "第一句。第二句。第三句。"
	chunk := &core.Chunk{
		Content: "完全不匹配的内容。",
		Metadata: map[string]any{
			"full_document": fullDoc,
		},
	}

	query := core.NewQuery("1", "测试查询", nil)
	chunks, err := expander.Enhance(context.Background(), query, []*core.Chunk{chunk})

	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "完全不匹配的内容。", chunks[0].Content)
}

func TestSplitIntoSentences(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "multiple sentences",
			text:     "这是第一句。这是第二句。这是第三句。",
			expected: 3,
		},
		{
			name:     "single sentence",
			text:     "只有一个句子。",
			expected: 1,
		},
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "no sentence ending",
			text:     "没有句号结尾",
			expected: 1,
		},
		{
			name:     "with question marks",
			text:     "这是问句？这是另一个问句？",
			expected: 2,
		},
		{
			name:     "with exclamation marks",
			text:     "太棒了！真厉害！",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentences := splitIntoSentences(tt.text)
			assert.Len(t, sentences, tt.expected)
		})
	}
}
