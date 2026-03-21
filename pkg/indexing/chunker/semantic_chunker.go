package chunker

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/google/uuid"
)

var _ core.SemanticChunker = (*SemanticChunker)(nil)

// SemanticChunker implements core.SemanticChunker for advanced RAG techniques.
// It wraps a base chunker (like TokenChunker or CharacterChunker) and adds
// hierarchical and contextual capabilities.
type SemanticChunker struct {
	BaseChunker     core.Chunker
	ParentChunkSize int
	ChildChunkSize  int
	Overlap         int
}

// DefaultSemanticChunker returns a SemanticChunker with a default TokenChunker as its base.
// It uses standard hierarchical sizes: Parent(1000), Child(250), Overlap(50).
func DefaultSemanticChunker() (*SemanticChunker, error) {
	base, err := DefaultTokenChunker()
	if err != nil {
		return nil, err
	}
	return NewSemanticChunker(base, 1000, 250, 50), nil
}

// NewSemanticChunker creates a new SemanticChunker wrapping a base text chunker.
func NewSemanticChunker(base core.Chunker, parentSize, childSize, overlap int) *SemanticChunker {
	return &SemanticChunker{
		BaseChunker:     base,
		ParentChunkSize: parentSize,
		ChildChunkSize:  childSize,
		Overlap:         overlap,
	}
}

// Chunk delegates to the base chunker.
func (s *SemanticChunker) Chunk(ctx context.Context, doc *core.Document) ([]*core.Chunk, error) {
	if s.BaseChunker == nil {
		return nil, fmt.Errorf("base chunker is not set")
	}
	return s.BaseChunker.Chunk(ctx, doc)
}

// HierarchicalChunk creates a two-level hierarchy of chunks.
// Parents are larger chunks (e.g., paragraphs), Children are smaller sub-chunks (e.g., sentences).
func (s *SemanticChunker) HierarchicalChunk(ctx context.Context, doc *core.Document) ([]*core.Chunk, []*core.Chunk, error) {
	// 1. Create Parent Chunker (fallback to Character if base not specified or token not needed for parent)
	parentSplitter := NewCharacterChunker(s.ParentChunkSize, s.Overlap)
	parentTexts, err := parentSplitter.chunkText(doc.Content)
	if err != nil {
		return nil, nil, err
	}

	var parents []*core.Chunk
	var children []*core.Chunk

	childSplitter := NewCharacterChunker(s.ChildChunkSize, s.Overlap/2)

	for _, pText := range parentTexts {
		parentID := uuid.New().String()

		parentMeta := make(map[string]any)
		for k, v := range doc.Metadata {
			parentMeta[k] = v
		}

		parentChunk := core.NewChunk(parentID, doc.ID, strings.TrimSpace(pText), 0, 0, parentMeta)
		parentChunk.Level = 1 // 1: Parent
		parents = append(parents, parentChunk)

		// 2. Split Parent into Children
		childTexts, err := childSplitter.chunkText(pText)
		if err != nil {
			return nil, nil, err
		}

		for _, cText := range childTexts {
			childMeta := make(map[string]any)
			for k, v := range parentMeta {
				childMeta[k] = v
			}

			childChunk := core.NewChunk(uuid.New().String(), doc.ID, strings.TrimSpace(cText), 0, 0, childMeta)
			childChunk.ParentID = parentID
			childChunk.Level = 2 // 2: Child
			children = append(children, childChunk)
		}
	}

	return parents, children, nil
}

// ContextualChunk injects a document-level summary into each chunk.
func (s *SemanticChunker) ContextualChunk(ctx context.Context, doc *core.Document, docSummary string) ([]*core.Chunk, error) {
	chunks, err := s.Chunk(ctx, doc)
	if err != nil {
		return nil, err
	}

	for _, chunk := range chunks {
		// Prepend summary context to the content
		chunk.Content = fmt.Sprintf("Document Context: %s\n\nChunk Content: %s", docSummary, chunk.Content)
		// Also store it in metadata
		if chunk.Metadata == nil {
			chunk.Metadata = make(map[string]any)
		}
		chunk.Metadata["document_summary"] = docSummary
	}

	return chunks, nil
}
