package image

import (
	"context"
	"fmt"
	"io"

	"github.com/DotNetAge/gorag/core"
	"github.com/google/uuid"
)

// Parser implements a parser for image files
type Parser struct {
	// Add configuration options if needed
}

// New creates a new image parser
func New() *Parser {
	return &Parser{}
}

// Parse parses an image file and returns chunks with media data
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses image and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	// Read the entire image data
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read image data: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("empty image data")
	}

	// Determine media type based on file signature
	mediaType := detectMediaType(data)
	if mediaType == "" {
		return fmt.Errorf("unknown image format")
	}

	// Create a single chunk for the entire image
	chunk := core.Chunk{
		ID:      uuid.New().String(),
		Content: "[Image content]", // Placeholder for text representation
		Metadata: map[string]string{
			"type":       "image",
			"media_type": mediaType,
			"size":       fmt.Sprintf("%d", len(data)),
		},
		MediaType: mediaType,
		MediaData: data,
	}

	// Call callback
	if err := callback(chunk); err != nil {
		return err
	}

	return nil
}

// SupportedFormats returns the list of supported image formats
func (p *Parser) SupportedFormats() []string {
	return []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}
}

// detectMediaType detects the media type of an image based on its signature
func detectMediaType(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	// Check for common image formats
	switch {
	case data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 && data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A:
		return "image/png"
	case data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
		return "image/gif"
	case data[0] == 0x42 && data[1] == 0x4D:
		return "image/bmp"
	case len(data) >= 12 && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return ""
	}
}

