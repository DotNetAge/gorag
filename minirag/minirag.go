// Package minirag 轻量移动端 RAG 索引器
// 可通过 gomobile bind 编译为 iOS/Android 原生 framework。
//
// iOS:  gomobile bind -target=ios     -o MiniRAG.xcframework ./minirag
// Android: gomobile bind -target=android -o MiniRAG.aar        ./minirag
//
// Embedder 由平台侧注入（iOS: Core ML, Android: ML Kit），
// ML 推理在平台侧完成，NewRAG 只负责向量存储与检索。
package minirag

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/store/vector/govector"
)

// Embedder 嵌入向量计算器 — 由平台层实现。
// EmbedText 接收文本，返回 float32 小端字节序的向量（4 字节/维）。
type Embedder interface {
	EmbedText(text string) ([]byte, error)
}

// NewRAG 轻量移动端 RAG 索引器
type NewRAG struct {
	store core.VectorStore
	emb   Embedder
}

// New 创建 NewRAG 实例。
// dataDir: 持久化目录（iOS 传 NSDocumentDirectory）
// dimension: 向量维度（与 ML 模型输出一致）
// emb: 平台侧 Embedder 实现
func New(dataDir string, dimension int, emb Embedder) (*NewRAG, error) {
	if emb == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	store, err := govector.NewStore(
		govector.WithCollection("minirag"),
		govector.WithDimension(dimension),
		govector.WithDBPath(dataDir+"/vectors.db"),
		govector.WithHNSW(true),
	)
	if err != nil {
		return nil, fmt.Errorf("vector store: %w", err)
	}
	return &NewRAG{store: store, emb: emb}, nil
}

// AddText 索引文本：分块 → 嵌入 → 存储。
// 返回 JSON: [{"id":"..","content":".."},...]
func (r *NewRAG) AddText(content string) ([]byte, error) {
	parts := splitText(content)
	if len(parts) == 0 {
		return []byte("[]"), nil
	}

	ctx := context.Background()
	type chunkItem struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	items := make([]chunkItem, 0, len(parts))

	for _, text := range parts {
		vecBytes, err := r.emb.EmbedText(text)
		if err != nil {
			return nil, fmt.Errorf("embed '%s': %w", truncate(text, 16), err)
		}
		id := contentID(text)
		if err := r.store.Upsert(ctx, []*core.Vector{{
			ID:      id,
			Values:  bytesToF32s(vecBytes),
			ChunkID: id,
			Metadata: map[string]any{
				"content": text,
			},
		}}); err != nil {
			return nil, fmt.Errorf("store: %w", err)
		}
		items = append(items, chunkItem{ID: id, Content: text})
	}

	return json.Marshal(items)
}

// AddFile 索引文件：读取文件内容 → 分块 → 嵌入 → 存储。
// 支持 .txt 文件及可读文本文件，自动检测编码。
// 返回 JSON: [{"id":"..","content":"..","filename":"..","filepath":".."},...]
func (r *NewRAG) AddFile(filePath string) ([]byte, error) {
	// 验证文件路径非空
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	// 规范化路径并验证文件存在
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", absPath)
		}
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// 验证是文件（非目录）
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", absPath)
	}

	// 检查文件大小限制 (最大 10MB)
	const maxFileSize = 10 << 20 // 10MB
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file too large (%d bytes, max %d): %s", info.Size(), maxFileSize, absPath)
	}

	// 读取文件内容
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	text := string(content)
	if strings.TrimSpace(text) == "" {
		return []byte("[]"), nil
	}

	// 获取文件名用于元数据
	filename := filepath.Base(absPath)

	// 复用分块逻辑
	parts := splitText(text)
	if len(parts) == 0 {
		return []byte("[]"), nil
	}

	ctx := context.Background()
	type fileChunkItem struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		Filename string `json:"filename"`
		Filepath string `json:"filepath"`
	}
	items := make([]fileChunkItem, 0, len(parts))

	for _, part := range parts {
		vecBytes, err := r.emb.EmbedText(part)
		if err != nil {
			return nil, fmt.Errorf("embed '%s': %w", truncate(part, 16), err)
		}
		id := contentID(part)
		if err := r.store.Upsert(ctx, []*core.Vector{{
			ID:      id,
			Values:  bytesToF32s(vecBytes),
			ChunkID: id,
			Metadata: map[string]any{
				"content":  part,
				"filename": filename,
				"filepath": absPath,
			},
		}}); err != nil {
			return nil, fmt.Errorf("store: %w", err)
		}
		items = append(items, fileChunkItem{
			ID:       id,
			Content:  part,
			Filename: filename,
			Filepath: absPath,
		})
	}

	return json.Marshal(items)
}

// Search 搜索文本：嵌入 → 向量检索。
// 返回 JSON: [{"id":"..","content":"..","score":0.95},...]
func (r *NewRAG) Search(query string, topK int) ([]byte, error) {
	if topK <= 0 {
		topK = 10
	}

	vecBytes, err := r.emb.EmbedText(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	vectors, scores, err := r.store.Search(context.Background(), bytesToF32s(vecBytes), topK, nil)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	type hitItem struct {
		ID      string  `json:"id"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	}
	results := make([]hitItem, len(vectors))
	for i, v := range vectors {
		content, _ := v.Metadata["content"].(string)
		results[i] = hitItem{
			ID:      v.ID,
			Content: content,
			Score:   float64(scores[i]),
		}
	}
	return json.Marshal(results)
}

// Delete 删除指定记录
func (r *NewRAG) Delete(id string) error {
	return r.store.Delete(context.Background(), id)
}

// Close 关闭索引器
func (r *NewRAG) Close() error {
	return r.store.Close(context.Background())
}

// ---

// splitText 将文本按段落分块。
// 每段作为一个独立 chunk，段落以双换行分隔。
func splitText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	paragraphs := strings.Split(text, "\n\n")
	var out []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		out = []string{text}
	}
	return out
}

func contentID(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}

func bytesToF32s(b []byte) []float32 {
	if len(b) == 0 {
		return nil
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4 : i*4+4]))
	}
	return out
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
