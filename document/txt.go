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
	return NewRawDoc(string(content)), nil
}
