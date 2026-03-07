package rag

import (
	"github.com/DotNetAge/gorag/parser/docx"
	"github.com/DotNetAge/gorag/parser/excel"
	"github.com/DotNetAge/gorag/parser/html"
	"github.com/DotNetAge/gorag/parser/image"
	"github.com/DotNetAge/gorag/parser/json"
	"github.com/DotNetAge/gorag/parser/pdf"
	"github.com/DotNetAge/gorag/parser/ppt"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/parser/yaml"
)

// loadDefaultParsers loads all built-in parsers
func (e *Engine) loadDefaultParsers() {
	// Text parser
	textParser := text.NewParser()
	for _, format := range textParser.SupportedFormats() {
		e.parsers[format] = textParser
	}

	// JSON parser
	jsonParser := json.NewParser()
	for _, format := range jsonParser.SupportedFormats() {
		e.parsers[format] = jsonParser
	}

	// HTML parser
	htmlParser := html.NewParser()
	for _, format := range htmlParser.SupportedFormats() {
		e.parsers[format] = htmlParser
	}

	// YAML parser
	yamlParser := yaml.NewParser()
	for _, format := range yamlParser.SupportedFormats() {
		e.parsers[format] = yamlParser
	}

	// Excel parser
	excelParser := excel.NewParser()
	for _, format := range excelParser.SupportedFormats() {
		e.parsers[format] = excelParser
	}

	// PDF parser
	pdfParser := pdf.NewParser()
	for _, format := range pdfParser.SupportedFormats() {
		e.parsers[format] = pdfParser
	}

	// DOCX parser
	docxParser := docx.NewParser()
	for _, format := range docxParser.SupportedFormats() {
		e.parsers[format] = docxParser
	}

	// PPT parser
	pptParser := ppt.NewParser()
	for _, format := range pptParser.SupportedFormats() {
		e.parsers[format] = pptParser
	}

	// Image parser
	imageParser := image.New()
	for _, format := range imageParser.SupportedFormats() {
		e.parsers[format] = imageParser
	}
}
