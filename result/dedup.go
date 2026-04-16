package result

import (
	"hash/fnv"
	"sort"
	"unicode"

	"github.com/DotNetAge/gorag/core"
)

// Dedup 对搜索结果语义去重
// 使用 MinHash 算法估算文本 Jaccard 相似度，对同 DocID 下相似度超过阈值的 chunk 进行合并
// 合并规则：同 DocID 下，内容相似度 >= threshold 的保留分数最高的，丢弃其余
func Dedup(hits []core.Hit) ([]core.Hit, error) {
	if len(hits) == 0 {
		return hits, nil
	}

	const (
		threshold = 0.85 // 语义相似度阈值
		numHash   = 128  // MinHash 签名长度
	)

	// 按 DocID 分组
	groups := make(map[string][]int)
	for i, h := range hits {
		groups[h.DocID] = append(groups[h.DocID], i)
	}

	// 对每个 DocID 组进行语义去重
	result := make([]core.Hit, 0, len(hits))
	for _, indices := range groups {
		if len(indices) == 1 {
			result = append(result, hits[indices[0]])
			continue
		}

		// 取出一组 hit
		group := make([]core.Hit, 0, len(indices))
		for _, idx := range indices {
			group = append(group, hits[idx])
		}

		// 计算 MinHash 签名
		signatures := make([][]uint64, len(group))
		for i := range group {
			signatures[i] = minHash(group[i].Content, numHash)
		}

		// 合并相似度超过阈值的
		merged := mergeGroup(group, signatures, threshold)
		result = append(result, merged...)
	}

	// 按分数降序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result, nil
}

// mergeGroup 对一组同 DocID 的 hit 进行语义合并
func mergeGroup(hits []core.Hit, sigs [][]uint64, threshold float32) []core.Hit {
	n := len(hits)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return hits
	}

	// cluster[i] = i 表示 i 属于自己的簇
	cluster := make([]int, n)
	for i := range cluster {
		cluster[i] = i
	}

	// 两两比较，合并相似度超过阈值的
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if cluster[i] == cluster[j] {
				continue
			}
			sim := similarity(sigs[i], sigs[j])
			if sim >= threshold {
				// 合并簇：把 j 所在簇的所有成员都指向 i 所在簇
				// 保留分数更高的作为代表
				repI, repJ := cluster[i], cluster[j]
				if hits[repJ].Score > hits[repI].Score {
					repI, repJ = repJ, repI
				}
				// 所有指向 repJ 的改成指向 repI
				for k := 0; k < n; k++ {
					if cluster[k] == repJ {
						cluster[k] = repI
					}
				}
			}
		}
	}

	// 收集每个簇的代表（分数最高的）
	seen := make(map[int]bool)
	out := make([]core.Hit, 0)
	for i := range hits {
		if !seen[cluster[i]] {
			seen[cluster[i]] = true
			out = append(out, hits[cluster[i]])
		}
	}
	return out
}

// similarity 计算两个 MinHash 签名的相似度
func similarity(a, b []uint64) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	match := 0
	for i := range a {
		if a[i] == b[i] {
			match++
		}
	}
	return float32(match) / float32(len(a))
}

// minHash 计算文本的 MinHash 签名
// 使用词级 n-gram (word unigram) 和多个 hash 函数模拟 MinHash
func minHash(text string, numHash uint) []uint64 {
	// 1. 生成词集合（分词）
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return make([]uint64, numHash)
	}

	// 2. 使用多个 hash 函数生成签名
	sigs := make([]uint64, numHash)
	for i := uint(0); i < numHash; i++ {
		sigs[i] = minHashForTokens(tokens, i)
	}
	return sigs
}

// minHashForTokens 对词集合计算单个 hash 函数的 MinHash 值
func minHashForTokens(tokens []string, seed uint) uint64 {
	h := fnv.New64a()
	h.Write([]byte(string(rune('a' + seed%26)))) // 不同 seed 不同初始字符

	minVal := uint64(1 << 63) // 最大值
	for _, tok := range tokens {
		// 组合 token 和 seed 生成 hash
		h2 := fnv.New64a()
		h2.Write([]byte(tok))
		h2.Write([]byte{byte(seed)})
		v := h2.Sum64()
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

// tokenize 简单分词：按空格/标点分割，转小写，过滤短词
func tokenize(text string) []string {
	var tokens []string
	var cur []rune
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if len(cur) > 0 {
				tok := string(cur)
				// 过滤停用词和过短的词
				if len(tok) > 2 {
					tokens = append(tokens, tok)
				}
				cur = nil
			}
		} else {
			cur = append(cur, unicode.ToLower(r))
		}
	}
	if len(cur) > 0 {
		tok := string(cur)
		if len(tok) > 2 {
			tokens = append(tokens, tok)
		}
	}
	return tokens
}
