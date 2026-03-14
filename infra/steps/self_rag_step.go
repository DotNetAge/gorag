package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/evaluation"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*SelfRAGStep)(nil)

// SelfRAGStep is an interceptor step that uses an LLM Evaluator to check the generated answer 
// for hallucinations (Faithfulness) before returning it to the user.
type SelfRAGStep struct {
	judge          evaluation.LLMJudge
	strictMode     bool
	scoreThreshold float32
}

// NewSelfRAGStep creates a Self-RAG reflection step.
func NewSelfRAGStep(judge evaluation.LLMJudge, strictMode bool, threshold float32) *SelfRAGStep {
	if threshold <= 0 {
		threshold = 0.8
	}
	return &SelfRAGStep{
		judge:          judge,
		strictMode:     strictMode,
		scoreThreshold: threshold,
	}
}

func (s *SelfRAGStep) Name() string {
	return "SelfRAGStep"
}

func (s *SelfRAGStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Answer == "" || len(state.RetrievedChunks) == 0 {
		// Missing data to evaluate, skip reflection
		return nil
	}

	// Flatten the retrieved chunks
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		// Missing data to evaluate, skip reflection
		return nil
	}

	score, reason, err := s.judge.EvaluateFaithfulness(ctx, state.Query.Text, allChunks, state.Answer)
	if err != nil {
		return fmt.Errorf("SelfRAGStep failed to evaluate: %w", err)
	}

	// Attach evaluation metrics to state for observability
	state.SelfRagScore = score
	state.SelfRagReason = reason

	if score < s.scoreThreshold {
		if s.strictMode {
			return fmt.Errorf("SelfRAG validation failed (score %f < %f). Reason: %s", score, s.scoreThreshold, reason)
		}
		// If not strict, maybe just append a warning to the answer
		state.Answer = state.Answer + "\n\n[Warning: System detected potential hallucinations in this answer.]"
	}

	return nil
}
