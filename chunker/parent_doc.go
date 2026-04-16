package chunker

import (
	"strings"

	"github.com/DotNetAge/gorag/core"
)

// ParentDocChunker implements two-level chunking
// Finds parent chunks (large) that contain child chunks (small)
// Provides both precision and contextual richness
type ParentDocChunker struct {
	parentChunker core.Chunker // parent chunker
	childChunker  core.Chunker // child chunker
	parentSize    int          // parent chunk size
	childSize     int          // child chunk size
	options       Options      // optional configuration
}

// NewParentDocChunker creates a new ParentDocChunker
func NewParentDocChunker(opts ...Option) *ParentDocChunker {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	// Create parent chunker (default: RecursiveChunker)
	parentChunker := NewRecursiveChunker(
		WithChunkSize(options.ParentSize),
		WithMinChunkSize(options.ParentSize/2),
	)

	// Create child chunker (default: SentenceChunker)
	childChunker := NewSentenceChunker(
		WithMaxSentences(5),
	)

	return &ParentDocChunker{
		parentChunker: parentChunker,
		childChunker:  childChunker,
		parentSize:    options.ParentSize,
		childSize:     options.ChildSize,
		options:       options,
	}
}

// Chunk implements the Chunker interface
// Returns all chunks (parent + child), with child chunks linked to parents via ParentID
func (c *ParentDocChunker) Chunk(
	structured *core.StructuredDocument,
	entities []*core.Entity,
) ([]*core.Chunk, error) {
	if structured == nil || structured.RawDoc == nil {
		return []*core.Chunk{}, nil
	}

	text := structured.RawDoc.GetContent()
	if text == "" {
		return []*core.Chunk{}, nil
	}

	// 1. Generate parent chunks
	parents, err := c.parentChunker.Chunk(structured, entities)
	if err != nil {
		return nil, err
	}

	// 2. Generate child chunks
	children, err := c.childChunker.Chunk(structured, entities)
	if err != nil {
		return nil, err
	}

	// Filter out image chunks from parents and children (they will be added at the end)
	parents = filterOutImageChunks(parents)
	children = filterOutImageChunks(children)

	// 3. Establish parent-child relationships
	c.establishParentChildRelationships(parents, children)

	// 4. Mark parent chunks
	for _, parent := range parents {
		if parent.Metadata == nil {
			parent.Metadata = make(map[string]any)
		}
		parent.Metadata["is_parent"] = true
	}

	// 5. Mark child chunks (ensure all children have is_parent = false)
	for _, child := range children {
		if child.Metadata == nil {
			child.Metadata = make(map[string]any)
		}
		if _, ok := child.Metadata["is_parent"]; !ok {
			child.Metadata["is_parent"] = false
		}
	}

	// 5. Combine all chunks (parents first, then children)
	allChunks := make([]*core.Chunk, 0, len(parents)+len(children))
	allChunks = append(allChunks, parents...)
	allChunks = append(allChunks, children...)

	// 6. Append image chunks as sub-chunks
	if imgChunks := ExtractImageChunks(structured); len(imgChunks) > 0 {
		allChunks = append(allChunks, imgChunks...)
	}

	// 7. Re-index chunks
	for i, chunk := range allChunks {
		chunk.ChunkMeta.Index = i
	}

	return allChunks, nil
}

// GetStrategy returns the chunk strategy type
func (c *ParentDocChunker) GetStrategy() core.ChunkStrategy {
	return StrategyParentDoc
}

// establishParentChildRelationships establishes parent-child relationships
// Finds the smallest parent that contains each child based on position
func (c *ParentDocChunker) establishParentChildRelationships(
	parents []*core.Chunk,
	children []*core.Chunk,
) {
	for _, child := range children {
		if child.Metadata == nil {
			child.Metadata = make(map[string]any)
		}
		// Find parent containing this child
		parent := c.findParentForChild(child, parents)
		if parent != nil {
			child.ParentID = parent.ID
			child.Metadata["parent_id"] = parent.ID
			child.Metadata["is_parent"] = false
		}
	}
}

// findParentForChild finds the parent chunk for a child chunk
// Position matching: child's StartPos and EndPos must be within parent's range
func (c *ParentDocChunker) findParentForChild(
	child *core.Chunk,
	parents []*core.Chunk,
) *core.Chunk {
	var bestParent *core.Chunk
	minSize := -1

	for _, parent := range parents {
		// Check if child is within parent bounds
		if child.ChunkMeta.StartPos >= parent.ChunkMeta.StartPos &&
			child.ChunkMeta.EndPos <= parent.ChunkMeta.EndPos {

			// Choose smallest parent (avoid overly large parents)
			parentSize := parent.ChunkMeta.EndPos - parent.ChunkMeta.StartPos
			if minSize == -1 || parentSize < minSize {
				bestParent = parent
				minSize = parentSize
			}
		}
	}

	return bestParent
}

// filterOutImageChunks filters out image chunks from the given chunks
func filterOutImageChunks(chunks []*core.Chunk) []*core.Chunk {
	var result []*core.Chunk
	for _, chunk := range chunks {
		if !strings.HasPrefix(chunk.MIMEType, "image") {
			result = append(result, chunk)
		}
	}
	return result
}
