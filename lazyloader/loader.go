// Package lazyloader provides lazy loading functionality for documents
//
// This package implements lazy loading of documents to optimize memory usage
// by loading content only when needed. It also provides memory management
// capabilities to control the total memory used by loaded documents.
//
// Example:
//
//     // Create a lazy document
//     doc := lazyloader.NewLazyDocument("path/to/document.pdf")
//     
//     // Load content on demand
//     content, err := doc.Load()
//     if err != nil {
//         log.Fatal(err)
//     }
//     
//     // Use content...
//     
//     // Unload to free memory
//     doc.Unload()
//
//     // Create a document manager with memory limit
//     manager := lazyloader.NewLazyDocumentManager(100 * 1024 * 1024) // 100MB
//     doc := manager.AddDocument("path/to/document.pdf")
//     content, err := manager.LoadDocument("path/to/document.pdf")
package lazyloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

// LazyDocument represents a document that is loaded on demand
//
// A LazyDocument loads its content only when needed, which helps optimize
// memory usage by avoiding loading large documents until they are actually required.
//
// Example:
//
//     // Create a lazy document from a file
//     doc := NewLazyDocument("path/to/document.pdf")
//     
//     // Check if loaded
//     fmt.Println("Is loaded:", doc.IsLoaded()) // false
//     
//     // Load content on demand
//     content, err := doc.Load()
//     if err != nil {
//         log.Fatal(err)
//     }
//     
//     // Check size
//     size, err := doc.Size()
//     if err != nil {
//         log.Fatal(err)
//     }
//     fmt.Printf("Document size: %d bytes\n", size)
//     
//     // Unload to free memory
//     doc.Unload()
//     fmt.Println("Is loaded:", doc.IsLoaded()) // false
type LazyDocument struct {
	path    string
	content []byte
	loaded  bool
	size    int64
	mu      sync.RWMutex
	loader  func() ([]byte, error)
}

// NewLazyDocument creates a new lazy document
//
// Parameters:
// - path: Path to the document file
//
// Returns:
// - *LazyDocument: New lazy document instance
func NewLazyDocument(path string) *LazyDocument {
	return &LazyDocument{
		path:   path,
		loader: defaultLoader(path),
	}
}

// NewLazyDocumentWithLoader creates a new lazy document with a custom loader
//
// Parameters:
// - path: Path to the document
// - loader: Custom loader function
//
// Returns:
// - *LazyDocument: New lazy document instance
func NewLazyDocumentWithLoader(path string, loader func() ([]byte, error)) *LazyDocument {
	return &LazyDocument{
		path:   path,
		loader: loader,
	}
}

// defaultLoader creates a default file loader
func defaultLoader(path string) func() ([]byte, error) {
	return func() ([]byte, error) {
		return os.ReadFile(path)
	}
}

// Path returns the document path
//
// Returns:
// - string: Document path
func (d *LazyDocument) Path() string {
	return d.path
}

// Size returns the document size (requires loading metadata)
//
// Returns:
// - int64: Document size in bytes
// - error: Error if metadata loading fails
func (d *LazyDocument) Size() (int64, error) {
	d.mu.RLock()
	if d.size > 0 {
		size := d.size
		d.mu.RUnlock()
		return size, nil
	}
	d.mu.RUnlock()

	// Get file info
	info, err := os.Stat(d.path)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	d.mu.Lock()
	d.size = info.Size()
	d.mu.Unlock()

	return d.size, nil
}

// IsLoaded returns whether the document is loaded
//
// Returns:
// - bool: True if the document is loaded
func (d *LazyDocument) IsLoaded() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.loaded
}

// Load loads the document content
//
// This method loads the document content if it's not already loaded.
// It uses double-checked locking to ensure thread safety.
//
// Returns:
// - []byte: Document content
// - error: Error if loading fails
func (d *LazyDocument) Load() ([]byte, error) {
	d.mu.RLock()
	if d.loaded {
		content := d.content
		d.mu.RUnlock()
		return content, nil
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Double-check after acquiring write lock
	if d.loaded {
		return d.content, nil
	}

	content, err := d.loader()
	if err != nil {
		return nil, fmt.Errorf("failed to load document: %w", err)
	}

	d.content = content
	d.loaded = true
	d.size = int64(len(content))

	return content, nil
}

// LoadWithContext loads the document with context cancellation
//
// This method loads the document content with support for context cancellation.
//
// Parameters:
// - ctx: Context for cancellation
//
// Returns:
// - []byte: Document content
// - error: Error if loading fails or context is cancelled
func (d *LazyDocument) LoadWithContext(ctx context.Context) ([]byte, error) {
	d.mu.RLock()
	if d.loaded {
		content := d.content
		d.mu.RUnlock()
		return content, nil
	}
	d.mu.RUnlock()

	type result struct {
		content []byte
		err     error
	}

	done := make(chan result, 1)
	go func() {
		content, err := d.Load()
		done <- result{content: content, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-done:
		return res.content, res.err
	}
}

// Unload unloads the document content to free memory
//
// This method unloads the document content and resets the loaded flag,
// freeing up memory occupied by the document.
func (d *LazyDocument) Unload() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.content = nil
	d.loaded = false
}

// Reader returns an io.Reader for the document
//
// This method returns an io.Reader for the document content,
// loading the content if it's not already loaded.
//
// Returns:
// - io.Reader: Reader for the document content
// - error: Error if loading fails
func (d *LazyDocument) Reader() (io.Reader, error) {
	content, err := d.Load()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(content), nil
}

// LazyDocumentManager manages multiple lazy documents
//
// The LazyDocumentManager manages a collection of lazy documents and
// tracks memory usage to ensure it stays within the specified limit.
// It automatically evicts documents when memory usage exceeds the limit.
//
// Example:
//
//     // Create a document manager with 100MB memory limit
//     manager := NewLazyDocumentManager(100 * 1024 * 1024)
//     
//     // Add documents
//     doc1 := manager.AddDocument("doc1.pdf")
//     doc2 := manager.AddDocument("doc2.pdf")
//     
//     // Load documents
//     content1, err := manager.LoadDocument("doc1.pdf")
//     if err != nil {
//         log.Fatal(err)
//     }
//     
//     // Check memory usage
//     fmt.Printf("Memory usage: %d bytes\n", manager.GetMemoryUsage())
//     
//     // Unload all documents
//     manager.UnloadAll()
//     fmt.Printf("Memory usage after unload: %d bytes\n", manager.GetMemoryUsage())
type LazyDocumentManager struct {
	documents   map[string]*LazyDocument
	mu          sync.RWMutex
	maxSize     int64
	currentSize int64
}

// NewLazyDocumentManager creates a new lazy document manager
//
// Parameters:
// - maxSize: Maximum memory usage in bytes
//
// Returns:
// - *LazyDocumentManager: New lazy document manager instance
func NewLazyDocumentManager(maxSize int64) *LazyDocumentManager {
	return &LazyDocumentManager{
		documents: make(map[string]*LazyDocument),
		maxSize:   maxSize,
	}
}

// AddDocument adds a document to the manager
//
// This method adds a document to the manager. If the document already exists,
// it returns the existing document.
//
// Parameters:
// - path: Path to the document file
//
// Returns:
// - *LazyDocument: Lazy document instance
func (m *LazyDocumentManager) AddDocument(path string) *LazyDocument {
	m.mu.Lock()
	defer m.mu.Unlock()

	if doc, exists := m.documents[path]; exists {
		return doc
	}

	doc := NewLazyDocument(path)
	m.documents[path] = doc
	return doc
}

// GetDocument gets a document from the manager
//
// Parameters:
// - path: Path to the document file
//
// Returns:
// - *LazyDocument: Lazy document instance
// - bool: True if the document exists
func (m *LazyDocumentManager) GetDocument(path string) (*LazyDocument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, exists := m.documents[path]
	return doc, exists
}

// RemoveDocument removes a document from the manager
//
// This method removes a document from the manager and updates the memory usage
// if the document was loaded.
//
// Parameters:
// - path: Path to the document file
func (m *LazyDocumentManager) RemoveDocument(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if doc, exists := m.documents[path]; exists {
		if doc.IsLoaded() {
			size, _ := doc.Size()
			m.currentSize -= size
		}
		delete(m.documents, path)
	}
}

// LoadDocument loads a document and tracks memory usage
//
// This method loads a document and tracks memory usage. If loading the document
// would exceed the memory limit, it evicts existing documents to make room.
//
// Parameters:
// - path: Path to the document file
//
// Returns:
// - []byte: Document content
// - error: Error if loading fails or memory limit is exceeded
func (m *LazyDocumentManager) LoadDocument(path string) ([]byte, error) {
	doc, exists := m.GetDocument(path)
	if !exists {
		return nil, fmt.Errorf("document not found: %s", path)
	}

	// Check memory limit
	size, err := doc.Size()
	if err != nil {
		return nil, err
	}

	if m.currentSize+size > m.maxSize {
		// Evict least recently used documents
		if err := m.evict(size); err != nil {
			return nil, err
		}
	}

	content, err := doc.Load()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.currentSize += size
	m.mu.Unlock()

	return content, nil
}

// evict removes documents to make room for new content
//
// This method evicts loaded documents to make room for new content.
// It uses a simple eviction strategy: unload all loaded documents.
//
// Parameters:
// - needed: Amount of memory needed in bytes
//
// Returns:
// - error: Error if insufficient memory can be freed
func (m *LazyDocumentManager) evict(needed int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simple eviction: unload all documents
	// In a real implementation, you'd use LRU or LFU
	for _, doc := range m.documents {
		if doc.IsLoaded() {
			size, _ := doc.Size()
			doc.Unload()
			m.currentSize -= size
			if m.currentSize+needed <= m.maxSize {
				return nil
			}
		}
	}

	if m.currentSize+needed > m.maxSize {
		return fmt.Errorf("cannot evict enough memory: need %d, have %d", needed, m.maxSize-m.currentSize)
	}

	return nil
}

// GetMemoryUsage returns the current memory usage
//
// Returns:
// - int64: Current memory usage in bytes
func (m *LazyDocumentManager) GetMemoryUsage() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentSize
}

// UnloadAll unloads all documents
//
// This method unloads all documents managed by the manager,
// freeing up all memory used by loaded documents.
func (m *LazyDocumentManager) UnloadAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, doc := range m.documents {
		doc.Unload()
	}
	m.currentSize = 0
}
