package enhancement

import (
	"context"
	"errors"
	"testing"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMClient struct {
	responseContent string
	responseError   error
}

func (m *mockLLMClient) Chat(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Response, error) {
	if m.responseError != nil {
		return nil, m.responseError
	}
	return &chat.Response{Content: m.responseContent}, nil
}

func (m *mockLLMClient) ChatStream(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Stream, error) {
	return nil, nil
}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (*chat.Response, error) {
	if m.responseError != nil {
		return nil, m.responseError
	}
	return &chat.Response{Content: m.responseContent}, nil
}

func TestExtract_Success(t *testing.T) {
	mock := &mockLLMClient{
		responseContent: `{"year": 2023, "author": "张三", "topic": "AI"}`,
	}
	extractor := NewFilterExtractor(mock)

	query := &core.Query{Text: "2023年张三写的关于AI的文章"}
	filters, err := extractor.Extract(context.Background(), query)

	assert.NoError(t, err)
	assert.Equal(t, float64(2023), filters["year"])
	assert.Equal(t, "张三", filters["author"])
	assert.Equal(t, "AI", filters["topic"])
}

func TestExtract_NilQuery(t *testing.T) {
	mock := &mockLLMClient{
		responseContent: `{}`,
	}
	extractor := NewFilterExtractor(mock)

	filters, err := extractor.Extract(context.Background(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, filters)
}

func TestExtract_EmptyQuery(t *testing.T) {
	mock := &mockLLMClient{
		responseContent: `{}`,
	}
	extractor := NewFilterExtractor(mock)

	query := &core.Query{Text: ""}
	filters, err := extractor.Extract(context.Background(), query)

	assert.NoError(t, err)
	assert.NotNil(t, filters)
}

func TestExtract_LLMError(t *testing.T) {
	mock := &mockLLMClient{
		responseError: errors.New("LLM 调用失败"),
	}
	extractor := NewFilterExtractor(mock)

	query := &core.Query{Text: "2023年的文章"}
	filters, err := extractor.Extract(context.Background(), query)

	assert.Error(t, err)
	assert.Nil(t, filters)
	assert.Contains(t, err.Error(), "LLM 调用失败")
}

func TestExtract_InvalidJSON(t *testing.T) {
	mock := &mockLLMClient{
		responseContent: `这不是有效的 JSON`,
	}
	extractor := NewFilterExtractor(mock)

	query := &core.Query{Text: "2023年的文章"}
	filters, err := extractor.Extract(context.Background(), query)

	assert.NoError(t, err)
	assert.NotNil(t, filters)
	assert.Equal(t, 0, len(filters))
}

func TestExtract_EmptyResponse(t *testing.T) {
	mock := &mockLLMClient{
		responseContent: `{}`,
	}
	extractor := NewFilterExtractor(mock)

	query := &core.Query{Text: "给我一些关于编程的文章"}
	filters, err := extractor.Extract(context.Background(), query)

	assert.NoError(t, err)
	assert.NotNil(t, filters)
	assert.Equal(t, 0, len(filters))
}

func TestExtract_WithMarkdownWrapper(t *testing.T) {
	mock := &mockLLMClient{
		responseContent: "```json\n{\"category\": \"技术\", \"language\": \"Go\"}\n```",
	}
	extractor := NewFilterExtractor(mock)

	query := &core.Query{Text: "关于 Go 语言的 技术文章"}
	filters, err := extractor.Extract(context.Background(), query)

	assert.NoError(t, err)
	assert.Equal(t, "技术", filters["category"])
	assert.Equal(t, "Go", filters["language"])
}
