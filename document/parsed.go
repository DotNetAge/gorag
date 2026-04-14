package document

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// Document 统一原始文件载体，支持文本、PDF、图片、视频等多模态文件
type parsedDocument struct {
	RawDocument
	source string
	mime   string
}

func New(content string, mime string) core.Document {
	if mime == "" {
		mime = "text/plain"
	}

	// 从 MIME 类型推断扩展名作为 source
	source := ""
	if ext, ok := core.ExtMimeTypes[mime]; ok {
		source = "file" + ext
	}

	pf := getParserByMIME(mime)

	if pf != nil {
		d, err := pf(strings.NewReader(content))
		if err != nil {
			// 解析失败时兜底创建纯文本 RawDocument
			return &parsedDocument{
				RawDocument: *NewRawDoc(content),
				source:      source,
				mime:        mime,
			}
		}

		return &parsedDocument{
			RawDocument: *d,
			source:      source,
			mime:        mime,
		}
	}

	// 如果 pf 为 nil，兜底创建纯文本 RawDocument
	return &parsedDocument{
		RawDocument: *NewRawDoc(content),
		source:      source,
		mime:        mime,
	}
}

func Open(filePath string) (core.Document, error) {
	ext := filepath.Ext(filePath)
	pf := getParserByExt(ext)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	doc, err := pf(strings.NewReader(string(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ext, err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	doc.SetValue("size", info.Size()).
		SetValue("mod_time", info.ModTime())

	mime := core.MimeTypes[ext]
	if mime == "" {
		// 未知扩展名默认为纯文本类型
		mime = "text/plain"
	}

	return &parsedDocument{
		RawDocument: *doc,
		source:      filePath,
		mime:        mime,
	}, nil
}

func (r *parsedDocument) GetID() string {
	return r.RawDocument.GetID()
}

func (r *parsedDocument) GetContent() string {
	return r.RawDocument.Text
}

func (r *parsedDocument) GetMimeType() string {
	return r.mime
}

func (r *parsedDocument) GetMeta() map[string]any {
	return r.RawDocument.Meta
}

func (r *parsedDocument) GetImages() []core.Image {
	return r.RawDocument.Images
}

func (r *parsedDocument) GetSource() string {
	return r.source
}

func (r *parsedDocument) GetExt() string {
	if r.source == "" {
		return ""
	}
	return filepath.Ext(r.source)
}
