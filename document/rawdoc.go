package document

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DotNetAge/gorag/v2/utils"
)

// =====================================================================
// RawDoc 4 个实现 + Open/New 工厂 + newParsedDoc 内部构造函数
// =====================================================================

// ---- 公共结构 ----

// baseRawDoc 4 种 RawDoc 实现的公共字段。
//
// 设计要点：
//   - fileName 必须为绝对路径（Open 强制校验，New 允许空字符串）
//   - content 已归一化（image=Base64 缩略图；doc=Markdown；text=原文；data=JSON）
//   - meta 由具体实现填充（如 image 的 mime_type/thumbnail_size，data 的 source_format 等）
type baseRawDoc struct {
	docType  RawDocType
	fileName string         // 绝对路径（New 场景允许为空）
	content  string         // 归一化后的内容
	size     int64          // 原始文件大小
	modTime  time.Time      // 原始文件修改时间
	meta     map[string]any // 元数据
}

// ID 返回基于 FileName 的 SHA256（空字符串时基于内容生成）。
func (b *baseRawDoc) ID() string {
	if b.fileName == "" {
		// New 场景：基于内容生成 ID，保证可索引
		return utils.GenerateID([]byte(b.content))
	}
	return utils.GenerateID([]byte(b.fileName))
}

// Type 返回归一化类型。
func (b *baseRawDoc) Type() RawDocType { return b.docType }

// FileName 返回绝对路径。
func (b *baseRawDoc) FileName() string { return b.fileName }

// Content 返回归一化内容。
func (b *baseRawDoc) Content() string { return b.content }

// Size 返回原始文件大小。
func (b *baseRawDoc) Size() int64 { return b.size }

// ModTime 返回原始文件修改时间。
func (b *baseRawDoc) ModTime() time.Time { return b.modTime }

// Meta 返回元数据。
func (b *baseRawDoc) Meta() map[string]any {
	if b.meta == nil {
		return map[string]any{}
	}
	return b.meta
}

// ---- 4 种实现类型（仅类型不同，方法共享 baseRawDoc） ----

// imageDoc 图片类型 RawDoc：jpg/png/gif 等，缩限最小边长后转 Base64。
type imageDoc struct{ baseRawDoc }

// docDoc 文档类型 RawDoc：epub/html/pdf/docx/md 等，统一为 Markdown。
type docDoc struct{ baseRawDoc }

// textDoc 纯文本类型 RawDoc：txt/md/代码等，内容不变。
type textDoc struct{ baseRawDoc }

// dataDoc 数据类型 RawDoc：csv/json/xml/yaml/toml/excel/eml/msg/log 等，统一为 JSON 字符串。
type dataDoc struct{ baseRawDoc }

// ---- 内部构造函数（供 ParseXXX 使用） ----

// newParsedDoc 由 ParseXXX 调用以构造 RawDoc 实现。
//
// 设计：
//   - ParseXXX 仅负责归一化字符串内容 + 提取业务元数据
//   - 文件级元数据（fileName/size/modTime）由 Open 工厂在调用后回填
//   - docType 由 ParseXXX 内部根据内容特性决定（如 csv→data、html→doc、jpg→image）
func newParsedDoc(content string, meta map[string]any, docType RawDocType) RawDoc {
	if meta == nil {
		meta = map[string]any{}
	}
	base := baseRawDoc{
		docType: docType,
		content: content,
		size:    int64(len(content)),
		modTime: time.Time{},
		meta:    meta,
	}
	switch docType {
	case RawDocImage:
		return &imageDoc{base}
	case RawDocDoc:
		return &docDoc{base}
	case RawDocData:
		return &dataDoc{base}
	case RawDocText:
		return &textDoc{base}
	default:
		// 兜底为 textDoc
		base.docType = RawDocText
		return &textDoc{base}
	}
}

// ---- 工厂方法 ----

// Open 基于文件路径打开并归一化为 RawDoc。
//
// 设计约束：
//   - path 必须为绝对路径，否则返回 error
//   - 自动根据扩展名归一化为 4 种类型之一
//   - 调用 ParseFunc 获取归一化内容，再回填 fileName/size/modTime
func Open(path string) (RawDoc, error) {
	if path == "" {
		return nil, fmt.Errorf("Open: 文件路径为空")
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("Open: 文件路径必须为绝对路径, 实际为 %q", path)
	}

	// 读取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("Open: 获取 %s 文件信息失败: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("Open: %s 是目录而非文件", path)
	}

	// 打开文件并调用 ParseFunc
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Open: 打开 %s 失败: %w", path, err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	pf := getParserByExt(ext)

	parsed, err := pf(f)
	if err != nil {
		return nil, fmt.Errorf("Open: 解析 %s 扩展名失败: %w", ext, err)
	}

	// 回填文件级元数据（fileName/size/modTime）
	return withFileMeta(parsed, path, info.Size(), info.ModTime()), nil
}

// New 从文本内容和类型构造 RawDoc（无文件场景）。
//
// 适用场景：
//   - 直接索引内存中的文本（如 API 接收的字符串）
//   - 测试用例构造
//   - 动态生成内容（如 LLM 输出）
//
// fileName 字段为空，ID() 基于内容生成。
// 调用方应保证 content 已归一化（New 不会再走 ParseFunc）。
func New(content string, docType RawDocType) RawDoc {
	meta := map[string]any{}
	base := baseRawDoc{
		docType:  docType,
		fileName: "",
		content:  content,
		size:     int64(len(content)),
		modTime:  time.Time{},
		meta:     meta,
	}
	switch docType {
	case RawDocImage:
		return &imageDoc{base}
	case RawDocDoc:
		return &docDoc{base}
	case RawDocData:
		return &dataDoc{base}
	case RawDocText:
		return &textDoc{base}
	default:
		// 兜底为 textDoc
		base.docType = RawDocText
		return &textDoc{base}
	}
}

// withFileMeta 将文件级元数据回填到 ParseFunc 返回的 RawDoc 上。
//
// 实现方式：通过类型断言获取 baseRawDoc 指针，直接修改字段。
// 这样 ParseXXX 函数只需关注内容归一化，文件元数据由 Open 统一回填。
func withFileMeta(doc RawDoc, fileName string, size int64, modTime time.Time) RawDoc {
	type fileMetaSetter interface {
		setFileMeta(fileName string, size int64, modTime time.Time)
	}
	if s, ok := doc.(fileMetaSetter); ok {
		s.setFileMeta(fileName, size, modTime)
	}
	return doc
}

// setFileMeta 由 baseRawDoc 实现，用于回填文件级元数据。
func (b *baseRawDoc) setFileMeta(fileName string, size int64, modTime time.Time) {
	b.fileName = fileName
	// 仅在 ParseFunc 未设置 size（即 0 或与文件大小不一致）时回填
	if b.size == 0 {
		b.size = size
	}
	b.modTime = modTime
}
