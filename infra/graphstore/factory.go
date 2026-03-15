package graphstore

import (
	"fmt"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)

// 使用 store.go 中定义的 StoreTypeNeo4j 等常量
// 此处不再重复定义 StoreType 和常量，直接使用已有的

// DefaultGraphStore 创建默认的图存储（使用 Neo4j）
// uri: Neo4j 连接 URI，例如 "bolt://localhost:7687"
// username: 用户名
// password: 密码
func DefaultGraphStore(uri, username, password string) (abstraction.GraphStore, error) {
	if uri == "" {
		uri = "bolt://localhost:7687"
	}
	if username == "" {
		username = "neo4j"
	}

	store, err := NewGraphStore(StoreTypeNeo4j)
	if err != nil {
		return nil, fmt.Errorf("failed to create default graph store: %w", err)
	}

	err = store.Initialize(map[string]interface{}{
		"uri":      uri,
		"username": username,
		"password": password,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize default graph store: %w", err)
	}

	return store, nil
}

// NewNeo4JStore 创建 Neo4j 图存储
// uri: Neo4j 连接 URI，例如 "bolt://localhost:7687"
// username: 用户名
// password: 密码
func NewNeo4JStore(uri, username, password string) (abstraction.GraphStore, error) {
	store, err := NewGraphStore(StoreTypeNeo4j)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j store: %w", err)
	}

	err = store.Initialize(map[string]interface{}{
		"uri":      uri,
		"username": username,
		"password": password,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Neo4j store: %w", err)
	}

	return store, nil
}

// NewStore 根据类型创建图存储（工厂方法）
// storeType: 存储类型
// config: 配置参数
func NewStore(storeType StoreType, config map[string]interface{}) (abstraction.GraphStore, error) {
	switch storeType {
	case StoreTypeNeo4j:
		uri, _ := config["uri"].(string)
		username, _ := config["username"].(string)
		password, _ := config["password"].(string)
		return NewNeo4JStore(uri, username, password)
	default:
		return nil, fmt.Errorf("unsupported graph store type: %s", storeType)
	}
}
