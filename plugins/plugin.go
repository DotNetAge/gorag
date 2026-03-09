package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"

	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/vectorstore"
)

// PluginType defines the type of plugin
//
// This type categorizes the different types of plugins that can be
// registered with the GoRAG framework.
type PluginType string

const (
	// PluginTypeParser represents a document parser plugin
	// These plugins add support for new file formats
	PluginTypeParser PluginType = "parser"
	// PluginTypeVectorStore represents a vector store plugin
	// These plugins add support for new vector database backends
	PluginTypeVectorStore PluginType = "vectorstore"
	// PluginTypeEmbedder represents an embedding provider plugin
	// These plugins add support for new embedding models
	PluginTypeEmbedder PluginType = "embedder"
	// PluginTypeLLM represents an LLM client plugin
	// These plugins add support for new LLM providers
	PluginTypeLLM PluginType = "llm"
)

// Plugin defines the interface for all plugins
//
// This is the base interface that all plugins must implement.
// It provides basic metadata and initialization functionality.
//
// Example implementation:
//
//     type MyPlugin struct {
//         name string
//         pluginType PluginType
//     }
//
//     func (p *MyPlugin) Name() string {
//         return p.name
//     }
//
//     func (p *MyPlugin) Type() PluginType {
//         return p.pluginType
//     }
//
//     func (p *MyPlugin) Init(config map[string]interface{}) error {
//         // Initialize plugin with config
//         return nil
//     }
type Plugin interface {
	// Name returns the name of the plugin
	//
	// Returns:
	// - string: Unique name of the plugin
	Name() string
	
	// Type returns the type of the plugin
	//
	// Returns:
	// - PluginType: Type of the plugin
	Type() PluginType
	
	// Init initializes the plugin with the given configuration
	//
	// Parameters:
	// - config: Configuration map for the plugin
	//
	// Returns:
	// - error: Error if initialization fails
	Init(config map[string]interface{}) error
}

// ParserPlugin defines the interface for parser plugins
//
// Parser plugins add support for new file formats to the RAG engine.
//
// Example implementation:
//
//     type MyParserPlugin struct {
//         MyPlugin
//         parser parser.Parser
//     }
//
//     func (p *MyParserPlugin) Parser() parser.Parser {
//         return p.parser
//     }
type ParserPlugin interface {
	Plugin
	
	// Parser returns the parser implementation
	//
	// Returns:
	// - parser.Parser: Parser implementation
	Parser() parser.Parser
}

// VectorStorePlugin defines the interface for vector store plugins
//
// Vector store plugins add support for new vector database backends.
//
// Example implementation:
//
//     type MyVectorStorePlugin struct {
//         MyPlugin
//         config map[string]interface{}
//     }
//
//     func (p *MyVectorStorePlugin) VectorStore(ctx context.Context) (vectorstore.Store, error) {
//         // Create and return vector store instance
//     }
type VectorStorePlugin interface {
	Plugin
	
	// VectorStore creates and returns a vector store instance
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	//
	// Returns:
	// - vectorstore.Store: Vector store instance
	// - error: Error if creation fails
	VectorStore(ctx context.Context) (vectorstore.Store, error)
}

// EmbedderPlugin defines the interface for embedding provider plugins
//
// Embedder plugins add support for new embedding models.
//
// Example implementation:
//
//     type MyEmbedderPlugin struct {
//         MyPlugin
//         embedder embedding.Provider
//     }
//
//     func (p *MyEmbedderPlugin) Embedder() embedding.Provider {
//         return p.embedder
//     }
type EmbedderPlugin interface {
	Plugin
	
	// Embedder returns the embedding provider implementation
	//
	// Returns:
	// - embedding.Provider: Embedding provider implementation
	Embedder() embedding.Provider
}

// LLMPlugin defines the interface for LLM client plugins
//
// LLM plugins add support for new LLM providers.
//
// Example implementation:
//
//     type MyLLMPlugin struct {
//         MyPlugin
//         llmClient llm.Client
//     }
//
//     func (p *MyLLMPlugin) LLM() llm.Client {
//         return p.llmClient
//     }
type LLMPlugin interface {
	Plugin
	
	// LLM returns the LLM client implementation
	//
	// Returns:
	// - llm.Client: LLM client implementation
	LLM() llm.Client
}

// Registry manages plugins
//
// The Registry is responsible for registering, storing, and retrieving plugins.
//
// Example usage:
//
//     registry := plugins.NewRegistry()
//     
//     // Register a plugin
//     err := registry.Register(myPlugin)
//     if err != nil {
//         log.Fatal(err)
//     }
//     
//     // Get a plugin by name
//     plugin := registry.Get("my-plugin")
//     
//     // Get all plugins of a type
//     parserPlugins := registry.GetByType(plugins.PluginTypeParser)
type Registry struct {
	mu      sync.RWMutex      // Mutex for thread safety
	plugins map[string]Plugin  // Map of plugin names to plugins
}

// NewRegistry creates a new plugin registry
//
// Returns:
// - *Registry: New plugin registry instance
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register registers a plugin
//
// Parameters:
// - plugin: Plugin to register
//
// Returns:
// - error: Error if registration fails (e.g., duplicate name)
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
//
// Parameters:
// - name: Name of the plugin
//
// Returns:
// - Plugin: Plugin instance or nil if not found
func (r *Registry) Get(name string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.plugins[name]
}

// GetByType returns all plugins of a specific type
//
// Parameters:
// - pluginType: Type of plugins to return
//
// Returns:
// - []Plugin: Slice of plugins of the specified type
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

	if p == nil {
		return fmt.Errorf("plugin.Open returned nil plugin")
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
	pluginInstance := constructor()
	if pluginInstance == nil {
		return fmt.Errorf("NewPlugin returned nil plugin")
	}

	// Register the plugin
	return r.Register(pluginInstance)
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

// LoadPluginsFromDirectory loads all plugins from a directory
//
// Parameters:
// - directory: Directory to load plugins from
//
// Returns:
// - int: Number of successfully loaded plugins
// - int: Number of failed plugins
// - error: Error if directory access fails
func (r *Registry) LoadPluginsFromDirectory(directory string) (int, int, error) {
	// Check if directory exists
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return 0, 0, fmt.Errorf("plugin directory does not exist: %s", directory)
	}

	successCount := 0
	errorCount := 0

	// Walk through the directory
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only load .so files
		if filepath.Ext(path) == ".so" {
			// Try to load the plugin
			if err := r.Load(path); err != nil {
				errorCount++
				fmt.Printf("Failed to load plugin %s: %v\n", path, err)
			} else {
				successCount++
				fmt.Printf("Successfully loaded plugin %s\n", path)
			}
		}

		return nil
	})

	if err != nil {
		return successCount, errorCount, fmt.Errorf("failed to walk plugin directory: %w", err)
	}

	return successCount, errorCount, nil
}
