package config

import (
	"fmt"
	"bytes"
	"github.com/DotNetAge/gorag/pkg/core"
	"bufio"
	"context"
	"io"
	"os"
	"regexp"
	"strings"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/config/types"
	"github.com/google/uuid"
)

// ensure interface implementation
var _ core.Parser = (*ConfigStreamParser)(nil)

// ConfigStreamParser implements the configuration file parser with streaming support
type ConfigStreamParser struct {
	chunkSize    int
	chunkOverlap int
	maskSecrets  bool
	expandEnv    bool
	autoDetect   bool
	format       types.ParserType
}

// NewConfigStreamParser creates a new configuration parser
func NewConfigStreamParser() *ConfigStreamParser {
	return &ConfigStreamParser{
		chunkSize:    500,
		chunkOverlap: 50,
		maskSecrets:  true,
		expandEnv:    false, // Default to false for security - users must explicitly enable
		autoDetect:   true,
		format:       types.UNKNOWN,
	}
}

// GetSupportedTypes returns the supported file formats
func (p *ConfigStreamParser) GetSupportedTypes() []string {
	return []string{".toml", ".ini", ".cfg", ".conf", ".properties", ".env", ".yaml", ".yml"}
}

// ParseStream reads the incoming io.Reader and yields chunks of the document via a channel
func (p *ConfigStreamParser) ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	outChan := make(chan *core.Document, 1)

	docMeta := make(map[string]any)
	for k, v := range metadata {
		docMeta[k] = v
	}
	docMeta["parser"] = "ConfigStreamParser"
	docMeta["streaming"] = "true"
	docMeta["masked"] = p.maskSecrets
	docMeta["env_expanded"] = p.expandEnv

	source := "unknown"
	if s, ok := metadata["source"].(string); ok {
		source = s
	}

	go func() {
		defer close(outChan)

		reader := bufio.NewReader(r)

		// State tracking
		var currentChunk strings.Builder
		var overlapBuffer strings.Builder
		var position int

		for {
			select {
			case <-ctx.Done():
				return
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

						docMetaCopy := copyMeta(docMeta)
						docMetaCopy["part_index"] = position
						docMetaCopy["position"] = position
						docMetaCopy["format"] = p.format.String()

						doc := core.NewDocument(
							uuid.New().String(),
							strings.TrimSpace(chunkText),
							source,
							"text/plain",
							docMetaCopy,
						)

						select {
						case <-ctx.Done():
							return
						case outChan <- doc:
						}
					}
					break
				}
				return
			}

			// Check context after successful read
			select {
			case <-ctx.Done():
				return
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

				docMetaCopy := copyMeta(docMeta)
				docMetaCopy["part_index"] = position
				docMetaCopy["position"] = position
				docMetaCopy["format"] = p.format.String()

				doc := core.NewDocument(
					uuid.New().String(),
					strings.TrimSpace(chunkText),
					source,
					"text/plain",
					docMetaCopy,
				)

				select {
				case <-ctx.Done():
					return
				case outChan <- doc:
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
		}
	}()

	return outChan, nil
}

// processLine processes a single line based on format
func (p *ConfigStreamParser) processLine(line string) string {
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
func (p *ConfigStreamParser) maskSensitiveLine(line string) string {
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

func copyMeta(m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		out[k] = v
	}
	return out
}


// Parse implements core.Parser interface.
func (p *ConfigStreamParser) Parse(ctx context.Context, content []byte, metadata map[string]any) (*core.Document, error) {
	docChan, err := p.ParseStream(ctx, bytes.NewReader(content), metadata)
	if err != nil {
		return nil, err
	}
	for doc := range docChan {
		return doc, nil
	}
	return nil, fmt.Errorf("no document produced")
}

func (p *ConfigStreamParser) Supports(contentType string) bool { return true }
