package graphstore

import (
	"fmt"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
)



// GraphStore 图存储实现接口
type GraphStore interface {
	abstraction.GraphStore

	// Initialize 初始化存储
	Initialize(options map[string]interface{}) error
}

// StoreType 图存储类型
type StoreType string

const (
	// StoreTypeNeo4j Neo4J 图存储
	StoreTypeNeo4j StoreType = "neo4j"
	// StoreTypeNebulaGraph NebulaGraph 图存储
	StoreTypeNebulaGraph StoreType = "nebulagraph"
	// StoreTypeTinkerGraph TinkerGraph 图存储
	StoreTypeTinkerGraph StoreType = "tinkergraph"
	// StoreTypeDgraph Dgraph 图存储
	StoreTypeDgraph StoreType = "dgraph"
	// StoreTypeArangoDB ArangoDB 图存储
	StoreTypeArangoDB StoreType = "arangodb"
)

// NewGraphStore 创建新的图存储实例
func NewGraphStore(storeType StoreType) (GraphStore, error) {
	switch storeType {
	case StoreTypeNeo4j:
		return NewNeo4jGraphStore(), nil
	default:
		return nil, fmt.Errorf("unsupported graph store type: %s", storeType)
	}
}



// NewGraphStoreFromConfig 根据配置创建图存储实例
func NewGraphStoreFromConfig(config map[string]interface{}) (GraphStore, error) {
	storeTypeStr, ok := config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("store type is required")
	}

	storeType := StoreType(storeTypeStr)
	store, err := NewGraphStore(storeType)
	if err != nil {
		return nil, err
	}

	options, ok := config["options"].(map[string]interface{})
	if !ok {
		options = make(map[string]interface{})
	}

	if err := store.Initialize(options); err != nil {
		return nil, err
	}

	return store, nil
}
