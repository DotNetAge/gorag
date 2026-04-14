package embedder

import (
	"github.com/DotNetAge/gorag/core"
)

// multimodelEmbedder 是 ChineseClipEmbedder 的包装器
type multimodelEmbedder struct {
	clip *ChineseClipEmbedder
}

// NewMultiModelEmbedder 创建多模型向量化器
// opts: 配置选项，如 WithModelDir, WithModel
func NewMultiModelEmbedder(opts ...ChineseClipOption) (core.Embedder, error) {
	return NewChineseClipEmbedder(opts...)
}

// Calc 计算单个 chunk 的向量
func (e *multimodelEmbedder) Calc(chunk *core.Chunk) (*core.Vector, error) {
	return e.clip.Calc(chunk)
}

// CalcText 计算文本的向量
func (e *multimodelEmbedder) CalcText(text string) (*core.Vector, error) {
	return e.clip.CalcText(text)
}

// Bulk 批量计算向量
func (e *multimodelEmbedder) Bulk(chunks []*core.Chunk) ([]*core.Vector, error) {
	return e.clip.Bulk(chunks)
}

// CalcImage 计算图像的向量
func (e *multimodelEmbedder) CalcImage(data []byte) (*core.Vector, error) {
	return e.clip.CalcImage(data)
}

// CLIP 创建 Chinese-CLIP 向量化器（默认配置）
func CLIP() core.Embedder {
	embedder, _ := NewChineseClipEmbedder()
	return &multimodelEmbedder{clip: embedder}
}

// BGEEmbedder 是 BGEEmbedder 的包装器
type BGEWrapper struct {
	bge *BGEEmbedder
}

// NewBGEEmbedder 创建 BGE 向量化器
// opts: 配置选项，如 WithModelDir, WithModel, WithVocab
func NewBGE(opts ...BGEOption) (core.Embedder, error) {
	return NewBGEEmbedder(opts...)
}

// Calc 计算单个 chunk 的向量
func (e *BGEWrapper) Calc(chunk *core.Chunk) (*core.Vector, error) {
	return e.bge.Calc(chunk)
}

// CalcText 计算文本的向量
func (e *BGEWrapper) CalcText(text string) (*core.Vector, error) {
	return e.bge.CalcText(text)
}

// Bulk 批量计算向量
func (e *BGEWrapper) Bulk(chunks []*core.Chunk) ([]*core.Vector, error) {
	return e.bge.Bulk(chunks)
}

// CalcImage BGE 不支持图像
func (e *BGEWrapper) CalcImage(data []byte) (*core.Vector, error) {
	return e.bge.CalcImage(data)
}

// BGE 创建 BGE 向量化器（默认配置）
func BGE() core.Embedder {
	embedder, _ := NewBGEEmbedder()
	return &BGEWrapper{bge: embedder}
}
