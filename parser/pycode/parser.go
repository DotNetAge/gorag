package pycode

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

// Parser implements a Python code parser
type Parser struct {
	chunkSize        int
	chunkOverlap     int
	extractFunctions bool
	extractClasses   bool
	extractComments  bool
}

// NewParser creates a new Python parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:        500,
		chunkOverlap:     50,
		extractFunctions: true,
		extractClasses:   true,
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

// SetExtractComments sets whether to extract comments
func (p *Parser) SetExtractComments(extract bool) {
	p.extractComments = extract
}

// Parse parses Python code into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses Python code and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
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
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		if p.extractFunctions && funcPattern.MatchString(line) {
			matches := funcPattern.FindStringSubmatch(line)
			if len(matches) >= 3 {
				if inElement && currentElement.Len() > 0 {
					p.processElement(&buffer, &currentElement, position, callback)
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
					p.processElement(&buffer, &currentElement, position, callback)
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
					p.processElement(&buffer, &currentElement, position, callback)
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
			chunk := p.createChunk(chunkText, position, "mixed")

			if err := callback(chunk); err != nil {
				return err
			}

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
		return fmt.Errorf("Python code scanning error: %w", err)
	}

	if inElement && currentElement.Len() > 0 {
		p.processElement(&buffer, &currentElement, position, callback)
		position++
	}

	if buffer.Len() > 0 {
		chunkText := strings.TrimSpace(buffer.String())
		chunk := p.createChunk(chunkText, position, "mixed")

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) processElement(buffer, element *strings.Builder, position int, callback func(core.Chunk) error) error {
	elementText := strings.TrimSpace(element.String())
	if elementText != "" {
		elementType := "mixed"
		if strings.Contains(elementText, "# FUNCTION:") {
			elementType = "function"
		} else if strings.Contains(elementText, "# CLASS:") {
			elementType = "class"
		}

		chunk := p.createChunk(elementText, position, elementType)
		return callback(chunk)
	}
	return nil
}

func (p *Parser) createChunk(content string, position int, elementType string) core.Chunk {
	metadata := map[string]string{
		"type":         "pycode",
		"position":     fmt.Sprintf("%d", position),
		"element_type": elementType,
	}

	if elementType == "function" {
		re := regexp.MustCompile(`# FUNCTION: (\w+)`)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			metadata["function_name"] = matches[1]
		}
	} else if elementType == "class" {
		re := regexp.MustCompile(`# CLASS: (\w+)`)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			metadata["class_name"] = matches[1]
		}
	}

	return core.Chunk{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

func (p *Parser) SupportedFormats() []string {
	return []string{".py"}
}
