package enhancer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSentenceWindowExpander_Enhance(t *testing.T) {
	tests := []struct {
		name           string
		chunk          *entity.Chunk
		windowSize     int
		maxChars       int
		expectedExpand bool
	}{
		{
			name: "successful sentence window expansion",
			chunk: &entity.Chunk{
				ID:      "c1",
				Content: "这是中间的一句话。",
				Metadata: map[string]any{
					"full_document": "完整文档内容。第一句。第二句。第三句。这是中间的一句话。下一句。再下一句。最后一句。",
				},
			},
			windowSize:     2,
			maxChars:       2000,
			expectedExpand: true,
		},
		{
			name: "no full document in metadata",
			chunk: &entity.Chunk{
				ID:       "c1",
				Content:  "Original chunk",
				Metadata: map[string]any{}, // No full_document
			},
			windowSize:     2,
			maxChars:       2000,
			expectedExpand: false,
		},
		{
			name: "max chars limit",
			chunk: &entity.Chunk{
				ID:      "c1",
				Content: "Middle sentence.",
				Metadata: map[string]any{
					"full_document": "Very long sentence 1. Very long sentence 2. Very long sentence 3. Middle sentence. Another long sentence 4. Another long sentence 5. Final sentence.",
				},
			},
			windowSize:     5,  // Would expand a lot
			maxChars:       50, // But limited by maxChars
			expectedExpand: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			expander := NewSentenceWindowExpander(
				WithWindowSize(tt.windowSize),
				WithMaxChars(tt.maxChars),
			)

			results := entity.NewRetrievalResult(
				uuid.New().String(),
				uuid.New().String(),
				[]*entity.Chunk{tt.chunk},
				[]float32{0.9},
				nil,
			)

			// Execute
			ctx := context.Background()
			expandedResults, err := expander.Enhance(ctx, results)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, expandedResults)
			assert.Len(t, expandedResults.Chunks, 1)

			if tt.expectedExpand {
				// Verify content is expanded
				expandedChunk := expandedResults.Chunks[0]
				assert.Greater(t, len(expandedChunk.Content), len(tt.chunk.Content))
				assert.Contains(t, expandedChunk.Content, tt.chunk.Content)
			} else {
				// Should return original chunk
				assert.Equal(t, tt.chunk.Content, expandedResults.Chunks[0].Content)
			}
		})
	}
}

func TestSplitIntoSentences(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int // Expected number of sentences
	}{
		{
			name:     "simple sentences",
			text:     "First sentence. Second sentence. Third sentence.",
			expected: 3,
		},
		{
			name:     "mixed punctuation",
			text:     "Question? Exclamation! Statement.",
			expected: 3,
		},
		{
			name:     "abbreviations",
			text:     "Dr. Smith works at ABC Corp. He is smart.",
			expected: 2, // "Dr." should not split
		},
		{
			name:     "single sentence",
			text:     "This is a single sentence without periods.",
			expected: 1,
		},
		{
			name:     "empty text",
			text:     "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentences := splitIntoSentences(tt.text)
			assert.Len(t, sentences, tt.expected)
		})
	}
}

func TestIsSentenceEnd(t *testing.T) {
	tests := []struct {
		name     string
		r        rune
		text     string
		pos      int
		expected bool
	}{
		{
			name:     "period at end",
			r:        '.',
			text:     "Sentence.",
			pos:      8,
			expected: true,
		},
		{
			name:     "question mark",
			r:        '?',
			text:     "Question?",
			pos:      8,
			expected: true,
		},
		{
			name:     "exclamation mark",
			r:        '!',
			text:     "Wow!",
			pos:      3,
			expected: true,
		},
		{
			name:     "period with lowercase after",
			r:        '.',
			text:     "Dr. Smith",
			pos:      2,
			expected: false, // Abbreviation
		},
		{
			name:     "comma is not sentence end",
			r:        ',',
			text:     "Hello, world",
			pos:      5,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSentenceEnd(tt.r, tt.text, tt.pos)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMinAndMax(t *testing.T) {
	// Test min function
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 0, min(0, 100))

	// Test max function
	assert.Equal(t, 5, max(3, 5))
	assert.Equal(t, 10, max(10, 5))
	assert.Equal(t, 100, max(0, 100))
}
