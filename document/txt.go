package document

import (
	"io"
)

// Load 读取文本文件，返回RawDocument
func ParseText(r io.Reader) (*RawDocument, error) {
	var content []byte
	var err error
	if content, err = io.ReadAll(r); err != nil {
		return nil, err
	}

	// 获取文件信息
	// 这里需要获取文件路径，但r只提供了io.Reader，所以需要从其他地方获取路径
	// info, err := os.Stat(path)
	// if err != nil {
	// 	return nil, err
	// }

	// 根据文件扩展名设置正确的FileType
	// ext := filepath.Ext(path)
	// fileType := core.MimeTypeTextPlain
	// if mime, ok := core.MimeTypes[ext]; ok {
	// 	fileType = mime
	// }

	// metadata := map[string]any{
	// 	"filename": path,
	// 	"size":     info.Size(),
	// 	"modTime":  info.ModTime(),
	// 	"encoding": "UTF-8",
	// }

	// return &Result{
	// 	Mime:    "text/plain",
	// 	Content: string(content),
	// 	Meta:    map[string]any{},
	// }, nil
	return NewRawDoc(string(content)), nil
}
