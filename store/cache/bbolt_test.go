package cache

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltCache_BasicOperations(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "test_cache.db")

	c, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer c.Close()

	type testValue struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	// Test Set + Get
	err = c.Set("key1", testValue{Name: "hello", Count: 42})
	require.NoError(t, err)

	var got testValue
	err = c.Get("key1", &got)
	require.NoError(t, err)
	assert.Equal(t, "hello", got.Name)
	assert.Equal(t, 42, got.Count)

	// Test Len
	assert.Equal(t, 1, c.Len())

	// Test Get non-existent key
	var empty testValue
	err = c.Get("nonexistent", &empty)
	require.NoError(t, err)
	assert.Equal(t, "", empty.Name)

	// Test Delete
	err = c.Delete("key1")
	require.NoError(t, err)
	assert.Equal(t, 0, c.Len())

	err = c.Get("key1", &empty)
	require.NoError(t, err)
	assert.Equal(t, "", empty.Name)
}

func TestBoltCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "persist_cache.db")

	type value struct {
		Data string `json:"data"`
	}

	// 第一轮：写入并关闭
	c1, err := NewBoltCache(cachePath)
	require.NoError(t, err)

	require.NoError(t, c1.Set("persist_key", value{Data: "survives_restart"}))
	require.NoError(t, c1.Flush()) // 确保刷盘
	require.NoError(t, c1.Close())

	// 第二轮：重新打开，验证数据持久化
	c2, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer c2.Close()

	assert.Equal(t, 1, c2.Len())

	var got value
	err = c2.Get("persist_key", &got)
	require.NoError(t, err)
	assert.Equal(t, "survives_restart", got.Data)
}

func TestBoltCache_CustomBucket(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "bucket_cache.db")

	c, err := NewBoltCache(cachePath, "custom_bucket")
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.Set("k", "v"))

	var got string
	require.NoError(t, c.Get("k", &got))
	assert.Equal(t, "v", got)
}

func TestBoltCache_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "concurrent_cache.db")

	c, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer c.Close()

	const goroutines = 50
	var wg sync.WaitGroup

	// 并发写入
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = c.Set("key", i)
		}(i)
	}
	wg.Wait()

	// 并发读取
	for range goroutines {
		wg.Go(func() {
			var v int
			_ = c.Get("key", &v)
		})
	}
	wg.Wait()

	// 并发删除 + 写入
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_ = c.Delete("key")
			} else {
				_ = c.Set("key", i)
			}
		}(i)
	}
	wg.Wait()

	// 最终应存在（因为有写入操作）
	assert.Equal(t, 1, c.Len())
}

func TestBoltCache_Flush(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "flush_cache.db")

	c, err := NewBoltCache(cachePath)
	require.NoError(t, err)

	// 批量写入
	for i := range 100 {
		require.NoError(t, c.Set("key_"+string(rune('0'+i%10)), i))
	}

	require.NoError(t, c.Flush())
	assert.Equal(t, 10, c.Len()) // 10 个唯一 key
	require.NoError(t, c.Close())

	// 重新打开验证
	c2, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer c2.Close()
	assert.Equal(t, 10, c2.Len())
}

func TestBoltCache_NilValue(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "nil_cache.db")

	c, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer c.Close()

	// 写入 nil 值
	require.NoError(t, c.Set("nil_key", nil))

	var got any
	err = c.Get("nil_key", &got)
	require.NoError(t, err)
	assert.Equal(t, 1, c.Len())
}

func TestBoltCache_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "empty_key_cache.db")

	c, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.Set("", "empty"))
	assert.Equal(t, 1, c.Len())

	var got string
	require.NoError(t, c.Get("", &got))
	assert.Equal(t, "empty", got)
}

func TestBoltCache_DirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "subdir1", "subdir2", "cache.db")

	c, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.Set("deep", "value"))
	assert.FileExists(t, cachePath)
}
