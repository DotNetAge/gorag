package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMain initializes the default registry before running tests
func TestMain(m *testing.M) {
	// Ensure DefaultRegistry is initialized before any tests run
	DefaultRegistry.EnsureInitialized()
	m.Run()
}

func TestParserTypeString(t *testing.T) {
	tests := []struct {
		name     string
		typeVal  ParserType
		expected string
	}{
		{"TEXT", TEXT, "text"},
		{"MARKDOWN", MARKDOWN, "markdown"},
		{"GOCODE", GOCODE, "gocode"},
		{"JAVACODE", JAVACODE, "javacode"},
		{"PYCODE", PYCODE, "pycode"},
		{"TSCODE", TSCODE, "tscode"},
		{"JSCODE", JSCODE, "jscode"},
		{"EXCEL", EXCEL, "excel"},
		{"CSV", CSV, "csv"},
		{"JSON", JSON, "json"},
		{"XML", XML, "xml"},
		{"YAML", YAML, "yaml"},
		{"LOG", LOG, "log"},
		{"HTML", HTML, "html"},
		{"EMAIL", EMAIL, "email"},
		{"DBSCHEMA", DBSCHEMA, "dbschema"},
		{"UNKNOWN", UNKNOWN, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.typeVal.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParserRegistry(t *testing.T) {
	// Create a new empty registry for testing (not initialized)
	// This tests the registry behavior before initialization
	registry := NewParserRegistry()

	// Manually ensure it's initialized to test normal behavior
	registry.EnsureInitialized()

	// Test initialized registry should have parsers
	allParsers := registry.GetAll()
	assert.NotEmpty(t, allParsers)

	// Test GetByTypes with initialized registry
	parsers := registry.GetByTypes(TEXT, MARKDOWN)
	assert.Len(t, parsers, 2)
}

func TestDefaultRegistry(t *testing.T) {
	// Test that default registry is initialized
	allParsers := DefaultRegistry.GetAll()
	assert.NotEmpty(t, allParsers)
	assert.GreaterOrEqual(t, len(allParsers), 15) // Should have 15+ parsers

	// Test Parsers function
	textParsers := Parsers(TEXT, MARKDOWN)
	assert.NotEmpty(t, textParsers)

	// Test AllParsers function
	all := AllParsers()
	assert.NotEmpty(t, all)
	assert.Equal(t, len(all), len(allParsers))
}

func TestDefaultParser(t *testing.T) {
	// Test successful parser creation
	parser, err := DefaultParser(TEXT)
	assert.NoError(t, err)
	assert.NotNil(t, parser)

	// Test invalid parser type
	invalidParser, err := DefaultParser(UNKNOWN)
	assert.Error(t, err)
	assert.Nil(t, invalidParser)
	assert.Equal(t, ErrParserNotFound, err)
}
