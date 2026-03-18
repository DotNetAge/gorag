package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/evaluation"
)

// selfRAG is an interceptor that uses an LLM Evaluator to check the generated answer
// for hallucinations (Faithfulness) before returning it to the user.
type selfRAG struct {
	judge          evaluation.LLMJudge
	strictMode     bool
	scoreThreshold float32
}

// SelfRAG creates a Self-RAG reflection step.
//
// Parameters:
//   - judge: LLM judge implementation
//   - strictMode: if true, fails when score < threshold; if false, appends warning
//   - threshold: minimum acceptable faithfulness score (default: 0.8)
//
// Example:
//
//	p.AddStep(agentic.SelfRAG(judge, true, 0.8))
func SelfRAG(judge evaluation.LLMJudge, strictMode bool, threshold float32) pipeline.Step[*entity.PipelineState] {
	if threshold <= 0 {
		threshold = 0.8
	}
	return &selfRAG{
		judge:          judge,
		strictMode:     strictMode,
		scoreThreshold: threshold,
	}
}

// Name returns the step name
func (s *selfRAG) Name() string {
	return "SelfRAG"
}

// Execute evaluates the generated answer for faithfulness and updates state accordingly.
func (s *selfRAG) Execute(ctx context.Context, state *entity.PipelineState) error {
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
		return fmt.Errorf("SelfRAG failed to evaluate: %w", err)
	}

	// Attach evaluation metrics to state for observability
	state.SelfRagScore = score
	state.SelfRagReason = reason

	if score < s.scoreThreshold {
		if s.strictMode {
			return fmt.Errorf("SelfRAG validation failed (score %f < %f). Reason: %s", score, s.scoreThreshold, reason)
		}
		// If not strict, append a warning to the answer
		state.Answer = state.Answer + "\n\n[Warning: System detected potential hallucinations in this answer.]"
	}

	return nil
}
