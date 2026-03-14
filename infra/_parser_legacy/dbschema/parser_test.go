package dbschema

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sqlContent := []byte(`CREATE TABLE users (
    id INT PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(255)
);`)

	r := bytes.NewReader(sqlContent)
	chunks, err := parser.Parse(ctx, r)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ParseWithCallback(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	sqlContent := []byte(`CREATE TABLE products (
    id INT PRIMARY KEY,
    name VARCHAR(100),
    price DECIMAL(10,2)
);

CREATE INDEX idx_name ON products(name);`)

	var chunkCount int
	var foundTable bool

	err := parser.ParseWithCallback(ctx, bytes.NewReader(sqlContent), func(chunk model.Chunk) error {
		chunkCount++
		assert.NotEmpty(t, chunk.ID)
		assert.Contains(t, chunk.Metadata["type"], "dbschema")
		if chunk.Metadata["chunk_type"] == "table" {
			foundTable = true
		}
		return nil
	})

	require.NoError(t, err)
	assert.Greater(t, chunkCount, 0)
	assert.True(t, foundTable, "Should find at least one table")
}

func TestParser_TableExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractTables(true)
	ctx := context.Background()

	sqlContent := []byte(`CREATE TABLE orders (
    id INT PRIMARY KEY,
    user_id INT,
    total DECIMAL(10,2)
);`)

	var foundOrders bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(sqlContent), func(chunk model.Chunk) error {
		if chunk.Metadata["chunk_type"] == "table" {
			if chunk.Metadata["table_name"] == "orders" {
				foundOrders = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundOrders, "Should extract orders table")
}

func TestParser_IndexExtraction(t *testing.T) {
	parser := NewParser()
	parser.SetExtractIndexes(true)
	ctx := context.Background()

	sqlContent := []byte(`CREATE INDEX idx_email ON users(email);`)

	var foundIndex bool
	err := parser.ParseWithCallback(ctx, bytes.NewReader(sqlContent), func(chunk model.Chunk) error {
		if chunk.Metadata["chunk_type"] == "index" {
			if chunk.Metadata["object_name"] == "idx_email" {
				foundIndex = true
			}
		}
		return nil
	})

	require.NoError(t, err)
	assert.True(t, foundIndex, "Should extract index")
}

func TestParser_EmptySQL(t *testing.T) {
	parser := NewParser()
	ctx := context.Background()

	sqlContent := []byte(``)
	chunks, err := parser.Parse(ctx, bytes.NewReader(sqlContent))
	require.NoError(t, err)
	_ = chunks
}

func TestParser_LargeSQL(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(100)
	ctx := context.Background()

	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("CREATE TABLE table_%d (id INT, name VARCHAR(100));\n", i))
	}

	chunks, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	require.NoError(t, err)
	assert.NotEmpty(t, chunks)
}

func TestParser_ContextCancellation(t *testing.T) {
	parser := NewParser()
	ctx, cancel := context.WithCancel(context.Background())

	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString(fmt.Sprintf("CREATE TABLE t%d (id INT);\n", i))
	}

	cancel()
	_, err := parser.Parse(ctx, strings.NewReader(sb.String()))
	assert.Error(t, err)
}

func TestParser_CallbackError(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(20)
	ctx := context.Background()

	sqlContent := []byte(`-- Comment 1
-- Comment 2
SELECT 1;`)

	err := parser.ParseWithCallback(ctx, bytes.NewReader(sqlContent), func(chunk model.Chunk) error {
		return assert.AnError
	})
	assert.Error(t, err)
}

func TestParser_ChunkConfiguration(t *testing.T) {
	parser := NewParser()
	parser.SetChunkSize(400)
	parser.SetChunkOverlap(40)

	assert.Equal(t, 400, parser.chunkSize)
	assert.Equal(t, 40, parser.chunkOverlap)
}

func TestParser_ConfigurationOptions(t *testing.T) {
	parser := NewParser()

	parser.SetExtractTables(false)
	parser.SetExtractColumns(false)
	parser.SetExtractIndexes(false)

	assert.False(t, parser.extractTables)
	assert.False(t, parser.extractColumns)
	assert.False(t, parser.extractIndexes)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Len(t, formats, 1)
	assert.Equal(t, ".sql", formats[0])
}

func TestParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewParser()
	ctx := context.Background()

	// Read all files in .data directory
	files, err := ioutil.ReadDir(dataDir)
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
			content, err := ioutil.ReadFile(filePath)
			require.NoError(t, err, "Failed to read test file: %s", filePath)

			// Create reader from file content
			reader := bytes.NewReader(content)

			// Parse the file
			chunks, err := parser.Parse(ctx, reader)
			require.NoError(t, err, "Failed to parse file: %s", filePath)

			// Verify chunks
			for i, chunk := range chunks {
				assert.NotEmpty(t, chunk.ID, "Chunk %d should have an ID", i)
				assert.NotEmpty(t, chunk.Content, "Chunk %d should have content", i)
				assert.Contains(t, chunk.Metadata["type"], "dbschema", "Chunk %d should have type 'dbschema'", i)
			}
		})
	}
}
