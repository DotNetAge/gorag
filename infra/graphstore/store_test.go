package graphstore

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/stretchr/testify/assert"
)

// TestGraphStoreFactory 测试图存储工厂函数
func TestGraphStoreFactory(t *testing.T) {
	tests := []struct {
		name     string
		storeType StoreType
		expected bool
	}{
		{"Neo4J", StoreTypeNeo4j, true},
		{"NebulaGraph", StoreTypeNebulaGraph, true},
		{"TinkerGraph", StoreTypeTinkerGraph, true},
		{"Dgraph", StoreTypeDgraph, true},
		{"ArangoDB", StoreTypeArangoDB, true},
		{"Unknown", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewGraphStore(tt.storeType)
			if tt.expected {
				assert.NoError(t, err)
				assert.NotNil(t, store)
			} else {
				assert.Error(t, err)
				assert.Nil(t, store)
			}
		})
	}
}

// TestGraphStoreFromConfig 测试从配置创建图存储
func TestGraphStoreFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"type": "neo4j",
		"options": map[string]interface{}{
			"uri":      "bolt://localhost:7687",
			"username": "neo4j",
			"password": "password",
		},
	}

	// 这里只是测试配置解析，不会实际连接数据库
	store, err := NewGraphStoreFromConfig(config)
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

// TestGraphStoreBasicOperations 测试图存储基本操作
func TestGraphStoreBasicOperations(t *testing.T) {
	// 这里我们只测试接口方法的存在性，不进行实际的数据库操作
	// 实际的集成测试需要在有数据库的环境中运行
	store, err := NewGraphStore(StoreTypeNeo4j)
	assert.NoError(t, err)
	assert.NotNil(t, store)

	// 测试接口方法是否存在
	ctx := context.Background()

	// 测试 CreateNode
	node := &abstraction.Node{
		ID:   "test-node",
		Type: "Person",
		Properties: map[string]any{
			"name": "Test Person",
			"age":  30,
		},
	}
	err = store.CreateNode(ctx, node)
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 CreateEdge
	edge := &abstraction.Edge{
		ID:     "test-edge",
		Type:   "KNOWS",
		Source: "test-node",
		Target: "test-node-2",
		Properties: map[string]any{
			"since": 2020,
		},
	}
	err = store.CreateEdge(ctx, edge)
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 GetNode
	_, err = store.GetNode(ctx, "test-node")
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 GetEdge
	_, err = store.GetEdge(ctx, "test-edge")
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 DeleteNode
	err = store.DeleteNode(ctx, "test-node")
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 DeleteEdge
	err = store.DeleteEdge(ctx, "test-edge")
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 Query
	_, err = store.Query(ctx, "MATCH (n) RETURN n", nil)
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 GetNeighbors
	_, err = store.GetNeighbors(ctx, "test-node", 10)
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 GetCommunitySummaries
	_, err = store.GetCommunitySummaries(ctx, 10)
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 UpsertNodes
	nodes := []*abstraction.Node{node}
	err = store.UpsertNodes(ctx, nodes)
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 UpsertEdges
	edges := []*abstraction.Edge{edge}
	err = store.UpsertEdges(ctx, edges)
	// 这里会失败，因为没有初始化连接，但我们只是测试方法存在性
	assert.Error(t, err)

	// 测试 Close
	err = store.Close(ctx)
	// 这里应该成功，因为 Close 方法通常是幂等的
	assert.NoError(t, err)
}
