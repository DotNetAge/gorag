package embedder

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"
)

//go:embed chinese-clip-vocab.txt
var embeddedChineseClipVocab []byte

//go:embed bge-vocab.txt
var embeddedBGEVocab []byte

// VocabTokenizer 基于 vocab 的 BERT 分词器
// 支持两种 vocab 格式:
//   - vocab.txt: 每行一个 token
//   - tokenizer.json: HuggingFace 标准格式
type VocabTokenizer struct {
	vocab     map[string]int // token -> id
	reverse   map[int]string // id -> token
	unkID     int            // unknown token id
	clsID     int            // cls token id
	sepID     int            // sep token id
	padID     int            // pad token id
	maxLength int            // 最大序列长度
}

const (
	DefaultMaxLength     = 52  // Chinese-CLIP 默认序列长度
	BGEDefaultMaxLength = 512 // BGE 默认序列长度
)

// loadVocabFromReader 从 Reader 加载 vocab
func loadVocabFromReader(r *bufio.Reader) (map[string]int, map[int]string, error) {
	vocab := make(map[string]int)
	reverse := make(map[int]string)

	id := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil && len(line) == 0 {
			break
		}
		token := strings.TrimSpace(line)
		if token != "" {
			vocab[token] = id
			reverse[id] = token
			id++
		}
		if err != nil {
			break
		}
	}
	return vocab, reverse, nil
}

// NewVocabTokenizer 创建基于内嵌 Chinese-CLIP vocab 的分词器
func NewVocabTokenizer(maxLength int) (*VocabTokenizer, error) {
	if maxLength <= 0 {
		maxLength = DefaultMaxLength
	}

	vocab, reverse, err := loadVocabFromReader(bufio.NewReader(bytes.NewReader(embeddedChineseClipVocab)))
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded vocab: %w", err)
	}

	return buildTokenizer(vocab, reverse, maxLength), nil
}

// NewBGEVocabTokenizer 创建基于内嵌 BGE vocab 的分词器
func NewBGEVocabTokenizer(maxLength int) (*VocabTokenizer, error) {
	if maxLength <= 0 {
		maxLength = BGEDefaultMaxLength
	}

	vocab, reverse, err := loadVocabFromReader(bufio.NewReader(bytes.NewReader(embeddedBGEVocab)))
	if err != nil {
		return nil, fmt.Errorf("failed to load BGE embedded vocab: %w", err)
	}

	return buildTokenizer(vocab, reverse, maxLength), nil
}

// NewVocabTokenizerFromFile 从外部文件加载 vocab
// 支持 vocab.txt (每行一个token) 和 HuggingFace tokenizer.json
func NewVocabTokenizerFromFile(vocabPath string, maxLength int) (*VocabTokenizer, error) {
	if maxLength <= 0 {
		maxLength = DefaultMaxLength
	}

	file, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open vocab file: %w", err)
	}
	defer file.Close()

	// 检测文件格式
	content, _ := bufio.NewReader(file).ReadBytes(0)
	file.Seek(0, 0)

	// 尝试解析为 tokenizer.json (HuggingFace格式)
	if strings.HasSuffix(vocabPath, "tokenizer.json") || strings.HasPrefix(string(content), "{\"version\"") {
		vocab, err := loadVocabFromTokenizerJSON(bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse tokenizer.json: %w", err)
		}
		reverse := make(map[int]string)
		for token, id := range vocab {
			reverse[id] = token
		}
		return buildTokenizer(vocab, reverse, maxLength), nil
	}

	// 否则当作 vocab.txt 解析
	vocab, reverse, err := loadVocabFromReader(bufio.NewReader(file))
	if err != nil {
		return nil, fmt.Errorf("failed to load vocab from file: %w", err)
	}

	return buildTokenizer(vocab, reverse, maxLength), nil
}

// loadVocabFromTokenizerJSON 从 HuggingFace tokenizer.json 提取 vocab
func loadVocabFromTokenizerJSON(r *bytes.Reader) (map[string]int, error) {
	// 使用 streaming JSON 解析
	decoder := json.NewDecoder(r)

	// 解析顶层对象
	var data map[string]any
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	// 提取 model.vocab
	model, ok := data["model"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing 'model' field in tokenizer.json")
	}

	vocabAny, ok := model["vocab"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing 'model.vocab' field in tokenizer.json")
	}

	// 转换为 map[string]int
	vocab := make(map[string]int)
	for token, idAny := range vocabAny {
		var id int
		switch v := idAny.(type) {
		case float64:
			id = int(v)
		case int:
			id = v
		case int64:
			id = int(v)
		default:
			continue
		}
		vocab[token] = id
	}

	return vocab, nil
}

// buildTokenizer 构建 VocabTokenizer
func buildTokenizer(vocab map[string]int, reverse map[int]string, maxLength int) *VocabTokenizer {
	unkID := vocab["[UNK]"]
	clsID := vocab["[CLS]"]
	sepID := vocab["[SEP]"]
	padID := vocab["[PAD]"]

	return &VocabTokenizer{
		vocab:     vocab,
		reverse:   reverse,
		unkID:     unkID,
		clsID:     clsID,
		sepID:     sepID,
		padID:     padID,
		maxLength: maxLength,
	}
}

// Tokenize 将文本转换为 BERT 输入特征
func (t *VocabTokenizer) Tokenize(text string) (inputIDs []int64, attentionMask []int64, err error) {
	// 1. 分词
	tokens := t.tokenize(text)

	// 2. 添加特殊 token: [CLS] ... [SEP]
	tokens = append([]string{t.reverse[t.clsID]}, tokens...)
	tokens = append(tokens, t.reverse[t.sepID])

	// 3. 转换为 IDs
	inputIDs = make([]int64, 0, len(tokens))
	for _, token := range tokens {
		if id, ok := t.vocab[token]; ok {
			inputIDs = append(inputIDs, int64(id))
		} else {
			// 未找到的 token
			inputIDs = append(inputIDs, int64(t.unkID))
		}
	}

	// 4. Truncate 或 Pad
	if len(inputIDs) > t.maxLength {
		inputIDs = inputIDs[:t.maxLength]
	} else {
		// Pad
		padding := make([]int64, t.maxLength-len(inputIDs))
		for i := range padding {
			padding[i] = int64(t.padID)
		}
		inputIDs = append(inputIDs, padding...)
	}

	// 5. Attention mask
	realLen := len(inputIDs)
	if realLen > t.maxLength {
		realLen = t.maxLength
	}
	attentionMask = make([]int64, t.maxLength)
	for i := 0; i < realLen; i++ {
		attentionMask[i] = 1
	}

	return inputIDs, attentionMask, nil
}

// tokenize 文本分词，返回 token 列表
func (t *VocabTokenizer) tokenize(text string) []string {
	var tokens []string

	// 简单按空格和标点分割
	var current string
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		tokens = append(tokens, current)
	}

	// 对每个 token 进行 Wordpiece
	var result []string
	for _, token := range tokens {
		result = append(result, t.wordpieceTokens(token)...)
	}

	return result
}

// wordpieceTokens 将 token 拆分为子词 token 列表
func (t *VocabTokenizer) wordpieceTokens(token string) []string {
	// 如果 token 在 vocab 中，直接返回
	if _, ok := t.vocab[token]; ok {
		return []string{token}
	}

	// 否则尝试字符级拆分 + ##前缀
	var tokens []string
	chars := []rune(token)
	if len(chars) == 0 {
		return tokens
	}

	// 尝试最长匹配
	start := 0
	for start < len(chars) {
		end := len(chars)
		found := false

		for start < end {
			sub := string(chars[start:end])
			if start > 0 {
				sub = "##" + sub
			}

			if _, ok := t.vocab[sub]; ok {
				tokens = append(tokens, sub)
				start = end
				found = true
				break
			}
			end--
		}

		if !found {
			// 未找到，使用 [UNK]
			if start == 0 {
				tokens = append(tokens, "[UNK]")
				start++
			} else {
				break
			}
		}
	}

	return tokens
}

// VocabSize 返回词汇表大小
func (t *VocabTokenizer) VocabSize() int {
	return len(t.vocab)
}
