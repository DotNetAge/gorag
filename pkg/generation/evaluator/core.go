package evaluation

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gorag/pkg/core"
)

// Re-export common types if needed, but prefer direct core usage
type Label = core.CRAGLabel

const (
	Relevant   = core.CRAGRelevant
	Irrelevant = core.CRAGIrrelevant
	Ambiguous  = core.CRAGAmbiguous
)

// TestCase represents a single evaluation entry.
type TestCase struct {
	Query       string `json:"query"`
	GroundTruth string `json:"ground_truth,omitempty"` // Optional: What we expect the answer to be
}

// CaseResult holds the evaluation result for a single test case.
type CaseResult struct {
	Query               string    `json:"query"`
	Answer              string    `json:"answer"`
	FaithfulnessScore   float32   `json:"faithfulness"`
	RelevanceScore      float32   `json:"relevance"`
	PrecisionScore      float32   `json:"precision"`
	Duration            time.Duration `json:"duration"`
}

// BenchmarkResult holds the overall results of a benchmark run.
type BenchmarkResult struct {
	TotalCases        int           `json:"total_cases"`
	AvgFaithfulness   float32       `json:"avg_faithfulness"`
	AvgRelevance      float32       `json:"avg_relevance"`
	AvgPrecision      float32       `json:"avg_precision"`
	TotalDuration     time.Duration `json:"total_duration"`
	Results           []CaseResult  `json:"results"`
}

// RunBenchmark executes a full evaluation suite against a retriever.
func RunBenchmark(ctx context.Context, retriever core.Retriever, judge LLMJudge, cases []TestCase, topK int) (*BenchmarkResult, error) {
	totalStart := time.Now()
	res := &BenchmarkResult{
		TotalCases: len(cases),
		Results:    make([]CaseResult, 0, len(cases)),
	}

	var sumFaith, sumRel, sumPrec float32

	for _, tc := range cases {
		caseStart := time.Now()
		
		// 1. Run Retrieval & Generation
		retResults, err := retriever.Retrieve(ctx, []string{tc.Query}, topK)
		if err != nil || len(retResults) == 0 {
			continue
		}
		ret := retResults[0]

		// 2. Evaluate with Judge
		fScore, _, _ := judge.EvaluateFaithfulness(ctx, tc.Query, ret.Chunks, ret.Answer)
		rScore, _, _ := judge.EvaluateAnswerRelevance(ctx, tc.Query, ret.Answer)
		pScore, _, _ := judge.EvaluateContextPrecision(ctx, tc.Query, ret.Chunks)

		caseRes := CaseResult{
			Query:             tc.Query,
			Answer:            ret.Answer,
			FaithfulnessScore: fScore,
			RelevanceScore:    rScore,
			PrecisionScore:    pScore,
			Duration:          time.Since(caseStart),
		}

		sumFaith += fScore
		sumRel += rScore
		sumPrec += pScore
		res.Results = append(res.Results, caseRes)
	}

	if len(res.Results) > 0 {
		count := float32(len(res.Results))
		res.AvgFaithfulness = sumFaith / count
		res.AvgRelevance = sumRel / count
		res.AvgPrecision = sumPrec / count
	}
	res.TotalDuration = time.Since(totalStart)

	return res, nil
}

// Summary returns a human-readable summary of the benchmark.
func (r *BenchmarkResult) Summary() string {
	return fmt.Sprintf("Benchmark completed in %v\nCases: %d\nAvg Faithfulness: %.2f\nAvg Relevance: %.2f\nAvg Precision: %.2f",
		r.TotalDuration, r.TotalCases, r.AvgFaithfulness, r.AvgRelevance, r.AvgPrecision)
}
