package graphstore

import (
	"context"
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
		return &neo4jPlaceholder{}, nil
	default:
		return nil, fmt.Errorf("unsupported graph store type: %s", storeType)
	}
}

// neo4jPlaceholder Neo4j 占位符实现（当 neo4j 构建标签未启用时）
type neo4jPlaceholder struct{}

func (n *neo4jPlaceholder) Initialize(options map[string]interface{}) error {
	return fmt.Errorf("neo4j support not enabled, please rebuild with -tags neo4j")
}

func (n *neo4jPlaceholder) CreateNode(ctx context.Context, node *abstraction.Node) error {
	return fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) CreateEdge(ctx context.Context, edge *abstraction.Edge) error {
	return fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) GetNode(ctx context.Context, id string) (*abstraction.Node, error) {
	return nil, fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) GetEdge(ctx context.Context, id string) (*abstraction.Edge, error) {
	return nil, fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) DeleteNode(ctx context.Context, id string) error {
	return fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) DeleteEdge(ctx context.Context, id string) error {
	return fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	return nil, fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) GetNeighbors(ctx context.Context, nodeID string, limit int) ([]*abstraction.Node, error) {
	return nil, fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) GetCommunitySummaries(ctx context.Context, limit int) ([]map[string]any, error) {
	return nil, fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) UpsertNodes(ctx context.Context, nodes []*abstraction.Node) error {
	return fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) UpsertEdges(ctx context.Context, edges []*abstraction.Edge) error {
	return fmt.Errorf("neo4j support not enabled")
}

func (n *neo4jPlaceholder) Close(ctx context.Context) error {
	return nil
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
