package log

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logContent := []byte(`2024-03-19 10:00:00 INFO Application started
2024-03-19 10:00:01 DEBUG Loading configuration
2024-03-19 10:00:02 INFO Server listening on port 8080`)

	r := bytes.NewReader(logContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	logContent := []byte(`INFO: Test log message
ERROR: Something went wrong
DEBUG: Debugging information`)
	var chunkCount int

	err := parser.ParseWithCallback(ctx, bytes.NewReader(logContent), func(chunk core.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "log")
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
}

func TestParser_EmptyLog(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	logContent := []byte(``)
	chunks, err := parser.Parse(ctx, bytes.NewReader(logContent))
	require.NoError(t, err)
	_ = chunks // May be empty
}

func TestParser_LargeLog(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	var sb strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "2024-03-19 10:%02d:00 INFO Log message %d\n", i%60, i)
	}

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&sb, "INFO: Log line %d\n", i)
	}

	cancel()
	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	logContent := []byte(`INFO: Test`)
	err := parser.ParseWithCallback(ctx, bytes.NewReader(logContent), func(chunk core.Chunk) error {
		return assert.AnError
	})
	assert.Error(t, err)
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(200)
	parser.SetChunkOverlap(20)

	assert.Equal(t, 200, parser.chunkSize)
	assert.Equal(t, 20, parser.chunkOverlap)
}

func TestParser_FormatDetection(t *testing.T) {
	parser := NewParser()

	// Test nginx format detection
	nginxLog := `192.168.1.1 - - [19/Mar/2024:10:00:00 +0000] "GET /api/users HTTP/1.1" 200 1234`
	format := parser.identifyFormat(nginxLog)
	assert.Equal(t, Nginx, format)

	// Test syslog format detection
	syslogLine := `Mar 19 10:00:00 hostname sshd[1234]: Connection from 192.168.1.1`
	format = parser.identifyFormat(syslogLine)
	assert.Equal(t, Syslog, format)
}

func TestParser_MetadataExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetFormat(Nginx)

	line := `192.168.1.1 - - [19/Mar/2024:10:00:00 +0000] "GET /api/users HTTP/1.1" 200 1234`
	metadata := parser.extractNginxMetadata(line)
	assert.NotEmpty(t, metadata)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 2)
	assert.Contains(t, formats, ".log")
	assert.Contains(t, formats, ".txt")
}
