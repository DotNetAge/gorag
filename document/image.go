package document

import (
	"encoding/base64"
	"io"
)

// Load 读取图片文件，返回RawDocument
func ParseImage(r io.Reader) (*RawDocument, error) {
	// 读取图片二进制内容
	var contentBytes []byte
	var err error
	if contentBytes, err = io.ReadAll(r); err != nil {
		return nil, err
	}

	// 将图片编码为 base64，存入 Content
	content := base64.StdEncoding.EncodeToString(contentBytes)
	return NewRawDoc(content), nil
}
