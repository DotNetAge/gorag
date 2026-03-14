package dbschema

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ dataprep.Parser = (*DBSchemaStreamParser)(nil)

// DBSchemaStreamParser implements a database schema parser for SQL DDL
type DBSchemaStreamParser struct {
	chunkSize      int
	chunkOverlap   int
	extractTables  bool
	extractColumns bool
	extractIndexes bool
}

// NewDBSchemaStreamParser creates a new database schema parser
func NewDBSchemaStreamParser() *DBSchemaStreamParser {
	return &DBSchemaStreamParser{
		chunkSize:      500,
		chunkOverlap:   50,
		extractTables:  true,
		extractColumns: true,
		extractIndexes: true,
	}
}

// GetSupportedTypes returns the supported file formats
func (p *DBSchemaStreamParser) GetSupportedTypes() []string {
	return []string{".sql"}
}

// ParseStream reads the incoming io.Reader and yields chunks of the document via a channel
func (p *DBSchemaStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "DBSchemaStreamParser"
	docMeta["type"] = "dbschema"

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

		var buffer strings.Builder
		var position int
		var inCreateTable bool
		var currentTable strings.Builder
		var tableName string
		var parenCount int

		createTablePattern := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
		createIndexPattern := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+ON\s+(\w+)`)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			trimmedLine := strings.TrimSpace(line)

			if !inCreateTable {
				if p.extractTables && createTablePattern.MatchString(trimmedLine) {
					matches := createTablePattern.FindStringSubmatch(trimmedLine)
					if len(matches) >= 2 {
						tableName = matches[1]
						inCreateTable = true
						parenCount = strings.Count(line, "(") - strings.Count(line, ")")
						currentTable.Reset()
						currentTable.WriteString("-- TABLE: " + tableName + "\n")
						currentTable.WriteString(line)
						currentTable.WriteString("\n")
						continue
					}
				}

				if p.extractIndexes && createIndexPattern.MatchString(trimmedLine) {
					matches := createIndexPattern.FindStringSubmatch(trimmedLine)
					if len(matches) >= 3 {
						indexName := matches[1]
						tableName := matches[2]
						docMetaCopy := copyMeta(docMeta)
						docMetaCopy["part_index"] = position
						docMetaCopy["position"] = position
						docMetaCopy["chunk_type"] = "index"
						docMetaCopy["table_name"] = tableName
						docMetaCopy["object_name"] = indexName

						doc := entity.NewDocument(
							uuid.New().String(),
							line,
							source,
							"text/plain",
							docMetaCopy,
						)

						select {
						case <-ctx.Done():
							return
						case outChan <- doc:
							position++
						}
						continue
					}
				}

				buffer.WriteString(line)
				buffer.WriteString("\n")

				if buffer.Len() >= p.chunkSize {
					chunkText := strings.TrimSpace(buffer.String())
					docMetaCopy := copyMeta(docMeta)
					docMetaCopy["part_index"] = position
					docMetaCopy["position"] = position
					docMetaCopy["chunk_type"] = "mixed"

					doc := entity.NewDocument(
						uuid.New().String(),
						chunkText,
						source,
						"text/plain",
						docMetaCopy,
					)

					select {
					case <-ctx.Done():
						return
					case outChan <- doc:
						position++
						if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
							remaining := buffer.String()[len(buffer.String())-p.chunkOverlap:]
							buffer.Reset()
							buffer.WriteString(remaining)
						} else {
							buffer.Reset()
						}
					}
				}
			} else {
				currentTable.WriteString(line)
				currentTable.WriteString("\n")
				parenCount += strings.Count(line, "(") - strings.Count(line, ")")

				if parenCount <= 0 {
					docMetaCopy := copyMeta(docMeta)
					docMetaCopy["part_index"] = position
					docMetaCopy["position"] = position
					docMetaCopy["chunk_type"] = "table"
					docMetaCopy["table_name"] = tableName

					doc := entity.NewDocument(
						uuid.New().String(),
						currentTable.String(),
						source,
						"text/plain",
						docMetaCopy,
					)

					select {
					case <-ctx.Done():
						return
					case outChan <- doc:
						position++
						inCreateTable = false
						currentTable.Reset()
						tableName = ""
					}
				}
				continue
			}
		}

		if err := scanner.Err(); err != nil {
			return
		}

		if inCreateTable && currentTable.Len() > 0 {
			docMetaCopy := copyMeta(docMeta)
			docMetaCopy["part_index"] = position
			docMetaCopy["position"] = position
			docMetaCopy["chunk_type"] = "table"
			docMetaCopy["table_name"] = tableName

			doc := entity.NewDocument(
				uuid.New().String(),
				currentTable.String(),
				source,
				"text/plain",
				docMetaCopy,
			)

			select {
			case <-ctx.Done():
				return
			case outChan <- doc:
				position++
			}
		}

		if buffer.Len() > 0 {
			chunkText := strings.TrimSpace(buffer.String())
			docMetaCopy := copyMeta(docMeta)
			docMetaCopy["part_index"] = position
			docMetaCopy["position"] = position
			docMetaCopy["chunk_type"] = "mixed"

			doc := entity.NewDocument(
				uuid.New().String(),
				chunkText,
				source,
				"text/plain",
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
