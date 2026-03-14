package log

import (
	"context"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestParser_New(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
	assert.Equal(t, 500, parser.chunkSize)
	assert.Equal(t, 50, parser.chunkOverlap)
	assert.Equal(t, Unknown, parser.format)
}

func TestParser_SetChunkSize(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(1000)
	assert.Equal(t, 1000, parser.chunkSize)
}

func TestParser_SetChunkOverlap(t *testing.T) {
	parser := NewParser()
	parser.SetChunkOverlap(100)
	assert.Equal(t, 100, parser.chunkOverlap)
}

func TestParser_SetFormat(t *testing.T) {
	parser := NewParser()
	parser.SetFormat(Nginx)
	assert.Equal(t, Nginx, parser.format)
}

func TestParser_SetPattern(t *testing.T) {
	parser := NewParser()
	err := parser.SetPattern(`^\d{4}-\d{2}-\d{2}`)
	assert.NoError(t, err)
	assert.Equal(t, Custom, parser.format)
	assert.NotNil(t, parser.pattern)

	// Test invalid pattern
	err = parser.SetPattern(`[invalid-regex`)
	assert.Error(t, err)
}

func TestParser_GetSupportedTypes(t *testing.T) {
	parser := NewParser()
	supported := parser.GetSupportedTypes()
	assert.Contains(t, supported, ".log")
	assert.Contains(t, supported, ".txt")
}

func TestParser_Format_String(t *testing.T) {
	assert.Equal(t, "nginx", Nginx.String())
	assert.Equal(t, "apache", Apache.String())
	assert.Equal(t, "syslog", Syslog.String())
	assert.Equal(t, "custom", Custom.String())
	assert.Equal(t, "unknown", Unknown.String())
}

func TestParser_ParseStream_Basic(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	parser.SetChunkOverlap(10)

	logContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	reader := strings.NewReader(logContent)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "Line 1")
	assert.Contains(t, docs[0].Content, "Line 5")
	assert.Equal(t, "log", docs[0].Metadata["type"])
	assert.Equal(t, "0", docs[0].Metadata["position"])
}

func TestParser_ParseStream_WithOverlap(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(20)
	parser.SetChunkOverlap(5)

	logContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6"
	reader := strings.NewReader(logContent)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 3)
}

func TestParser_ParseStream_NginxFormat(t *testing.T) {
	parser := NewParser()
	parser.SetFormat(Nginx)
	parser.SetChunkSize(200)

	nginxLog := `192.168.1.1 - - [01/Jan/2023:12:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`
	reader := strings.NewReader(nginxLog)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "192.168.1.1")
	assert.Contains(t, docs[0].Content, "GET /index.html")
	assert.Equal(t, "nginx", docs[0].Metadata["format"])
}

func TestParser_ParseStream_ApacheFormat(t *testing.T) {
	parser := NewParser()
	parser.SetFormat(Apache)
	parser.SetChunkSize(200)

	apacheLog := `192.168.1.1 - - [01/Jan/2023:12:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`
	reader := strings.NewReader(apacheLog)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "192.168.1.1")
	assert.Equal(t, "apache", docs[0].Metadata["format"])
}

func TestParser_ParseStream_SyslogFormat(t *testing.T) {
	parser := NewParser()
	parser.SetFormat(Syslog)
	parser.SetChunkSize(200)

	syslog := `Jan  1 12:00:00 server1 sshd[1234]: Accepted publickey for user from 192.168.1.1 port 22 ssh2`
	reader := strings.NewReader(syslog)

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 1)
	assert.Contains(t, docs[0].Content, "server1")
	assert.Contains(t, docs[0].Content, "sshd")
	assert.Equal(t, "syslog", docs[0].Metadata["format"])
}

func TestParser_ParseStream_EmptyFile(t *testing.T) {
	parser := NewParser()
	reader := strings.NewReader("")

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 0)
}

func TestParser_ParseStream_OnlyEmptyLines(t *testing.T) {
	parser := NewParser()
	reader := strings.NewReader("\n\n\n")

	ctx := context.Background()
	docChan, err := parser.ParseStream(ctx, reader, nil)
	assert.NoError(t, err)

	docs := []*entity.Document{}
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Len(t, docs, 0)
}

func TestParser_DetectFormat_Nginx(t *testing.T) {
	parser := NewParser()
	nginxLog := `192.168.1.1 - - [01/Jan/2023:12:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`
	reader := strings.NewReader(nginxLog)

	format, err := parser.DetectFormat(reader)
	assert.NoError(t, err)
	assert.Equal(t, Nginx, format)
}

func TestParser_DetectFormat_Syslog(t *testing.T) {
	parser := NewParser()
	syslog := `Jan  1 12:00:00 server1 sshd[1234]: Accepted publickey for user from 192.168.1.1 port 22 ssh2`
	reader := strings.NewReader(syslog)

	format, err := parser.DetectFormat(reader)
	assert.NoError(t, err)
	assert.Equal(t, Syslog, format)
}

func TestParser_DetectFormat_Unknown(t *testing.T) {
	parser := NewParser()
	unknownLog := `This is not a recognized log format`
	reader := strings.NewReader(unknownLog)

	format, err := parser.DetectFormat(reader)
	assert.NoError(t, err)
	assert.Equal(t, Unknown, format)
}

func TestParser_ExtractMetadata_Nginx(t *testing.T) {
	parser := NewParser()
	parser.SetFormat(Nginx)

	nginxLog := `192.168.1.1 - - [01/Jan/2023:12:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`
	metadata := parser.extractMetadata(nginxLog)
	assert.Contains(t, metadata, "192.168.1.1")
	assert.Contains(t, metadata, "GET")
	assert.Contains(t, metadata, "/index.html")
	assert.Contains(t, metadata, "200")
}

func TestParser_ExtractMetadata_Syslog(t *testing.T) {
	parser := NewParser()
	parser.SetFormat(Syslog)

	syslog := `Jan  1 12:00:00 server1 sshd[1234]: Accepted publickey for user from 192.168.1.1 port 22 ssh2`
	metadata := parser.extractMetadata(syslog)
	assert.Contains(t, metadata, "Jan  1 12:00:00")
	assert.Contains(t, metadata, "server1")
	assert.Contains(t, metadata, "sshd")
}

func TestParser_IdentifyFormat(t *testing.T) {
	parser := NewParser()

	// Test nginx format
	nginxLog := `192.168.1.1 - - [01/Jan/2023:12:00:00 +0000] "GET /index.html HTTP/1.1" 200 1234`
	assert.Equal(t, Nginx, parser.identifyFormat(nginxLog))

	// Test syslog format
	syslog := `Jan  1 12:00:00 server1 sshd[1234]: Accepted publickey`
	assert.Equal(t, Syslog, parser.identifyFormat(syslog))

	// Test unknown format
	unknown := `This is not a log line`
	assert.Equal(t, Unknown, parser.identifyFormat(unknown))
}
