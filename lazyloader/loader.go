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
type LazyDocument struct {
	path    string
	content []byte
	loaded  bool
	size    int64
	mu      sync.RWMutex
	loader  func() ([]byte, error)
}

// NewLazyDocument creates a new lazy document
func NewLazyDocument(path string) *LazyDocument {
	return &LazyDocument{
		path:   path,
		loader: defaultLoader(path),
	}
}

// NewLazyDocumentWithLoader creates a new lazy document with a custom loader
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
func (d *LazyDocument) Path() string {
	return d.path
}

// Size returns the document size (requires loading metadata)
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
func (d *LazyDocument) IsLoaded() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.loaded
}

// Load loads the document content
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
func (d *LazyDocument) Unload() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.content = nil
	d.loaded = false
}

// Reader returns an io.Reader for the document
func (d *LazyDocument) Reader() (io.Reader, error) {
	content, err := d.Load()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(content), nil
}

// LazyDocumentManager manages multiple lazy documents
type LazyDocumentManager struct {
	documents   map[string]*LazyDocument
	mu          sync.RWMutex
	maxSize     int64
	currentSize int64
}

// NewLazyDocumentManager creates a new lazy document manager
func NewLazyDocumentManager(maxSize int64) *LazyDocumentManager {
	return &LazyDocumentManager{
		documents: make(map[string]*LazyDocument),
		maxSize:   maxSize,
	}
}

// AddDocument adds a document to the manager
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
func (m *LazyDocumentManager) GetDocument(path string) (*LazyDocument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, exists := m.documents[path]
	return doc, exists
}

// RemoveDocument removes a document from the manager
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
func (m *LazyDocumentManager) GetMemoryUsage() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentSize
}

// UnloadAll unloads all documents
func (m *LazyDocumentManager) UnloadAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, doc := range m.documents {
		doc.Unload()
	}
	m.currentSize = 0
}
