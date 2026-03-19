package dbschema

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBSchemaStreamParser_ParseStream(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sqlContent := []byte(`CREATE TABLE users (
    id INT PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(255)
);`)

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(sqlContent), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestDBSchemaStreamParser_TableAndIndexExtraction(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	ctx := context.Background()

	sqlContent := []byte(`CREATE TABLE products (
    id INT PRIMARY KEY,
    name VARCHAR(100),
    price DECIMAL(10,2)
);

CREATE INDEX idx_name ON products(name);`)

	var docCount int
	var foundTable bool
	var foundIndex bool

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(sqlContent), nil)
	require.NoError(t, err)

	for doc := range docChan {
		docCount++
		assert.NotEmpty(t, doc.ID)
		assert.Equal(t, "dbschema", doc.Metadata["type"])
		if doc.Metadata["chunk_type"] == "table" {
			foundTable = true
		}
		if doc.Metadata["chunk_type"] == "index" {
			foundIndex = true
		}
	}

	assert.Greater(t, docCount, 0)
	assert.True(t, foundTable, "Should find at least one table")
	assert.True(t, foundIndex, "Should find at least one index")
}

func TestDBSchemaStreamParser_TableExtraction(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	parser.extractTables = true
	ctx := context.Background()

	sqlContent := []byte(`CREATE TABLE orders (
    id INT PRIMARY KEY,
    user_id INT,
    total DECIMAL(10,2)
);`)

	var foundOrders bool

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(sqlContent), nil)
	require.NoError(t, err)

	for doc := range docChan {
		if doc.Metadata["chunk_type"] == "table" {
			if doc.Metadata["table_name"] == "orders" {
				foundOrders = true
			}
		}
	}

	assert.True(t, foundOrders, "Should extract orders table")
}

func TestDBSchemaStreamParser_IndexExtraction(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	parser.extractIndexes = true
	ctx := context.Background()

	sqlContent := []byte(`CREATE INDEX idx_email ON users(email);`)

	var foundIndex bool

	docChan, err := parser.ParseStream(ctx, bytes.NewReader(sqlContent), nil)
	require.NoError(t, err)

	for doc := range docChan {
		if doc.Metadata["chunk_type"] == "index" {
			if doc.Metadata["object_name"] == "idx_email" {
				foundIndex = true
			}
		}
	}

	assert.True(t, foundIndex, "Should extract index")
}

func TestDBSchemaStreamParser_EmptySQL(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	ctx := context.Background()

	sqlContent := []byte(``)
	docChan, err := parser.ParseStream(ctx, bytes.NewReader(sqlContent), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Empty(t, docs)
}

func TestDBSchemaStreamParser_LargeSQL(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	parser.chunkSize = 100
	ctx := context.Background()

	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("CREATE TABLE table_%d (id INT, name VARCHAR(100));\n", i))
	}

	docChan, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.NotEmpty(t, docs)
}

func TestDBSchemaStreamParser_ContextCancellation(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	ctx, cancel := context.WithCancel(context.Background())

	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("CREATE TABLE t%d (id INT);\n", i))
	}

	cancel()
	docChan, err := parser.ParseStream(ctx, strings.NewReader(sb.String()), nil)
	require.NoError(t, err)

	docs := make([]*core.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Empty(t, docs)
}

func TestDBSchemaStreamParser_ChunkConfiguration(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	parser.chunkSize = 400
	parser.chunkOverlap = 40

	assert.Equal(t, 400, parser.chunkSize)
	assert.Equal(t, 40, parser.chunkOverlap)
}

func TestDBSchemaStreamParser_ConfigurationOptions(t *testing.T) {
	parser := NewDBSchemaStreamParser()

	parser.extractTables = false
	parser.extractColumns = false
	parser.extractIndexes = false

	assert.False(t, parser.extractTables)
	assert.False(t, parser.extractColumns)
	assert.False(t, parser.extractIndexes)
}

func TestDBSchemaStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewDBSchemaStreamParser()
	formats := parser.GetSupportedTypes()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".sql", formats[0])
}

func TestDBSchemaStreamParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewDBSchemaStreamParser()
	ctx := context.Background()

	// Read all files in .data directory
	files, err := os.ReadDir(dataDir)
	require.NoError(t, err, "Failed to read .data directory")
	require.NotEmpty(t, files, "No files found in .data directory")

	// Test each file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(dataDir, file.Name())
		t.Run(file.Name(), func(t *testing.T) {
			// Read file content
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Failed to read test file: %s", filePath)

			// Create reader from file content
			reader := bytes.NewReader(content)

			// Parse the file
			docChan, err := parser.ParseStream(ctx, reader, nil)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify documents
			docCount := 0
			for doc := range docChan {
				docCount++
				assert.NotEmpty(t, doc.ID, "Document should have an ID")
				assert.NotEmpty(t, doc.Content, "Document should have content")
				assert.Equal(t, "dbschema", doc.Metadata["type"], "Document should have type 'dbschema'")
			}
			assert.Greater(t, docCount, 0, "Should have at least one document")
		})
	}
}
