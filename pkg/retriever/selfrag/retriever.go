package selfrag

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/retrieval/answer"
	"github.com/DotNetAge/gorag/pkg/steps/enrich"
	"github.com/DotNetAge/gorag/pkg/steps/generate"
	"github.com/DotNetAge/gorag/pkg/steps/vector"
)

type selfRAGRetriever struct {
	pipeline *pipeline.Pipeline[*core.RetrievalContext]
}

// NewRetriever creates a new Self-RAG retriever with self-reflection capabilities.
func NewRetriever(
	vectorStore core.VectorStore,
	embedder embedding.Provider,
	evaluator core.RAGEvaluator,
	llm chat.Client,
	opts ...Option,
) core.Retriever {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	if options.logger == nil {
		options.logger = logging.NewNoopLogger()
	}

	p := pipeline.New[*core.RetrievalContext]()

	// 1. Initial Retrieval
	p.AddStep(vector.Search(vectorStore, embedder, vector.SearchOptions{
		TopK: options.topK,
	}))

	// 1.5 DocStore Enrichment (PDR)
	if options.docStore != nil {
		p.AddStep(enrich.EnrichWithDocStore(options.docStore, options.logger))
	}

	// 2. Initial Generation
	gen := answer.New(llm, answer.WithLogger(options.logger))
	p.AddStep(stepgen.Generate(gen, options.logger, nil))

	// 3. Self-Reflection & Refinement Loop
	p.AddStep(&refinementStep{
		evaluator:  evaluator,
		llm:        llm,
		threshold:  options.threshold,
		maxRetries: options.maxRetries,
		logger:     options.logger,
	})

	return &selfRAGRetriever{pipeline: p}
}

func (r *selfRAGRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		retrievalCtx := core.NewRetrievalContext(ctx, q)

		if err := r.pipeline.Execute(ctx, retrievalCtx); err != nil {
			return nil, err
		}

		var allChunks []*core.Chunk
		for _, group := range retrievalCtx.RetrievedChunks {
			allChunks = append(allChunks, group...)
		}

		res := &core.RetrievalResult{
			Query:  q,
			Chunks: allChunks,
			Answer: retrievalCtx.Answer.Answer,
		}
		
		// Attach Self-RAG metrics to metadata
		if eval, ok := retrievalCtx.Custom["self_rag_evaluation"].(*core.RAGEvaluation); ok {
			if res.Metadata == nil {
				res.Metadata = make(map[string]any)
			}
			res.Metadata["self_rag_score"] = eval.OverallScore
			res.Metadata["self_rag_passed"] = eval.Passed
			res.Metadata["self_rag_feedback"] = eval.Feedback
		}

		results = append(results, res)
	}

	return results, nil
}

// refinementStep implements the Self-RAG critique and refinement loop.
type refinementStep struct {
	evaluator  core.RAGEvaluator
	llm        chat.Client
	threshold  float32
	maxRetries int
	logger     logging.Logger
}

func (s *refinementStep) Name() string {
	return "SelfRefinement"
}

func (s *refinementStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	var lastEval *core.RAGEvaluation

	for i := 0; i < s.maxRetries; i++ {
		// 1. Build context string from chunks
		var contextBuilder strings.Builder
		for _, group := range context.RetrievedChunks {
			for _, chunk := range group {
				contextBuilder.WriteString(chunk.Content + "\n")
			}
		}
		contextStr := contextBuilder.String()

		// 2. Evaluate current answer
		eval, err := s.evaluator.Evaluate(ctx, context.Query.Text, context.Answer.Answer, contextStr)
		if err != nil {
			s.logger.Error("Self-RAG evaluation failed", err)
			return nil // Non-fatal, keep current answer
		}
		lastEval = eval

		if eval.OverallScore >= s.threshold || eval.Passed {
			s.logger.Info("Self-RAG: answer passed evaluation", map[string]any{
				"score": eval.OverallScore,
				"retry": i,
			})
			break
		}

		s.logger.Warn("Self-RAG: answer failed evaluation, refining", map[string]any{
			"score":    eval.OverallScore,
			"retry":    i,
			"feedback": eval.Feedback,
		})

		// 3. Refine answer based on feedback
		refinePrompt := fmt.Sprintf(`The previous answer was not good enough. 
Feedback: %s

Please provide a better answer based on the context and the feedback.

[Context]
%s

[Question]
%s

[Previous Answer]
%s

New Answer:`, eval.Feedback, contextStr, context.Query.Text, context.Answer.Answer)

		messages := []chat.Message{chat.NewUserMessage(refinePrompt)}
		resp, err := s.llm.Chat(ctx, messages)
		if err != nil {
			s.logger.Error("Self-RAG refinement Chat failed", err)
			break
		}

		context.Answer.Answer = resp.Content
	}

	if context.Custom == nil {
		context.Custom = make(map[string]any)
	}
	context.Custom["self_rag_evaluation"] = lastEval

	return nil
}
// Options for Self-RAG retriever
type Options struct {
	topK       int
	threshold  float32
	maxRetries int
	docStore   store.DocStore
	logger     logging.Logger
}

func defaultOptions() *Options {
	return &Options{
		topK:       5,
		threshold:  0.7,
		maxRetries: 3,
	}
}

type Option func(*Options)

func WithTopK(k int) Option {
	return func(o *Options) {
		o.topK = k
	}
}

func WithThreshold(t float32) Option {
	return func(o *Options) {
		o.threshold = t
	}
}

func WithMaxRetries(r int) Option {
	return func(o *Options) {
		o.maxRetries = r
	}
}

func WithDocStore(s store.DocStore) Option {
	return func(o *Options) {
		o.docStore = s
	}
}

func WithLogger(l logging.Logger) Option {
...
	return func(o *Options) {
		o.logger = l
	}
}
