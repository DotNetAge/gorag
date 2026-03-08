package log

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

// Format represents log format type
type Format int

const (
	Unknown Format = iota
	Nginx
	Apache
	Syslog
	Custom
)

// Parser implements a log parser with multiple format support
type Parser struct {
	chunkSize    int
	chunkOverlap int
	format       Format
	pattern      *regexp.Regexp
}

// NewParser creates a new log parser
func NewParser() *Parser {
	return &Parser{
		chunkSize:    500,
		chunkOverlap: 50,
		format:       Unknown,
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

// SetFormat sets the log format
func (p *Parser) SetFormat(format Format) {
	p.format = format
}

// SetPattern sets custom regex pattern
func (p *Parser) SetPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	p.pattern = re
	p.format = Custom
	return nil
}

// Parse parses logs into chunks
func (p *Parser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
	var chunks []core.Chunk
	err := p.ParseWithCallback(ctx, r, func(chunk core.Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	return chunks, err
}

// ParseWithCallback parses logs and calls the callback for each chunk
func (p *Parser) ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024) // 10MB buffer

	var buffer strings.Builder
	var position int
	var currentLine int

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		currentLine++

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Add line to buffer with metadata
		if p.format != Unknown {
			// Add format-specific metadata
			metadata := p.extractMetadata(line)
			if metadata != "" {
				buffer.WriteString(metadata)
				buffer.WriteString(" ")
			}
		}

		buffer.WriteString(line)
		buffer.WriteString("\n")

		// Check if we have enough content for a chunk
		if buffer.Len() >= p.chunkSize {
			chunkText := strings.TrimSpace(buffer.String())
			chunk := p.createChunk(chunkText, position)

			if err := callback(chunk); err != nil {
				return err
			}

			// Keep overlap for next chunk
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
		return fmt.Errorf("log scanning error: %w", err)
	}

	// Process remaining content
	if buffer.Len() > 0 {
		chunkText := strings.TrimSpace(buffer.String())
		chunk := p.createChunk(chunkText, position)

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}

// extractMetadata extracts format-specific metadata
func (p *Parser) extractMetadata(line string) string {
	switch p.format {
	case Nginx:
		return p.extractNginxMetadata(line)
	case Apache:
		return p.extractApacheMetadata(line)
	case Syslog:
		return p.extractSyslogMetadata(line)
	default:
		return ""
	}
}

// extractNginxMetadata extracts metadata from nginx log line
func (p *Parser) extractNginxMetadata(line string) string {
	// Common nginx log format:
	// IP - - [timestamp] "METHOD URL PROTOCOL" status size "referer" "user-agent"
	re := regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"(\S+)\s+(\S+)\s+([^"]+)"\s+(\d+)\s+(\d+)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) >= 8 {
		ip := matches[1]
		timestamp := matches[2]
		method := matches[3]
		url := matches[4]
		status := matches[6]

		return fmt.Sprintf("[%s] %s %s -> %s (%s)", timestamp, ip, method, url, status)
	}

	return ""
}

// extractApacheMetadata extracts metadata from apache log line
func (p *Parser) extractApacheMetadata(line string) string {
	// Similar to nginx
	return p.extractNginxMetadata(line)
}

// extractSyslogMetadata extracts metadata from syslog line
func (p *Parser) extractSyslogMetadata(line string) string {
	// Syslog format: Mon DD HH:MM:SS hostname process[pid]: message
	re := regexp.MustCompile(`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[(\d+)\])?:\s*(.*)$`)
	matches := re.FindStringSubmatch(line)

	if len(matches) >= 6 {
		timestamp := matches[1]
		hostname := matches[2]
		process := matches[3]

		return fmt.Sprintf("[%s] %s %s", timestamp, hostname, process)
	}

	return ""
}

// createChunk creates a new chunk with metadata
func (p *Parser) createChunk(content string, position int) core.Chunk {
	metadata := map[string]string{
		"type":     "log",
		"position": fmt.Sprintf("%d", position),
	}

	if p.format != Unknown {
		metadata["format"] = p.format.String()
	}

	return core.Chunk{
		ID:       uuid.New().String(),
		Content:  content,
		Metadata: metadata,
	}
}

// String converts Format to string
func (f Format) String() string {
	switch f {
	case Nginx:
		return "nginx"
	case Apache:
		return "apache"
	case Syslog:
		return "syslog"
	case Custom:
		return "custom"
	default:
		return "unknown"
	}
}

// SupportedFormats returns supported formats
func (p *Parser) SupportedFormats() []string {
	return []string{".log", ".txt"}
}

// DetectFormat auto-detects log format from first few lines
func (p *Parser) DetectFormat(r io.Reader) (Format, error) {
	scanner := bufio.NewScanner(r)

	// Read first few lines to detect format
	linesRead := 0
	for scanner.Scan() && linesRead < 5 {
		line := scanner.Text()
		linesRead++

		if format := p.identifyFormat(line); format != Unknown {
			p.format = format
			return format, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return Unknown, err
	}

	// Default to unknown
	p.format = Unknown
	return Unknown, nil
}

// identifyFormat identifies format from a single line
func (p *Parser) identifyFormat(line string) Format {
	// Check for nginx/apache format
	if matched, _ := regexp.MatchString(`^\S+\s+\S+\s+\S+\s+\[`, line); matched {
		return Nginx
	}

	// Check for syslog format
	if matched, _ := regexp.MatchString(`^\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`, line); matched {
		return Syslog
	}

	return Unknown
}
