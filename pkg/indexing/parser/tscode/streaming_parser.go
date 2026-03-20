package tscode

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"github.com/google/uuid"
)

// Parser implements a TypeScript code parser
type Parser struct {
	chunkSize         int
	chunkOverlap      int
	extractFunctions  bool
	extractClasses    bool
	extractInterfaces bool
	extractComments   bool
}

// NewParser creates a new TypeScript parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:         500,
		chunkOverlap:      50,
		extractFunctions:  true,
		extractClasses:    true,
		extractInterfaces: true,
		extractComments:   true,
	}
}

// SetChunkSize sets the chunk size
func (p *Parser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *Parser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// SetExtractFunctions sets whether to extract functions
func (p *Parser) SetExtractFunctions(extract bool) {
	p.extractFunctions = extract
}

// SetExtractClasses sets whether to extract classes
func (p *Parser) SetExtractClasses(extract bool) {
	p.extractClasses = extract
}

// SetExtractInterfaces sets whether to extract interfaces
func (p *Parser) SetExtractInterfaces(extract bool) {
	p.extractInterfaces = extract
}

// SetExtractComments sets whether to extract comments
func (p *Parser) SetExtractComments(extract bool) {
	p.extractComments = extract
}

// ParseStream implements the core.Parser interface
func (p *Parser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	docCh := make(chan *core.Document)

	go func() {
		defer close(docCh)

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

		var buffer strings.Builder
		var position int
		var braceCount int
		var inElement bool
		var currentElement strings.Builder
		var elementName string
		var elementType string

		funcPattern := regexp.MustCompile(`(?:function\s+(\w+)|(?:const|let|var)\s+(\w+)\s*:\s*\([^)]*\)\s*=>|(?:async\s+)?(\w+)\s*\([^)]*\)\s*:\s*\w+\s*{)`)
		classPattern := regexp.MustCompile(`(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
		interfacePattern := regexp.MustCompile(`(?:export\s+)?interface\s+(\w+)`)
		typePattern := regexp.MustCompile(`(?:export\s+)?type\s+(\w+)\s*=`)
		commentSinglePattern := regexp.MustCompile(`^\s*//`)
		inMultiLineComment := false

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			if inMultiLineComment {
				if strings.Contains(line, "*/") {
					inMultiLineComment = false
					if p.extractComments {
						buffer.WriteString(line)
						buffer.WriteString("\n")
					}
				}
				continue
			}

			if strings.Contains(line, "/*") {
				if p.extractComments {
					buffer.WriteString(line)
					buffer.WriteString("\n")
				}
				if !strings.Contains(line, "*/") {
					inMultiLineComment = true
				}
				continue
			}

			if p.extractComments && commentSinglePattern.MatchString(line) {
				buffer.WriteString(line)
				buffer.WriteString("\n")
				continue
			}

			// Check for interface
			if p.extractInterfaces && interfacePattern.MatchString(line) {
				matches := interfacePattern.FindStringSubmatch(line)
				if len(matches) >= 2 {
					if inElement && currentElement.Len() > 0 {
						doc := p.createDocument(currentElement.String(), position, elementType, elementName)
						select {
						case <-ctx.Done():
							return
						case docCh <- doc:
						}
						position++
					}
					inElement = true
					elementType = "interface"
					elementName = matches[1]
					braceCount = strings.Count(line, "{") - strings.Count(line, "}")
					currentElement.Reset()
					currentElement.WriteString(fmt.Sprintf("// INTERFACE: %s\n", elementName))
					currentElement.WriteString(line)
					currentElement.WriteString("\n")

					if braceCount <= 0 {
						doc := p.createDocument(currentElement.String(), position, elementType, elementName)
						select {
						case <-ctx.Done():
							return
						case docCh <- doc:
						}
						position++
						inElement = false
						currentElement.Reset()
					}
					continue
				}
			}

			// Check for type alias
			if p.extractInterfaces && typePattern.MatchString(line) {
				matches := typePattern.FindStringSubmatch(line)
				if len(matches) >= 2 {
					typeName := matches[1]
					doc := p.createDocument(line, position, "type", typeName)
					select {
					case <-ctx.Done():
						return
					case docCh <- doc:
					}
					position++
					continue
				}
			}

			// Check for class
			if p.extractClasses && classPattern.MatchString(line) {
				matches := classPattern.FindStringSubmatch(line)
				if len(matches) >= 2 {
					if inElement && currentElement.Len() > 0 {
						doc := p.createDocument(currentElement.String(), position, elementType, elementName)
						select {
						case <-ctx.Done():
							return
						case docCh <- doc:
						}
						position++
					}
					inElement = true
					elementType = "class"
					elementName = matches[1]
					braceCount = strings.Count(line, "{") - strings.Count(line, "}")
					currentElement.Reset()
					currentElement.WriteString(fmt.Sprintf("// CLASS: %s\n", elementName))
					currentElement.WriteString(line)
					currentElement.WriteString("\n")
					continue
				}
			}

			// Check for function
			if p.extractFunctions && funcPattern.MatchString(line) {
				matches := funcPattern.FindStringSubmatch(line)
				for i := 1; i <= len(matches)-1; i++ {
					if matches[i] != "" {
						elementName = matches[i]
						break
					}
				}
				if elementName != "" {
					if inElement && currentElement.Len() > 0 {
						doc := p.createDocument(currentElement.String(), position, elementType, elementName)
						select {
						case <-ctx.Done():
							return
						case docCh <- doc:
						}
						position++
					}
					inElement = true
					elementType = "function"
					braceCount = strings.Count(line, "{") - strings.Count(line, "}")
					currentElement.Reset()
					currentElement.WriteString(fmt.Sprintf("// FUNCTION: %s\n", elementName))
					currentElement.WriteString(line)
					currentElement.WriteString("\n")
					continue
				}
			}

			if inElement {
				currentElement.WriteString(line)
				currentElement.WriteString("\n")
				braceCount += strings.Count(line, "{") - strings.Count(line, "}")

				if braceCount <= 0 {
					doc := p.createDocument(currentElement.String(), position, elementType, elementName)
					select {
					case <-ctx.Done():
						return
					case docCh <- doc:
					}
					position++
					inElement = false
					currentElement.Reset()
				}
				continue
			}

			buffer.WriteString(line)
			buffer.WriteString("\n")

			if buffer.Len() >= p.chunkSize {
				chunkText := strings.TrimSpace(buffer.String())
				doc := p.createDocument(chunkText, position, "mixed", "")

				select {
				case <-ctx.Done():
					return
				case docCh <- doc:
				}

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

		if err := scanner.Err(); err != nil {
			return
		}

		if inElement && currentElement.Len() > 0 {
			doc := p.createDocument(currentElement.String(), position, elementType, elementName)
			select {
			case <-ctx.Done():
				return
			case docCh <- doc:
			}
			position++
		}

		if buffer.Len() > 0 {
			chunkText := strings.TrimSpace(buffer.String())
			doc := p.createDocument(chunkText, position, "mixed", "")

			select {
			case <-ctx.Done():
				return
			case docCh <- doc:
			}
		}
	}()

	return docCh, nil
}

func (p *Parser) createDocument(content string, position int, chunkType string, name string) *core.Document {
	metadata := map[string]any{
		"type":       "tscode",
		"position":   fmt.Sprintf("%d", position),
		"chunk_type": chunkType,
		"parser":     "tscode",
	}

	if chunkType == "function" && name != "" {
		metadata["function_name"] = name
	} else if chunkType == "class" && name != "" {
		metadata["class_name"] = name
	} else if chunkType == "interface" && name != "" {
		metadata["interface_name"] = name
	} else if chunkType == "type" && name != "" {
		metadata["type_name"] = name
	}

	return &core.Document{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

// GetSupportedTypes returns the supported file extensions
func (p *Parser) GetSupportedTypes() []string {
	return []string{".ts", ".tsx"}
}

// Supports checks if the content type is supported
func (p *Parser) Supports(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return contentType == ".ts" || contentType == ".tsx" || contentType == "text/typescript" || contentType == "application/typescript"
}

// Parse implements the core.Parser interface
func (p *Parser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, strings.NewReader(string(content)), metadata)
	if err != nil {
		return nil, err
	}

	var firstDoc *core.Document
	for doc := range docChan {
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	if firstDoc == nil {
		return nil, fmt.Errorf("no document parsed")
	}

	return firstDoc, nil
}
