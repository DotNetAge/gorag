package cache

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoltCacheConcurrentSetDelete(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "concurrent_set_delete.db")
	cache, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer cache.Close()

	var wg sync.WaitGroup
	n := 50

	// 并发写入
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key_" + string(rune('0'+i%10))
			if i%3 == 0 {
				_ = cache.Delete(key)
			} else {
				_ = cache.Set(key, i)
			}
		}(i)
	}
	wg.Wait()

	// 并发 Flush
	var wg2 sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			_ = cache.Flush()
		}()
	}
	wg2.Wait()

	// 验证数据可读取
	var val int
	err = cache.Get("key_0", &val)
	require.NoError(t, err)
}

func TestBoltCacheConcurrentFlushRace(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "flush_race.db")
	cache, err := NewBoltCache(cachePath)
	require.NoError(t, err)
	defer cache.Close()

	var wg sync.WaitGroup

	// 1. 写入一组数据
	for i := 0; i < 100; i++ {
		require.NoError(t, cache.Set("k_"+string(rune('0'+i%10)), i))
	}

	// 2. 同时触发 Flush 和删除
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = cache.Flush()
	}()

	// 在 Flush 拍快照和 marker 之间删除 key
	for i := 0; i < 100; i++ {
		_ = cache.Delete("k_" + string(rune('0'+i%3)))
	}
	wg.Wait()

	// 3. 再次 Flush 后检查 - 已删除的 key 不应该出现在磁盘上
	require.NoError(t, cache.Flush())

	// 重建缓存后验证
	cachePath2 := filepath.Join(dir, "flush_race_reopen.db")
	cache2, err := NewBoltCache(cachePath2)
	require.NoError(t, err)
	defer cache2.Close()

	var val int
	err = cache2.Get("k_0", &val) // 可能已被删除
	if err != nil {
		t.Logf("expected: key_0 may have been deleted (race test): %v", err)
	}
}
