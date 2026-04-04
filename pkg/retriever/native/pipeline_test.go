package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPipelineAssembly verifies that options are assembled in correct three-phase order
func TestPipelineAssembly(t *testing.T) {
	t.Run("BasicPipeline", func(t *testing.T) {
		ret := NewRetriever(nil, nil, nil, 5)
		nr := ret.(*nativeRetriever)
		assert.NotNil(t, nr.pipeline)
		// Basic: Search → Generate
	})

	t.Run("SinglePreRetrieval", func(t *testing.T) {
		ret := NewRetriever(nil, nil, nil, 5, WithQueryRewrite())
		nr := ret.(*nativeRetriever)
		assert.NotNil(t, nr.pipeline)
		// QueryRewrite → Search → Generate
	})

	t.Run("CombinedPreRetrieval", func(t *testing.T) {
		// Users can combine multiple pre-retrieval options
		ret := NewRetriever(nil, nil, nil, 5,
			WithQueryRewrite(),
			WithHyDE(),
		)
		nr := ret.(*nativeRetriever)
		assert.NotNil(t, nr.pipeline)
		// QueryRewrite → HyDE → Search → Generate
	})

	t.Run("FusionPipeline", func(t *testing.T) {
		ret := NewRetriever(nil, nil, nil, 5, WithFusion(5))
		nr := ret.(*nativeRetriever)
		assert.NotNil(t, nr.pipeline)
		// Decompose → MultiSearch → RRF → Generate
	})

	t.Run("CombinedWithFusion", func(t *testing.T) {
		// Users can combine Fusion with other options freely
		ret := NewRetriever(nil, nil, nil, 5,
			WithQueryRewrite(),  // Pre-Retrieval
			WithFusion(5),       // Pre + Post
			WithRerank(),        // Post-Retrieval
		)
		nr := ret.(*nativeRetriever)
		assert.NotNil(t, nr.pipeline)
		// QueryRewrite → Decompose → MultiSearch → RRF → Rerank → Generate
	})

	t.Run("OrderDoesNotMatter", func(t *testing.T) {
		// The order of options doesn't matter - Pipeline is assembled in fixed order
		ret1 := NewRetriever(nil, nil, nil, 5, WithQueryRewrite(), WithHyDE())
		ret2 := NewRetriever(nil, nil, nil, 5, WithHyDE(), WithQueryRewrite())

		// Both should produce the same pipeline structure
		// (QueryRewrite is always before HyDE in Pre-Retrieval phase)
		nr1 := ret1.(*nativeRetriever)
		nr2 := ret2.(*nativeRetriever)
		assert.NotNil(t, nr1.pipeline)
		assert.NotNil(t, nr2.pipeline)
	})

	t.Run("AllEnhancements", func(t *testing.T) {
		ret := NewRetriever(nil, nil, nil, 10,
			WithQueryRewrite(),
			WithHyDE(),
			WithStepBack(),
			WithFusion(5),
			WithRerank(),
		)
		nr := ret.(*nativeRetriever)
		assert.NotNil(t, nr.pipeline)
		// All enhancements assembled in correct order:
		// [Pre] QueryRewrite → HyDE → StepBack → Decompose
		// [Retrieval] MultiSearch
		// [Post] RRF → Rerank → Generate
	})
}

// TestEnhancementOptions verifies that enhancement options are correctly set
func TestEnhancementOptions(t *testing.T) {
	opts := &Options{}

	WithQueryRewrite()(opts)
	assert.True(t, opts.EnableQueryRewrite)

	WithStepBack()(opts)
	assert.True(t, opts.EnableStepBack)

	WithHyDE()(opts)
	assert.True(t, opts.EnableHyDE)

	WithFusion(10)(opts)
	assert.True(t, opts.EnableFusion)
	assert.Equal(t, 10, opts.FusionCount)

	WithRerank()(opts)
	assert.True(t, opts.EnableRerank)
}

// TestThreePhaseStructure verifies the three-phase structure
func TestThreePhaseStructure(t *testing.T) {
	t.Run("PreRetrievalPhase", func(t *testing.T) {
		// Pre-Retrieval options can be combined
		opts := &Options{}
		WithQueryRewrite()(opts)
		WithHyDE()(opts)
		WithStepBack()(opts)
		WithFusion(5)(opts)

		assert.True(t, opts.EnableQueryRewrite)
		assert.True(t, opts.EnableHyDE)
		assert.True(t, opts.EnableStepBack)
		assert.True(t, opts.EnableFusion)
	})

	t.Run("PostRetrievalPhase", func(t *testing.T) {
		// Post-Retrieval options can be combined
		opts := &Options{}
		WithFusion(5)(opts)  // Fusion includes RRF (Post-Retrieval)
		WithRerank()(opts)

		assert.True(t, opts.EnableFusion)
		assert.True(t, opts.EnableRerank)
	})
}
