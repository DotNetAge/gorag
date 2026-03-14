package graphstore

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/neo4j"
)

// TestNeo4jGraphStoreIntegration 测试 Neo4J 图存储集成
func TestNeo4jGraphStoreIntegration(t *testing.T) {
	// 启动 Neo4J 容器
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 创建 Neo4J 容器
	neo4jContainer, err := neo4j.RunContainer(ctx, testcontainers.WithImage("neo4j:5.17.0"))
	assert.NoError(t, err)
	defer func() {
		if err := neo4jContainer.Terminate(ctx); err != nil {
			t.Fatalf("Failed to terminate container: %s", err)
		}
	}()

	// 获取 Neo4J 连接信息
	boltURI, err := neo4jContainer.BoltUrl(ctx)
	assert.NoError(t, err)

	// 初始化 Neo4J GraphStore
	store := NewNeo4jGraphStore()
	assert.NotNil(t, store)

	// 初始化存储
	err = store.Initialize(map[string]interface{}{
		"uri":      boltURI,
		"username": "neo4j",
		"password": "password",
	})
	assert.NoError(t, err)

	// 测试创建节点
	node := &abstraction.Node{
		ID:   "test-node-1",
		Type: "Person",
		Properties: map[string]any{
			"name": "Test Person",
			"age":  30,
		},
	}
	err = store.CreateNode(ctx, node)
	assert.NoError(t, err)

	// 测试获取节点
	retrievedNode, err := store.GetNode(ctx, "test-node-1")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedNode)
	assert.Equal(t, "test-node-1", retrievedNode.ID)
	assert.Equal(t, "Person", retrievedNode.Type)
	assert.Equal(t, "Test Person", retrievedNode.Properties["name"])
	assert.Equal(t, 30, retrievedNode.Properties["age"])

	// 测试创建第二个节点
	node2 := &abstraction.Node{
		ID:   "test-node-2",
		Type: "Person",
		Properties: map[string]any{
			"name": "Test Person 2",
			"age":  25,
		},
	}
	err = store.CreateNode(ctx, node2)
	assert.NoError(t, err)

	// 测试创建边
	edge := &abstraction.Edge{
		ID:     "test-edge-1",
		Type:   "KNOWS",
		Source: "test-node-1",
		Target: "test-node-2",
		Properties: map[string]any{
			"since": 2020,
		},
	}
	err = store.CreateEdge(ctx, edge)
	assert.NoError(t, err)

	// 测试获取邻居
	neighbors, err := store.GetNeighbors(ctx, "test-node-1", 10)
	assert.NoError(t, err)
	assert.Len(t, neighbors, 1)
	assert.Equal(t, "test-node-2", neighbors[0].ID)

	// 测试批量操作
	nodes := []*abstraction.Node{
		{
			ID:   "test-node-3",
			Type: "Person",
			Properties: map[string]any{
				"name": "Test Person 3",
				"age":  35,
			},
		},
	}
	err = store.UpsertNodes(ctx, nodes)
	assert.NoError(t, err)

	// 测试删除节点
	err = store.DeleteNode(ctx, "test-node-3")
	assert.NoError(t, err)

	// 测试删除边
	err = store.DeleteEdge(ctx, "test-edge-1")
	assert.NoError(t, err)

	// 测试关闭连接
	err = store.Close(ctx)
	assert.NoError(t, err)
}
