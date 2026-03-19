package stepinx

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/DotNetAge/gochat/pkg/pipeline"
)

// multi parses documents using multiple parsers with intelligent routing.
type multi struct {
	parsers []core.Parser
}

// Multi creates a new multi-parser step supporting multiple parsers.
//
// Parameters:
//   - parsers: variadic list of parsers to use
//
// Example:
//
//	p.AddStep(indexing.Multi(parser1, parser2, parser3))
func Multi(parsers ...core.Parser) pipeline.Step[*core.State] {
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
func (s *multi) Execute(ctx context.Context, state *core.State) error {
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
	defer file.Close()

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
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Pass parsed documents to next step via channel
	state.Documents = docChan

	return nil
}
