package excel

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	// Create a simple Excel file content
	// Note: This is a simplified XML structure and won't actually work with excelize
	// For a real test, we would need to create a proper Excel file
	excelContent := `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
  </sheets>
  <relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
    <relationship id="rId1" type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" target="worksheets/sheet1.xml"/>
  </relationships>
</workbook>`

	parser := NewParser()
	ctx := context.Background()

	// This test will fail with the simplified XML, but it tests the method signature
	_, err := parser.Parse(ctx, strings.NewReader(excelContent))
	// We expect an error due to the invalid Excel content
	assert.Error(t, err)
}

func TestParser_SupportedFormats(t *testing.T) {
	parser := NewParser()
	formats := parser.SupportedFormats()
	assert.Contains(t, formats, ".xlsx")
	assert.Contains(t, formats, ".xls")
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
				assert.NotEmpty(t, chunk.Metadata, "Chunk %d should have metadata", i)
				assert.Equal(t, "excel", chunk.Metadata["type"], "Chunk %d should have type 'excel'", i)
			}
		})
	}
}


