package vectorstore

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/infra/vectorstore/govector"
	"github.com/DotNetAge/gorag/infra/vectorstore/memory"
	"github.com/DotNetAge/gorag/infra/vectorstore/milvus"
	"github.com/DotNetAge/gorag/infra/vectorstore/pinecone"
	"github.com/DotNetAge/gorag/infra/vectorstore/qdrant"
	"github.com/DotNetAge/gorag/infra/vectorstore/weaviate"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)

// StoreType 向量存储类型
type StoreType string

const (
	// MemoryStore 内存存储（用于测试）
	MemoryStore StoreType = "memory"
	// GoVectorStore goRAG 内置的向量存储
	GoVectorStore StoreType = "govector"
	// QdrantStore Qdrant 向量存储
	QdrantStore StoreType = "qdrant"
	// MilvusStore Milvus 向量存储
	MilvusStore StoreType = "milvus"
	// PineconeStore Pinecone 向量存储
	PineconeStore StoreType = "pinecone"
	// WeaviateStore Weaviate 向量存储
	WeaviateStore StoreType = "weaviate"
)

// DefaultVectorStore 创建默认的向量存储（使用 govector）
// dbPath: 数据库文件路径，默认为 "./data/vectorstore/govector"
func DefaultVectorStore(dbPath string) (abstraction.VectorStore, error) {
	if dbPath == "" {
		dbPath = "./data/vectorstore/govector"
	}

	ctx := context.Background()
	return govector.NewStore(ctx,
		govector.WithDBPath(dbPath),
		govector.WithCollection("gorag"),
	)
}

// NewMemoryStore 创建内存向量存储（用于测试和原型开发）
func NewMemoryStore() abstraction.VectorStore {
	return memory.NewStore()
}

// NewQdrantStore 创建 Qdrant 向量存储
// addr: Qdrant 服务器地址，例如 "localhost:6334"
// apiKey: API 密钥（可选）
// collection: 集合名称，默认为 "gorag"
func NewQdrantStore(addr string, apiKey string, collection string) (abstraction.VectorStore, error) {
	ctx := context.Background()

	opts := []qdrant.Option{}
	if collection != "" {
		opts = append(opts, qdrant.WithCollection(collection))
	}

	return qdrant.NewStore(ctx, addr, opts...)
}

// NewMilvusStore 创建 Milvus 向量存储
// addr: Milvus 服务器地址，例如 "localhost:19530"
// username: 用户名（可选）
// password: 密码（可选）
// collection: 集合名称，默认为 "gorag"
func NewMilvusStore(addr string, username string, password string, collection string) (abstraction.VectorStore, error) {
	ctx := context.Background()

	opts := []milvus.Option{}
	if collection != "" {
		opts = append(opts, milvus.WithCollection(collection))
	}

	return milvus.NewStore(ctx, addr, opts...)
}

// NewPineconeStore 创建 Pinecone 向量存储
// apiKey: Pinecone API 密钥
// environment: Pinecone 环境，例如 "gcp-starter"
// indexName: 索引名称，默认为 "gorag"
func NewPineconeStore(apiKey string, environment string, indexName string) (abstraction.VectorStore, error) {
	opts := []pinecone.Option{}
	if indexName != "" {
		opts = append(opts, pinecone.WithIndex(indexName))
	}
	if environment != "" {
		opts = append(opts, pinecone.WithEnvironment(environment))
	}

	return pinecone.NewStore(apiKey, opts...)
}

// NewWeaviateStore 创建 Weaviate 向量存储
// addr: Weaviate 服务器地址，例如 "localhost:8080"
// scheme: 协议 (http/https)，默认为 "http"
// apiKey: API 密钥（可选）
// collection: 集合名称，默认为 "GoRAG"
func NewWeaviateStore(addr string, scheme string, apiKey string, collection string) (abstraction.VectorStore, error) {
	opts := []weaviate.Option{}
	if collection != "" {
		opts = append(opts, weaviate.WithCollection(collection))
	}

	// 构建完整的地址
	fullAddr := addr
	if scheme != "" {
		fullAddr = fmt.Sprintf("%s://%s", scheme, addr)
	}

	return weaviate.NewStore(fullAddr, apiKey, opts...)
}

// NewStore 根据类型创建向量存储（工厂方法）
// storeType: 存储类型
// config: 配置参数
func NewStore(storeType StoreType, config map[string]interface{}) (abstraction.VectorStore, error) {
	switch storeType {
	case MemoryStore:
		return NewMemoryStore(), nil
	case GoVectorStore:
		dbPath, _ := config["db_path"].(string)
		return DefaultVectorStore(dbPath)
	case QdrantStore:
		addr, _ := config["addr"].(string)
		apiKey, _ := config["api_key"].(string)
		collection, _ := config["collection"].(string)
		return NewQdrantStore(addr, apiKey, collection)
	case MilvusStore:
		addr, _ := config["addr"].(string)
		username, _ := config["username"].(string)
		password, _ := config["password"].(string)
		collection, _ := config["collection"].(string)
		return NewMilvusStore(addr, username, password, collection)
	case PineconeStore:
		apiKey, _ := config["api_key"].(string)
		environment, _ := config["environment"].(string)
		indexName, _ := config["index_name"].(string)
		return NewPineconeStore(apiKey, environment, indexName)
	case WeaviateStore:
		addr, _ := config["addr"].(string)
		scheme, _ := config["scheme"].(string)
		apiKey, _ := config["api_key"].(string)
		collection, _ := config["collection"].(string)
		return NewWeaviateStore(addr, scheme, apiKey, collection)
	default:
		return nil, fmt.Errorf("unsupported store type: %s", storeType)
	}
}
