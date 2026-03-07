package parser_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/parser"
	"github.com/DotNetAge/gorag/parser/html"
	"github.com/DotNetAge/gorag/parser/json"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/utils"
)

const (
	testDir     = "./testdata"
	largeFileMB = 10 // 10MB test file
)

func TestLargeFileParsing(t *testing.T) {
	// Create test directory if it doesn't exist
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test files
	testFiles := []struct {
		name     string
		generate func(string, int) error
		ext      string
		parser   parser.Parser
	}{
		{
			name:     "text",
			generate: utils.GenerateLargeTextFile,
			ext:      ".txt",
			parser:   text.NewParser(),
		},
		{
			name:     "json",
			generate: utils.GenerateLargeJSONFile,
			ext:      ".json",
			parser:   json.NewParser(),
		},
		{
			name:     "html",
			generate: utils.GenerateLargeHTMLFile,
			ext:      ".html",
			parser:   html.NewParser(),
		},
	}

	for _, test := range testFiles {
		t.Run(test.name, func(t *testing.T) {
			// Generate large file
			filePath := filepath.Join(testDir, test.name+"_large"+test.ext)
			err := test.generate(filePath, largeFileMB)
			if err != nil {
				t.Fatalf("Failed to generate large %s file: %v", test.name, err)
			}
			defer os.Remove(filePath)

			// Open file
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("Failed to open file: %v", err)
			}
			defer f.Close()

			// Test standard Parse method
			t.Run("Parse", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				start := time.Now()
				chunks, err := test.parser.Parse(ctx, f)
				if err != nil {
					t.Fatalf("Failed to parse %s file: %v", test.name, err)
				}
				duration := time.Since(start)

				t.Logf("Parsed %s file in %v, generated %d chunks", test.name, duration, len(chunks))

				// Verify we got some chunks
				if len(chunks) == 0 {
					t.Error("Expected at least one chunk, got zero")
				}
			})

			// Test ParseWithCallback method
			t.Run("ParseWithCallback", func(t *testing.T) {
				// Reset file position
				_, err := f.Seek(0, 0)
				if err != nil {
					t.Fatalf("Failed to seek file: %v", err)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				start := time.Now()
				var chunkCount int
				err = test.parser.(parser.StreamingParser).ParseWithCallback(ctx, f, func(chunk parser.Chunk) error {
					chunkCount++
					return nil
				})
				if err != nil {
					t.Fatalf("Failed to parse %s file with callback: %v", test.name, err)
				}
				duration := time.Since(start)

				t.Logf("Parsed %s file with callback in %v, generated %d chunks", test.name, duration, chunkCount)

				// Verify we got some chunks
				if chunkCount == 0 {
					t.Error("Expected at least one chunk, got zero")
				}
			})
		})
	}
}
