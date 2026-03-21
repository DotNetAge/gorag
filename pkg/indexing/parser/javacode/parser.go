package javacode

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

// Parser implements a Java code parser
type Parser struct {
	chunkSize       int
	chunkOverlap    int
	extractMethods  bool
	extractClasses  bool
	extractComments bool
}

// DefaultParser creates a new Java parser
func DefaultParser() *Parser {
	return &Parser{
		chunkSize:       500,
		chunkOverlap:    50,
		extractMethods:  true,
		extractClasses:  true,
		extractComments: true,
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

// SetExtractMethods sets whether to extract methods
func (p *Parser) SetExtractMethods(extract bool) {
	p.extractMethods = extract
}

// SetExtractClasses sets whether to extract classes
func (p *Parser) SetExtractClasses(extract bool) {
	p.extractClasses = extract
}

// SetExtractComments sets whether to extract comments
func (p *Parser) SetExtractComments(extract bool) {
	p.extractComments = extract
}

// Parse parses Java code into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses Java code and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var buffer strings.Builder
	var position int
	var braceCount int
	var inMethod bool
	var currentMethod strings.Builder
	var methodName string
	var className string

	methodPattern := regexp.MustCompile(`(?:public|private|protected)?\s*(?:static)?\s*(?:\w+(?:<\w+>)?\s+)+(\w+)\s*\(`)
	classPattern := regexp.MustCompile(`(?:public\s+)?class\s+(\w+)`)
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

		if p.extractClasses && classPattern.MatchString(line) {
			matches := classPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				if inMethod && currentMethod.Len() > 0 {
					chunk := p.createChunk(currentMethod.String(), position, "method", className, methodName)
					if err := callback(chunk); err != nil {
						return err
					}
					position++
					inMethod = false
					currentMethod.Reset()
				}
				className = matches[1]
				chunk := p.createChunk(line, position, "class", className, "")
				if err := callback(chunk); err != nil {
					return err
				}
				position++
				continue
			}
		}

		if p.extractMethods && methodPattern.MatchString(line) {
			matches := methodPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				if inMethod && currentMethod.Len() > 0 {
					chunk := p.createChunk(currentMethod.String(), position, "method", className, methodName)
					if err := callback(chunk); err != nil {
						return err
					}
					position++
				}
				inMethod = true
				methodName = matches[1]
				braceCount = strings.Count(line, "{") - strings.Count(line, "}")
				currentMethod.Reset()
				currentMethod.WriteString(fmt.Sprintf("// METHOD: %s\n", methodName))
				currentMethod.WriteString(line)
				currentMethod.WriteString("\n")
				continue
			}
		}

		if inMethod {
			currentMethod.WriteString(line)
			currentMethod.WriteString("\n")
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")

			if braceCount <= 0 {
				chunk := p.createChunk(currentMethod.String(), position, "method", className, methodName)
				if err := callback(chunk); err != nil {
					return err
				}
				position++
				inMethod = false
				currentMethod.Reset()
			}
			continue
		}

		buffer.WriteString(line)
		buffer.WriteString("\n")

		if buffer.Len() >= p.chunkSize {
			chunkText := strings.TrimSpace(buffer.String())
			chunk := p.createChunk(chunkText, position, "mixed", "", "")

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
		return fmt.Errorf("Java code scanning error: %w", err)
	}

	if inMethod && currentMethod.Len() > 0 {
		chunk := p.createChunk(currentMethod.String(), position, "method", className, methodName)
		if err := callback(chunk); err != nil {
			return err
		}
		position++
	}

	if buffer.Len() > 0 {
		chunkText := strings.TrimSpace(buffer.String())
		chunk := p.createChunk(chunkText, position, "mixed", "", "")

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) createChunk(content string, position int, chunkType string, className string, methodName string) core.Chunk {
	metadata := map[string]any{
		"type":       "javacode",
		"position":   fmt.Sprintf("%d", position),
		"chunk_type": chunkType,
	}

	if className != "" {
		metadata["class_name"] = className
	}

	if methodName != "" {
		metadata["method_name"] = methodName
	}

	return core.Chunk{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

func (p *Parser) SupportedFormats() []string {
	return []string{".java"}
}
