package jscode

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

// Parser implements a JavaScript code parser
type Parser struct {
	chunkSize        int
	chunkOverlap     int
	extractFunctions bool
	extractClasses   bool
	extractComments  bool
}

// NewParser creates a new JavaScript parser
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

// Parse parses JavaScript code into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses JavaScript code and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var buffer strings.Builder
	var position int
	var braceCount int
	var inFunction bool
	var currentFunction strings.Builder
	var functionName string

	funcPattern := regexp.MustCompile(`(?:function\s+(\w+)|(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\([^)]*\)\s*=>|(?:async\s+)?(\w+)\s*\([^)]*\)\s*{)`)
	classPattern := regexp.MustCompile(`class\s+(\w+)`)
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

		if p.extractFunctions && funcPattern.MatchString(line) {
			matches := funcPattern.FindStringSubmatch(line)
			for i := 1; i <= len(matches)-1; i++ {
				if matches[i] != "" {
					functionName = matches[i]
					break
				}
			}
			if functionName != "" {
				if inFunction && currentFunction.Len() > 0 {
					chunk := p.createChunk(currentFunction.String(), position, "function", "")
					if err := callback(chunk); err != nil {
						return err
					}
					position++
				}
				inFunction = true
				braceCount = strings.Count(line, "{") - strings.Count(line, "}")
				currentFunction.Reset()
				currentFunction.WriteString(fmt.Sprintf("// FUNCTION: %s\n", functionName))
				currentFunction.WriteString(line)
				currentFunction.WriteString("\n")
				continue
			}
		}

		if p.extractClasses && classPattern.MatchString(line) {
			matches := classPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				className := matches[1]
				if inFunction && currentFunction.Len() > 0 {
					chunk := p.createChunk(currentFunction.String(), position, "function", "")
					if err := callback(chunk); err != nil {
						return err
					}
					position++
				}
				inFunction = true
				braceCount = strings.Count(line, "{") - strings.Count(line, "}")
				currentFunction.Reset()
				currentFunction.WriteString(fmt.Sprintf("// CLASS: %s\n", className))
				currentFunction.WriteString(line)
				currentFunction.WriteString("\n")
				continue
			}
		}

		if inFunction {
			currentFunction.WriteString(line)
			currentFunction.WriteString("\n")
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")

			if braceCount <= 0 {
				// Extract function name from content
				var fname string
				if matches := regexp.MustCompile(`// FUNCTION: (\w+)`).FindStringSubmatch(currentFunction.String()); len(matches) > 1 {
					fname = matches[1]
				}
				chunk := p.createChunk(currentFunction.String(), position, "function", fname)
				if err := callback(chunk); err != nil {
					return err
				}
				position++
				inFunction = false
				currentFunction.Reset()
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
		return fmt.Errorf("JavaScript code scanning error: %w", err)
	}

	if inFunction && currentFunction.Len() > 0 {
		chunk := p.createChunk(currentFunction.String(), position, "function", "")
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

func (p *Parser) createChunk(content string, position int, elementType string, name string) core.Chunk {
	metadata := map[string]any{
		"type":         "jscode",
		"position":     fmt.Sprintf("%d", position),
		"element_type": elementType,
	}

	if elementType == "function" && name != "" {
		metadata["function_name"] = name
	}

	return core.Chunk{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

func (p *Parser) SupportedFormats() []string {
	return []string{".js", ".jsx", ".mjs"}
}
