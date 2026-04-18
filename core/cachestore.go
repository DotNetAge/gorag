package core

// CacheStore 通用持久化缓存接口
// 提供基于 key-value 的缓存读写能力，value 为任意 JSON 可序列化数据
// 实现可以是 bbolt、Redis、BadgerDB 等任意持久化存储
type CacheStore interface {
	// Get 根据 key 获取缓存值，反序列化到 value
	// key 不存在时返回 nil, nil
	Get(key string, value any) error

	// Set 写入缓存，value 会被 JSON 序列化
	Set(key string, value any) error

	// Delete 删除指定 key
	Delete(key string) error

	// Len 返回缓存条目数量
	Len() int

	// Flush 强制将内存中的脏数据刷写到磁盘
	Flush() error

	// Close 关闭缓存，释放资源
	Close() error
}
