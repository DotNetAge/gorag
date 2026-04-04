package query

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.EntityExtractor = (*keywordExtractor)(nil)

// keywordExtractor extracts entities using keyword-based heuristics.
// It uses capitalization patterns, special characters, and common entity patterns
// to identify potential entities without requiring an LLM.
type keywordExtractor struct {
	stopWords    map[string]bool
	minWordLen   int
	maxEntities  int
	logger       logging.Logger
	collector    observability.Collector
	entityFilter func(word string) bool
}

// KeywordExtractorOption configures a keywordExtractor instance.
type KeywordExtractorOption func(*keywordExtractor)

// WithKeywordExtractorStopWords sets custom stop words to exclude.
func WithKeywordExtractorStopWords(stopWords []string) KeywordExtractorOption {
	return func(e *keywordExtractor) {
		if len(stopWords) > 0 {
			e.stopWords = make(map[string]bool)
			for _, word := range stopWords {
				e.stopWords[strings.ToLower(word)] = true
			}
		}
	}
}

// WithKeywordExtractorMinWordLen sets minimum word length to consider.
// Default is 2.
func WithKeywordExtractorMinWordLen(minLen int) KeywordExtractorOption {
	return func(e *keywordExtractor) {
		if minLen > 0 {
			e.minWordLen = minLen
		}
	}
}

// WithKeywordExtractorMaxEntities sets maximum entities to return.
// Default is 10.
func WithKeywordExtractorMaxEntities(max int) KeywordExtractorOption {
	return func(e *keywordExtractor) {
		if max > 0 {
			e.maxEntities = max
		}
	}
}

// WithKeywordExtractorLogger sets a structured logger.
func WithKeywordExtractorLogger(logger logging.Logger) KeywordExtractorOption {
	return func(e *keywordExtractor) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithKeywordExtractorCollector sets an observability collector.
func WithKeywordExtractorCollector(collector observability.Collector) KeywordExtractorOption {
	return func(e *keywordExtractor) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// WithKeywordExtractorFilter sets a custom filter function for entity candidates.
func WithKeywordExtractorFilter(filter func(word string) bool) KeywordExtractorOption {
	return func(e *keywordExtractor) {
		e.entityFilter = filter
	}
}

// defaultStopWords contains common English and Chinese stop words.
var defaultStopWords = []string{
	// English
	"the", "a", "an", "is", "are", "was", "were", "be", "been", "being",
	"have", "has", "had", "do", "does", "did", "will", "would", "could", "should",
	"may", "might", "must", "shall", "can", "need", "dare", "ought", "used",
	"to", "of", "in", "for", "on", "with", "at", "by", "from", "as", "into",
	"through", "during", "before", "after", "above", "below", "between",
	"and", "but", "or", "nor", "so", "yet", "both", "either", "neither",
	"not", "only", "own", "same", "than", "too", "very", "just",
	"what", "which", "who", "whom", "whose", "when", "where", "why", "how",
	"this", "that", "these", "those", "it", "its", "they", "them", "their",
	"he", "him", "his", "she", "her", "hers", "we", "us", "our", "you", "your",
	"i", "me", "my", "mine", "yourself", "himself", "herself", "itself",
	// Chinese
	"的", "是", "在", "了", "和", "与", "或", "有", "被", "把",
	"这", "那", "这个", "那个", "这些", "那些", "什么", "怎么", "如何",
	"为什么", "哪里", "谁", "哪个", "多少", "几个", "怎样", "如果",
	"但是", "因为", "所以", "虽然", "而且", "或者", "以及", "不是",
	"可以", "可能", "应该", "需要", "能够", "想要", "希望", "认为",
	"一个", "一些", "许多", "所有", "任何", "每个", "各个", "某种",
}

// NewKeywordExtractor creates a new keyword-based entity extractor.
// It identifies potential entities using heuristics like capitalization,
// quoted strings, and non-stopwords, without requiring an LLM.
func NewKeywordExtractor(opts ...KeywordExtractorOption) *keywordExtractor {
	e := &keywordExtractor{
		stopWords:   make(map[string]bool),
		minWordLen:  2,
		maxEntities: 10,
		logger:      logging.DefaultNoopLogger(),
		collector:   observability.DefaultNoopCollector(),
	}

	// Initialize default stop words
	for _, word := range defaultStopWords {
		e.stopWords[word] = true
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Extract extracts entities from the query using keyword-based heuristics.
func (e *keywordExtractor) Extract(ctx context.Context, query *core.Query) (*core.EntityExtractionResult, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("keyword_entity_extraction", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		return nil, nil
	}

	text := query.Text
	entities := make([]string, 0)
	seen := make(map[string]bool)

	// 1. Extract quoted strings (often entities)
	quotedEntities := e.extractQuotedStrings(text)
	for _, entity := range quotedEntities {
		if !seen[entity] {
			seen[entity] = true
			entities = append(entities, entity)
		}
	}

	// 2. Extract capitalized words (English names, organizations, etc.)
	capitalizedEntities := e.extractCapitalizedWords(text)
	for _, entity := range capitalizedEntities {
		if !seen[entity] {
			seen[entity] = true
			entities = append(entities, entity)
		}
	}

	// 3. Extract Chinese entity patterns (proper nouns, technical terms)
	chineseEntities := e.extractChineseEntities(text)
	for _, entity := range chineseEntities {
		if !seen[entity] {
			seen[entity] = true
			entities = append(entities, entity)
		}
	}

	// 4. Extract non-stopwords that might be entities
	keywordEntities := e.extractKeywords(text)
	for _, entity := range keywordEntities {
		if !seen[entity] {
			seen[entity] = true
			entities = append(entities, entity)
		}
	}

	// Limit entities
	if len(entities) > e.maxEntities {
		entities = entities[:e.maxEntities]
	}

	e.logger.Debug("keyword entity extraction completed", map[string]any{
		"query":    text,
		"entities": entities,
	})

	return &core.EntityExtractionResult{Entities: entities}, nil
}

// extractQuotedStrings extracts text within quotes.
func (e *keywordExtractor) extractQuotedStrings(text string) []string {
	var entities []string

	// Double quotes
	doubleQuoteRegex := regexp.MustCompile(`"([^"]+)"`)
	matches := doubleQuoteRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 && len(match[1]) >= e.minWordLen {
			entities = append(entities, match[1])
		}
	}

	// Single quotes
	singleQuoteRegex := regexp.MustCompile(`'([^']+)'`)
	matches = singleQuoteRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 && len(match[1]) >= e.minWordLen {
			entities = append(entities, match[1])
		}
	}

	return entities
}

// extractCapitalizedWords extracts consecutive capitalized words.
func (e *keywordExtractor) extractCapitalizedWords(text string) []string {
	var entities []string

	// Match consecutive capitalized words (e.g., "New York", "OpenAI")
	regex := regexp.MustCompile(`\b([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)\b`)
	matches := regex.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 1 {
			word := match[1]
			lowerWord := strings.ToLower(word)
			// Skip if it's a stop word
			if !e.stopWords[lowerWord] && len(word) >= e.minWordLen {
				// Apply custom filter if set
				if e.entityFilter == nil || e.entityFilter(word) {
					entities = append(entities, word)
				}
			}
		}
	}

	return entities
}

// extractChineseEntities extracts Chinese proper nouns and technical terms.
func (e *keywordExtractor) extractChineseEntities(text string) []string {
	var entities []string

	// Chinese entity patterns (e.g., "公司", "银行", "大学" suffixes)
	// This is a simplified approach - real NER would be more sophisticated
	chineseEntityPattern := regexp.MustCompile(`[\p{Han}]{2,}(?:公司|银行|大学|学院|医院|集团|科技|网络|软件|系统|平台|项目|产品)`)

	matches := chineseEntityPattern.FindAllString(text, -1)
	for _, match := range matches {
		if len(match) >= e.minWordLen {
			entities = append(entities, match)
		}
	}

	return entities
}

// extractKeywords extracts non-stopword keywords.
func (e *keywordExtractor) extractKeywords(text string) []string {
	var entities []string

	// Tokenize by splitting on whitespace and punctuation
	words := regexp.MustCompile(`[\s\p{P}]+`).Split(text, -1)

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) < e.minWordLen {
			continue
		}

		lowerWord := strings.ToLower(word)

		// Skip stop words
		if e.stopWords[lowerWord] {
			continue
		}

		// Skip pure numbers
		if isAllDigits(word) {
			continue
		}

		// Skip single characters
		if len(word) <= 1 {
			continue
		}

		// Apply custom filter if set
		if e.entityFilter != nil && !e.entityFilter(word) {
			continue
		}

		entities = append(entities, word)
	}

	return entities
}

// isAllDigits checks if a string contains only digits.
func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0
}
