package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/pattern"
)

func main() {
	fmt.Println("=== GoRAG QuickStart ===")

	ctx := context.Background()

	// 准备测试数据
	testFile := "sample.txt"
	content := `GoRAG 是一个支持多模态和图谱增强的生产级 RAG 框架。
它具有高度解耦的架构，通过 Pattern 提供简单易用的入口。
核心特性包括:
1. 支持 NativeRAG, AdvancedRAG 和 GraphRAG
2. 彻底分离了 VectorStore, DocStore 和 GraphStore
3. 类型安全的 Option 模式配置
欢迎使用 GoRAG 构建下一代 AI 应用！`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		log.Fatalf("创建测试文件失败: %v", err)
	}
	defer os.Remove(testFile)

	sep := strings.Repeat("=", 50)

	// ==========================================
	// NativeRAG 示例
	// ==========================================
	fmt.Printf("\n%s\n", sep)
	fmt.Println("NativeRAG 示例 - 标准向量检索")
	fmt.Printf("%s\n", sep)

	nativeRAG, err := pattern.NativeRAG("demo-native",
		pattern.WithBGE("bge-small-zh-v1.5"),
	)
	if err != nil {
		log.Fatalf("创建 NativeRAG 失败: %v", err)
	}
	fmt.Println("✓ NativeRAG 创建成功")

	// 文件索引
	if err := nativeRAG.IndexFile(ctx, testFile); err != nil {
		log.Fatalf("NativeRAG 索引失败: %v", err)
	}
	fmt.Println("✓ 文件索引完成")

	// 纯文字索引（新增功能）
	if err := nativeRAG.IndexText(ctx, "这是直接索引的纯文字内容，无需文件。"); err != nil {
		log.Fatalf("文字索引失败: %v", err)
	}
	fmt.Println("✓ 文字索引完成")

	// 批量文字索引
	texts := []string{"第一段文字", "第二段文字", "第三段文字"}
	if err := nativeRAG.IndexTexts(ctx, texts); err != nil {
		log.Fatalf("批量文字索引失败: %v", err)
	}
	fmt.Println("✓ 批量文字索引完成")

	// 检索
	results, err := nativeRAG.Retrieve(ctx, []string{"GoRAG 有哪些特性？"}, 2)
	if err != nil {
		log.Fatalf("NativeRAG 检索失败: %v", err)
	}
	printResults("NativeRAG", results)

	// ==========================================
	// NativeRAG + Fusion 示例 (需要 LLM)
	// ==========================================
	fmt.Printf("\n%s\n", sep)
	fmt.Println("NativeRAG + Fusion 示例 - 多查询融合检索")
	fmt.Printf("%s\n", sep)

	// Fusion 需要 LLM 进行查询分解
	// 实际使用时需要配置 LLM: pattern.WithLLM(yourLLMClient)
	fusionRAG, err := pattern.NativeRAG("demo-fusion",
		pattern.WithBGE("bge-small-zh-v1.5"),
		// pattern.WithLLM(yourLLMClient), // 需要配置 LLM
		// pattern.WithFusion(5),           // 启用多查询融合
	)
	if err != nil {
		log.Fatalf("创建 NativeRAG+Fusion 失败: %v", err)
	}
	fmt.Println("✓ NativeRAG+Fusion 创建成功")
	fmt.Println("  提示: 启用 Fusion 需要 LLM 和 WithFusion() 选项")

	if err := fusionRAG.IndexFile(ctx, testFile); err != nil {
		log.Fatalf("NativeRAG+Fusion 索引失败: %v", err)
	}
	fmt.Println("✓ NativeRAG+Fusion 索引完成")

	// ==========================================
	// GraphRAG 示例
	// ==========================================
	fmt.Printf("\n%s\n", sep)
	fmt.Println("GraphRAG 示例 - 知识图谱增强")
	fmt.Printf("%s\n", sep)

	graphRAG, err := pattern.GraphRAG("demo-graph",
		pattern.WithBGE("bge-small-zh-v1.5"),
	)
	if err != nil {
		log.Fatalf("创建 GraphRAG 失败: %v", err)
	}
	fmt.Println("✓ GraphRAG 创建成功 (内嵌 GoGraph)")

	if err := graphRAG.IndexFile(ctx, testFile); err != nil {
		log.Fatalf("GraphRAG 索引失败: %v", err)
	}
	fmt.Println("✓ GraphRAG 索引完成")

	// 图操作示例
	fmt.Println("\n--- 图操作示例 ---")

	// 添加节点
	node1 := &core.Node{
		ID:   "person-1",
		Type: "Person",
		Properties: map[string]any{
			"name": "张三",
			"age":  30,
		},
	}
	if err := graphRAG.AddNode(ctx, node1); err != nil {
		log.Fatalf("添加节点失败: %v", err)
	}
	fmt.Println("✓ 添加节点: Person(张三)")

	node2 := &core.Node{
		ID:   "person-2",
		Type: "Person",
		Properties: map[string]any{
			"name": "李四",
			"age":  25,
		},
	}
	if err := graphRAG.AddNode(ctx, node2); err != nil {
		log.Fatalf("添加节点失败: %v", err)
	}
	fmt.Println("✓ 添加节点: Person(李四)")

	// 添加边
	edge := &core.Edge{
		ID:     "edge-1",
		Type:   "KNOWS",
		Source: "person-1",
		Target: "person-2",
		Properties: map[string]any{
			"since": 2020,
		},
	}
	if err := graphRAG.AddEdge(ctx, edge); err != nil {
		log.Fatalf("添加边失败: %v", err)
	}
	fmt.Println("✓ 添加边: KNOWS(张三 -> 李四)")

	// 查询节点
	node, err := graphRAG.GetNode(ctx, "person-1")
	if err != nil {
		log.Fatalf("获取节点失败: %v", err)
	}
	fmt.Printf("✓ 获取节点: %v\n", node.Properties["name"])

	// 获取邻居
	neighbors, edges, err := graphRAG.GetNeighbors(ctx, "person-1", 1, 10)
	if err != nil {
		log.Fatalf("获取邻居失败: %v", err)
	}
	fmt.Printf("✓ 获取邻居: %d 个节点, %d 条边\n", len(neighbors), len(edges))

	// 删除边和节点
	if err := graphRAG.DeleteEdge(ctx, "edge-1"); err != nil {
		log.Fatalf("删除边失败: %v", err)
	}
	fmt.Println("✓ 删除边: edge-1")

	if err := graphRAG.DeleteNode(ctx, "person-2"); err != nil {
		log.Fatalf("删除节点失败: %v", err)
	}
	fmt.Println("✓ 删除节点: person-2")

	// ==========================================
	// 总结
	// ==========================================
	fmt.Printf("\n%s\n", sep)
	fmt.Println("RAG 模式对比")
	fmt.Printf("%s\n", sep)
	fmt.Println("NativeRAG:    标准向量检索，适合简单场景")
	fmt.Println("NativeRAG 可配置选项:")
	fmt.Println("  WithQueryRewrite() - 查询重写，适合歧义查询 (需要 LLM)")
	fmt.Println("  WithStepBack()     - 后退抽象，适合推理查询 (需要 LLM)")
	fmt.Println("  WithHyDE()         - 假设文档嵌入，适合模糊查询 (需要 LLM)")
	fmt.Println("  WithFusion(n)      - 多查询融合，适合复杂查询 (需要 LLM)")
	fmt.Println("")
	fmt.Println("GraphRAG:     知识图谱增强，适合复杂关系查询")

	fmt.Println("\n新增功能:")
	fmt.Println("  - IndexText: 直接索引纯文字")
	fmt.Println("  - IndexTexts: 批量索引文字")
	fmt.Println("  - IndexDocuments: 索引文档对象")
	fmt.Println("  - DeleteDocument: 删除文档及其向量")
	fmt.Println("  - GetDocument: 获取文档")
	fmt.Println("  - GraphRAG 支持: AddNode/DeleteNode/AddEdge/DeleteEdge")

	fmt.Println("\n=== QuickStart 完成 ===")
}

func printResults(name string, results []*core.RetrievalResult) {
	fmt.Printf("\n%s 检索结果:\n", name)
	for _, res := range results {
		fmt.Printf("  查询: %s\n", res.Query)
		for i, chunk := range res.Chunks {
			score := float32(0)
			if i < len(res.Scores) {
				score = res.Scores[i]
			}
			fmt.Printf("    [%d] %.4f - %s\n", i+1, score, truncate(chunk.Content, 50))
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
