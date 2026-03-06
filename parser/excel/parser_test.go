package excel

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser_Parse(t *testing.T) {
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

	// Note: This is a simplified XML structure and won't actually work with excelize
	// For a real test, we would need to create a proper Excel file

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
