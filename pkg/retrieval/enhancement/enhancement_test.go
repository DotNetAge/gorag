package enhancement

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRelevanceScores_ValidJSON(t *testing.T) {
	content := `[{"chunk_index": 0, "relevance": 0.9, "reason": "good"}, {"chunk_index": 1, "relevance": 0.3, "reason": "poor"}]`

	scores, err := parseRelevanceScores(content, 2)

	assert.NoError(t, err)
	assert.Len(t, scores, 2)
	assert.Equal(t, float32(0.9), scores[0])
	assert.Equal(t, float32(0.3), scores[1])
}

func TestParseRelevanceScores_WithTextBeforeJSON(t *testing.T) {
	content := `Here is the analysis:
[{"chunk_index": 0, "relevance": 0.8, "reason": "good"}, {"chunk_index": 1, "relevance": 0.6, "reason": "okay"}]`

	scores, err := parseRelevanceScores(content, 2)

	assert.NoError(t, err)
	assert.Len(t, scores, 2)
	assert.Equal(t, float32(0.8), scores[0])
	assert.Equal(t, float32(0.6), scores[1])
}

func TestParseRelevanceScores_WithTextAfterJSON(t *testing.T) {
	content := `[{"chunk_index": 0, "relevance": 0.7}]
This is additional text after the JSON.`

	scores, err := parseRelevanceScores(content, 1)

	assert.NoError(t, err)
	assert.Len(t, scores, 1)
	assert.Equal(t, float32(0.7), scores[0])
}

func TestParseRelevanceScores_InvalidJSON(t *testing.T) {
	content := `This is not valid JSON [{chunk_index}]`

	scores, err := parseRelevanceScores(content, 2)

	assert.Error(t, err)
	assert.Nil(t, scores)
}

func TestParseRelevanceScores_OutOfRangeIndex(t *testing.T) {
	content := `[{"chunk_index": 5, "relevance": 0.9, "reason": "bad index"}]`

	scores, err := parseRelevanceScores(content, 2)

	assert.NoError(t, err)
	assert.Len(t, scores, 2)
	assert.Equal(t, float32(0.0), scores[0])
	assert.Equal(t, float32(0.0), scores[1])
}

func TestParseRelevanceScores_NegativeIndex(t *testing.T) {
	content := `[{"chunk_index": -1, "relevance": 0.5, "reason": "negative"}]`

	scores, err := parseRelevanceScores(content, 2)

	assert.NoError(t, err)
	assert.Len(t, scores, 2)
	assert.Equal(t, float32(0.0), scores[0])
}

func TestParseRelevanceScores_PartialIndices(t *testing.T) {
	content := `[{"chunk_index": 0, "relevance": 0.9}, {"chunk_index": 2, "relevance": 0.7}]`

	scores, err := parseRelevanceScores(content, 3)

	assert.NoError(t, err)
	assert.Len(t, scores, 3)
	assert.Equal(t, float32(0.9), scores[0])
	assert.Equal(t, float32(0.0), scores[1])
	assert.Equal(t, float32(0.7), scores[2])
}

func TestParseRelevanceScores_EmptyArray(t *testing.T) {
	content := `[]`

	scores, err := parseRelevanceScores(content, 3)

	assert.NoError(t, err)
	assert.Len(t, scores, 3)
	for _, s := range scores {
		assert.Equal(t, float32(0.0), s)
	}
}

func TestParseRelevanceScores_EmptyContent(t *testing.T) {
	content := ``

	scores, err := parseRelevanceScores(content, 2)

	assert.Error(t, err)
	assert.Nil(t, scores)
}

func TestParseRelevanceScores_NoBrackets(t *testing.T) {
	content := `just some text without brackets`

	scores, err := parseRelevanceScores(content, 2)

	assert.Error(t, err)
	assert.Nil(t, scores)
}
