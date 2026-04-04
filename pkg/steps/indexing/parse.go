package stepinx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/config/types"
)

// multiFactory parses documents using a dynamic factory registry to ensure thread-safety
type multiFactory struct {
	registry *types.ParserRegistry
}

// MultiFactory creates a new multi-parser step that dynamically spawns parsers.
func MultiFactory(registry *types.ParserRegistry) pipeline.Step[*core.IndexingContext] {
	return &multiFactory{registry: registry}
}

// Name returns the step name
func (s *multiFactory) Name() string {
	return "ParseFactory"
}

// Execute streams and parses documents from the file.
func (s *multiFactory) Execute(ctx context.Context, state *core.IndexingContext) error {
	if s.registry == nil {
		return fmt.Errorf("no parser registry configured")
	}

	ext := strings.ToLower(filepath.Ext(state.FilePath))
	parser, ok := s.registry.CreateByExtension(ext)
	if !ok {
		return fmt.Errorf("no parser factory found for file extension: %s", ext)
	}

	// Open file for streaming parse
	file, err := os.Open(state.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	// Build metadata map
	metadataMap := map[string]any{
		"source":   state.Metadata.Source,
		"filename": state.Metadata.FileName,
		"size":     state.Metadata.Size,
		"mod_time": state.Metadata.ModTime,
	}

	// Stream parse the file using the thread-safe, newly created parser instance
	docChan, err := parser.ParseStream(ctx, file, metadataMap)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Pass parsed documents to next step via channel.
	// Since parser.ParseStream starts a goroutine to read from the file, we CANNOT close it here.
	// We must delegate the closure to the parser or wrap the file.
	// We'll wrap the channel to close the file when the parser finishes.
	wrappedDocChan := make(chan *core.Document)
	go func() {
		defer file.Close()
		for doc := range docChan {
			wrappedDocChan <- doc
		}
		close(wrappedDocChan)
	}()

	state.Documents = wrappedDocChan

	return nil
}

// legacy multi parses documents using multiple instances (deprecated, use MultiFactory)
type multi struct {
	parsers []core.Parser
}

// Multi creates a new multi-parser step supporting multiple parsers.
// Deprecated: Use MultiFactory to prevent concurrency and state-sharing bugs.
func Multi(parsers ...core.Parser) pipeline.Step[*core.IndexingContext] {
	return &multi{parsers: parsers}
}

// Name returns the step name
func (s *multi) Name() string {
	return "Parse"
}

// selectParser intelligently routes to the appropriate parser based on file extension.
func (s *multi) selectParser(filePath string) (core.Parser, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	for _, parser := range s.parsers {
		supportedTypes := parser.GetSupportedTypes()
		for _, supportedType := range supportedTypes {
			if strings.ToLower(supportedType) == ext {
				return parser, nil
			}
		}
	}

	return nil, fmt.Errorf("no parser found for file extension: %s", ext)
}

// Execute streams and parses documents from the file.
func (s *multi) Execute(ctx context.Context, state *core.IndexingContext) error {
	if len(s.parsers) == 0 {
		return fmt.Errorf("no parsers configured")
	}

	// Select appropriate parser based on file type
	parser, err := s.selectParser(state.FilePath)
	if err != nil {
		return err
	}

	// Open file for streaming parse
	file, err := os.Open(state.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	// Build metadata map
	metadataMap := map[string]any{
		"source":   state.Metadata.Source,
		"filename": state.Metadata.FileName,
		"size":     state.Metadata.Size,
		"mod_time": state.Metadata.ModTime,
	}

	// Stream parse the file
	docChan, err := parser.ParseStream(ctx, file, metadataMap)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to parse file: %w", err)
	}

	wrappedDocChan := make(chan *core.Document)
	go func() {
		defer file.Close()
		for doc := range docChan {
			wrappedDocChan <- doc
		}
		close(wrappedDocChan)
	}()

	// Pass parsed documents to next step via channel
	state.Documents = wrappedDocChan

	return nil
}
