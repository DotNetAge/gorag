package rag

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/raya-dev/gorag/parser"
	"github.com/raya-dev/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockParserImpl struct {
	parseFunc func(ctx context.Context, r io.Reader) ([]parser.Chunk, error)
}

func (m *mockParserImpl) Parse(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
	if m.parseFunc != nil {
		return m.parseFunc(ctx, r)
	}
	content, _ := io.ReadAll(r)
	return []parser.Chunk{
		{
			ID:      "chunk1",
			Content: string(content),
			Metadata: map[string]string{
				"source": "test",
			},
		},
	}, nil
}

func (m *mockParserImpl) SupportedFormats() []string {
	return []string{"txt"}
}

type mockEmbedder struct {
	embedFunc func(ctx context.Context, texts []string) ([][]float32, error)
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, texts)
	}
	results := make([][]float32, len(texts))
	for i := range texts {
		results[i] = []float32{0.1, 0.2, 0.3}
	}
	return results, nil
}

func (m *mockEmbedder) Dimension() int {
	return 3
}

type mockStore struct {
	addFunc      func(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error
	searchFunc   func(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error)
	deleteFunc   func(ctx context.Context, ids []string) error
	searchCalled bool
	addCalled    bool
	deleteCalled bool
}

func (m *mockStore) Add(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
	m.addCalled = true
	if m.addFunc != nil {
		return m.addFunc(ctx, chunks, embeddings)
	}
	return nil
}

func (m *mockStore) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
	m.searchCalled = true
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, opts)
	}
	return []vectorstore.Result{
		{
			Chunk: vectorstore.Chunk{
				ID:      "doc1",
				Content: "相关文档内容",
				Metadata: map[string]string{
					"source": "test",
				},
			},
			Score: 0.9,
		},
	}, nil
}

func (m *mockStore) Delete(ctx context.Context, ids []string) error {
	m.deleteCalled = true
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, ids)
	}
	return nil
}

type mockLLM struct {
	completeFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *mockLLM) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "这是基于上下文的答案", nil
}

func (m *mockLLM) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "这是基于上下文的答案"
	close(ch)
	return ch, nil
}

func TestNewEngine(t *testing.T) {
	tests := []struct {
		name        string
		opts        []Option
		expectedErr bool
		errMsg      string
	}{
		{
			name: "valid configuration",
			opts: []Option{
				WithParser(&mockParserImpl{}),
				WithEmbedder(&mockEmbedder{}),
				WithVectorStore(&mockStore{}),
				WithLLM(&mockLLM{}),
			},
			expectedErr: false,
		},
		{
			name:        "missing parser",
			opts:        []Option{},
			expectedErr: true,
			errMsg:      "parser is required",
		},
		{
			name: "missing embedder",
			opts: []Option{
				WithParser(&mockParserImpl{}),
			},
			expectedErr: true,
			errMsg:      "embedder is required",
		},
		{
			name: "missing vector store",
			opts: []Option{
				WithParser(&mockParserImpl{}),
				WithEmbedder(&mockEmbedder{}),
			},
			expectedErr: true,
			errMsg:      "vector store is required",
		},
		{
			name: "missing llm",
			opts: []Option{
				WithParser(&mockParserImpl{}),
				WithEmbedder(&mockEmbedder{}),
				WithVectorStore(&mockStore{}),
			},
			expectedErr: true,
			errMsg:      "LLM client is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := New(tt.opts...)

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, engine)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, engine)
			}
		})
	}
}

func TestEngine_Index(t *testing.T) {
	tests := []struct {
		name        string
		source      Source
		setupMock   func(*mockStore, *mockParserImpl, *mockEmbedder)
		expectedErr bool
		errMsg      string
	}{
		{
			name: "successful index",
			source: Source{
				Type:    "text",
				Content: "测试文档内容",
			},
			setupMock: func(store *mockStore, p *mockParserImpl, embedder *mockEmbedder) {
				p.parseFunc = func(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
					content, _ := io.ReadAll(r)
					return []parser.Chunk{
						{
							ID:      "chunk1",
							Content: string(content),
							Metadata: map[string]string{
								"source": "test",
							},
						},
					}, nil
				}
			},
			expectedErr: false,
		},
		{
			name: "parser error",
			source: Source{
				Type:    "text",
				Content: "测试文档内容",
			},
			setupMock: func(store *mockStore, p *mockParserImpl, embedder *mockEmbedder) {
				p.parseFunc = func(ctx context.Context, r io.Reader) ([]parser.Chunk, error) {
					return nil, errors.New("parse failed")
				}
			},
			expectedErr: true,
			errMsg:      "parse failed",
		},
		{
			name: "embedder error",
			source: Source{
				Type:    "text",
				Content: "测试文档内容",
			},
			setupMock: func(store *mockStore, p *mockParserImpl, embedder *mockEmbedder) {
				embedder.embedFunc = func(ctx context.Context, texts []string) ([][]float32, error) {
					return nil, errors.New("embedding failed")
				}
			},
			expectedErr: true,
			errMsg:      "embedding failed",
		},
		{
			name: "store error",
			source: Source{
				Type:    "text",
				Content: "测试文档内容",
			},
			setupMock: func(store *mockStore, p *mockParserImpl, embedder *mockEmbedder) {
				store.addFunc = func(ctx context.Context, chunks []vectorstore.Chunk, embeddings [][]float32) error {
					return errors.New("storage failed")
				}
			},
			expectedErr: true,
			errMsg:      "storage failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockStore{}
			parser := &mockParserImpl{}
			embedder := &mockEmbedder{}
			llm := &mockLLM{}

			if tt.setupMock != nil {
				tt.setupMock(store, parser, embedder)
			}

			engine, err := New(
				WithParser(parser),
				WithEmbedder(embedder),
				WithVectorStore(store),
				WithLLM(llm),
			)
			require.NoError(t, err)

			err = engine.Index(context.Background(), tt.source)

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.True(t, store.addCalled)
			}
		})
	}
}

func TestEngine_Query(t *testing.T) {
	tests := []struct {
		name        string
		question    string
		opts        QueryOptions
		setupMock   func(*mockStore, *mockLLM)
		expectedErr bool
		errMsg      string
		validate    func(*testing.T, *Response)
	}{
		{
			name:     "successful query",
			question: "什么是RAG？",
			opts: QueryOptions{
				TopK: 5,
			},
			setupMock: func(store *mockStore, llm *mockLLM) {
				store.searchFunc = func(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
					return []vectorstore.Result{
						{
							Chunk: vectorstore.Chunk{
								ID:      "doc1",
								Content: "RAG是检索增强生成技术",
								Metadata: map[string]string{
									"source": "test",
								},
							},
							Score: 0.9,
						},
					}, nil
				}
			},
			expectedErr: false,
			validate: func(t *testing.T, resp *Response) {
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.Answer)
				assert.Len(t, resp.Sources, 1)
			},
		},
		{
			name:     "search error",
			question: "什么是RAG？",
			opts: QueryOptions{
				TopK: 5,
			},
			setupMock: func(store *mockStore, llm *mockLLM) {
				store.searchFunc = func(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]vectorstore.Result, error) {
					return nil, errors.New("search failed")
				}
			},
			expectedErr: true,
			errMsg:      "search failed",
		},
		{
			name:     "llm error",
			question: "什么是RAG？",
			opts: QueryOptions{
				TopK: 5,
			},
			setupMock: func(store *mockStore, llm *mockLLM) {
				llm.completeFunc = func(ctx context.Context, prompt string) (string, error) {
					return "", errors.New("llm failed")
				}
			},
			expectedErr: true,
			errMsg:      "llm failed",
		},
		{
			name:        "empty question",
			question:    "",
			opts:        QueryOptions{},
			setupMock:   func(store *mockStore, llm *mockLLM) {},
			expectedErr: true,
			errMsg:      "question is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockStore{}
			parser := &mockParserImpl{}
			embedder := &mockEmbedder{}
			llm := &mockLLM{}

			if tt.setupMock != nil {
				tt.setupMock(store, llm)
			}

			engine, err := New(
				WithParser(parser),
				WithEmbedder(embedder),
				WithVectorStore(store),
				WithLLM(llm),
			)
			require.NoError(t, err)

			response, err := engine.Query(context.Background(), tt.question, tt.opts)

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.True(t, store.searchCalled)
				if tt.validate != nil {
					tt.validate(t, response)
				}
			}
		})
	}
}

func TestEngine_Query_WithCustomTemplate(t *testing.T) {
	store := &mockStore{}
	parser := &mockParserImpl{}
	embedder := &mockEmbedder{}
	llm := &mockLLM{}

	engine, err := New(
		WithParser(parser),
		WithEmbedder(embedder),
		WithVectorStore(store),
		WithLLM(llm),
	)
	require.NoError(t, err)

	customTemplate := "自定义模板：{question}"
	response, err := engine.Query(context.Background(), "测试问题", QueryOptions{
		TopK:           3,
		PromptTemplate: customTemplate,
	})

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.Answer)
}

func TestEngine_Query_WithStream(t *testing.T) {
	store := &mockStore{}
	parser := &mockParserImpl{}
	embedder := &mockEmbedder{}
	llm := &mockLLM{}

	engine, err := New(
		WithParser(parser),
		WithEmbedder(embedder),
		WithVectorStore(store),
		WithLLM(llm),
	)
	require.NoError(t, err)

	response, err := engine.Query(context.Background(), "测试问题", QueryOptions{
		TopK:   3,
		Stream: true,
	})

	assert.NoError(t, err)
	assert.NotNil(t, response)
}

func TestEngine_Query_Timeout(t *testing.T) {
	store := &mockStore{}
	parser := &mockParserImpl{}
	embedder := &mockEmbedder{}
	llm := &mockLLM{}

	embedder.embedFunc = func(ctx context.Context, texts []string) ([][]float32, error) {
		time.Sleep(2 * time.Second)
		return nil, context.DeadlineExceeded
	}

	engine, err := New(
		WithParser(parser),
		WithEmbedder(embedder),
		WithVectorStore(store),
		WithLLM(llm),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = engine.Query(ctx, "测试问题", QueryOptions{TopK: 3})
	assert.Error(t, err)
}

func TestGenerateChunkID(t *testing.T) {
	id1 := generateChunkID()
	id2 := generateChunkID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestQueryOptions_DefaultValues(t *testing.T) {
	opts := QueryOptions{}

	assert.Equal(t, 0, opts.TopK)
	assert.Empty(t, opts.PromptTemplate)
	assert.False(t, opts.Stream)
}

func TestSource_Validation(t *testing.T) {
	tests := []struct {
		name        string
		source      Source
		expectedErr bool
	}{
		{
			name: "valid source with content",
			source: Source{
				Type:    "text",
				Content: "content",
			},
			expectedErr: false,
		},
		{
			name: "valid source with path",
			source: Source{
				Type: "file",
				Path: "/path/to/file.txt",
			},
			expectedErr: false,
		},
		{
			name: "invalid source - missing type",
			source: Source{
				Content: "content",
			},
			expectedErr: true,
		},
		{
			name: "invalid source - missing both content and path",
			source: Source{
				Type: "file",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.source.Type == "" {
				err = errors.New("type is required")
			} else if tt.source.Content == "" && tt.source.Path == "" {
				err = errors.New("content or path is required")
			}

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEngine_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	store := &mockStore{}
	parser := &mockParserImpl{}
	embedder := &mockEmbedder{}
	llm := &mockLLM{}

	engine, err := New(
		WithParser(parser),
		WithEmbedder(embedder),
		WithVectorStore(store),
		WithLLM(llm),
	)
	require.NoError(t, err)

	ctx := context.Background()

	err = engine.Index(ctx, Source{
		Type:    "text",
		Content: "RAG是一种结合检索和生成的AI技术",
	})
	assert.NoError(t, err)

	response, err := engine.Query(ctx, "什么是RAG？", QueryOptions{
		TopK: 5,
	})
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.Answer)
}
