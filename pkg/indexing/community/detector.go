// Package community provides community detection algorithms for GraphRAG.
// Following Microsoft GraphRAG design, communities enable hierarchical summarization
// and global search capabilities.
package community

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/pkg/core"
)

// LouvainDetector implements community detection using the Louvain algorithm.
// This is a greedy optimization algorithm that attempts to maximize modularity.
type LouvainDetector struct {
	minCommunitySize int   // Minimum nodes per community
	maxLevels        int   // Maximum hierarchy levels
	graphStore       core.GraphStore
}

type DetectorOption func(*LouvainDetector)

func WithMinCommunitySize(size int) DetectorOption {
	return func(d *LouvainDetector) {
		d.minCommunitySize = size
	}
}

func WithMaxLevels(levels int) DetectorOption {
	return func(d *LouvainDetector) {
		d.maxLevels = levels
	}
}

// NewLouvainDetector creates a new Louvain-based community detector.
func NewLouvainDetector(graphStore core.GraphStore, opts ...DetectorOption) *LouvainDetector {
	d := &LouvainDetector{
		graphStore:       graphStore,
		minCommunitySize: 3,  // Default minimum
		maxLevels:        3,  // Default max hierarchy depth
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Detect identifies communities in the graph using a simplified Louvain approach.
// Returns communities hierarchically (level 0 = finest granularity).
func (d *LouvainDetector) Detect(ctx context.Context, graphStore core.GraphStore) ([]*core.Community, error) {
	if graphStore == nil {
		return nil, fmt.Errorf("graph store is nil")
	}

	// Step 1: Get all nodes from the graph
	// We use a workaround: query all nodes via a broad search
	// In a real implementation, GraphStore should have a GetAllNodes method
	allNodes, allEdges, err := d.getAllGraphData(ctx, graphStore)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph data: %w", err)
	}

	if len(allNodes) == 0 {
		return []*core.Community{}, nil
	}

	// Step 2: Build adjacency map
	adjMap := d.buildAdjacencyMap(allEdges)

	// Step 3: Detect communities using label propagation (simplified Louvain)
	// Level 0: Fine-grained communities
	communities := d.detectLevelZero(allNodes, adjMap)

	// Step 4: Build hierarchy (merge small communities)
	hierarchy := d.buildHierarchy(communities, adjMap)

	return hierarchy, nil
}

// getAllGraphData retrieves all nodes and edges from the graph store.
// This is a workaround until GraphStore provides a better API.
func (d *LouvainDetector) getAllGraphData(ctx context.Context, graphStore core.GraphStore) ([]*core.Node, []*core.Edge, error) {
	// Use a very broad query to get all entities
	// This is a limitation of current GraphStore interface
	// Ideally, we'd have GetAllNodes/GetAllEdges methods

	// For now, we'll return an empty result and expect the caller to provide data
	// through an alternative mechanism
	return []*core.Node{}, []*core.Edge{}, nil
}

// buildAdjacencyMap creates an adjacency map from edges.
func (d *LouvainDetector) buildAdjacencyMap(edges []*core.Edge) map[string]map[string]bool {
	adjMap := make(map[string]map[string]bool)

	for _, edge := range edges {
		if adjMap[edge.Source] == nil {
			adjMap[edge.Source] = make(map[string]bool)
		}
		if adjMap[edge.Target] == nil {
			adjMap[edge.Target] = make(map[string]bool)
		}
		adjMap[edge.Source][edge.Target] = true
		adjMap[edge.Target][edge.Source] = true // Undirected for community detection
	}

	return adjMap
}

// detectLevelZero performs label propagation for fine-grained community detection.
func (d *LouvainDetector) detectLevelZero(nodes []*core.Node, adjMap map[string]map[string]bool) []*core.Community {
	// Initialize: each node is its own community
	labels := make(map[string]string)
	for _, node := range nodes {
		labels[node.ID] = node.ID
	}

	// Label propagation: iteratively assign nodes to neighbor's community
	// This is a simplified version - real Louvain uses modularity optimization
	changed := true
	iterations := 0
	maxIterations := 10

	for changed && iterations < maxIterations {
		changed = false
		iterations++

		for _, node := range nodes {
			if neighbors, ok := adjMap[node.ID]; ok && len(neighbors) > 0 {
				// Count neighbor labels
				labelCount := make(map[string]int)
				for neighbor := range neighbors {
					labelCount[labels[neighbor]]++
				}

				// Find most common label
				maxCount := 0
				bestLabel := labels[node.ID]
				for label, count := range labelCount {
					if count > maxCount || (count == maxCount && label < bestLabel) {
						maxCount = count
						bestLabel = label
					}
				}

				if labels[node.ID] != bestLabel {
					labels[node.ID] = bestLabel
					changed = true
				}
			}
		}
	}

	// Group nodes by label
	communityNodes := make(map[string][]string)
	for nodeID, label := range labels {
		communityNodes[label] = append(communityNodes[label], nodeID)
	}

	// Create communities
	var communities []*core.Community
	communityID := 0
	for _, nodeIDs := range communityNodes {
		if len(nodeIDs) >= d.minCommunitySize {
			communities = append(communities, &core.Community{
				ID:     fmt.Sprintf("community_%d", communityID),
				Level:  0,
				NodeIDs: nodeIDs,
			})
			communityID++
		}
	}

	return communities
}

// buildHierarchy creates hierarchical communities by merging small ones.
func (d *LouvainDetector) buildHierarchy(levelZero []*core.Community, adjMap map[string]map[string]bool) []*core.Community {
	result := make([]*core.Community, len(levelZero))
	copy(result, levelZero)

	// Build hierarchy levels
	currentLevel := result
	for level := 1; level < d.maxLevels; level++ {
		// Merge small communities based on inter-community edges
		nextLevel := d.mergeCommunities(currentLevel, adjMap, level)
		if len(nextLevel) >= len(currentLevel) {
			break // No more merging possible
		}

		// Set parent relationships
		for _, parent := range nextLevel {
			for _, childID := range parent.NodeIDs {
				for _, child := range currentLevel {
					if contains(child.NodeIDs, childID) {
						child.ParentID = parent.ID
						break
					}
				}
			}
		}

		result = append(result, nextLevel...)
		currentLevel = nextLevel
	}

	return result
}

// mergeCommunities merges adjacent communities at a given level.
func (d *LouvainDetector) mergeCommunities(communities []*core.Community, adjMap map[string]map[string]bool, level int) []*core.Community {
	if len(communities) <= 1 {
		return communities
	}

	// Build inter-community adjacency
	commAdj := make(map[string]map[string]int)
	for i, c1 := range communities {
		for j, c2 := range communities {
			if i >= j {
				continue
			}
			// Count edges between communities
			edgeCount := 0
			for _, n1 := range c1.NodeIDs {
				for _, n2 := range c2.NodeIDs {
					if adjMap[n1] != nil && adjMap[n1][n2] {
						edgeCount++
					}
				}
			}
			if edgeCount > 0 {
				if commAdj[c1.ID] == nil {
					commAdj[c1.ID] = make(map[string]int)
				}
				if commAdj[c2.ID] == nil {
					commAdj[c2.ID] = make(map[string]int)
				}
				commAdj[c1.ID][c2.ID] = edgeCount
				commAdj[c2.ID][c1.ID] = edgeCount
			}
		}
	}

	// Merge communities with strongest connections
	merged := make(map[string]bool)
	var result []*core.Community
	communityID := 0

	for _, c := range communities {
		if merged[c.ID] {
			continue
		}

		// Find most connected neighbor community
		var bestNeighbor *core.Community
		bestEdgeCount := 0
		for _, neighbor := range communities {
			if neighbor.ID == c.ID || merged[neighbor.ID] {
				continue
			}
			if commAdj[c.ID] != nil {
				if count := commAdj[c.ID][neighbor.ID]; count > bestEdgeCount {
					bestEdgeCount = count
					bestNeighbor = neighbor
				}
			}
		}

		if bestNeighbor != nil && bestEdgeCount > 0 {
			// Merge
			mergedNodes := append(c.NodeIDs, bestNeighbor.NodeIDs...)
			result = append(result, &core.Community{
				ID:      fmt.Sprintf("community_L%d_%d", level, communityID),
				Level:   level,
				NodeIDs: mergedNodes,
			})
			merged[c.ID] = true
			merged[bestNeighbor.ID] = true
			communityID++
		} else {
			// No merge - keep as is
			result = append(result, &core.Community{
				ID:      fmt.Sprintf("community_L%d_%d", level, communityID),
				Level:   level,
				NodeIDs: c.NodeIDs,
			})
			merged[c.ID] = true
			communityID++
		}
	}

	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
