package repository

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
)

// entityRepository implements Repository with automatic index synchronization.
// It supports storing any Entity type in different collections.
type entityRepository struct {
	docStore  core.DocStore
	vecStore  core.VectorStore
	embedder  embedding.Provider
	chunker   core.SemanticChunker
}

// NewRepository creates a generic Repository with index synchronization.
func NewRepository(
	docStore core.DocStore,
	vecStore core.VectorStore,
	embedder embedding.Provider,
	chunker core.SemanticChunker,
) core.Repository {
	return &entityRepository{
		docStore: docStore,
		vecStore: vecStore,
		embedder: embedder,
		chunker:  chunker,
	}
}

// Create stores an entity and indexes the content.
// The content is chunked and each chunk is vectorized.
// Chunks are linked to the entity via entity.GetID().
func (r *entityRepository) Create(ctx context.Context, collection string, entity core.Entity, content string) error {
	if content == "" {
		return nil // Nothing to index
	}

	entityID := entity.GetID()
	chunks := r.chunkContent(ctx, content, collection, entityID)
	
	if len(chunks) == 0 {
		return nil
	}

	// 1. Store chunks in DocStore
	if r.docStore != nil {
		if err := r.docStore.SetChunks(ctx, chunks); err != nil {
			return fmt.Errorf("failed to store chunks: %w", err)
		}
	}

	// 2. Generate and store vectors
	if r.vecStore != nil && r.embedder != nil {
		texts := make([]string, len(chunks))
		for i, chunk := range chunks {
			texts[i] = chunk.Content
		}

		embeddings, err := r.embedder.Embed(ctx, texts)
		if err != nil {
			return fmt.Errorf("failed to embed chunks: %w", err)
		}

		vectors := make([]*core.Vector, len(chunks))
		for i, chunk := range chunks {
			vectors[i] = &core.Vector{
				ID:     chunk.ID,
				Values: embeddings[i],
				Metadata: map[string]any{
					"collection": collection,
					"entity_id":  entityID,
				},
			}
		}

		if err := r.vecStore.Upsert(ctx, vectors); err != nil {
			return fmt.Errorf("failed to store vectors: %w", err)
		}
	}

	return nil
}

// chunkContent splits content into chunks linked to the entity.
func (r *entityRepository) chunkContent(ctx context.Context, content, collection, entityID string) []*core.Chunk {
	var chunks []*core.Chunk

	if r.chunker != nil {
		// Create a temporary document for chunking
		doc := &core.Document{
			ID:      entityID,
			Content: content,
		}
		
		// Use semantic chunker
		semanticChunks, err := r.chunker.Chunk(ctx, doc)
		if err != nil || len(semanticChunks) == 0 {
			// Fallback to single chunk on error
			return []*core.Chunk{{
				ID:         fmt.Sprintf("%s_%s_chunk_0", collection, entityID),
				DocumentID: entityID,
				Content:    content,
				Metadata: map[string]any{
					"collection": collection,
					"entity_id":  entityID,
				},
			}}
		}
		
		for i, c := range semanticChunks {
			chunks = append(chunks, &core.Chunk{
				ID:         fmt.Sprintf("%s_%s_chunk_%d", collection, entityID, i),
				DocumentID: entityID,
				Content:    c.Content,
				StartIndex: c.StartIndex,
				EndIndex:   c.EndIndex,
				Metadata: map[string]any{
					"collection": collection,
					"entity_id":  entityID,
				},
			})
		}
	} else {
		// Default: single chunk if no chunker configured
		chunks = []*core.Chunk{{
			ID:         fmt.Sprintf("%s_%s_chunk_0", collection, entityID),
			DocumentID: entityID,
			Content:    content,
			Metadata: map[string]any{
				"collection": collection,
				"entity_id":  entityID,
			},
		}}
	}

	return chunks
}

// Read retrieves an entity by ID from the specified collection.
// Note: This requires storage backend implementation.
func (r *entityRepository) Read(ctx context.Context, collection string, id string) (core.Entity, error) {
	return nil, fmt.Errorf("Read requires storage backend implementation")
}

// Update modifies an entity and re-indexes the content.
func (r *entityRepository) Update(ctx context.Context, collection string, entity core.Entity, content string) error {
	// Delete old chunks and vectors first
	if err := r.Delete(ctx, collection, entity.GetID()); err != nil {
		return err
	}
	// Create new chunks and vectors
	return r.Create(ctx, collection, entity, content)
}

// Delete removes an entity and all its chunks and vectors.
func (r *entityRepository) Delete(ctx context.Context, collection string, id string) error {
	// 1. Get chunks to delete their vectors
	if r.docStore != nil {
		chunks, err := r.docStore.GetChunksByDocID(ctx, id)
		if err == nil && len(chunks) > 0 {
			// Delete vectors
			if r.vecStore != nil {
				for _, chunk := range chunks {
					r.vecStore.Delete(ctx, chunk.ID)
				}
			}
			// Note: DocStore should cascade delete chunks when document is deleted
		}
	}

	// 2. Delete from DocStore (cascades to chunks)
	if r.docStore != nil {
		r.docStore.DeleteDocument(ctx, id)
	}

	return nil
}

// List retrieves entities matching the filter.
func (r *entityRepository) List(ctx context.Context, collection string, filter map[string]any) ([]core.Entity, error) {
	return nil, fmt.Errorf("List requires storage backend implementation")
}

// ============================================================================
// Typed Repository Wrapper
// ============================================================================

// typedRepository is a type-safe wrapper for Repository.
type typedRepository[T core.Entity] struct {
	repo       core.Repository
	collection string
}

// NewTypedRepository creates a type-safe repository for a specific entity type.
func NewTypedRepository[T core.Entity](repo core.Repository, collection string) core.TypedRepository[T] {
	return &typedRepository[T]{
		repo:       repo,
		collection: collection,
	}
}

func (r *typedRepository[T]) Create(ctx context.Context, collection string, entity T, content string) error {
	return r.repo.Create(ctx, collection, entity, content)
}

func (r *typedRepository[T]) Read(ctx context.Context, collection string, id string) (T, error) {
	entity, err := r.repo.Read(ctx, collection, id)
	if err != nil {
		var zero T
		return zero, err
	}
	return entity.(T), nil
}

func (r *typedRepository[T]) Update(ctx context.Context, collection string, entity T, content string) error {
	return r.repo.Update(ctx, collection, entity, content)
}

func (r *typedRepository[T]) Delete(ctx context.Context, collection string, id string) error {
	return r.repo.Delete(ctx, collection, id)
}

func (r *typedRepository[T]) List(ctx context.Context, collection string, filter map[string]any) ([]T, error) {
	entities, err := r.repo.List(ctx, collection, filter)
	if err != nil {
		return nil, err
	}

	result := make([]T, len(entities))
	for i, e := range entities {
		result[i] = e.(T)
	}
	return result, nil
}
