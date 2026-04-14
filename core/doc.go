package core

type Image struct {
	data []byte
}

func NewImage(data []byte) *Image {
	return &Image{
		data: data,
	}
}

func (i *Image) Data() []byte {
	return i.data
}

type Document interface {
	GetID() string           // 原始文档唯一ID，用于与 StructuredDocument、Entity、Relation 关联
	GetContent() string      // 文件的纯文本内容（核心）
	GetMimeType() string     // 文件内容的类型
	GetMeta() map[string]any // 基础文件元数据（文件名、大小、修改时间、所有者等）
	GetImages() []Image      // 附带内容(例如包含在文件内的其它附件，如图片、视频、音频等)
	GetSource() string       // 文件来源（路径/URL/URI）
	GetExt() string          // 文件扩展名
}
