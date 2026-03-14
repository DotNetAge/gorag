package pycode

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/google/uuid"
)

// PycodeStreamParser implements the dataprep.Parser interface for Python code files
type PycodeStreamParser struct {
	chunkSize        int
	chunkOverlap     int
	extractFunctions bool
	extractClasses   bool
	extractComments  bool
}

// NewPycodeStreamParser creates a new Python code stream parser
func NewPycodeStreamParser() *PycodeStreamParser {
	return &PycodeStreamParser{
		chunkSize:        500,
		chunkOverlap:     50,
		extractFunctions: true,
		extractClasses:   true,
		extractComments:  true,
	}
}

// SetChunkSize sets the chunk size
func (p *PycodeStreamParser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *PycodeStreamParser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// SetExtractFunctions sets whether to extract functions
func (p *PycodeStreamParser) SetExtractFunctions(extract bool) {
	p.extractFunctions = extract
}

// SetExtractClasses sets whether to extract classes
func (p *PycodeStreamParser) SetExtractClasses(extract bool) {
	p.extractClasses = extract
}

// SetExtractComments sets whether to extract comments
func (p *PycodeStreamParser) SetExtractComments(extract bool) {
	p.extractComments = extract
}

// GetSupportedTypes returns the supported formats
func (p *PycodeStreamParser) GetSupportedTypes() []string {
	return []string{".py"}
}

// ParseStream parses Python code from a reader and returns a channel of documents
func (p *PycodeStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error) {
	outChan := make(chan *entity.Document, 10)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "PycodeStreamParser"

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
		var currentElement strings.Builder
		var inElement bool
		var elementIndent int

		funcPattern := regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\(`)
		classPattern := regexp.MustCompile(`^(\s*)class\s+(\w+)`)
		commentPattern := regexp.MustCompile(`^\s*#`)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			if p.extractFunctions && funcPattern.MatchString(line) {
				matches := funcPattern.FindStringSubmatch(line)
				if len(matches) >= 3 {
					if inElement && currentElement.Len() > 0 {
						p.processElement(&buffer, &currentElement, position, source, docMeta, outChan, ctx)
						position++
					}
					inElement = true
					elementIndent = len(matches[1])
					funcName := matches[2]
					currentElement.Reset()
					currentElement.WriteString(fmt.Sprintf("# FUNCTION: %s\n", funcName))
					currentElement.WriteString(line)
					currentElement.WriteString("\n")
					continue
				}
			}

			if p.extractClasses && classPattern.MatchString(line) {
				matches := classPattern.FindStringSubmatch(line)
				if len(matches) >= 3 {
					if inElement && currentElement.Len() > 0 {
						p.processElement(&buffer, &currentElement, position, source, docMeta, outChan, ctx)
						position++
					}
					inElement = true
					elementIndent = len(matches[1])
					className := matches[2]
					currentElement.Reset()
					currentElement.WriteString(fmt.Sprintf("# CLASS: %s\n", className))
					currentElement.WriteString(line)
					currentElement.WriteString("\n")
					continue
				}
			}

			if p.extractComments && commentPattern.MatchString(line) {
				if !inElement {
					buffer.WriteString(line)
					buffer.WriteString("\n")
				} else {
					currentElement.WriteString(line)
					currentElement.WriteString("\n")
				}
				continue
			}

			if inElement {
				if strings.TrimSpace(line) != "" {
					currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
					if currentIndent <= elementIndent && !strings.HasPrefix(strings.TrimSpace(line), "#") {
						p.processElement(&buffer, &currentElement, position, source, docMeta, outChan, ctx)
						position++
						inElement = false
					} else {
						currentElement.WriteString(line)
						currentElement.WriteString("\n")
					}
				} else {
					currentElement.WriteString(line)
					currentElement.WriteString("\n")
				}
			} else {
				buffer.WriteString(line)
				buffer.WriteString("\n")
			}

			if buffer.Len() >= p.chunkSize {
				chunkText := strings.TrimSpace(buffer.String())
				p.createAndSendDocument(chunkText, position, "mixed", source, docMeta, outChan, ctx)

				if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
					remaining := buffer.String()[len(buffer.String())-p.chunkOverlap:]
					buffer.Reset()
					buffer.WriteString(remaining)
				} else {
					buffer.Reset()
				}

				position++
			}
		}

		if err := scanner.Err(); err != nil {
			return
		}

		if inElement && currentElement.Len() > 0 {
			p.processElement(&buffer, &currentElement, position, source, docMeta, outChan, ctx)
			position++
		}

		if buffer.Len() > 0 {
			chunkText := strings.TrimSpace(buffer.String())
			p.createAndSendDocument(chunkText, position, "mixed", source, docMeta, outChan, ctx)
		}
	}()

	return outChan, nil
}

func (p *PycodeStreamParser) processElement(buffer, element *strings.Builder, position int, source string, docMeta map[string]any, outChan chan *entity.Document, ctx context.Context) {
	elementText := strings.TrimSpace(element.String())
	if elementText != "" {
		elementType := "mixed"
		if strings.Contains(elementText, "# FUNCTION:") {
			elementType = "function"
		} else if strings.Contains(elementText, "# CLASS:") {
			elementType = "class"
		}

		p.createAndSendDocument(elementText, position, elementType, source, docMeta, outChan, ctx)
	}
}

func (p *PycodeStreamParser) createAndSendDocument(content string, position int, elementType string, source string, docMeta map[string]any, outChan chan *entity.Document, ctx context.Context) {
	docMetaCopy := make(map[string]any)
	for k, v := range docMeta {
		docMetaCopy[k] = v
	}
	docMetaCopy["type"] = "pycode"
	docMetaCopy["position"] = fmt.Sprintf("%d", position)
	docMetaCopy["element_type"] = elementType

	if elementType == "function" {
		re := regexp.MustCompile(`# FUNCTION: (\w+)`)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			docMetaCopy["function_name"] = matches[1]
		}
	} else if elementType == "class" {
		re := regexp.MustCompile(`# CLASS: (\w+)`)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			docMetaCopy["class_name"] = matches[1]
		}
	}

	doc := entity.NewDocument(
		uuid.New().String(),
		content,
		source,
		"text/x-python",
		docMetaCopy,
	)

	select {
	case <-ctx.Done():
		return
	case outChan <- doc:
	}
}