package pattern

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexer"
)

type basePattern struct {
	idx  indexer.Indexer
	ret  core.Retriever
	repo core.Repository
}

func (p *basePattern) Indexer() indexer.Indexer {
	return p.idx
}

func (p *basePattern) Retriever() core.Retriever {
	return p.ret
}

func (p *basePattern) Repository() core.Repository {
	return p.repo
}

func (p *basePattern) IndexFile(ctx context.Context, filePath string) error {
	_, err := p.idx.IndexFile(ctx, filePath)
	return err
}

func (p *basePattern) IndexDirectory(ctx context.Context, dirPath string, recursive bool) error {
	return p.idx.IndexDirectory(ctx, dirPath, recursive)
}

func (p *basePattern) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	return p.ret.Retrieve(ctx, queries, topK)
}

func (p *basePattern) IndexText(ctx context.Context, text string, metadata ...map[string]any) error {
	return p.idx.IndexText(ctx, text, metadata...)
}

func (p *basePattern) IndexTexts(ctx context.Context, texts []string, metadata ...map[string]any) error {
	return p.idx.IndexTexts(ctx, texts, metadata...)
}

func (p *basePattern) Delete(ctx context.Context, id string) error {
	return p.idx.DeleteDocument(ctx, id)
}
