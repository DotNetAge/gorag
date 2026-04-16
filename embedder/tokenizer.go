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

	// 先读取文件内容用于格式检测
	content, err := bufio.NewReader(file).ReadBytes(0)
	if err != nil && len(content) == 0 {
		return nil, fmt.Errorf("failed to read vocab file: %w", err)
	}
	file.Seek(0, 0)

	// 尝试解析为 tokenizer.json (HuggingFace格式)
	// 格式检测：路径以 tokenizer.json 结尾 或 内容以 {"version" 开头
	isTokenizerJSON := strings.HasSuffix(vocabPath, "tokenizer.json") ||
		(len(content) > 10 && bytes.HasPrefix(bytes.TrimSpace(content), []byte("{\"version\"")))

	if isTokenizerJSON {
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
	unkID := getSpecialTokenID(vocab, "[UNK]", 100)
	clsID := getSpecialTokenID(vocab, "[CLS]", 101)
	sepID := getSpecialTokenID(vocab, "[SEP]", 102)
	padID := getSpecialTokenID(vocab, "[PAD]", 0)

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

// getSpecialTokenID 安全获取特殊 token ID，不存在时返回默认值
func getSpecialTokenID(vocab map[string]int, token string, defaultID int) int {
	if id, ok := vocab[token]; ok {
		return id
	}
	return defaultID
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

	// 改进的分词：处理英文单词、数字串、标点符号
	var current string
	for i, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			// 遇到分隔符，先处理当前积累的 token
			if current != "" {
				// 英文单词整体处理
				if isASCIIAlphaString(current) {
					tokens = append(tokens, current)
				} else {
					// 中文或其他字符按字符分词后做 wordpiece
					for _, c := range current {
						tokens = append(tokens, string(c))
					}
				}
				current = ""
			}
			// 跳过分隔符
		} else if unicode.IsDigit(r) {
			// 数字：积累连续数字串
			start := i
			for j := i; j < len(text); j++ {
				if !unicode.IsDigit(rune(text[j])) {
					break
				}
				i = j
			}
			numStr := text[start : i+1]
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
			// 数字串整体作为一个 token
			tokens = append(tokens, numStr)
		} else if isASCIIAlphaRune(r) {
			// 英文/拉丁字母：积累连续字母串
			start := i
			for j := i; j < len(text); j++ {
				if !isASCIIAlphaRune(rune(text[j])) {
					break
				}
				i = j
			}
			word := text[start : i+1]
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
			tokens = append(tokens, word)
		} else {
			// 中文或其他字符：积累连续字符
			current += string(r)
		}
	}

	// 处理最后剩余的 token
	if current != "" {
		if isASCIIAlphaString(current) {
			tokens = append(tokens, current)
		} else {
			for _, c := range current {
				tokens = append(tokens, string(c))
			}
		}
	}

	// 对每个 token 进行 Wordpiece
	var result []string
	for _, token := range tokens {
		result = append(result, t.wordpieceTokens(token)...)
	}

	return result
}

// isASCIIAlphaString 检查字符串是否只包含 ASCII 字母
func isASCIIAlphaString(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return len(s) > 0
}

// isASCIIAlphaRune 检查 rune 是否为 ASCII 字母
func isASCIIAlphaRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
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
				// 剩余字符无法匹配，追加 [UNK] 而非直接丢弃
				tokens = append(tokens, "[UNK]")
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

// Close 关闭分词器（用于释放资源）
func (t *VocabTokenizer) Close() error {
	// 当前 VocabTokenizer 不持有需要关闭的资源
	// 保留此方法以备未来扩展
	return nil
}
