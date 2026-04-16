package chunker

import (
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// TODO: 这是一个过度设计的类，在项目中并没有在任何地方被应用；
// ChunkingAdvisor recommends chunk strategies based on content type and metadata
type ChunkingAdvisor interface {
	// RecommendStrategy recommends a chunk strategy
	RecommendStrategy(contentType string, metadata map[string]any) core.ChunkStrategy
}

// DecisionRule defines a rule for strategy recommendation
type DecisionRule struct {
	Condition RuleCondition      // condition to match
	Strategy  core.ChunkStrategy // recommended strategy
	Priority  int                // priority (lower number = higher priority)
}

// RuleCondition defines conditions for matching a rule
type RuleCondition struct {
	ContentType    string                             // content type (MIME type)
	HasHeadings    *bool                              // whether content has heading structure
	AvgSentenceLen *int                               // average sentence length
	FileExtension  string                             // file extension
	CustomCheck    func(metadata map[string]any) bool // custom condition check
}

// DefaultAdvisor is the default implementation of ChunkingAdvisor
type DefaultAdvisor struct {
	rules []DecisionRule
}

// NewDefaultAdvisor creates a new DefaultAdvisor with default rules
func NewDefaultAdvisor() *DefaultAdvisor {
	advisor := &DefaultAdvisor{
		rules: []DecisionRule{},
	}

	// Add default rules (sorted by priority)
	advisor.addDefaultRules()

	return advisor
}

// addDefaultRules adds the default decision rules
func (a *DefaultAdvisor) addDefaultRules() {
	// Rule 1: Code files
	a.AddRule(DecisionRule{
		Condition: RuleCondition{
			CustomCheck: func(metadata map[string]any) bool {
				contentType, ok := metadata["content_type"].(string)
				if !ok {
					return false
				}
				// Check if content type indicates code
				codeTypes := []string{
					"text/x-go", "text/x-python", "text/x-java",
					"text/x-javascript", "text/x-typescript",
					"text/x-rust", "text/x-c", "text/x-cpp",
				}
				for _, t := range codeTypes {
					if strings.Contains(contentType, t) {
						return true
					}
				}
				return false
			},
		},
		Strategy: StrategyCode,
		Priority: 1,
	})

	// Rule 2: Markdown documents
	a.AddRule(DecisionRule{
		Condition: RuleCondition{
			ContentType: "text/markdown",
		},
		Strategy: StrategyRecursive,
		Priority: 2,
	})

	// Rule 3: PDF documents with heading structure
	a.AddRule(DecisionRule{
		Condition: RuleCondition{
			ContentType: "application/pdf",
			CustomCheck: func(metadata map[string]any) bool {
				hasHeadings, ok := metadata["has_structured_headings"].(bool)
				return ok && hasHeadings
			},
		},
		Strategy: StrategyRecursive,
		Priority: 3,
	})

	// Rule 4: HTML documents
	a.AddRule(DecisionRule{
		Condition: RuleCondition{
			ContentType: "text/html",
		},
		Strategy: StrategyParagraph,
		Priority: 4,
	})

	// Rule 5: Structured data
	a.AddRule(DecisionRule{
		Condition: RuleCondition{
			CustomCheck: func(metadata map[string]any) bool {
				contentType, ok := metadata["content_type"].(string)
				if !ok {
					return false
				}
				structuredTypes := []string{
					"application/json",
					"text/csv",
					"application/xml",
				}
				for _, t := range structuredTypes {
					if strings.Contains(contentType, t) {
						return true
					}
				}
				return false
			},
		},
		Strategy: StrategySemantic,
		Priority: 5,
	})

	// Rule 6: FAQ/dialogue content
	a.AddRule(DecisionRule{
		Condition: RuleCondition{
			CustomCheck: func(metadata map[string]any) bool {
				contentType, ok := metadata["content_type"].(string)
				if !ok {
					return false
				}
				return strings.Contains(contentType, "text/x-faq") ||
					strings.Contains(contentType, "text/x-dialogue")
			},
		},
		Strategy: StrategySentence,
		Priority: 6,
	})

	// Rule 7: Content requiring context enrichment
	a.AddRule(DecisionRule{
		Condition: RuleCondition{
			CustomCheck: func(metadata map[string]any) bool {
				contextRequired, ok := metadata["context_required"].(bool)
				return ok && contextRequired
			},
		},
		Strategy: StrategyParentDoc,
		Priority: 7,
	})
}

// AddRule adds a decision rule
func (a *DefaultAdvisor) AddRule(rule DecisionRule) {
	a.rules = append(a.rules, rule)
	// Sort by priority (bubble sort for small arrays)
	for i := len(a.rules) - 1; i > 0; i-- {
		if a.rules[i].Priority < a.rules[i-1].Priority {
			a.rules[i], a.rules[i-1] = a.rules[i-1], a.rules[i]
		}
	}
}

// RecommendStrategy recommends a chunk strategy
func (a *DefaultAdvisor) RecommendStrategy(contentType string, metadata map[string]any) core.ChunkStrategy {
	// Check if strategy is explicitly specified in metadata
	if strategy, ok := metadata["chunk_strategy"].(core.ChunkStrategy); ok {
		return strategy
	}

	// Create local metadata map if nil
	if metadata == nil {
		metadata = map[string]any{}
	}

	// Ensure contentType is in metadata
	metadata["content_type"] = contentType

	// Match rules by priority
	for _, rule := range a.rules {
		if a.matchRule(rule, metadata) {
			return rule.Strategy
		}
	}

	// Default to recursive chunking
	return StrategyRecursive
}

// matchRule checks if a rule's conditions are satisfied
func (a *DefaultAdvisor) matchRule(rule DecisionRule, metadata map[string]any) bool {
	cond := rule.Condition

	// Check content type
	if cond.ContentType != "" {
		contentType, ok := metadata["content_type"].(string)
		if !ok || !strings.Contains(contentType, cond.ContentType) {
			return false
		}
	}

	// Check heading structure
	if cond.HasHeadings != nil {
		hasHeadings, ok := metadata["has_structured_headings"].(bool)
		if !ok || hasHeadings != *cond.HasHeadings {
			return false
		}
	}

	// Check average sentence length
	if cond.AvgSentenceLen != nil {
		avgLen, ok := metadata["avg_sentence_length"].(int)
		if !ok || avgLen != *cond.AvgSentenceLen {
			return false
		}
	}

	// Check file extension
	if cond.FileExtension != "" {
		ext, ok := metadata["file_extension"].(string)
		if !ok || ext != cond.FileExtension {
			return false
		}
	}

	// Execute custom check
	if cond.CustomCheck != nil {
		return cond.CustomCheck(metadata)
	}

	return true
}

// ChunkingPipeline integrates strategy selection, chunker creation, and chunking
type ChunkingPipeline struct {
	factory *ChunkingFactory
	advisor ChunkingAdvisor
}

// NewChunkingPipeline creates a new ChunkingPipeline
func NewChunkingPipeline() *ChunkingPipeline {
	return &ChunkingPipeline{
		factory: NewChunkingFactory(),
		advisor: NewDefaultAdvisor(),
	}
}

// Chunk executes chunking with optional strategy override
// If preferredStrategy is empty, uses advisor recommendation
func (p *ChunkingPipeline) Chunk(
	structured *core.StructuredDocument,
	entities []*core.Entity,
	preferredStrategy core.ChunkStrategy,
	opts ...Option,
) ([]*core.Chunk, error) {
	if structured == nil || structured.RawDoc == nil {
		return []*core.Chunk{}, nil
	}

	// Determine strategy
	strategy := preferredStrategy
	if strategy == "" {
		contentType := structured.RawDoc.GetMimeType()
		metadata := structured.RawDoc.GetMeta()
		strategy = p.advisor.RecommendStrategy(contentType, metadata)
	}

	// Create chunker
	chunker, err := p.factory.CreateChunker(strategy, opts...)
	if err != nil {
		return nil, err
	}

	// Execute chunking
	return chunker.Chunk(structured, entities)
}

// SetAdvisor sets the chunking advisor
func (p *ChunkingPipeline) SetAdvisor(advisor ChunkingAdvisor) {
	p.advisor = advisor
}

// SetFactory sets the chunking factory
func (p *ChunkingPipeline) SetFactory(factory *ChunkingFactory) {
	p.factory = factory
}
