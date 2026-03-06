package plugins

import (
	"context"
	"fmt"
	"plugin"
	"sync"

	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/vectorstore"
)

// PluginType defines the type of plugin
type PluginType string

const (
	// PluginTypeParser represents a document parser plugin
	PluginTypeParser PluginType = "parser"
	// PluginTypeVectorStore represents a vector store plugin
	PluginTypeVectorStore PluginType = "vectorstore"
	// PluginTypeEmbedder represents an embedding provider plugin
	PluginTypeEmbedder PluginType = "embedder"
	// PluginTypeLLM represents an LLM client plugin
	PluginTypeLLM PluginType = "llm"
)

// Plugin defines the interface for all plugins
type Plugin interface {
	Name() string
	Type() PluginType
	Init(config map[string]interface{}) error
}

// ParserPlugin defines the interface for parser plugins
type ParserPlugin interface {
	Plugin
	Parser() parser.Parser
}

// VectorStorePlugin defines the interface for vector store plugins
type VectorStorePlugin interface {
	Plugin
	VectorStore(ctx context.Context) (vectorstore.Store, error)
}

// EmbedderPlugin defines the interface for embedding provider plugins
type EmbedderPlugin interface {
	Plugin
	Embedder() embedding.Provider
}

// LLMPlugin defines the interface for LLM client plugins
type LLMPlugin interface {
	Plugin
	LLM() llm.Client
}

// Registry manages plugins
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register registers a plugin
func (r *Registry) Register(plugin Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[plugin.Name()]; exists {
		return fmt.Errorf("plugin %s already registered", plugin.Name())
	}

	r.plugins[plugin.Name()] = plugin
	return nil
}

// Get returns a plugin by name
func (r *Registry) Get(name string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.plugins[name]
}

// GetByType returns all plugins of a specific type
func (r *Registry) GetByType(pluginType PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, plugin := range r.plugins {
		if plugin.Type() == pluginType {
			result = append(result, plugin)
		}
	}

	return result
}

// Load loads a plugin from a shared library
func (r *Registry) Load(path string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	// Get the plugin constructor
	symbol, err := p.Lookup("NewPlugin")
	if err != nil {
		return fmt.Errorf("failed to find NewPlugin symbol: %w", err)
	}

	// Type assert the constructor
	constructor, ok := symbol.(func() Plugin)
	if !ok {
		return fmt.Errorf("invalid NewPlugin symbol type")
	}

	// Create the plugin
	plugin := constructor()

	// Register the plugin
	return r.Register(plugin)
}

// List returns all registered plugins
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, plugin := range r.plugins {
		result = append(result, plugin)
	}

	return result
}
