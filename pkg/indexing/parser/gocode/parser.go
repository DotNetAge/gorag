package gocode

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"github.com/google/uuid"
)

// Parser implements Go code parser with streaming support (default behavior)
type Parser struct {
	chunkSize        int
	chunkOverlap     int
	extractFunctions bool
	extractTypes     bool
	extractComments  bool
}

// DefaultParser creates a new Go code parser (streaming by default)
func DefaultParser() *Parser {
	return &Parser{
		chunkSize:        500,
		chunkOverlap:     50,
		extractFunctions: true,
		extractTypes:     true,
		extractComments:  true,
	}
}

// Parse parses Go code using streaming processing (default behavior)
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses Go code and calls the callback for each chunk
// This is the primary method - all parsing is streaming by default
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	// Read all content for AST parsing (necessary for go/parser)
	content, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	// Parse Go source file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		// If parsing fails, fall back to line-by-line streaming
		return p.parseLineByLine(ctx, strings.NewReader(string(content)), callback)
	}

	// Extract elements based on configuration
	var elements []CodeElement

	if p.extractFunctions {
		elements = append(elements, p.extractFunctionsFromFile(fset, file, content)...)
	}

	if p.extractTypes {
		elements = append(elements, p.extractTypesFromFile(fset, file, content)...)
	}

	if p.extractComments {
		elements = append(elements, p.extractCommentsFromFile(fset, file, content)...)
	}

	// Convert elements to text for chunking
	text := p.elementsToText(elements)

	// Stream through the text and create chunks
	return p.chunkText(ctx, text, callback)
}

// parseLineByLine falls back to line-by-line streaming when AST parsing fails
func (p *Parser) parseLineByLine(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	reader := bufio.NewReader(r)

	var currentChunk strings.Builder
	var overlapBuffer strings.Builder
	var position int

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				if currentChunk.Len() > 0 {
					chunkText := currentChunk.String()
					if overlapBuffer.Len() > 0 {
						chunkText = overlapBuffer.String() + chunkText
					}
					chunk := p.createChunk(chunkText, position, "lines")
					if err := callback(chunk); err != nil {
						return err
					}
				}
				break
			}
			return fmt.Errorf("failed to read input: %w", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		currentChunk.Write(line)

		if currentChunk.Len() >= p.chunkSize {
			chunkText := currentChunk.String()
			if overlapBuffer.Len() > 0 {
				chunkText = overlapBuffer.String() + chunkText
			}

			chunk := p.createChunk(chunkText, position, "lines")
			if err := callback(chunk); err != nil {
				return err
			}

			lastPart := chunkText
			if len(lastPart) > p.chunkOverlap {
				lastPart = lastPart[len(lastPart)-p.chunkOverlap:]
			}
			overlapBuffer.Reset()
			overlapBuffer.WriteString(lastPart)

			currentChunk.Reset()
			position++
		}
	}

	return nil
}

// CodeElement represents a code element (function, type, comment)
type CodeElement struct {
	Kind     string // "function", "type", "comment"
	Name     string
	Start    int
	End      int
	Content  string
	Metadata map[string]string
}

// extractFunctionsFromFile extracts function declarations
func (p *Parser) extractFunctionsFromFile(fset *token.FileSet, file *ast.File, content []byte) []CodeElement {
	var elements []CodeElement

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			start := fset.Position(d.Pos()).Offset
			end := fset.Position(d.End()).Offset

			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				// Method - include receiver type
				recvType := getTypeName(d.Recv.List[0].Type)
				name = recvType + "." + name
			}

			element := CodeElement{
				Kind:    "function",
				Name:    name,
				Start:   start,
				End:     end,
				Content: string(content[start:end]),
				Metadata: map[string]string{
					"function_name": name,
					"is_method":     fmt.Sprintf("%v", d.Recv != nil),
				},
			}
			elements = append(elements, element)
		}
	}

	return elements
}

// extractTypesFromFile extracts type declarations
func (p *Parser) extractTypesFromFile(fset *token.FileSet, file *ast.File, content []byte) []CodeElement {
	var elements []CodeElement

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					start := fset.Position(typeSpec.Pos()).Offset
					end := fset.Position(typeSpec.End()).Offset

					element := CodeElement{
						Kind:    "type",
						Name:    typeSpec.Name.Name,
						Start:   start,
						End:     end,
						Content: string(content[start:end]),
						Metadata: map[string]string{
							"type_name": typeSpec.Name.Name,
						},
					}
					elements = append(elements, element)
				}
			}
		}
	}

	return elements
}

// extractCommentsFromFile extracts comments
func (p *Parser) extractCommentsFromFile(fset *token.FileSet, file *ast.File, content []byte) []CodeElement {
	var elements []CodeElement

	for _, commentGroup := range file.Comments {
		start := fset.Position(commentGroup.Pos()).Offset
		end := fset.Position(commentGroup.End()).Offset

		text := commentGroup.Text()
		element := CodeElement{
			Kind:    "comment",
			Name:    fmt.Sprintf("comment_%d", start),
			Start:   start,
			End:     end,
			Content: text,
			Metadata: map[string]string{
				"comment_type": "documentation",
			},
		}
		elements = append(elements, element)
	}

	return elements
}

// elementsToText converts elements to readable text
func (p *Parser) elementsToText(elements []CodeElement) string {
	var builder strings.Builder

	for i, elem := range elements {
		if i > 0 {
			builder.WriteString("\n\n")
		}

		builder.WriteString(fmt.Sprintf("// %s: %s\n", strings.ToUpper(elem.Kind), elem.Name))
		builder.WriteString(elem.Content)
	}

	return builder.String()
}

// chunkText streams through text and creates chunks
func (p *Parser) chunkText(ctx context.Context, text string, callback func(core.Chunk) error) error {
	if len(text) == 0 {
		return nil
	}

	var currentChunk strings.Builder
	var overlapBuffer strings.Builder
	var position int

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		currentChunk.WriteRune(runes[i])

		if currentChunk.Len() >= p.chunkSize {
			chunkText := currentChunk.String()
			if overlapBuffer.Len() > 0 {
				chunkText = overlapBuffer.String() + chunkText
			}

			chunk := p.createChunk(chunkText, position, "mixed")
			if err := callback(chunk); err != nil {
				return err
			}

			lastPart := chunkText
			if len(lastPart) > p.chunkOverlap {
				lastPart = lastPart[len(lastPart)-p.chunkOverlap:]
			}
			overlapBuffer.Reset()
			overlapBuffer.WriteString(lastPart)

			currentChunk.Reset()
			position++
		}
	}

	// Process remaining content
	if currentChunk.Len() > 0 {
		chunkText := currentChunk.String()
		if overlapBuffer.Len() > 0 {
			chunkText = overlapBuffer.String() + chunkText
		}
		chunk := p.createChunk(chunkText, position, "mixed")
		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

// createChunk creates a chunk with metadata
func (p *Parser) createChunk(content string, position int, elementType string) core.Chunk {
	metadata := map[string]any{
		"type":         "gocode",
		"position":     fmt.Sprintf("%d", position),
		"streaming":    "true",
		"element_type": elementType,
	}

	return core.Chunk{
		ID:       uuid.New().String(),
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}
}

// SupportedFormats returns supported file formats
func (p *Parser) SupportedFormats() []string {
	return []string{".go"}
}

// SetChunkSize sets the chunk size
func (p *Parser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *Parser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// SetExtractFunctions enables or disables function extraction
func (p *Parser) SetExtractFunctions(enabled bool) {
	p.extractFunctions = enabled
}

// SetExtractTypes enables or disables type extraction
func (p *Parser) SetExtractTypes(enabled bool) {
	p.extractTypes = enabled
}

// SetExtractComments enables or disables comment extraction
func (p *Parser) SetExtractComments(enabled bool) {
	p.extractComments = enabled
}

// Helper function to get type name from expression
func getTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	}
	return ""
}
