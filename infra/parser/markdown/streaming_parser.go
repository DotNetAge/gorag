package markdown

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

var _ dataprep.Parser = (*MarkdownStreamParser)(nil)

// MarkdownStreamParser reads a markdown file and yields documents split roughly by top-level headers.
type MarkdownStreamParser struct {
	splitOnHeaderLevel int // e.g. 1 means split on "# ", 2 means split on "## "
}

func NewMarkdownStreamParser(splitLevel int) *MarkdownStreamParser {
	if splitLevel <= 0 || splitLevel > 6 {
		splitLevel = 1 // Default to H1
	}
	return &MarkdownStreamParser{
		splitOnHeaderLevel: splitLevel,
	}
}

func (p *MarkdownStreamParser) GetSupportedTypes() []string {
	return []string{".md", ".markdown", "text/markdown"}
}

func (p *MarkdownStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "MarkdownStreamParser"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	headerPrefix := strings.Repeat("#", p.splitOnHeaderLevel) + " "

	go func() {
		defer close(outChan)

		scanner := bufio.NewScanner(r)
		var sb strings.Builder
		partIndex := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			// If we hit a new section, and we already have data, yield it.
			if strings.HasPrefix(line, headerPrefix) && sb.Len() > 0 {
				docMetaCopy := copyMeta(docMeta)
				docMetaCopy["part_index"] = partIndex
				
				doc := entity.NewDocument(
					uuid.New().String(),
					sb.String(),
					source,
					"text/markdown",
					docMetaCopy,
				)

				select {
				case <-ctx.Done():
					return
				case outChan <- doc:
					sb.Reset()
					partIndex++
				}
			}

			sb.WriteString(line)
			sb.WriteString("\n")
		}

		if sb.Len() > 0 {
			docMetaCopy := copyMeta(docMeta)
			docMetaCopy["part_index"] = partIndex
			doc := entity.NewDocument(
				uuid.New().String(),
				sb.String(),
				source,
				"text/markdown",
				docMetaCopy,
			)
			select {
			case <-ctx.Done():
				return
			case outChan <- doc:
			}
		}
	}()

	return outChan, nil
}

func copyMeta(m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		out[k] = v
	}
	return out
}
