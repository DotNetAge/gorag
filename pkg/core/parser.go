package core

import (
	"context"
	"io"
)

// Parser defines the interface for document parsing implementations.
// Parsers convert various file formats (PDF, DOCX, Markdown, etc.) into structured Document objects.
// They support both batch and streaming parsing modes for handling files of any size.
type Parser interface {
	Parse(ctx context.Context, content []byte, metadata map[string]any) (*Document, error)
	Supports(contentType string) bool
	// 使用 io.Reader 以支持流式解析大文件
	ParseStream(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *Document, error)
	GetSupportedTypes() []string
}
