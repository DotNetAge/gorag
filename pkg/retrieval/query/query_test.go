package query

import (
	"context"
	"errors"
	"testing"
	"time"

	gchat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMForClassifier struct {
	response *gchat.Response
	err      error
}

func (m *mockLLMForClassifier) Chat(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMForClassifier) ChatStream(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Stream, error) {
	return nil, nil
}

type mockLoggerForClassifier struct{}

func (m *mockLoggerForClassifier) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForClassifier) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForClassifier) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForClassifier) Error(msg string, err error, fields ...map[string]any) {}

type mockCollectorForClassifier struct{}

func (m *mockCollectorForClassifier) RecordCount(name, value string, labels map[string]string) {}
func (m *mockCollectorForClassifier) RecordDuration(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockCollectorForClassifier) RecordValue(name string, value float64, labels map[string]string) {
}

func TestNewIntentRouter(t *testing.T) {
	llm := &mockLLMForClassifier{response: &gchat.Response{Content: "{}"}}
	router := NewIntentRouter(llm)

	assert.NotNil(t, router)
	assert.Equal(t, core.IntentDomainSpecific, router.defaultIntent)
	assert.Equal(t, float32(0.7), router.minConfidence)
}

func TestNewIntentRouter_WithOptions(t *testing.T) {
	llm := &mockLLMForClassifier{response: &gchat.Response{Content: "{}"}}
	router := NewIntentRouter(llm,
		WithIntentPromptTemplate("custom prompt"),
		WithDefaultIntent(core.IntentChat),
		WithMinConfidence(0.5),
		WithIntentRouterLogger(&mockLoggerForClassifier{}),
		WithIntentRouterCollector(&mockCollectorForClassifier{}),
	)

	assert.NotNil(t, router)
	assert.Equal(t, core.IntentChat, router.defaultIntent)
	assert.Equal(t, float32(0.5), router.minConfidence)
}

func TestClassify_Success(t *testing.T) {
	llm := &mockLLMForClassifier{
		response: &gchat.Response{
			Content: `{"intent":"chat","confidence":0.9,"reason":"simple greeting"}`,
		},
	}
	router := NewIntentRouter(llm)

	result, err := router.Classify(context.Background(), core.NewQuery("1", "Hello", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, core.IntentChat, result.Intent)
}

func TestClassify_NilQuery(t *testing.T) {
	router := NewIntentRouter(nil)

	result, err := router.Classify(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nil or empty")
}

func TestClassify_EmptyQuery(t *testing.T) {
	router := NewIntentRouter(nil)

	result, err := router.Classify(context.Background(), core.NewQuery("1", "", nil))

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClassify_LLMError(t *testing.T) {
	llm := &mockLLMForClassifier{err: errors.New("LLM error")}
	router := NewIntentRouter(llm)

	result, err := router.Classify(context.Background(), core.NewQuery("1", "test query", nil))

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "intent classification failed")
}

func TestClassify_InvalidJSONUsesDefault(t *testing.T) {
	llm := &mockLLMForClassifier{
		response: &gchat.Response{Content: "not valid json"},
	}
	router := NewIntentRouter(llm, WithDefaultIntent(core.IntentGlobal))

	result, err := router.Classify(context.Background(), core.NewQuery("1", "test", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, core.IntentGlobal, result.Intent)
}

func TestClassify_ConfidenceBelowMin(t *testing.T) {
	llm := &mockLLMForClassifier{
		response: &gchat.Response{
			Content: `{"intent":"chat","confidence":0.3,"reason":"low confidence"}`,
		},
	}
	router := NewIntentRouter(llm, WithDefaultIntent(core.IntentDomainSpecific))

	result, err := router.Classify(context.Background(), core.NewQuery("1", "test", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, core.IntentDomainSpecific, result.Intent)
}

func TestClassify_WithJSONWrapper(t *testing.T) {
	llm := &mockLLMForClassifier{
		response: &gchat.Response{
			Content: "Here is the result: {\"intent\":\"relational\",\"confidence\":0.8,\"reason\":\"entity question\"}",
		},
	}
	router := NewIntentRouter(llm)

	result, err := router.Classify(context.Background(), core.NewQuery("1", "Who is the CEO?", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, core.IntentRelational, result.Intent)
}

func TestWithIntentPromptTemplate_EmptyString(t *testing.T) {
	llm := &mockLLMForClassifier{response: &gchat.Response{Content: "{}"}}
	router := NewIntentRouter(llm, WithIntentPromptTemplate(""))

	assert.NotNil(t, router)
}

func TestWithMinConfidence_ZeroValue(t *testing.T) {
	llm := &mockLLMForClassifier{response: &gchat.Response{Content: "{}"}}
	router := NewIntentRouter(llm, WithMinConfidence(0.5))

	assert.NotNil(t, router)
	assert.Equal(t, float32(0.5), router.minConfidence)
}
