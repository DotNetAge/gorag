package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConnectionPool(t *testing.T) {
	// Create a simple connection pool
	pool := NewConnectionPool(PoolOptions{
		CreateConn: func() (interface{}, error) {
			return "connection", nil
		},
		ValidateConn: func(conn interface{}) bool {
			return true
		},
		CloseConn: func(conn interface{}) error {
			return nil
		},
		MaxIdle:     5,
		MaxActive:   10,
		IdleTimeout: 30 * time.Second,
	})

	assert.NotNil(t, pool)
}

func TestConnectionPool_Get(t *testing.T) {
	// Create a connection pool
	pool := NewConnectionPool(PoolOptions{
		CreateConn: func() (interface{}, error) {
			return "connection", nil
		},
		ValidateConn: func(conn interface{}) bool {
			return true
		},
		CloseConn: func(conn interface{}) error {
			return nil
		},
		MaxIdle:     5,
		MaxActive:   10,
		IdleTimeout: 30 * time.Second,
	})

	// Get a connection
	conn, err := pool.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "connection", conn)

	// Put the connection back
	pool.Put(conn)

	// Get the connection again (should be from the pool)
	conn2, err := pool.Get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "connection", conn2)
}

func TestConnectionPool_Put_Nil(t *testing.T) {
	// Create a connection pool
	pool := NewConnectionPool(PoolOptions{
		CreateConn: func() (interface{}, error) {
			return "connection", nil
		},
		ValidateConn: func(conn interface{}) bool {
			return true
		},
		CloseConn: func(conn interface{}) error {
			return nil
		},
		MaxIdle:     5,
		MaxActive:   10,
		IdleTimeout: 30 * time.Second,
	})

	// Put a nil connection (should not panic)
	pool.Put(nil)
}

func TestConnectionPool_Close(t *testing.T) {
	// Create a connection pool
	pool := NewConnectionPool(PoolOptions{
		CreateConn: func() (interface{}, error) {
			return "connection", nil
		},
		ValidateConn: func(conn interface{}) bool {
			return true
		},
		CloseConn: func(conn interface{}) error {
			return nil
		},
		MaxIdle:     5,
		MaxActive:   10,
		IdleTimeout: 30 * time.Second,
	})

	// Get and put a connection
	conn, err := pool.Get(context.Background())
	require.NoError(t, err)
	pool.Put(conn)

	// Close the pool
	err = pool.Close()
	require.NoError(t, err)
}
