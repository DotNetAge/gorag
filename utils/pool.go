package utils

import (
	"context"
	"sync"
	"time"
)

// Pool defines the interface for connection pooling
type Pool interface {
	// Get retrieves a connection from the pool
	Get(ctx context.Context) (interface{}, error)
	// Put returns a connection to the pool
	Put(conn interface{})
	// Close closes all connections in the pool
	Close() error
}

// ConnectionPool implements a generic connection pool
type ConnectionPool struct {
	createConn   func() (interface{}, error)
	validateConn func(interface{}) bool
	closeConn    func(interface{}) error
	maxIdle      int
	maxActive    int
	idleTimeout  time.Duration

	idleConns   chan interface{}
	activeConns int
	mu          sync.Mutex
}

// PoolOptions configures the connection pool
type PoolOptions struct {
	CreateConn   func() (interface{}, error)
	ValidateConn func(interface{}) bool
	CloseConn    func(interface{}) error
	MaxIdle      int
	MaxActive    int
	IdleTimeout  time.Duration
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(opts PoolOptions) *ConnectionPool {
	if opts.MaxIdle <= 0 {
		opts.MaxIdle = 10
	}
	if opts.MaxActive <= 0 {
		opts.MaxActive = 50
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 30 * time.Minute
	}

	pool := &ConnectionPool{
		createConn:   opts.CreateConn,
		validateConn: opts.ValidateConn,
		closeConn:    opts.CloseConn,
		maxIdle:      opts.MaxIdle,
		maxActive:    opts.MaxActive,
		idleTimeout:  opts.IdleTimeout,
		idleConns:    make(chan interface{}, opts.MaxIdle),
	}

	// Start a goroutine to clean up idle connections
	go pool.cleanupIdleConns()

	return pool
}

// Get retrieves a connection from the pool
func (p *ConnectionPool) Get(ctx context.Context) (interface{}, error) {
	for {
		// Try to get an idle connection
		select {
		case conn := <-p.idleConns:
			// Validate the connection
			if p.validateConn != nil && !p.validateConn(conn) {
				// Connection is invalid, close it and try again
				if p.closeConn != nil {
					_ = p.closeConn(conn)
				}
				p.mu.Lock()
				p.activeConns--
				p.mu.Unlock()
				continue
			}
			return conn, nil
		default:
			// No idle connections, create a new one if under max active
			p.mu.Lock()
			if p.activeConns >= p.maxActive {
				// Max active connections reached, wait for an idle connection
				p.mu.Unlock()
				select {
				case conn := <-p.idleConns:
					// Validate the connection
					if p.validateConn != nil && !p.validateConn(conn) {
						// Connection is invalid, close it and try again
						if p.closeConn != nil {
							_ = p.closeConn(conn)
						}
						p.mu.Lock()
						p.activeConns--
						p.mu.Unlock()
						continue
					}
					return conn, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			p.activeConns++
			p.mu.Unlock()

			// Create a new connection
			conn, err := p.createConn()
			if err != nil {
				p.mu.Lock()
				p.activeConns--
				p.mu.Unlock()
				return nil, err
			}
			return conn, nil
		}
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn interface{}) {
	if conn == nil {
		p.mu.Lock()
		p.activeConns--
		p.mu.Unlock()
		return
	}

	// Validate the connection before returning to pool
	if p.validateConn != nil && !p.validateConn(conn) {
		if p.closeConn != nil {
			_ = p.closeConn(conn)
		}
		p.mu.Lock()
		p.activeConns--
		p.mu.Unlock()
		return
	}

	// Return to idle pool if not full
	select {
	case p.idleConns <- conn:
		// Connection returned to pool
	default:
		// Idle pool is full, close the connection
		if p.closeConn != nil {
			_ = p.closeConn(conn)
		}
		p.mu.Lock()
		p.activeConns--
		p.mu.Unlock()
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	close(p.idleConns)

	for conn := range p.idleConns {
		if p.closeConn != nil {
			if err := p.closeConn(conn); err != nil {
				return err
			}
		}
	}

	return nil
}

// cleanupIdleConns cleans up idle connections that have exceeded the idle timeout
func (p *ConnectionPool) cleanupIdleConns() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		p.cleanupIdleConnsOnce()
	}
}

// cleanupIdleConnsOnce cleans up idle connections once
func (p *ConnectionPool) cleanupIdleConnsOnce() {
	// Create a temporary channel to hold valid idle connections
	tempConns := make(chan interface{}, p.maxIdle)

	// Process all idle connections
	for {
		select {
		case conn := <-p.idleConns:
			// Check if connection is still valid
			if p.validateConn != nil && !p.validateConn(conn) {
				// Connection is invalid, close it
				if p.closeConn != nil {
					_ = p.closeConn(conn)
				}
				p.mu.Lock()
				p.activeConns--
				p.mu.Unlock()
				continue
			}

			// Connection is valid, keep it
			select {
			case tempConns <- conn:
				// Connection moved to temporary pool
			default:
				// Temporary pool is full, close the connection
				if p.closeConn != nil {
					_ = p.closeConn(conn)
				}
				p.mu.Lock()
				p.activeConns--
				p.mu.Unlock()
			}
		default:
			// No more idle connections
			break
		}
	}

	// Replace the idle connections channel with the temporary one
	close(p.idleConns)
	p.idleConns = tempConns
}
