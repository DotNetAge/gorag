package chunker

import (
	"math"
	"sort"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/document"
)

// ValidationReport contains the results of chunk validation
type ValidationReport struct {
	TotalChunks    int                 // total number of chunks
	ValidChunks    int                 // number of valid chunks
	InvalidChunks  int                 // number of invalid chunks
	Errors         []ValidationError   // list of errors
	Warnings       []ValidationWarning // list of warnings
	CohesionScore  float64             // intra-chunk cohesion (0-1)
	DiversityScore float64             // inter-chunk diversity (0-1)
	SizeStats      SizeStats           // chunk size statistics
}

// ValidationError represents a validation error
type ValidationError struct {
	ChunkIndex int    // chunk index
	ErrorType  string // error type
	Message    string // error message
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	ChunkIndex  int    // chunk index
	WarningType string // warning type
	Message     string // warning message
}

// SizeStats contains chunk size statistics
type SizeStats struct {
	Mean   float64 // mean size
	Median float64 // median size
	StdDev float64 // standard deviation
	Min    int     // minimum size
	Max    int     // maximum size
}

// ChunkValidator validates chunk quality
type ChunkValidator struct {
	minChunkSize int          // 最小块大小
	maxChunkSize int          // 最大块大小
	embedder     core.Embedder // 嵌入模型（可选，用于语义验证）
}

// NewChunkValidator 创建验证器
func NewChunkValidator(opts ...Option) *ChunkValidator {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	return &ChunkValidator{
		minChunkSize: options.MinChunkSize,
		maxChunkSize: options.MaxChunkSize,
		embedder:     nil, // 嵌入模型可选
	}
}

// Validate 验证分块质量
func (v *ChunkValidator) Validate(chunks []*core.Chunk) *ValidationReport {
	report := &ValidationReport{
		TotalChunks: len(chunks),
		Errors:      []ValidationError{},
		Warnings:    []ValidationWarning{},
	}

	// 1. Basic checks
	for i, chunk := range chunks {
		// Check for empty content
		if chunk.Content == "" {
			report.InvalidChunks++
			report.Errors = append(report.Errors, ValidationError{
				ChunkIndex: i,
				ErrorType:  "empty_chunk",
				Message:    "Chunk content is empty",
			})
			continue
		}

		// Check chunk size
		chunkSize := len(chunk.Content)
		if chunkSize < v.minChunkSize {
			report.Warnings = append(report.Warnings, ValidationWarning{
				ChunkIndex: i,
				WarningType: "small_chunk",
				Message:    "Chunk size is smaller than minimum",
			})
		} else if chunkSize > v.maxChunkSize {
			report.Warnings = append(report.Warnings, ValidationWarning{
				ChunkIndex: i,
				WarningType: "large_chunk",
				Message:    "Chunk size is larger than maximum",
			})
		}

		report.ValidChunks++
	}

	// 2. Calculate size statistics
	report.SizeStats = v.calculateSizeStats(chunks)

	// 3. Semantic validation (if embedder is provided)
	if v.embedder != nil && len(chunks) > 1 {
		report.CohesionScore = v.checkIntraChunkCohesion(chunks)
		report.DiversityScore = v.checkInterChunkDiversity(chunks)
	} else {
		report.CohesionScore = 0
		report.DiversityScore = 0
	}

	return report
}

// calculateSizeStats computes chunk size statistics
func (v *ChunkValidator) calculateSizeStats(chunks []*core.Chunk) SizeStats {
	if len(chunks) == 0 {
		return SizeStats{}
	}

	sizes := make([]int, len(chunks))
	for i, chunk := range chunks {
		sizes[i] = len(chunk.Content)
	}

	// Calculate mean
	var sum int
	for _, s := range sizes {
		sum += s
	}
	mean := float64(sum) / float64(len(sizes))

	// Calculate standard deviation
	var variance float64
	for _, s := range sizes {
		variance += math.Pow(float64(s)-mean, 2)
	}
	variance /= float64(len(sizes))
	stdDev := math.Sqrt(variance)

	// Calculate median
	sortedSizes := make([]int, len(sizes))
	copy(sortedSizes, sizes)
	sort.Ints(sortedSizes)
	n := len(sortedSizes)
	var median float64
	if n%2 == 0 {
		median = float64(sortedSizes[n/2-1]+sortedSizes[n/2]) / 2
	} else {
		median = float64(sortedSizes[n/2])
	}

	// Calculate min and max
	min, max := sizes[0], sizes[0]
	for _, s := range sizes {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}

	return SizeStats{
		Mean:   mean,
		Median: median,
		StdDev: stdDev,
		Min:    min,
		Max:    max,
	}
}

// checkIntraChunkCohesion calculates the average similarity between sentences within each chunk
func (v *ChunkValidator) checkIntraChunkCohesion(chunks []*core.Chunk) float64 {
	if v.embedder == nil {
		return 0
	}

	var totalCohesion float64
	validChunks := 0

	for _, chunk := range chunks {
		// Split chunk into sentences
		sentenceChunker := NewSentenceChunker()
		doc := document.New(chunk.Content, chunk.MIMEType)
		structured := &core.StructuredDocument{
			RawDoc: doc,
			Title:  "chunk",
			Root:   nil,
		}
		sentences, err := sentenceChunker.Chunk(structured, nil)
		if err != nil || len(sentences) < 2 {
			continue
		}

		// Compute sentence embeddings
		sentenceTexts := make([]string, len(sentences))
		for i, s := range sentences {
			sentenceTexts[i] = s.Content
		}

		embeddings, err := embedBatch(v.embedder, sentenceTexts)
		if err != nil {
			continue
		}

		// Calculate similarity between adjacent sentences
		var cohesion float64
		for i := 0; i < len(embeddings)-1; i++ {
			cohesion += float64(v.cosineSimilarity(embeddings[i], embeddings[i+1]))
		}
		cohesion /= float64(len(embeddings) - 1)

		totalCohesion += cohesion
		validChunks++
	}

	if validChunks == 0 {
		return 0
	}

	return totalCohesion / float64(validChunks)
}

// checkInterChunkDiversity calculates the average cosine distance between different chunks
func (v *ChunkValidator) checkInterChunkDiversity(chunks []*core.Chunk) float64 {
	if v.embedder == nil || len(chunks) < 2 {
		return 0
	}

	// Compute chunk embeddings
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = chunk.Content
	}

	embeddings, err := embedBatch(v.embedder, chunkTexts)
	if err != nil {
		return 0
	}

	// Calculate inter-chunk similarity
	var totalSimilarity float64
	pairs := 0

	for i := 0; i < len(embeddings); i++ {
		for j := i + 1; j < len(embeddings); j++ {
			totalSimilarity += float64(v.cosineSimilarity(embeddings[i], embeddings[j]))
			pairs++
		}
	}

	if pairs == 0 {
		return 0
	}

	// Diversity = 1 - average similarity
	return 1 - totalSimilarity/float64(pairs)
}

// cosineSimilarity computes cosine similarity (delegates to package function)
func (v *ChunkValidator) cosineSimilarity(a, b []float32) float32 {
	return CosineSimilarity(a, b)
}

// IsValid checks if the validation report has passed
func (r *ValidationReport) IsValid() bool {
	return len(r.Errors) == 0
}
