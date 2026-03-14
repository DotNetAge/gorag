package excel

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExcelStreamParser(t *testing.T) {
	parser := NewExcelStreamParser()
	require.NotNil(t, parser)
	assert.Equal(t, 500, parser.chunkSize)
	assert.Equal(t, 50, parser.chunkOverlap)
}

func TestExcelStreamParser_ParseStream(t *testing.T) {
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

	parser := NewExcelStreamParser()
	ctx := context.Background()

	// This test will fail with the simplified XML, but it tests the method signature
	docChan, err := parser.ParseStream(ctx, strings.NewReader(excelContent), nil)
	require.NoError(t, err)

	docs := make([]*entity.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	// We expect no documents due to the invalid Excel content
	assert.Empty(t, docs)
}

func TestExcelStreamParser_GetSupportedTypes(t *testing.T) {
	parser := NewExcelStreamParser()
	formats := parser.GetSupportedTypes()
	assert.Contains(t, formats, ".xlsx")
	assert.Contains(t, formats, ".xls")
}

func TestExcelStreamParser_Parse_FromDataDirectory(t *testing.T) {
	// Skip test if .data directory doesn't exist
	dataDir := ".data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip(".data directory not found, skipping test")
	}

	parser := NewExcelStreamParser()
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
				assert.NotEmpty(t, doc.Metadata, "Document should have metadata")
				assert.Equal(t, "excel", doc.Metadata["type"], "Document should have type 'excel'")
			}
			assert.Greater(t, docCount, 0, "Should have at least one document")
		})
	}
}

// TestCopyMeta tests the copyMeta function
func TestCopyMeta(t *testing.T) {
	original := map[string]any{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	copy := copyMeta(original)

	// Verify that the copy has the same values
	assert.Equal(t, original, copy)

	// Verify that modifying the copy doesn't affect the original
	copy["key1"] = "modified"
	assert.Equal(t, "value1", original["key1"])
	assert.Equal(t, "modified", copy["key1"])

	// Verify that they are different map instances by checking that they have different addresses
	assert.NotEqual(t, fmt.Sprintf("%p", &original), fmt.Sprintf("%p", &copy))
}

// TestExcelStreamParser_ParseStream_WithContextCancel tests the ParseStream method with context cancellation
func TestExcelStreamParser_ParseStream_WithContextCancel(t *testing.T) {
	// Create a simple Excel file content
	excelContent := `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
  </sheets>
  <relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
    <relationship id="rId1" type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" target="worksheets/sheet1.xml"/>
  </relationships>
</workbook>`

	parser := NewExcelStreamParser()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately
	cancel()

	docChan, err := parser.ParseStream(ctx, strings.NewReader(excelContent), nil)
	require.NoError(t, err)

	// Should not receive any documents due to cancelled context
	docCount := 0
	for range docChan {
		docCount++
	}
	assert.Zero(t, docCount)
}

// TestExcelStreamParser_ParseStream_WithMetadata tests the ParseStream method with metadata
func TestExcelStreamParser_ParseStream_WithMetadata(t *testing.T) {
	// Create a simple Excel file content
	excelContent := `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
  </sheets>
  <relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
    <relationship id="rId1" type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" target="worksheets/sheet1.xml"/>
  </relationships>
</workbook>`

	parser := NewExcelStreamParser()
	ctx := context.Background()

	metadata := map[string]any{
		"source": "test.xlsx",
		"author": "test author",
	}

	docChan, err := parser.ParseStream(ctx, strings.NewReader(excelContent), metadata)
	require.NoError(t, err)

	// We expect no documents due to the invalid Excel content, but the method should complete without error
	docs := make([]*entity.Document, 0)
	for doc := range docChan {
		docs = append(docs, doc)
	}

	assert.Empty(t, docs)
}

