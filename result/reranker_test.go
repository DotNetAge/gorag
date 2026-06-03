package result

import (
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	testCases := []struct {
		name     string
		a, b     []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 2, 3},
			b:        []float32{1, 2, 3},
			expected: 1.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0},
			b:        []float32{0, 1},
			expected: 0.0,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1, 2},
			b:        []float32{1, 2, 3},
			expected: 0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cosineSimilarity(tc.a, tc.b)
			if tc.expected == 1.0 || tc.expected == -1.0 || tc.expected == 0.0 {
				if result != tc.expected {
					t.Errorf("expected %.4f, got %.4f", tc.expected, result)
				}
			} else {
				if absFloatDiff(result, tc.expected) > 0.001 {
					t.Errorf("expected ~%.4f, got %.4f", tc.expected, result)
				}
			}
		})
	}
}

func absFloatDiff(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}
