package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVector(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		values     []float32
		chunkID    string
		metadata   map[string]any
		wantLength int
	}{
		{
			name:       "basic vector",
			id:         "vec-1",
			values:     []float32{0.1, 0.2, 0.3},
			chunkID:    "chunk-1",
			metadata:   map[string]any{"dim": 3},
			wantLength: 3,
		},
		{
			name:       "nil metadata",
			id:         "vec-2",
			values:     []float32{1.0, 2.0},
			chunkID:    "chunk-2",
			metadata:   nil,
			wantLength: 2,
		},
		{
			name:       "empty values",
			id:         "vec-3",
			values:     []float32{},
			chunkID:    "chunk-3",
			metadata:   map[string]any{},
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vector := NewVector(tt.id, tt.values, tt.chunkID, tt.metadata)

			assert.Equal(t, tt.id, vector.ID)
			assert.Equal(t, tt.chunkID, vector.ChunkID)
			assert.Equal(t, tt.metadata, vector.Metadata)
			assert.Len(t, vector.Values, tt.wantLength)
			if len(tt.values) > 0 {
				assert.Equal(t, tt.values, vector.Values)
			}
		})
	}
}
