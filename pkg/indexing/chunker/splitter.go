package chunker

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/core"
)

// TextSplitter is the generalized LlamaIndex "NodeParser" / Langchain "TextSplitter".
type TextSplitter interface {
	// SplitText turns a raw string into meaningful chunk strings
	SplitText(text string) ([]string, error)
	
	// SplitDocument extends raw string logic with ID mapping and metadata (Chunk = Node in LlamaIndex)
	SplitDocument(ctx context.Context, doc *core.Document) ([]*core.Chunk, error)
}
