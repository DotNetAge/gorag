package document

import (
	"io"
	"time"
)

// RawDocType 文档归一化后的 4 种类型。
//
// 按文件内容本质归一化为 4 类，消除 MIME/扩展名歧义。
// 分块器、结构化器、提取器均依据 RawDocType 路由到对应实现。
type RawDocType string

const (
	// RawDocImage 图片类型：jpg/png/gif/webp/bmp/tiff 等，缩限最小边长后转 Base64。
	RawDocImage RawDocType = "image"

	// RawDocDoc 文档类型：pdf/docx/html/epub/pptx 等，统一为 Markdown。
	RawDocDoc RawDocType = "document"
	// RawDocText 纯文本类型：txt/md/代码等，内容不变。
	RawDocText RawDocType = "text"
	// RawDocData 数据类型：csv/json/xml/yaml/toml/excel/eml/msg/log 等，统一为 JSON。
	RawDocData RawDocType = "data"
)

// RawDoc 归一化后的文档接口。
//
// 设计要点：
//   - 接口化保证文档原子性：实现可以是基于文件、内存或网络的任意来源
//   - 7 个方法统一文档基础信息：ID/Type/FileName/Content/Size/ModTime/Meta
//   - FileName() 必须返回绝对路径（相对路径视为 critical bug）
//   - 不支持内嵌附件
//
// 实现由 rawdoc.go 提供：imageDoc / docDoc / textDoc / dataDoc 4 种类型。
// 工厂方法 Open(path) / New(content, docType) 也在 rawdoc.go 中实现。
type RawDoc interface {
	// ID 返回文档唯一标识（FileName 的 SHA256；New 场景基于内容生成）。
	ID() string

	// Type 返回归一化类型，用于路由分块器/结构化器/提取器实现。
	Type() RawDocType
	// FileName 返回文件名（必须是绝对路径；New 场景允许为空）。
	FileName() string
	// Content 返回归一化后的内容（文档为 Markdown，数据为 JSON，图片为 Base64，文本为原文）。
	Content() string
	// Size 返回原始文件大小（字节）。
	Size() int64
	// ModTime 返回原始文件修改时间。
	ModTime() time.Time
	// Meta 返回元数据（如 title/pages/doc_type/thumbnail_size 等可选扩展信息）。
	Meta() map[string]any
}

// ParseFunc 文件解析器函数签名。
//
// 解析器返回 RawDoc 接口实现，由具体类型（image/doc/text/data）决定归一化策略。
// fileName/size/modTime 等文件级元数据由 Open 工厂在调用 ParseFunc 之后回填。
type ParseFunc func(r io.Reader) (RawDoc, error)
