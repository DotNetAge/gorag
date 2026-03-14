package config

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/google/uuid"
)

// Format represents the configuration file format
type Format int

const (
	// Unknown represents an unknown format
	Unknown Format = iota
	// TOML format
	TOML
	// INI format
	INI
	// Properties format
	Properties
	// ENV format
	ENV
	// YAML format
	YAML
)

// Parser implements the configuration file parser with streaming support (default behavior)
type Parser struct {
	chunkSize    int
	chunkOverlap int
	maskSecrets  bool
	expandEnv    bool
	autoDetect   bool
	format       Format
}

// NewParser creates a new configuration parser (streaming by default)
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
		maskSecrets:  true,
		expandEnv:    false, // Default to false for security - users must explicitly enable
		autoDetect:   true,
	}
}

// Parse parses configuration files into chunks using streaming processing (default behavior)
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]model.Chunk, error) {
	var chunks []model.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk model.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses configuration and calls the callback for each chunk
// This is the primary method - all parsing is streaming by default
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(model.Chunk) error) error {
	reader := bufio.NewReader(r)

	// State tracking
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
				// Process remaining content in buffer
				if currentChunk.Len() > 0 {
					chunkText := currentChunk.String()

					// Add overlap from previous chunk
					if overlapBuffer.Len() > 0 {
						chunkText = overlapBuffer.String() + chunkText
					}

					chunk := p.createChunk(chunkText, position, Unknown)
					if err := callback(chunk); err != nil {
						return err
					}
				}
				break
			}
			return fmt.Errorf("failed to read input: %w", err)
		}

		// Check context after successful read
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Process line
		processedLine := p.processLine(string(line))
		currentChunk.WriteString(processedLine)

		// Check if we should emit a chunk
		if currentChunk.Len() >= p.chunkSize {
			chunkText := currentChunk.String()

			// Add overlap from previous chunk
			if overlapBuffer.Len() > 0 {
				chunkText = overlapBuffer.String() + chunkText
			}

			chunk := p.createChunk(chunkText, position, Unknown)
			if err := callback(chunk); err != nil {
				return err
			}

			// Prepare overlap for next chunk
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

// detectFormatFromReader detects format by reading first few lines
func (p *Parser) detectFormatFromReader(reader *bufio.Reader) Format {
	// Peek at the first few lines without consuming them
	var preview strings.Builder
	maxLines := 10

	for i := 0; i < maxLines; i++ {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}
		preview.Write(line)

		// Try to detect format
		text := preview.String()
		if format := p.detectFormatInText(text); format != Unknown {
			return format
		}
	}

	return Unknown
}

// detectFormatInText detects format from text
func (p *Parser) detectFormatInText(text string) Format {
	// Check for INI indicators ([section] or key=value)
	if regexp.MustCompile(`^\[[\w\-\.]+\]`).MatchString(text) {
		return INI
	}

	// Check for TOML indicators ([[section]] or key = value)
	if regexp.MustCompile(`^\[\[[\w\-\.]+\]\]`).MatchString(text) ||
		regexp.MustCompile(`^\w+\s*=\s*`).MatchString(text) {
		return TOML
	}

	// Check for ENV indicators (KEY=value or export KEY=value)
	if regexp.MustCompile(`^(export\s+)?[A-Z_][A-Z0-9_]*\s*=`).MatchString(text) {
		return ENV
	}

	// Check for Properties indicators (key=value or key:value)
	if regexp.MustCompile(`^[\w\.\-]+\s*[=:]\s*`).MatchString(text) {
		return Properties
	}

	// Check for YAML indicators (--- for frontmatter or key: value)
	if strings.Contains(text, "---") || regexp.MustCompile(`^\w+:\s*`).MatchString(text) {
		return YAML
	}

	return Unknown
}

// processLine processes a single line based on format
func (p *Parser) processLine(line string) string {
	// Skip comments
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return ""
	}

	// Mask secrets if enabled
	if p.maskSecrets {
		line = p.maskSensitiveLine(line)
	}

	// Expand environment variables if enabled
	if p.expandEnv {
		line = os.ExpandEnv(line)
	}

	return line
}

// maskSensitiveLine masks sensitive data in a line
func (p *Parser) maskSensitiveLine(line string) string {
	sensitivePatterns := []string{
		`(?i)(password|passwd|pwd)\s*[=:]\s*\S+`,
		`(?i)(secret|token|api_key|apikey)\s*[=:]\s*\S+`,
		`(?i)(auth|credential)\s*[=:]\s*\S+`,
	}

	result := line
	for _, pattern := range sensitivePatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(result) {
			// Replace value with masked version
			result = re.ReplaceAllString(result, "${1}=***MASKED***")
		}
	}

	return result
}

// createChunk creates a chunk with metadata
func (p *Parser) createChunk(content string, position int, format Format) model.Chunk {
	metadata := map[string]string{
		"type":         "config",
		"position":     fmt.Sprintf("%d", position),
		"format":       format.String(),
		"streaming":    "true",
		"masked":       fmt.Sprintf("%v", p.maskSecrets),
		"env_expanded": fmt.Sprintf("%v", p.expandEnv),
	}

	return model.Chunk{
		ID:       uuid.New().String(),
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}
}

// SupportedFormats returns the supported file formats
func (p *Parser) SupportedFormats() []string {
	return []string{".toml", ".ini", ".cfg", ".conf", ".properties", ".env", ".yaml", ".yml"}
}

// SetChunkSize sets the chunk size
func (p *Parser) SetChunkSize(size int) {
	p.chunkSize = size
}

// SetChunkOverlap sets the chunk overlap
func (p *Parser) SetChunkOverlap(overlap int) {
	p.chunkOverlap = overlap
}

// SetMaskSecrets enables or disables secret masking
func (p *Parser) SetMaskSecrets(enabled bool) {
	p.maskSecrets = enabled
}

// SetExpandEnv enables or disables environment variable expansion
func (p *Parser) SetExpandEnv(enabled bool) {
	p.expandEnv = enabled
}

// SetAutoDetect enables or disables auto-detection of format
func (p *Parser) SetAutoDetect(enabled bool) {
	p.autoDetect = enabled
}

// String returns the string representation of Format
func (f Format) String() string {
	switch f {
	case TOML:
		return "TOML"
	case INI:
		return "INI"
	case Properties:
		return "Properties"
	case ENV:
		return "ENV"
	case YAML:
		return "YAML"
	default:
		return "Unknown"
	}
}
