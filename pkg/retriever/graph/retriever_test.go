package graph

import (
	"context"
	"testing"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks
type mockVectorStore struct {
	mock.Mock
}

func (m *mockVectorStore) Upsert(ctx context.Context, vectors []*core.Vector) error {
	args := m.Called(ctx, vectors)
	return args.Error(0)
}

func (m *mockVectorStore) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	args := m.Called(ctx, query, topK, filters)
	return args.Get(0).([]*core.Vector), args.Get(1).([]float32), args.Error(2)
}

func (m *mockVectorStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockVectorStore) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type mockGraphStore struct {
	mock.Mock
}

func (m *mockGraphStore) UpsertNodes(ctx context.Context, nodes []*core.Node) error {
	args := m.Called(ctx, nodes)
	return args.Error(0)
}

func (m *mockGraphStore) UpsertEdges(ctx context.Context, edges []*core.Edge) error {
	args := m.Called(ctx, edges)
	return args.Error(0)
}

func (m *mockGraphStore) GetNode(ctx context.Context, id string) (*core.Node, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*core.Node), args.Error(1)
}

func (m *mockGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int, limit int) ([]*core.Node, []*core.Edge, error) {
	args := m.Called(ctx, nodeID, depth, limit)
	return args.Get(0).([]*core.Node), args.Get(1).([]*core.Edge), args.Error(2)
}

func (m *mockGraphStore) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	args := m.Called(ctx, query, params)
	return args.Get(0).([]map[string]any), args.Error(1)
}

func (m *mockGraphStore) GetCommunitySummaries(ctx context.Context, level int) ([]map[string]any, error) {
	args := m.Called(ctx, level)
	return args.Get(0).([]map[string]any), args.Error(1)
}

func (m *mockGraphStore) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type mockEmbedder struct {
	mock.Mock
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	return args.Get(0).([][]float32), args.Error(1)
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	return args.Get(0).([]float32), args.Error(1)
}

func (m *mockEmbedder) Dimension() int {
	args := m.Called()
	return args.Int(0)
}

type mockLLM struct {
	mock.Mock
}

func (m *mockLLM) Chat(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Response, error) {
	args := m.Called(ctx, messages, opts)
	return args.Get(0).(*chat.Response), args.Error(1)
}

func (m *mockLLM) ChatStream(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Stream, error) {
	args := m.Called(ctx, messages, opts)
	return args.Get(0).(*chat.Stream), args.Error(1)
}

func TestGraphRetriever_Retrieve(t *testing.T) {
	mVS := new(mockVectorStore)
	mGS := new(mockGraphStore)
	mEmb := new(mockEmbedder)
	mLLM := new(mockLLM)

	ctx := context.Background()
	queryText := "Who is the CEO of Anthropic?"

	// 1. Entity Extraction Mock
	mLLM.On("Chat", ctx, mock.MatchedBy(func(msgs []chat.Message) bool {
		return len(msgs) > 0 && msgs[0].Role == chat.RoleUser
	}), mock.Anything).Return(&chat.Response{
		Content: `{"entities": ["Anthropic", "CEO"]}`,
	}, nil).Once()

	// 2. Graph Search Mock
	mGS.On("GetNeighbors", mock.Anything, "Anthropic", 1, 10).Return(
		[]*core.Node{{ID: "Dario Amodei", Type: "PERSON"}},
		[]*core.Edge{{Source: "Dario Amodei", Target: "Anthropic", Type: "CEO_OF"}},
		nil,
	).Once()
	mGS.On("GetNeighbors", mock.Anything, "CEO", 1, 10).Return(
		[]*core.Node{},
		[]*core.Edge{},
		nil,
	).Once()

	// 3. Vector Search Mock
	queryVec := []float32{0.1, 0.2, 0.3}
	mEmb.On("Embed", mock.Anything, []string{queryText}).Return([][]float32{queryVec}, nil).Once()
	mVS.On("Search", mock.Anything, queryVec, 5, mock.Anything).Return(
		[]*core.Vector{{ID: "vec1", ChunkID: "chunk1"}},
		[]float32{0.9},
		nil,
	).Once()

	// 4. Generation Mock
	mLLM.On("Chat", ctx, mock.MatchedBy(func(msgs []chat.Message) bool {
		return len(msgs) > 0 && msgs[0].Role == chat.RoleUser
	}), mock.Anything).Return(&chat.Response{
		Content: "The CEO of Anthropic is Dario Amodei.",
	}, nil).Once()

	retriever := NewRetriever(mVS, mGS, mEmb, mLLM)
	results, err := retriever.Retrieve(ctx, []string{queryText}, 5)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "The CEO of Anthropic is Dario Amodei.", results[0].Answer)
	assert.Equal(t, queryText, results[0].Query)

	mVS.AssertExpectations(t)
	mGS.AssertExpectations(t)
	mEmb.AssertExpectations(t)
	mLLM.AssertExpectations(t)
}
