package result

import (
	"hash/fnv"
	"sort"
	"unicode"

	"github.com/DotNetAge/gorag/v2/core"
)

// Dedup 对检索结果中的 Chunks 做语义去重。
//
// 使用 MinHash 算法估算文本 Jaccard 相似度，对同 DocID 下相似度超过阈值的 ChunkHit
// 进行合并：同 DocID 下，内容相似度 >= threshold 的保留分数最高的，丢弃其余。
//
// 仅对 hit.Chunks 做去重，hit.Nodes 和 hit.Edges 保持不变。
// 返回新的 *Hit（不修改入参 hit），其 Chunks 已去重并按分数降序排序。
func Dedup(hit *core.Hit) (*core.Hit, error) {
	if hit == nil {
		return nil, nil
	}
	if len(hit.Chunks) == 0 {
		// 没有需要去重的 Chunks，返回原 Hit 的浅拷贝（保持 Nodes/Edges 不变）
		return cloneHit(hit), nil
	}

	const (
		threshold = 0.85 // 语义相似度阈值
		numHash   = 128  // MinHash 签名长度
	)

	// 按 DocID 分组（记录在 hit.Chunks 中的下标）
	groups := make(map[string][]int)
	for i, ch := range hit.Chunks {
		if ch.Chunk == nil {
			continue
		}
		groups[ch.DocID] = append(groups[ch.DocID], i)
	}

	// 对每个 DocID 组进行语义去重
	dedupedChunks := make([]core.ChunkHit, 0, len(hit.Chunks))
	for _, indices := range groups {
		if len(indices) == 1 {
			dedupedChunks = append(dedupedChunks, hit.Chunks[indices[0]])
			continue
		}

		// 取出一组 ChunkHit
		group := make([]core.ChunkHit, 0, len(indices))
		for _, idx := range indices {
			group = append(group, hit.Chunks[idx])
		}

		// 计算 MinHash 签名
		signatures := make([][]uint64, len(group))
		for i := range group {
			signatures[i] = minHash(group[i].Content, numHash)
		}

		// 合并相似度超过阈值的
		merged := mergeChunkGroup(group, signatures, threshold)
		dedupedChunks = append(dedupedChunks, merged...)
	}

	// 按分数降序排列
	sort.Slice(dedupedChunks, func(i, j int) bool {
		return dedupedChunks[i].Score > dedupedChunks[j].Score
	})

	// 构建去重后的 Hit（保留原 Query/Score/Nodes/Edges）
	return &core.Hit{
		Query:  hit.Query,
		Score:  hit.Score,
		Chunks: dedupedChunks,
		Nodes:  hit.Nodes,
		Edges:  hit.Edges,
	}, nil
}

// cloneHit 返回 hit 的浅拷贝（共享 Chunk/Node/Edge 指针，但 Hit 结构体独立）。
func cloneHit(hit *core.Hit) *core.Hit {
	if hit == nil {
		return nil
	}
	return &core.Hit{
		Query:  hit.Query,
		Score:  hit.Score,
		Chunks: hit.Chunks,
		Nodes:  hit.Nodes,
		Edges:  hit.Edges,
	}
}

// mergeChunkGroup 对一组同 DocID 的 ChunkHit 进行语义合并。
// 保留每个簇中分数最高的作为代表。
func mergeChunkGroup(hits []core.ChunkHit, sigs [][]uint64, threshold float32) []core.ChunkHit {
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
	out := make([]core.ChunkHit, 0)
	for i := range hits {
		if !seen[cluster[i]] {
			seen[cluster[i]] = true
			out = append(out, hits[cluster[i]])
		}
	}
	return out
}

// similarity 计算两个 MinHash 签名的相似度（Jaccard 近似值）。
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

// minHash 计算文本的 MinHash 签名。
// 使用词级 n-gram（word unigram）和多个 hash 函数模拟 MinHash。
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

// minHashForTokens 对词集合计算单个 hash 函数的 MinHash 值。
func minHashForTokens(tokens []string, seed uint) uint64 {
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

// tokenize 简单分词：按空格/标点分割，转小写，过滤短词。
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
