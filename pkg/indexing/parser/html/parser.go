package html

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"io"
	"strings"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

// Parser implements an HTML document parser
type Parser struct {
	chunkSize    int
	chunkOverlap int
	cleanScripts bool // Remove <script> tags
	cleanStyles  bool // Remove <style> tags
	extractLinks bool // Extract links
}

// DefaultParser creates a new HTML parser
func DefaultParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
		cleanScripts: true,  // Default: remove scripts
		cleanStyles:  true,  // Default: remove styles
		extractLinks: false, // Default: don't extract links
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

// SetCleanScripts sets whether to remove <script> tags
func (p *Parser) SetCleanScripts(clean bool) {
	p.cleanScripts = clean
}

// SetCleanStyles sets whether to remove <style> tags
func (p *Parser) SetCleanStyles(clean bool) {
	p.cleanStyles = clean
}

// SetExtractLinks sets whether to extract links
func (p *Parser) SetExtractLinks(extract bool) {
	p.extractLinks = extract
}

// Parse parses HTML into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses HTML and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	// Create HTML tokenizer
	tokenizer := html.NewTokenizer(r)

	var buffer strings.Builder
	var position int
	var inSkipTag bool // Track if we're inside a script/style tag

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			tokenType := tokenizer.Next()

			switch tokenType {
			case html.ErrorToken:
				// End of document
				if tokenizer.Err() != io.EOF {
					return tokenizer.Err()
				}
				// Process remaining content
			if buffer.Len() > 0 {
				// 获取文件路径
				filePath := ""
				if path, ok := ctx.Value("file_path").(string); ok {
					filePath = path
				}

				chunk := core.Chunk{
					ID:      uuid.New().String(),
					Content: strings.TrimSpace(buffer.String()),
					Metadata: map[string]any{
						"type":      "html",
						"position":  fmt.Sprintf("%d", position),
						"file_path": filePath,
					},
				}

				if err := callback(chunk); err != nil {
					return err
				}
			}
				return nil

			case html.StartTagToken:
				// Check if we should skip this tag
				tagName := tokenizer.Token().Data
				if (tagName == "script" && p.cleanScripts) || (tagName == "style" && p.cleanStyles) {
					inSkipTag = true
				}

			case html.EndTagToken:
				// Exit skip mode
				tagName := tokenizer.Token().Data
				if tagName == "script" || tagName == "style" {
					inSkipTag = false
				}

			case html.TextToken:
				// Skip text inside script/style tags
				if inSkipTag {
					continue
				}

				// Extract text content
				text := string(tokenizer.Text())
				buffer.WriteString(text)

				// Check if we have enough content for a chunk
				if buffer.Len() >= p.chunkSize {
					// Create chunk with overlap
					chunkText := buffer.String()
					if len(chunkText) > p.chunkSize {
						chunkText = chunkText[:p.chunkSize]
					}

					// 获取文件路径
					filePath := ""
					if path, ok := ctx.Value("file_path").(string); ok {
						filePath = path
					}

					// Create chunk
				chunk := core.Chunk{
					ID:      uuid.New().String(),
					Content: strings.TrimSpace(chunkText),
					Metadata: map[string]any{
						"type":      "html",
						"position":  fmt.Sprintf("%d", position),
						"file_path": filePath,
					},
				}

					// Call callback
					if err := callback(chunk); err != nil {
						return err
					}

					// Keep overlap for next chunk
					if p.chunkOverlap > 0 && buffer.Len() > p.chunkOverlap {
						remaining := buffer.String()[p.chunkSize-p.chunkOverlap:]
						buffer.Reset()
						buffer.WriteString(remaining)
					} else {
						buffer.Reset()
					}

					position++
				}

			// Ignore other token types
			default:
				// Do nothing
			}
		}
	}
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".html", ".htm"}
}
