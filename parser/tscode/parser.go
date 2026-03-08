package tscode

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
)

// Parser implements a TypeScript code parser
type Parser struct {
	chunkSize       int
	chunkOverlap    int
	extractFunctions bool
	extractClasses   bool
	extractInterfaces bool
	extractComments  bool
}

// NewParser creates a new TypeScript parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:        500,
		chunkOverlap:     50,
		extractFunctions: true,
		extractClasses:   true,
		extractInterfaces: true,
		extractComments:  true,
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

// Parse parses TypeScript code into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses TypeScript code and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
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
			return ctx.Err()
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
					chunk := p.createChunk(currentElement.String(), position, elementType, elementName)
					if err := callback(chunk); err != nil {
						return err
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
					chunk := p.createChunk(currentElement.String(), position, elementType, elementName)
					if err := callback(chunk); err != nil {
						return err
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
				chunk := p.createChunk(line, position, "type", typeName)
				if err := callback(chunk); err != nil {
					return err
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
					chunk := p.createChunk(currentElement.String(), position, elementType, elementName)
					if err := callback(chunk); err != nil {
						return err
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
					chunk := p.createChunk(currentElement.String(), position, elementType, elementName)
					if err := callback(chunk); err != nil {
						return err
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
				chunk := p.createChunk(currentElement.String(), position, elementType, elementName)
				if err := callback(chunk); err != nil {
					return err
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
			chunk := p.createChunk(chunkText, position, "mixed", "")

			if err := callback(chunk); err != nil {
				return err
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
		return fmt.Errorf("TypeScript code scanning error: %w", err)
	}

	if inElement && currentElement.Len() > 0 {
		chunk := p.createChunk(currentElement.String(), position, elementType, elementName)
		if err := callback(chunk); err != nil {
			return err
		}
		position++
	}

	if buffer.Len() > 0 {
		chunkText := strings.TrimSpace(buffer.String())
		chunk := p.createChunk(chunkText, position, "mixed", "")

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) createChunk(content string, position int, chunkType string, name string) core.Chunk {
	metadata := map[string]string{
		"type":         "tscode",
		"position":     fmt.Sprintf("%d", position),
		"chunk_type":   chunkType,
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

	return core.Chunk{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

func (p *Parser) SupportedFormats() []string {
	return []string{".ts", ".tsx"}
}
