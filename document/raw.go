package document

import (
	"io"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/utils"
)

type ParseFunc func(r io.Reader) (*RawDocument, error)

type RawDocument struct {
	Text   string
	Images []core.Image
	Meta   map[string]any
}

func NewRawDoc(text string) *RawDocument {
	return &RawDocument{
		Text:   text,
		Images: make([]core.Image, 0),
		Meta:   make(map[string]any),
	}
}

func (r *RawDocument) GetID() string {
	if r.Text == "" {
		return ""
	}
	return utils.GenerateID([]byte(r.Text))
}

func (r *RawDocument) AddImage(data []byte) *RawDocument {
	r.Images = append(r.Images, *core.NewImage(data))

	return r
}

func (r *RawDocument) AddImages(data [][]byte) *RawDocument {
	for _, d := range data {
		r.AddImage(d)
	}
	return r
}

func (r *RawDocument) SetValue(key string, value any) *RawDocument {
	r.Meta[key] = value
	return r
}
