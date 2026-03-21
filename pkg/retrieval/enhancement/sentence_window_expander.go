package enhancement

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.ResultEnhancer = (*SentenceWindowExpander)(nil)

// SentenceWindowExpander expands retrieved chunks with surrounding sentences.
type SentenceWindowExpander struct {
	windowSize int
	maxChars   int
	logger     logging.Logger
	collector  observability.Collector
}

// SentenceWindowExpanderOption configures a SentenceWindowExpander instance.
type SentenceWindowExpanderOption func(*SentenceWindowExpander)

// WithWindowSize sets the number of sentences to expand.
func WithWindowSize(size int) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if size > 0 {
			e.windowSize = size
		}
	}
}

// WithMaxChars sets the maximum characters per expanded chunk.
func WithMaxChars(maxChars int) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if maxChars > 0 {
			e.maxChars = maxChars
		}
	}
}

// WithExpanderLogger sets a structured logger.
func WithExpanderLogger(logger logging.Logger) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithExpanderCollector sets an observability collector.
func WithExpanderCollector(collector observability.Collector) SentenceWindowExpanderOption {
	return func(e *SentenceWindowExpander) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// NewSentenceWindowExpander creates a new sentence window expander.
func NewSentenceWindowExpander(opts ...SentenceWindowExpanderOption) *SentenceWindowExpander {
	e := &SentenceWindowExpander{
		windowSize: 2,
		maxChars:   2000,
		logger:     logging.DefaultNoopLogger(),
		collector:  observability.DefaultNoopCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Enhance implements core.ResultEnhancer.
func (e *SentenceWindowExpander) Enhance(ctx context.Context, query *core.Query, chunks []*core.Chunk) ([]*core.Chunk, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("sentence_window_expansion", time.Since(start), nil)
	}()

	if len(chunks) == 0 {
		return chunks, nil
	}

	expandedChunks := make([]*core.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		expandedChunks = append(expandedChunks, e.expandChunk(chunk))
	}

	return expandedChunks, nil
}

// expandChunk expands a single chunk with surrounding sentences.
func (e *SentenceWindowExpander) expandChunk(chunk *core.Chunk) *core.Chunk {
	fullContent, ok := chunk.Metadata["full_document"].(string)
	if !ok {
		return chunk
	}

	sentences := splitIntoSentences(fullContent)
	chunkStart := -1
	searchLen := 50
	if len(chunk.Content) < searchLen {
		searchLen = len(chunk.Content)
	}
	
	for i, sent := range sentences {
		if strings.Contains(sent, chunk.Content[:searchLen]) {
			chunkStart = i
			break
		}
	}

	if chunkStart == -1 {
		return chunk
	}

	windowStart := 0
	if chunkStart > e.windowSize {
		windowStart = chunkStart - e.windowSize
	}
	
	windowEnd := len(sentences) - 1
	if chunkStart+e.windowSize < windowEnd {
		windowEnd = chunkStart + e.windowSize
	}

	var builder strings.Builder
	for i := windowStart; i <= windowEnd; i++ {
		builder.WriteString(sentences[i])
		if i < windowEnd {
			builder.WriteString(" ")
		}
		if builder.Len() >= e.maxChars {
			break
		}
	}

	newChunk := *chunk
	newChunk.Content = builder.String()
	newChunk.EndIndex = chunk.StartIndex + builder.Len()
	return &newChunk
}

// splitIntoSentences splits text into sentences.
func splitIntoSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i, r := range text {
		current.WriteRune(r)
		if isSentenceEnd(r, text, i) {
			sentence := strings.TrimSpace(current.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			current.Reset()
		}
	}

	if current.Len() > 0 {
		sentence := strings.TrimSpace(current.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// isSentenceEnd checks if a rune marks a sentence end.
func isSentenceEnd(r rune, text string, pos int) bool {
	if r != '.' && r != '!' && r != '?' {
		return false
	}
	if pos+1 < len(text) && unicode.IsLower(rune(text[pos+1])) {
		return false
	}
	return true
}
