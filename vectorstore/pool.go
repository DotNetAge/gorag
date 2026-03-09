package vectorstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/gorag/core"
)

// PooledStore wraps a Store with connection pooling
type PooledStore struct {
	store       Store
	maxConns    int
	idleConns   int
	idleTimeout time.Duration
	connChan    chan struct{}
	mu          sync.RWMutex
	activeCount int
	closed      bool
	wg          sync.WaitGroup
}

// PoolOptions configures the connection pool
type PoolOptions struct {
	MaxConns    int
	IdleConns   int
	IdleTimeout time.Duration
}

// DefaultPoolOptions returns default pool options
func DefaultPoolOptions() PoolOptions {
	return PoolOptions{
		MaxConns:    10,
		IdleConns:   5,
		IdleTimeout: 30 * time.Minute,
	}
}

// NewPooledStore creates a new pooled store wrapper
func NewPooledStore(store Store, opts PoolOptions) *PooledStore {
	if opts.MaxConns <= 0 {
		opts.MaxConns = 10
	}
	if opts.IdleConns <= 0 {
		opts.IdleConns = 5
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 30 * time.Minute
	}

	return &PooledStore{
		store:       store,
		maxConns:    opts.MaxConns,
		idleConns:   opts.IdleConns,
		idleTimeout: opts.IdleTimeout,
		connChan:    make(chan struct{}, opts.MaxConns),
	}
}

// acquire acquires a connection from the pool
func (p *PooledStore) acquire(ctx context.Context) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return fmt.Errorf("pool is closed")
	}
	p.mu.RUnlock()

	select {
	case p.connChan <- struct{}{}:
		p.mu.Lock()
		p.activeCount++
		p.wg.Add(1)
		p.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// release releases a connection back to the pool
func (p *PooledStore) release() {
	<-p.connChan
	p.mu.Lock()
	p.activeCount--
	p.wg.Done()
	p.mu.Unlock()
}

// Add adds chunks to the store with pooling
func (p *PooledStore) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	if err := p.acquire(ctx); err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer p.release()
	return p.store.Add(ctx, chunks, embeddings)
}

// Search searches the store with pooling
func (p *PooledStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]core.Result, error) {
	if err := p.acquire(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer p.release()
	return p.store.Search(ctx, query, opts)
}

// Delete deletes chunks from the store with pooling
func (p *PooledStore) Delete(ctx context.Context, ids []string) error {
	if err := p.acquire(ctx); err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer p.release()
	return p.store.Delete(ctx, ids)
}

// SearchByMetadata searches by metadata with pooling
func (p *PooledStore) SearchByMetadata(ctx context.Context, metadata map[string]string) ([]core.Chunk, error) {
	if err := p.acquire(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer p.release()
	return p.store.SearchByMetadata(ctx, metadata)
}

// Close closes the pooled store and waits for all active operations to complete
func (p *PooledStore) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Wait for all active operations to complete
	p.wg.Wait()

	// Now safe to close the channel
	close(p.connChan)
	return nil
}

// Stats returns pool statistics
func (p *PooledStore) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PoolStats{
		ActiveConnections: p.activeCount,
		MaxConnections:    p.maxConns,
		IdleConnections:   p.idleConns,
	}
}

// PoolStats represents pool statistics
type PoolStats struct {
	ActiveConnections int
	MaxConnections    int
	IdleConnections   int
}
