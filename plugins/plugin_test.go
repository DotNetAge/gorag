package plugins

import (
	"context"
	"io"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/vectorstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPlugin is a mock plugin for testing
type mockPlugin struct {
	name  string
	ptype PluginType
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) Type() PluginType {
	return m.ptype
}

func (m *mockPlugin) Init(config map[string]interface{}) error {
	return nil
}

// mockParserPlugin is a mock parser plugin for testing
type mockParserPlugin struct {
	mockPlugin
	parser parser.Parser
}

func (m *mockParserPlugin) Parser() parser.Parser {
	return m.parser
}

// mockVectorStorePlugin is a mock vector store plugin for testing
type mockVectorStorePlugin struct {
	mockPlugin
	store vectorstore.Store
}

func (m *mockVectorStorePlugin) VectorStore(ctx context.Context) (vectorstore.Store, error) {
	return m.store, nil
}

// mockEmbedderPlugin is a mock embedder plugin for testing
type mockEmbedderPlugin struct {
	mockPlugin
	embedder embedding.Provider
}

func (m *mockEmbedderPlugin) Embedder() embedding.Provider {
	return m.embedder
}

// mockLLMPlugin is a mock LLM plugin for testing
type mockLLMPlugin struct {
	mockPlugin
	llmClient llm.Client
}

func (m *mockLLMPlugin) LLM() llm.Client {
	return m.llmClient
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	require.NotNil(t, registry)
	assert.NotNil(t, registry.plugins)
	assert.Empty(t, registry.plugins)
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	// Test registering a plugin
	plugin := &mockPlugin{
		name:  "test-plugin",
		ptype: PluginTypeParser,
	}

	err := registry.Register(plugin)
	require.NoError(t, err)

	// Test registering the same plugin again
	err = registry.Register(plugin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	// Test getting a non-existent plugin
	plugin := registry.Get("non-existent")
	assert.Nil(t, plugin)

	// Test getting a registered plugin
	expectedPlugin := &mockPlugin{
		name:  "test-plugin",
		ptype: PluginTypeParser,
	}

	err := registry.Register(expectedPlugin)
	require.NoError(t, err)

	actualPlugin := registry.Get("test-plugin")
	assert.Equal(t, expectedPlugin, actualPlugin)
}

func TestRegistry_GetByType(t *testing.T) {
	registry := NewRegistry()

	// Register plugins of different types
	parserPlugin := &mockPlugin{
		name:  "parser-plugin",
		ptype: PluginTypeParser,
	}

	vectorStorePlugin := &mockPlugin{
		name:  "vectorstore-plugin",
		ptype: PluginTypeVectorStore,
	}

	embedderPlugin := &mockPlugin{
		name:  "embedder-plugin",
		ptype: PluginTypeEmbedder,
	}

	llmPlugin := &mockPlugin{
		name:  "llm-plugin",
		ptype: PluginTypeLLM,
	}

	err := registry.Register(parserPlugin)
	require.NoError(t, err)

	err = registry.Register(vectorStorePlugin)
	require.NoError(t, err)

	err = registry.Register(embedderPlugin)
	require.NoError(t, err)

	err = registry.Register(llmPlugin)
	require.NoError(t, err)

	// Test getting plugins by type
	parserPlugins := registry.GetByType(PluginTypeParser)
	assert.Len(t, parserPlugins, 1)
	assert.Equal(t, parserPlugin, parserPlugins[0])

	vectorStorePlugins := registry.GetByType(PluginTypeVectorStore)
	assert.Len(t, vectorStorePlugins, 1)
	assert.Equal(t, vectorStorePlugin, vectorStorePlugins[0])

	embedderPlugins := registry.GetByType(PluginTypeEmbedder)
	assert.Len(t, embedderPlugins, 1)
	assert.Equal(t, embedderPlugin, embedderPlugins[0])

	llmPlugins := registry.GetByType(PluginTypeLLM)
	assert.Len(t, llmPlugins, 1)
	assert.Equal(t, llmPlugin, llmPlugins[0])

	// Test getting plugins of a non-existent type
	nonExistentPlugins := registry.GetByType("non-existent")
	assert.Empty(t, nonExistentPlugins)
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// Test listing plugins when none are registered
	plugins := registry.List()
	assert.Empty(t, plugins)

	// Register some plugins
	plugin1 := &mockPlugin{
		name:  "plugin1",
		ptype: PluginTypeParser,
	}

	plugin2 := &mockPlugin{
		name:  "plugin2",
		ptype: PluginTypeVectorStore,
	}

	err := registry.Register(plugin1)
	require.NoError(t, err)

	err = registry.Register(plugin2)
	require.NoError(t, err)

	// Test listing plugins
	plugins = registry.List()
	assert.Len(t, plugins, 2)
	assert.Contains(t, plugins, plugin1)
	assert.Contains(t, plugins, plugin2)
}

func TestPluginType_Values(t *testing.T) {
	assert.Equal(t, PluginType("parser"), PluginTypeParser)
	assert.Equal(t, PluginType("vectorstore"), PluginTypeVectorStore)
	assert.Equal(t, PluginType("embedder"), PluginTypeEmbedder)
	assert.Equal(t, PluginType("llm"), PluginTypeLLM)
}

func TestParserPlugin_Interface(t *testing.T) {
	mockParser := &mockParserImpl{}
	plugin := &mockParserPlugin{
		mockPlugin: mockPlugin{
			name:  "test-parser",
			ptype: PluginTypeParser,
		},
		parser: mockParser,
	}

	assert.Implements(t, (*Plugin)(nil), plugin)
	assert.Implements(t, (*ParserPlugin)(nil), plugin)
	assert.Equal(t, "test-parser", plugin.Name())
	assert.Equal(t, PluginTypeParser, plugin.Type())
	assert.Equal(t, mockParser, plugin.Parser())
}

func TestVectorStorePlugin_Interface(t *testing.T) {
	mockStore := &mockStoreImpl{}
	plugin := &mockVectorStorePlugin{
		mockPlugin: mockPlugin{
			name:  "test-vectorstore",
			ptype: PluginTypeVectorStore,
		},
		store: mockStore,
	}

	assert.Implements(t, (*Plugin)(nil), plugin)
	assert.Implements(t, (*VectorStorePlugin)(nil), plugin)
	assert.Equal(t, "test-vectorstore", plugin.Name())
	assert.Equal(t, PluginTypeVectorStore, plugin.Type())

	store, err := plugin.VectorStore(context.Background())
	require.NoError(t, err)
	assert.Equal(t, mockStore, store)
}

func TestEmbedderPlugin_Interface(t *testing.T) {
	mockEmbedder := &mockEmbedderImpl{}
	plugin := &mockEmbedderPlugin{
		mockPlugin: mockPlugin{
			name:  "test-embedder",
			ptype: PluginTypeEmbedder,
		},
		embedder: mockEmbedder,
	}

	assert.Implements(t, (*Plugin)(nil), plugin)
	assert.Implements(t, (*EmbedderPlugin)(nil), plugin)
	assert.Equal(t, "test-embedder", plugin.Name())
	assert.Equal(t, PluginTypeEmbedder, plugin.Type())
	assert.Equal(t, mockEmbedder, plugin.Embedder())
}

func TestLLMPlugin_Interface(t *testing.T) {
	mockLLM := &mockLLMImpl{}
	plugin := &mockLLMPlugin{
		mockPlugin: mockPlugin{
			name:  "test-llm",
			ptype: PluginTypeLLM,
		},
		llmClient: mockLLM,
	}

	assert.Implements(t, (*Plugin)(nil), plugin)
	assert.Implements(t, (*LLMPlugin)(nil), plugin)
	assert.Equal(t, "test-llm", plugin.Name())
	assert.Equal(t, PluginTypeLLM, plugin.Type())
	assert.Equal(t, mockLLM, plugin.LLM())
}

// mockParserImpl is a mock parser implementation
type mockParserImpl struct{}

func (m *mockParserImpl) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	return []core.Chunk{}, nil
}

func (m *mockParserImpl) SupportedFormats() []string {
	return []string{}
}

// mockStoreImpl is a mock vector store implementation
type mockStoreImpl struct{}

func (m *mockStoreImpl) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	return nil
}

func (m *mockStoreImpl) Search(ctx context.Context, query []float32, opts vectorstore.SearchOptions) ([]core.Result, error) {
	return []core.Result{}, nil
}

func (m *mockStoreImpl) Delete(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockStoreImpl) SearchByMetadata(ctx context.Context, metadata map[string]string) ([]core.Chunk, error) {
	return []core.Chunk{}, nil
}

// mockEmbedderImpl is a mock embedding provider implementation
type mockEmbedderImpl struct{}

func (m *mockEmbedderImpl) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return [][]float32{}, nil
}

func (m *mockEmbedderImpl) Dimension() int {
	return 0
}

// mockLLMImpl is a mock LLM client implementation
type mockLLMImpl struct{}

func (m *mockLLMImpl) Complete(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (m *mockLLMImpl) CompleteStream(ctx context.Context, prompt string) (<-chan string, error) {
	return nil, nil
}
