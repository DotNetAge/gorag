package rerank

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRerankScores_ValidJSON(t *testing.T) {
	content := `[0.9, 0.3, 0.7, 0.1]`
	scores, err := parseRerankScores(content, 4)

	assert.NoError(t, err)
	assert.Len(t, scores, 4)
	assert.Equal(t, float32(0.9), scores[0])
	assert.Equal(t, float32(0.3), scores[1])
	assert.Equal(t, float32(0.7), scores[2])
	assert.Equal(t, float32(0.1), scores[3])
}

func TestParseRerankScores_WithTextBeforeJSON(t *testing.T) {
	content := `Here are the scores:
[0.8, 0.6, 0.4]`

	scores, err := parseRerankScores(content, 3)

	assert.NoError(t, err)
	assert.Len(t, scores, 3)
	assert.Equal(t, float32(0.8), scores[0])
}

func TestParseRerankScores_WithTextAfterJSON(t *testing.T) {
	content := `[0.5, 0.9]
These scores indicate...`

	scores, err := parseRerankScores(content, 2)

	assert.NoError(t, err)
	assert.Len(t, scores, 2)
	assert.Equal(t, float32(0.5), scores[0])
	assert.Equal(t, float32(0.9), scores[1])
}

func TestParseRerankScores_InvalidJSON(t *testing.T) {
	content := `not valid json`

	scores, err := parseRerankScores(content, 3)

	assert.Error(t, err)
	assert.Nil(t, scores)
}

func TestParseRerankScores_PartialScores(t *testing.T) {
	content := `[0.8, 0.6]`

	scores, err := parseRerankScores(content, 3)

	assert.NoError(t, err)
	assert.Len(t, scores, 2)
}

func TestParseRerankScores_ExtraScores(t *testing.T) {
	content := `[0.8, 0.6, 0.4, 0.2, 0.1]`

	scores, err := parseRerankScores(content, 3)

	assert.NoError(t, err)
	assert.Len(t, scores, 5)
}

func TestParseRerankScores_EmptyArray(t *testing.T) {
	content := `[]`

	scores, err := parseRerankScores(content, 0)

	assert.NoError(t, err)
	assert.Len(t, scores, 0)
}

func TestParseRerankScores_NoBrackets(t *testing.T) {
	content := `scores: 0.5, 0.9`

	scores, err := parseRerankScores(content, 2)

	assert.Error(t, err)
	assert.Nil(t, scores)
}

func TestParseRerankScores_EmptyContent(t *testing.T) {
	content := ``

	scores, err := parseRerankScores(content, 2)

	assert.Error(t, err)
	assert.Nil(t, scores)
}

func TestParseRerankScores_OnlyWhitespace(t *testing.T) {
	content := `   `

	scores, err := parseRerankScores(content, 2)

	assert.Error(t, err)
	assert.Nil(t, scores)
}

func TestParseRerankScores_SpaceBeforeBrackets(t *testing.T) {
	content := `  [0.7, 0.3]  `

	scores, err := parseRerankScores(content, 2)

	assert.NoError(t, err)
	assert.Len(t, scores, 2)
	assert.Equal(t, float32(0.7), scores[0])
}

func TestParseRerankScores_ScoresOutOfOrder(t *testing.T) {
	content := `[0.1, 0.9, 0.5]`

	scores, err := parseRerankScores(content, 3)

	assert.NoError(t, err)
	assert.Equal(t, float32(0.1), scores[0])
	assert.Equal(t, float32(0.9), scores[1])
	assert.Equal(t, float32(0.5), scores[2])
}
