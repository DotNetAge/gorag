package cache

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/DotNetAge/gorag/core"
	bolt "go.etcd.io/bbolt"
)

const defaultBucket = "cache"

// BoltCache 基于 bbolt 的持久化缓存实现
// 实现 core.CacheStore 接口
// 采用内存 + 磁盘双层存储：读取走内存，写入同步到内存并异步刷盘
// 异步刷盘操作通过单 goroutine 串行化，保证磁盘写入顺序与内存操作一致
type BoltCache struct {
	db      *bolt.DB
	mem     map[string][]byte
	bucket  []byte
	mu      sync.RWMutex
	dirty   bool
	opCh    chan diskOp // 序列化磁盘操作的通道
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// diskOp 表示一个待执行的磁盘操作
type diskOp struct {
	fn   func()
	done chan struct{} // 用于 Flush 同步等待
}

// NewBoltCache 创建或打开基于 bbolt 的持久化缓存
// cachePath: 缓存数据库文件路径，如 "/path/to/cache.db"
// bucketName: bbolt bucket 名称，默认为 "cache"
func NewBoltCache(cachePath string, bucketName ...string) (*BoltCache, error) {
	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := bolt.Open(cachePath, 0600, nil)
	if err != nil {
		return nil, err
	}

	bucket := []byte(defaultBucket)
	if len(bucketName) > 0 && bucketName[0] != "" {
		bucket = []byte(bucketName[0])
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}

	c := &BoltCache{
		db:      db,
		mem:     make(map[string][]byte),
		bucket:  bucket,
		opCh:    make(chan diskOp, 256), // 带缓冲避免阻塞调用方
		closeCh: make(chan struct{}),
	}

	if err := c.loadAll(); err != nil {
		db.Close()
		return nil, err
	}

	// 启动单 goroutine 串行化所有磁盘写入
	c.wg.Add(1)
	go c.diskLoop()

	return c, nil
}

// diskLoop 单 goroutine 串行执行磁盘操作，保证写入顺序与内存操作一致
func (c *BoltCache) diskLoop() {
	defer c.wg.Done()
	for {
		select {
		case op, ok := <-c.opCh:
			if !ok {
				return // channel 已关闭
			}
			op.fn()
			if op.done != nil {
				op.done <- struct{}{}
			}
			c.mu.Lock()
			c.dirty = false
			c.mu.Unlock()
		case <-c.closeCh:
			return
		}
	}
}

// submitOp 提交磁盘操作
func (c *BoltCache) submitOp(op diskOp) {
	select {
	case c.opCh <- op:
	default:
		// 通道已满时同步执行，避免死锁
		op.fn()
		if op.done != nil {
			op.done <- struct{}{}
		}
		c.mu.Lock()
		c.dirty = false
		c.mu.Unlock()
	}
}

func (c *BoltCache) loadAll() error {
	return c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.bucket)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			buf := make([]byte, len(v))
			copy(buf, v)
			c.mem[string(k)] = buf
			return nil
		})
	})
}

// Get 实现 core.CacheStore 接口
func (c *BoltCache) Get(key string, value any) error {
	c.mu.RLock()
	data, ok := c.mem[key]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	if err := json.Unmarshal(data, value); err != nil {
		c.mu.Lock()
		delete(c.mem, key)
		c.mu.Unlock()
		return err
	}

	return nil
}

// Set 实现 core.CacheStore 接口
func (c *BoltCache) Set(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.mem[key] = data
	c.dirty = true
	c.mu.Unlock()

	c.submitOp(diskOp{
		fn: func() {
			_ = c.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(c.bucket)
				return b.Put([]byte(key), data)
			})
		},
	})

	return nil
}

// Delete 实现 core.CacheStore 接口
func (c *BoltCache) Delete(key string) error {
	c.mu.Lock()
	delete(c.mem, key)
	c.dirty = true
	c.mu.Unlock()

	c.submitOp(diskOp{
		fn: func() {
			_ = c.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket(c.bucket)
				return b.Delete([]byte(key))
			})
		},
	})

	return nil
}

// Flush 实现 core.CacheStore 接口
// 同步等待所有挂起的异步操作完成，然后将当前内存快照全量刷盘
func (c *BoltCache) Flush() error {
	c.mu.RLock()
	snapshot := make(map[string][]byte, len(c.mem))
	maps.Copy(snapshot, c.mem)
	c.mu.RUnlock()

	// 先等待所有挂起的异步操作完成（提交一个 marker 操作）
	done := make(chan struct{}, 1)
	c.submitOp(diskOp{
		fn: func() {
			// marker: 标记前面的异步操作都已完成
		},
		done: done,
	})
	<-done // 等待 marker 执行完毕

	// 全量刷盘
	return c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.bucket)
		for k, v := range snapshot {
			if err := b.Put([]byte(k), v); err != nil {
				return err
			}
		}
		return nil
	})
}

// Len 实现 core.CacheStore 接口
func (c *BoltCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.mem)
}

// Close 实现 core.CacheStore 接口
func (c *BoltCache) Close() error {
	// Flush 会等待所有挂起的异步操作完成 + 全量刷盘
	_ = c.Flush()
	// 通知 diskLoop 退出并等待其完成，确保没有正在执行的 db 操作
	close(c.closeCh)
	c.wg.Wait()
	return c.db.Close()
}

// Ensure BoltCache implements core.CacheStore
var _ core.CacheStore = (*BoltCache)(nil)
