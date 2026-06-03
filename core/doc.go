// Package core provides fundamental types and interfaces for the goRAG framework.
package core

// Image represents binary image data extracted from documents.
// Images can be embedded within various document formats (PDF, DOCX, etc.)
// and need to be handled separately from text content.
type Image struct {
	data []byte
}

// NewImage creates a new Image instance with the given binary data.
func NewImage(data []byte) *Image {
	return &Image{
		data: data,
	}
}

// Data returns the underlying binary data of the image.
func (i *Image) Data() []byte {
	return i.data
}

// Document defines the interface for document objects in the RAG system.
// Documents are the input to the processing pipeline and carry raw content,
// metadata, and optional embedded resources like images.
type Document interface {
	GetID() string           // 原始文档唯一ID，用于与 StructuredDocument、Entity、Relation 关联
	GetContent() string      // 文件的纯文本内容（核心）
	GetMimeType() string     // 文件内容的类型
	GetMeta() map[string]any // 基础文件元数据（文件名、大小、修改时间、所有者等）
	GetImages() []Image      // 附带内容(例如包含在文件内的其它附件，如图片、视频、音频等)
	GetSource() string       // 文件来源（路径/URL/URI）
	GetExt() string          // 文件扩展名
}
