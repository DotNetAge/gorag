package rag

import (
	"github.com/DotNetAge/gorag/parser/config"
	"github.com/DotNetAge/gorag/parser/csv"
	"github.com/DotNetAge/gorag/parser/dbschema"
	"github.com/DotNetAge/gorag/parser/docx"
	"github.com/DotNetAge/gorag/parser/email"
	"github.com/DotNetAge/gorag/parser/excel"
	"github.com/DotNetAge/gorag/parser/gocode"
	"github.com/DotNetAge/gorag/parser/html"
	"github.com/DotNetAge/gorag/parser/image"
	"github.com/DotNetAge/gorag/parser/javacode"
	"github.com/DotNetAge/gorag/parser/jscode"
	"github.com/DotNetAge/gorag/parser/json"
	"github.com/DotNetAge/gorag/parser/log"
	"github.com/DotNetAge/gorag/parser/markdown"
	"github.com/DotNetAge/gorag/parser/pdf"
	"github.com/DotNetAge/gorag/parser/ppt"
	"github.com/DotNetAge/gorag/parser/pycode"
	"github.com/DotNetAge/gorag/parser/text"
	"github.com/DotNetAge/gorag/parser/tscode"
	"github.com/DotNetAge/gorag/parser/xml"
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

	// Config parser
	configParser := config.NewParser()
	for _, format := range configParser.SupportedFormats() {
		e.parsers[format] = configParser
	}

	// CSV parser
	csvParser := csv.NewParser()
	for _, format := range csvParser.SupportedFormats() {
		e.parsers[format] = csvParser
	}

	// DB schema parser
	dbschemaParser := dbschema.NewParser()
	for _, format := range dbschemaParser.SupportedFormats() {
		e.parsers[format] = dbschemaParser
	}

	// Email parser
	emailParser := email.NewParser()
	for _, format := range emailParser.SupportedFormats() {
		e.parsers[format] = emailParser
	}

	// Go code parser
	gocodeParser := gocode.NewParser()
	for _, format := range gocodeParser.SupportedFormats() {
		e.parsers[format] = gocodeParser
	}

	// Java code parser
	javacodeParser := javacode.NewParser()
	for _, format := range javacodeParser.SupportedFormats() {
		e.parsers[format] = javacodeParser
	}

	// JS code parser
	jscodeParser := jscode.NewParser()
	for _, format := range jscodeParser.SupportedFormats() {
		e.parsers[format] = jscodeParser
	}

	// Log parser
	logParser := log.NewParser()
	for _, format := range logParser.SupportedFormats() {
		e.parsers[format] = logParser
	}

	// Markdown parser
	markdownParser := markdown.NewParser()
	for _, format := range markdownParser.SupportedFormats() {
		e.parsers[format] = markdownParser
	}

	// Python code parser
	pycodeParser := pycode.NewParser()
	for _, format := range pycodeParser.SupportedFormats() {
		e.parsers[format] = pycodeParser
	}

	// TypeScript code parser
	tscodeParser := tscode.NewParser()
	for _, format := range tscodeParser.SupportedFormats() {
		e.parsers[format] = tscodeParser
	}

	// XML parser
	xmlParser := xml.NewParser()
	for _, format := range xmlParser.SupportedFormats() {
		e.parsers[format] = xmlParser
	}
}
