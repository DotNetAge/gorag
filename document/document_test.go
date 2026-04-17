package document

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDataDir = ".test"

func openTestFile(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join(testDataDir, name))
	if err != nil {
		t.Fatalf("failed to open test file %s: %v", name, err)
	}
	return f
}

func skipIfFileMissing(t *testing.T, name string) {
	t.Helper()
	path := filepath.Join(testDataDir, name)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Skipf("Skipping: test file %s not found", name)
	}
	if info.Size() < 1024 {
		t.Skipf("Skipping: test file %s is only %d bytes (possible LFS placeholder)", name, info.Size())
	}
}

// ========================= ParseCSV =========================

func TestParseCSV_SimpleFile(t *testing.T) {
	f := openTestFile(t, "simple.csv")
	defer f.Close()

	doc, err := ParseCSV(f)
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseCSV returned empty text")
	}

	// 验证输出是 Markdown 表格格式
	if !strings.Contains(doc.Text, "|") {
		t.Fatal("ParseCSV output should contain table markers '|'")
	}
	if !strings.Contains(doc.Text, "---") {
		t.Fatal("ParseCSV output should contain table separator '---'")
	}

	// 验证 meta 中的 rows 和 columns
	rows, ok := doc.Meta["rows"]
	if !ok {
		t.Fatal("Meta should contain 'rows'")
	}
	if rows.(int) <= 0 {
		t.Fatalf("Expected rows > 0, got %d", rows)
	}

	cols, ok := doc.Meta["columns"]
	if !ok {
		t.Fatal("Meta should contain 'columns'")
	}
	if cols.(int) <= 0 {
		t.Fatalf("Expected columns > 0, got %d", cols)
	}

	t.Logf("CSV: %d rows, %d columns, text length: %d", rows, cols, len(doc.Text))
}

func TestParseCSV_EmptyInput(t *testing.T) {
	doc, err := ParseCSV(strings.NewReader(""))
	if err == nil {
		t.Fatal("Expected error for empty CSV input")
	}
	if doc != nil {
		t.Fatal("Expected nil doc for empty CSV input")
	}
}

func TestParseCSV_DataIntegrity(t *testing.T) {
	input := `Name,Age,City
Alice,30,Beijing
Bob,25,Shanghai
Charlie,35,Guangzhou`

	doc, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	// 第一行应该是表头
	lines := strings.Split(strings.TrimSpace(doc.Text), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines (header + separator), got %d", len(lines))
	}

	header := lines[0]
	if !strings.Contains(header, "Name") || !strings.Contains(header, "Age") || !strings.Contains(header, "City") {
		t.Fatalf("Header row should contain 'Name', 'Age', 'City', got: %s", header)
	}

	if !strings.Contains(doc.Text, "Alice") || !strings.Contains(doc.Text, "Beijing") {
		t.Fatal("CSV data rows should contain 'Alice' and 'Beijing'")
	}

	if doc.Meta["rows"].(int) != 4 {
		t.Fatalf("Expected 4 rows (1 header + 3 data), got %d", doc.Meta["rows"])
	}
	if doc.Meta["columns"].(int) != 3 {
		t.Fatalf("Expected 3 columns, got %d", doc.Meta["columns"])
	}
}

func TestParseCSV_PipeEscaping(t *testing.T) {
	input := `Col1,Col2
a|b,c|d`

	doc, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	// 管道符应该被转义
	if !strings.Contains(doc.Text, `a\|b`) {
		t.Fatal("Pipe characters in CSV fields should be escaped with backslash")
	}
}

// ========================= ParseText =========================

func TestParseText_SimpleInput(t *testing.T) {
	input := "Hello, World!\nThis is a test."
	doc, err := ParseText(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}
	if doc.Text != input {
		t.Fatalf("ParseText should return input verbatim, got: %q", doc.Text)
	}
}

func TestParseText_EmptyInput(t *testing.T) {
	doc, err := ParseText(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseText failed for empty input: %v", err)
	}
	if doc.Text != "" {
		t.Fatalf("Expected empty text, got: %q", doc.Text)
	}
}

func TestParseText_UTF8(t *testing.T) {
	input := "你好，世界！\n这是一段中文测试内容。"
	doc, err := ParseText(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}
	if doc.Text != input {
		t.Fatal("ParseText should preserve UTF-8 content verbatim")
	}
}

// ========================= ParseHTML =========================

func TestParseHTML_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.html")
	f := openTestFile(t, "simple.html")
	defer f.Close()

	doc, err := ParseHTML(f)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseHTML returned empty text")
	}

	// 验证 title 被正确提取
	title, ok := doc.Meta["title"]
	if !ok {
		t.Fatal("Meta should contain 'title'")
	}
	titleStr := title.(string)
	if titleStr == "" {
		t.Fatal("Title should not be empty for simple.html")
	}
	if !strings.Contains(titleStr, "AI Search") && !strings.Contains(titleStr, "RAG") {
		t.Fatalf("Title should contain relevant keywords, got: %q", titleStr)
	}

	// HTML 标签不应该出现在输出中
	if strings.Contains(doc.Text, "<html") || strings.Contains(doc.Text, "</div>") {
		t.Fatal("Output should not contain raw HTML tags")
	}

	// 标题应该被转换为 Markdown ATX 格式
	if !strings.Contains(doc.Text, "# ") {
		t.Fatal("Output should contain Markdown headings (# )")
	}

	t.Logf("HTML title: %q", titleStr)
	t.Logf("HTML output length: %d", len(doc.Text))
}

func TestParseHTML_DataIntegrity(t *testing.T) {
	input := `<html><head><title>Test Title</title></head>
<body>
<h1>Main Heading</h1>
<p>This is a paragraph with <strong>bold</strong> and <em>italic</em> text.</p>
<ul><li>Item 1</li><li>Item 2</li></ul>
<table><tr><th>Name</th><th>Value</th></tr>
<tr><td>A</td><td>1</td></tr></table>
<a href="https://example.com">Link</a>
</body></html>`

	doc, err := ParseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	// 验证 title
	if doc.Meta["title"].(string) != "Test Title" {
		t.Fatalf("Expected title 'Test Title', got: %q", doc.Meta["title"])
	}

	// 验证内容元素
	content := doc.Text
	checkContains(t, content, "Main Heading", "heading text")
	checkContains(t, content, "bold", "bold text")
	checkContains(t, content, "italic", "italic text")
	checkContains(t, content, "Item 1", "list item")
	checkContains(t, content, "Name", "table header")
	checkContains(t, content, "https://example.com", "link URL")

	// 不应包含 HTML 标签
	if strings.Contains(content, "<h1>") || strings.Contains(content, "</p>") {
		t.Fatal("Output should not contain raw HTML tags")
	}
}

func TestParseHTML_EmptyInput(t *testing.T) {
	doc, err := ParseHTML(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseHTML failed for empty input: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseHTML should not return nil for empty input")
	}
}

func TestParseHTML_NoTitle(t *testing.T) {
	input := `<html><body><p>Hello</p></body></html>`
	doc, err := ParseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}
	if _, ok := doc.Meta["title"]; ok {
		t.Fatal("Meta should not contain 'title' when HTML has no <title> tag")
	}
	if !strings.Contains(doc.Text, "Hello") {
		t.Fatal("Output should contain 'Hello'")
	}
}

func TestParseHTML_TitleEntityDecoding(t *testing.T) {
	input := `<html><head><title>Test &amp; Title &lt;3&gt;</title></head><body><p>body</p></body></html>`
	doc, err := ParseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}
	title := doc.Meta["title"].(string)
	if !strings.Contains(title, "&") || !strings.Contains(title, "<") || !strings.Contains(title, ">") {
		t.Fatalf("HTML entities should be decoded in title, got: %q", title)
	}
}

// ========================= ParseImage =========================

func TestParseImage_JPEG(t *testing.T) {
	skipIfFileMissing(t, "simple.jpg")
	f := openTestFile(t, "simple.jpg")
	defer f.Close()

	doc, err := ParseImage(f)
	if err != nil {
		t.Fatalf("ParseImage failed for JPEG: %v", err)
	}

	// 输出应该是 base64 编码的缩略图
	if doc.Text == "" {
		t.Fatal("ParseImage returned empty text")
	}

	_, err = base64.StdEncoding.DecodeString(doc.Text)
	if err != nil {
		t.Fatalf("ParseImage output should be valid base64, decode error: %v", err)
	}

	// MIME 类型应该是 image/jpeg
	mime, ok := doc.Meta["mime_type"]
	if !ok {
		t.Fatal("Meta should contain 'mime_type'")
	}
	if mime != "image/jpeg" {
		t.Fatalf("Expected mime_type 'image/jpeg', got: %q", mime)
	}

	size, ok := doc.Meta["thumbnail_size"]
	if !ok {
		t.Fatal("Meta should contain 'thumbnail_size'")
	}
	if size != thumbnailSize {
		t.Fatalf("Expected thumbnail_size %d, got %d", thumbnailSize, size)
	}

	t.Logf("JPEG: mime=%s, thumbnail_size=%d, base64_len=%d", mime, size, len(doc.Text))
}

func TestParseImage_PNG(t *testing.T) {
	skipIfFileMissing(t, "simple.png")
	f := openTestFile(t, "simple.png")
	defer f.Close()

	doc, err := ParseImage(f)
	if err != nil {
		t.Fatalf("ParseImage failed for PNG: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseImage returned empty text")
	}

	_, err = base64.StdEncoding.DecodeString(doc.Text)
	if err != nil {
		t.Fatalf("ParseImage output should be valid base64, decode error: %v", err)
	}

	mime := doc.Meta["mime_type"]
	if mime != "image/png" {
		t.Fatalf("Expected mime_type 'image/png', got: %q", mime)
	}
}

func TestParseImage_InvalidInput(t *testing.T) {
	_, err := ParseImage(strings.NewReader("not an image"))
	if err == nil {
		t.Fatal("Expected error for non-image input")
	}
}

func TestParseImage_EmptyInput(t *testing.T) {
	_, err := ParseImage(strings.NewReader(""))
	if err == nil {
		t.Fatal("Expected error for empty input")
	}
}

// ========================= ParsePDF =========================

func TestParsePDF_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.pdf")
	f := openTestFile(t, "simple.pdf")
	defer f.Close()

	doc, err := ParsePDF(f)
	if err != nil {
		t.Fatalf("ParsePDF failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParsePDF returned empty text")
	}

	// 验证 meta 信息
	pages, ok := doc.Meta["pages"]
	if !ok {
		t.Fatal("Meta should contain 'pages'")
	}
	if pages.(int) <= 0 {
		t.Fatalf("Expected pages > 0, got %d", pages)
	}

	// 输出应包含页面标记
	if !strings.Contains(doc.Text, "Page") {
		t.Fatal("PDF output should contain 'Page' markers")
	}

	t.Logf("PDF: %d pages, text length: %d", pages, len(doc.Text))
}

func TestParsePDF_InvalidInput(t *testing.T) {
	_, err := ParsePDF(strings.NewReader("not a pdf"))
	if err == nil {
		t.Fatal("Expected error for non-PDF input")
	}
}

func TestParsePDF_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.pdf")
	f := openTestFile(t, "simple.pdf")
	defer f.Close()

	doc, err := ParsePDF(f)
	if err != nil {
		t.Fatalf("ParsePDF failed: %v", err)
	}

	// 验证页面分隔符格式
	if !strings.Contains(doc.Text, "---") {
		t.Fatal("PDF output should contain page separators '---'")
	}

	// 验证 Markdown 标题格式（## Page N）
	if !strings.Contains(doc.Text, "## Page") {
		t.Fatal("PDF output should contain '## Page N' headings")
	}
}

// ========================= ParseDocx =========================

func TestParseDocx_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.docx")
	f := openTestFile(t, "simple.docx")
	defer f.Close()

	doc, err := ParseDocx(f)
	if err != nil {
		t.Fatalf("ParseDocx failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseDocx returned empty text")
	}

	if len(doc.Text) < 10 {
		t.Fatalf("ParseDocx output seems too short: %q", doc.Text)
	}

	t.Logf("DOCX: text length: %d", len(doc.Text))
}

func TestParseDocx_InvalidInput(t *testing.T) {
	_, err := ParseDocx(strings.NewReader("not a docx"))
	if err == nil {
		t.Fatal("Expected error for non-DOCX input")
	}
}

func TestParseDocx_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.docx")
	f := openTestFile(t, "simple.docx")
	defer f.Close()

	doc, err := ParseDocx(f)
	if err != nil {
		t.Fatalf("ParseDocx failed: %v", err)
	}

	// 输出不应包含原始 XML 标签
	if strings.Contains(doc.Text, "<w:t>") || strings.Contains(doc.Text, "<w:p>") {
		t.Fatal("DOCX output should not contain raw XML tags")
	}

	// 验证文档结构 - 段落之间应有换行
	if strings.Count(doc.Text, "\n\n") == 0 && strings.Count(doc.Text, "\n") == 0 {
		t.Fatal("DOCX output should contain newlines between paragraphs")
	}
}

// ========================= ParsePPTX =========================

func TestParsePPTX_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.pptx")
	f := openTestFile(t, "simple.pptx")
	defer f.Close()

	doc, err := ParsePPTX(f)
	if err != nil {
		t.Fatalf("ParsePPTX failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParsePPTX returned empty text")
	}

	// 验证 slide_count
	slideCount, ok := doc.Meta["slide_count"]
	if !ok {
		t.Fatal("Meta should contain 'slide_count'")
	}
	if slideCount.(int) <= 0 {
		t.Fatalf("Expected slide_count > 0, got %d", slideCount)
	}

	t.Logf("PPTX: %d slides, text length: %d", slideCount, len(doc.Text))
}

func TestParsePPTX_InvalidInput(t *testing.T) {
	_, err := ParsePPTX(strings.NewReader("not a pptx"))
	if err == nil {
		t.Fatal("Expected error for non-PPTX input")
	}
}

func TestParsePPTX_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.pptx")
	f := openTestFile(t, "simple.pptx")
	defer f.Close()

	doc, err := ParsePPTX(f)
	if err != nil {
		t.Fatalf("ParsePPTX failed: %v", err)
	}

	// 输出应包含 Slide 标记
	if !strings.Contains(doc.Text, "Slide") {
		t.Fatal("PPTX output should contain 'Slide' markers")
	}

	// 幻灯片之间应有分隔符
	if !strings.Contains(doc.Text, "---") {
		t.Fatal("PPTX output should contain slide separators '---'")
	}

	// 不应包含 XML 标签
	if strings.Contains(doc.Text, "<p:") || strings.Contains(doc.Text, "<a:p>") {
		t.Fatal("PPTX output should not contain raw XML tags")
	}
}

// ========================= ParseXlsx =========================

func TestParseXlsx_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.xlsx")
	f := openTestFile(t, "simple.xlsx")
	defer f.Close()

	doc, err := ParseXlsx(f)
	if err != nil {
		t.Fatalf("ParseXlsx failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseXlsx returned empty text")
	}

	// 验证 sheet_count
	sheetCount, ok := doc.Meta["sheet_count"]
	if !ok {
		t.Fatal("Meta should contain 'sheet_count'")
	}
	if sheetCount.(int) <= 0 {
		t.Fatalf("Expected sheet_count > 0, got %d", sheetCount)
	}

	// 验证输出是 Markdown 表格
	if !strings.Contains(doc.Text, "|") {
		t.Fatal("XLSX output should contain table markers '|'")
	}

	// 验证 Sheet 标记
	if !strings.Contains(doc.Text, "Sheet") {
		t.Fatal("XLSX output should contain 'Sheet' markers")
	}

	t.Logf("XLSX: %d sheets, text length: %d", sheetCount, len(doc.Text))
}

func TestParseXlsx_InvalidInput(t *testing.T) {
	_, err := ParseXlsx(strings.NewReader("not a xlsx"))
	if err == nil {
		t.Fatal("Expected error for non-XLSX input")
	}
}

func TestParseXlsx_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.xlsx")
	f := openTestFile(t, "simple.xlsx")
	defer f.Close()

	doc, err := ParseXlsx(f)
	if err != nil {
		t.Fatalf("ParseXlsx failed: %v", err)
	}

	// 表格应有分隔符
	if !strings.Contains(doc.Text, "---") {
		t.Fatal("XLSX output should contain table separator '---'")
	}

	// 不应包含 XML
	if strings.Contains(doc.Text, "<sheet") || strings.Contains(doc.Text, "<row>") {
		t.Fatal("XLSX output should not contain raw XML tags")
	}
}

// ========================= Parser Registry (dict.go) =========================

func TestGetParserByExt(t *testing.T) {
	tests := []struct {
		ext         string
		description string
	}{
		{".pdf", "PDF extension"},
		{".docx", "DOCX extension"},
		{".pptx", "PPTX extension"},
		{".xlsx", "XLSX extension"},
		{".csv", "CSV extension"},
		{".html", "HTML extension"},
		{".htm", "HTM extension"},
		{".jpg", "JPG extension"},
		{".jpeg", "JPEG extension"},
		{".png", "PNG extension"},
		{".txt", "TXT fallback"},
		{".unknown", "Unknown extension"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			pf := getParserByExt(tt.ext)
			if pf == nil {
				t.Fatalf("getParserByExt(%q) should not return nil for %s", tt.ext, tt.description)
			}
		})
	}
}

func TestGetParserByMIME(t *testing.T) {
	tests := []struct {
		mime        string
		description string
	}{
		{"application/pdf", "PDF MIME"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "DOCX MIME"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "PPTX MIME"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "XLSX MIME"},
		{"text/csv", "CSV MIME"},
		{"text/html", "HTML MIME"},
		{"image/jpeg", "JPEG MIME"},
		{"image/png", "PNG MIME"},
		{"text/plain", "Plain text MIME"},
		{"application/json", "JSON MIME (text/* fallback)"},
		{"application/octet-stream", "Binary MIME (ParseText fallback)"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("mime_%s", strings.ReplaceAll(tt.mime, "/", "_")), func(t *testing.T) {
			pf := getParserByMIME(tt.mime)
			if pf == nil {
				t.Fatalf("getParserByMIME(%q) returned nil for %s", tt.mime, tt.description)
			}
		})
	}
}

// ========================= Open / New Integration =========================

func TestOpen_CSV(t *testing.T) {
	skipIfFileMissing(t, "simple.csv")
	doc, err := Open(filepath.Join(testDataDir, "simple.csv"))
	if err != nil {
		t.Fatalf("Open CSV failed: %v", err)
	}
	if doc.GetContent() == "" {
		t.Fatal("CSV document content should not be empty")
	}
	if doc.GetMimeType() != "text/csv" {
		t.Fatalf("Expected MIME 'text/csv', got: %q", doc.GetMimeType())
	}
	if doc.GetSource() == "" {
		t.Fatal("Source should not be empty")
	}
}

func TestOpen_HTML(t *testing.T) {
	skipIfFileMissing(t, "simple.html")
	doc, err := Open(filepath.Join(testDataDir, "simple.html"))
	if err != nil {
		t.Fatalf("Open HTML failed: %v", err)
	}
	if doc.GetContent() == "" {
		t.Fatal("HTML document content should not be empty")
	}
	if doc.GetMimeType() != "text/html" {
		t.Fatalf("Expected MIME 'text/html', got: %q", doc.GetMimeType())
	}
}

func TestOpen_PDF(t *testing.T) {
	skipIfFileMissing(t, "simple.pdf")
	doc, err := Open(filepath.Join(testDataDir, "simple.pdf"))
	if err != nil {
		t.Fatalf("Open PDF failed: %v", err)
	}
	if doc.GetContent() == "" {
		t.Fatal("PDF document content should not be empty")
	}
	if doc.GetMimeType() != "application/pdf" {
		t.Fatalf("Expected MIME 'application/pdf', got: %q", doc.GetMimeType())
	}
}

func TestOpen_Image(t *testing.T) {
	skipIfFileMissing(t, "simple.jpg")
	doc, err := Open(filepath.Join(testDataDir, "simple.jpg"))
	if err != nil {
		t.Fatalf("Open JPEG failed: %v", err)
	}
	if doc.GetContent() == "" {
		t.Fatal("Image document content should not be empty")
	}
	if doc.GetMimeType() != "image/jpeg" {
		t.Fatalf("Expected MIME 'image/jpeg', got: %q", doc.GetMimeType())
	}
}

func TestNew_HTML(t *testing.T) {
	htmlContent := `<html><head><title>Test</title></head><body><p>Hello</p></body></html>`
	doc := New(htmlContent, "text/html")
	if doc == nil {
		t.Fatal("New should not return nil")
	}
	if doc.GetMimeType() != "text/html" {
		t.Fatalf("Expected MIME 'text/html', got: %q", doc.GetMimeType())
	}
	if doc.GetContent() == "" {
		t.Fatal("HTML document content should not be empty")
	}
	if !strings.Contains(doc.GetContent(), "Hello") {
		t.Fatal("HTML document content should contain 'Hello'")
	}
}

func TestNew_CSV(t *testing.T) {
	csvContent := "a,b,c\n1,2,3"
	doc := New(csvContent, "text/csv")
	if doc == nil {
		t.Fatal("New should not return nil")
	}
	if doc.GetMimeType() != "text/csv" {
		t.Fatalf("Expected MIME 'text/csv', got: %q", doc.GetMimeType())
	}
}

// ========================= Helper =========================

func checkContains(t *testing.T, s, substr, label string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("Output should contain %s %q", label, substr)
	}
}
