package document

import (
	"encoding/base64"
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
		t.Fatalf("打开测试文件 %s 失败: %v", name, err)
	}
	return f
}

func skipIfFileMissing(t *testing.T, name string) {
	t.Helper()
	path := filepath.Join(testDataDir, name)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Skipf("跳过：测试文件 %s 不存在", name)
	}
	if info.Size() < 1024 {
		t.Skipf("跳过：测试文件 %s 仅 %d 字节（疑似 LFS 占位符）", name, info.Size())
	}
}

// ========================= ParseCSV =========================

func TestParseCSV_SimpleFile(t *testing.T) {
	f := openTestFile(t, "simple.csv")
	defer f.Close()

	doc, err := ParseCSV(f)
	if err != nil {
		t.Fatalf("ParseCSV 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseCSV 返回空内容")
	}

	// 输出应为 JSON 数组字符串
	if !strings.HasPrefix(doc.Content(), "[") {
		t.Fatalf("ParseCSV 输出应为 JSON 数组, 实际前缀: %q", doc.Content()[:min(10, len(doc.Content()))])
	}

	// 验证 meta 中的 rows 和 columns
	meta := doc.Meta()
	rows, ok := meta["rows"]
	if !ok {
		t.Fatal("元数据应包含 'rows'")
	}
	if rows.(int) <= 0 {
		t.Fatalf("期望 rows > 0, 实际 %d", rows)
	}

	cols, ok := meta["columns"]
	if !ok {
		t.Fatal("元数据应包含 'columns'")
	}
	if cols.(int) <= 0 {
		t.Fatalf("期望 columns > 0, 实际 %d", cols)
	}

	//CSV 归一化为 RawDocData
	if doc.Type() != RawDocData {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	t.Logf("CSV: %d 行, %d 列, JSON 长度: %d", rows, cols, len(doc.Content()))
}

func TestParseCSV_EmptyInput(t *testing.T) {
	doc, err := ParseCSV(strings.NewReader(""))
	if err == nil {
		t.Fatal("空 CSV 输入应返回错误")
	}
	if doc != nil {
		t.Fatal("空 CSV 输入应返回 nil doc")
	}
}

func TestParseCSV_DataIntegrity(t *testing.T) {
	input := `Name,Age,City
Alice,30,Beijing
Bob,25,Shanghai
Charlie,35,Guangzhou`

	doc, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV 失败: %v", err)
	}

	// 输出应为 JSON 数组字符串，包含 3 个数据行对象
	content := doc.Content()
	if !strings.HasPrefix(content, "[") {
		t.Fatalf("期望 JSON 数组, 实际: %q", content[:min(20, len(content))])
	}

	// 验证 JSON 包含表头作为 key 和数据值
	checkContains(t, content, "Name", "JSON key")
	checkContains(t, content, "Age", "JSON key")
	checkContains(t, content, "City", "JSON key")
	checkContains(t, content, "Alice", "数据值")
	checkContains(t, content, "Beijing", "数据值")

	meta := doc.Meta()
	if meta["rows"].(int) != 4 {
		t.Fatalf("期望 4 行（1 表头 + 3 数据）, 实际 %d", meta["rows"])
	}
	if meta["columns"].(int) != 3 {
		t.Fatalf("期望 3 列, 实际 %d", meta["columns"])
	}
}

func TestParseCSV_PipeEscaping(t *testing.T) {
	input := `Col1,Col2
a|b,c|d`

	doc, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV 失败: %v", err)
	}

	// JSON 输出中管道符不需转义，应原样保留
	if !strings.Contains(doc.Content(), "a|b") {
		t.Fatal("CSV 字段中的管道符应在 JSON 输出中保留")
	}
}

// min 返回两个整数中的较小值（Go 1.21 之前不自带）。
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ========================= ParseText =========================

func TestParseText_SimpleInput(t *testing.T) {
	input := "Hello, World!\nThis is a test."
	doc, err := ParseText(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseText 失败: %v", err)
	}
	if doc.Content() != input {
		t.Fatalf("ParseText 应原样返回输入, 实际: %q", doc.Content())
	}
	//纯文本归一化为 RawDocText
	if doc.Type() != RawDocText {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocText, doc.Type())
	}
}

func TestParseText_EmptyInput(t *testing.T) {
	doc, err := ParseText(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseText 处理空输入失败: %v", err)
	}
	if doc.Content() != "" {
		t.Fatalf("期望空内容, 实际: %q", doc.Content())
	}
}

func TestParseText_UTF8(t *testing.T) {
	input := "你好，世界！\n这是一段中文测试内容。"
	doc, err := ParseText(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseText 失败: %v", err)
	}
	if doc.Content() != input {
		t.Fatal("ParseText 应原样保留 UTF-8 内容")
	}
}

// ========================= ParseHTML =========================

func TestParseHTML_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.html")
	f := openTestFile(t, "simple.html")
	defer f.Close()

	doc, err := ParseHTML(f)
	if err != nil {
		t.Fatalf("ParseHTML 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseHTML 返回空内容")
	}

	// 验证 title 被正确提取
	meta := doc.Meta()
	title, ok := meta["title"]
	if !ok {
		t.Fatal("元数据应包含 'title'")
	}
	titleStr := title.(string)
	if titleStr == "" {
		t.Fatal("simple.html 的 title 不应为空")
	}
	if !strings.Contains(titleStr, "AI Search") && !strings.Contains(titleStr, "RAG") {
		t.Fatalf("title 应包含相关关键词, 实际: %q", titleStr)
	}

	// HTML 标签不应该出现在输出中
	if strings.Contains(doc.Content(), "<html") || strings.Contains(doc.Content(), "</div>") {
		t.Fatal("输出不应包含原始 HTML 标签")
	}

	// 标题应该被转换为 Markdown ATX 格式
	if !strings.Contains(doc.Content(), "# ") {
		t.Fatal("输出应包含 Markdown 标题 (# )")
	}

	// HTML 归一化为 RawDocDoc
	if doc.Type() != RawDocDoc {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}

	t.Logf("HTML title: %q", titleStr)
	t.Logf("HTML 输出长度: %d", len(doc.Content()))
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
		t.Fatalf("ParseHTML 失败: %v", err)
	}

	// 验证 title
	if doc.Meta()["title"].(string) != "Test Title" {
		t.Fatalf("期望 title 'Test Title', 实际: %q", doc.Meta()["title"])
	}

	// 验证内容元素
	content := doc.Content()
	checkContains(t, content, "Main Heading", "标题文本")
	checkContains(t, content, "bold", "粗体文本")
	checkContains(t, content, "italic", "斜体文本")
	checkContains(t, content, "Item 1", "列表项")
	checkContains(t, content, "Name", "表头")
	checkContains(t, content, "https://example.com", "链接 URL")

	// 不应包含 HTML 标签
	if strings.Contains(content, "<h1>") || strings.Contains(content, "</p>") {
		t.Fatal("输出不应包含原始 HTML 标签")
	}
}

func TestParseHTML_EmptyInput(t *testing.T) {
	doc, err := ParseHTML(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseHTML 处理空输入失败: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseHTML 处理空输入不应返回 nil")
	}
}

func TestParseHTML_NoTitle(t *testing.T) {
	input := `<html><body><p>Hello</p></body></html>`
	doc, err := ParseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHTML 失败: %v", err)
	}
	if _, ok := doc.Meta()["title"]; ok {
		t.Fatal("HTML 无 <title> 标签时元数据不应包含 'title'")
	}
	if !strings.Contains(doc.Content(), "Hello") {
		t.Fatal("输出应包含 'Hello'")
	}
}

func TestParseHTML_TitleEntityDecoding(t *testing.T) {
	input := `<html><head><title>Test &amp; Title &lt;3&gt;</title></head><body><p>body</p></body></html>`
	doc, err := ParseHTML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHTML 失败: %v", err)
	}
	title := doc.Meta()["title"].(string)
	if !strings.Contains(title, "&") || !strings.Contains(title, "<") || !strings.Contains(title, ">") {
		t.Fatalf("title 中的 HTML 实体应被解码, 实际: %q", title)
	}
}

// ========================= ParseImage =========================

func TestParseImage_JPEG(t *testing.T) {
	skipIfFileMissing(t, "simple.jpg")
	f := openTestFile(t, "simple.jpg")
	defer f.Close()

	doc, err := ParseImage(f)
	if err != nil {
		t.Fatalf("ParseImage 处理 JPEG 失败: %v", err)
	}

	// 输出应该是 base64 编码的缩略图
	if doc.Content() == "" {
		t.Fatal("ParseImage 返回空内容")
	}

	_, err = base64.StdEncoding.DecodeString(doc.Content())
	if err != nil {
		t.Fatalf("ParseImage 输出应为合法 base64, 解码错误: %v", err)
	}

	// 图片归一化为 RawDocImage
	if doc.Type() != RawDocImage {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocImage, doc.Type())
	}

	// 图片元数据中保留 mime_type（原始 MIME）
	meta := doc.Meta()
	mime, ok := meta["mime_type"]
	if !ok {
		t.Fatal("元数据应包含 'mime_type'")
	}
	if mime != "image/jpeg" {
		t.Fatalf("期望 mime_type 'image/jpeg', 实际: %q", mime)
	}

	size, ok := meta["thumbnail_size"]
	if !ok {
		t.Fatal("元数据应包含 'thumbnail_size'")
	}
	if size != thumbnailSize {
		t.Fatalf("期望 thumbnail_size %d, 实际 %d", thumbnailSize, size)
	}

	t.Logf("JPEG: mime=%s, thumbnail_size=%d, base64 长度=%d", mime, size, len(doc.Content()))
}

func TestParseImage_PNG(t *testing.T) {
	skipIfFileMissing(t, "simple.png")
	f := openTestFile(t, "simple.png")
	defer f.Close()

	doc, err := ParseImage(f)
	if err != nil {
		t.Fatalf("ParseImage 处理 PNG 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseImage 返回空内容")
	}

	_, err = base64.StdEncoding.DecodeString(doc.Content())
	if err != nil {
		t.Fatalf("ParseImage 输出应为合法 base64, 解码错误: %v", err)
	}

	mime := doc.Meta()["mime_type"]
	if mime != "image/png" {
		t.Fatalf("期望 mime_type 'image/png', 实际: %q", mime)
	}
}

func TestParseImage_InvalidInput(t *testing.T) {
	_, err := ParseImage(strings.NewReader("not an image"))
	if err == nil {
		t.Fatal("非图片输入应返回错误")
	}
}

func TestParseImage_EmptyInput(t *testing.T) {
	_, err := ParseImage(strings.NewReader(""))
	if err == nil {
		t.Fatal("空输入应返回错误")
	}
}

// ========================= ParsePDF =========================

func TestParsePDF_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.pdf")
	f := openTestFile(t, "simple.pdf")
	defer f.Close()

	doc, err := ParsePDF(f)
	if err != nil {
		t.Fatalf("ParsePDF 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParsePDF 返回空内容")
	}

	// 验证 meta 信息
	meta := doc.Meta()
	pages, ok := meta["pages"]
	if !ok {
		t.Fatal("元数据应包含 'pages'")
	}
	if pages.(int) <= 0 {
		t.Fatalf("期望 pages > 0, 实际 %d", pages)
	}

	// 输出应包含页面标记
	if !strings.Contains(doc.Content(), "Page") {
		t.Fatal("PDF 输出应包含 'Page' 标记")
	}

	// PDF 归一化为 RawDocDoc
	if doc.Type() != RawDocDoc {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}

	t.Logf("PDF: %d 页, 文本长度: %d", pages, len(doc.Content()))
}

func TestParsePDF_InvalidInput(t *testing.T) {
	_, err := ParsePDF(strings.NewReader("not a pdf"))
	if err == nil {
		t.Fatal("非 PDF 输入应返回错误")
	}
}

func TestParsePDF_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.pdf")
	f := openTestFile(t, "simple.pdf")
	defer f.Close()

	doc, err := ParsePDF(f)
	if err != nil {
		t.Fatalf("ParsePDF 失败: %v", err)
	}

	// 验证页面分隔符格式
	if !strings.Contains(doc.Content(), "---") {
		t.Fatal("PDF 输出应包含页面分隔符 '---'")
	}

	// 验证 Markdown 标题格式（## Page N）
	if !strings.Contains(doc.Content(), "## Page") {
		t.Fatal("PDF 输出应包含 '## Page N' 标题")
	}
}

// ========================= ParseDocx =========================

func TestParseDocx_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.docx")
	f := openTestFile(t, "simple.docx")
	defer f.Close()

	doc, err := ParseDocx(f)
	if err != nil {
		t.Fatalf("ParseDocx 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseDocx 返回空内容")
	}

	if len(doc.Content()) < 10 {
		t.Fatalf("ParseDocx 输出过短: %q", doc.Content())
	}

	// DOCX 归一化为 RawDocDoc
	if doc.Type() != RawDocDoc {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}

	t.Logf("DOCX: 文本长度: %d", len(doc.Content()))
}

func TestParseDocx_InvalidInput(t *testing.T) {
	_, err := ParseDocx(strings.NewReader("not a docx"))
	if err == nil {
		t.Fatal("非 DOCX 输入应返回错误")
	}
}

func TestParseDocx_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.docx")
	f := openTestFile(t, "simple.docx")
	defer f.Close()

	doc, err := ParseDocx(f)
	if err != nil {
		t.Fatalf("ParseDocx 失败: %v", err)
	}

	// 输出不应包含原始 XML 标签
	if strings.Contains(doc.Content(), "<w:t>") || strings.Contains(doc.Content(), "<w:p>") {
		t.Fatal("DOCX 输出不应包含原始 XML 标签")
	}

	// 验证文档结构 - 段落之间应有换行
	if strings.Count(doc.Content(), "\n\n") == 0 && strings.Count(doc.Content(), "\n") == 0 {
		t.Fatal("DOCX 输出应在段落之间包含换行")
	}
}

// ========================= ParsePPTX =========================

func TestParsePPTX_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.pptx")
	f := openTestFile(t, "simple.pptx")
	defer f.Close()

	doc, err := ParsePPTX(f)
	if err != nil {
		t.Fatalf("ParsePPTX 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParsePPTX 返回空内容")
	}

	// 验证 slide_count
	meta := doc.Meta()
	slideCount, ok := meta["slide_count"]
	if !ok {
		t.Fatal("元数据应包含 'slide_count'")
	}
	if slideCount.(int) <= 0 {
		t.Fatalf("期望 slide_count > 0, 实际 %d", slideCount)
	}

	// PPTX 归一化为 RawDocDoc
	if doc.Type() != RawDocDoc {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}

	t.Logf("PPTX: %d 张幻灯片, 文本长度: %d", slideCount, len(doc.Content()))
}

func TestParsePPTX_InvalidInput(t *testing.T) {
	_, err := ParsePPTX(strings.NewReader("not a pptx"))
	if err == nil {
		t.Fatal("非 PPTX 输入应返回错误")
	}
}

func TestParsePPTX_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.pptx")
	f := openTestFile(t, "simple.pptx")
	defer f.Close()

	doc, err := ParsePPTX(f)
	if err != nil {
		t.Fatalf("ParsePPTX 失败: %v", err)
	}

	// 输出应包含 Slide 标记
	if !strings.Contains(doc.Content(), "Slide") {
		t.Fatal("PPTX 输出应包含 'Slide' 标记")
	}

	// 幻灯片之间应有分隔符
	if !strings.Contains(doc.Content(), "---") {
		t.Fatal("PPTX 输出应包含幻灯片分隔符 '---'")
	}

	// 不应包含 XML 标签
	if strings.Contains(doc.Content(), "<p:") || strings.Contains(doc.Content(), "<a:p>") {
		t.Fatal("PPTX 输出不应包含原始 XML 标签")
	}
}

// ========================= ParseXlsx =========================

func TestParseXlsx_SimpleFile(t *testing.T) {
	skipIfFileMissing(t, "simple.xlsx")
	f := openTestFile(t, "simple.xlsx")
	defer f.Close()

	doc, err := ParseXlsx(f)
	if err != nil {
		t.Fatalf("ParseXlsx 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseXlsx 返回空内容")
	}

	// 验证 sheet_count
	meta := doc.Meta()
	sheetCount, ok := meta["sheet_count"]
	if !ok {
		t.Fatal("元数据应包含 'sheet_count'")
	}
	if sheetCount.(int) <= 0 {
		t.Fatalf("期望 sheet_count > 0, 实际 %d", sheetCount)
	}

	// 输出应为 JSON 数组字符串
	if !strings.HasPrefix(doc.Content(), "[") {
		t.Fatalf("XLSX 输出应为 JSON 数组, 实际前缀: %q", doc.Content()[:min(10, len(doc.Content()))])
	}

	// 验证 JSON 包含 sheet 字段
	checkContains(t, doc.Content(), "sheet", "JSON sheet 字段")

	// XLSX 归一化为 RawDocData
	if doc.Type() != RawDocData {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}

	t.Logf("XLSX: %d 个 sheet, JSON 长度: %d", sheetCount, len(doc.Content()))
}

func TestParseXlsx_InvalidInput(t *testing.T) {
	_, err := ParseXlsx(strings.NewReader("not a xlsx"))
	if err == nil {
		t.Fatal("非 XLSX 输入应返回错误")
	}
}

func TestParseXlsx_EmptyInput(t *testing.T) {
	doc, err := ParseXlsx(strings.NewReader(""))
	if err == nil {
		t.Fatal("空 XLSX 输入应返回错误")
	}
	if doc != nil {
		t.Fatal("空 XLSX 输入应返回 nil doc")
	}
}

func TestParseXlsx_DataIntegrity(t *testing.T) {
	skipIfFileMissing(t, "simple.xlsx")
	f := openTestFile(t, "simple.xlsx")
	defer f.Close()

	doc, err := ParseXlsx(f)
	if err != nil {
		t.Fatalf("ParseXlsx 失败: %v", err)
	}

	// JSON 输出不应包含 XML 标签
	if strings.Contains(doc.Content(), "<sheet") || strings.Contains(doc.Content(), "<row>") {
		t.Fatal("XLSX 输出不应包含原始 XML 标签")
	}
}

// ========================= Parser Registry (dict.go) =========================

func TestGetParserByExt(t *testing.T) {
	tests := []struct {
		ext         string
		description string
	}{
		{".pdf", "PDF 扩展名"},
		{".docx", "DOCX 扩展名"},
		{".pptx", "PPTX 扩展名"},
		{".xlsx", "XLSX 扩展名"},
		{".csv", "CSV 扩展名"},
		{".html", "HTML 扩展名"},
		{".htm", "HTM 扩展名"},
		{".jpg", "JPG 扩展名"},
		{".jpeg", "JPEG 扩展名"},
		{".png", "PNG 扩展名"},
		// 图片类
		{".gif", "GIF 扩展名"},
		{".webp", "WebP 扩展名"},
		{".bmp", "BMP 扩展名"},
		{".tiff", "TIFF 扩展名"},
		{".tif", "TIF 扩展名"},
		// 数据类
		{".json", "JSON 扩展名"},
		{".yml", "YML 扩展名"},
		{".yaml", "YAML 扩展名"},
		{".xml", "XML 扩展名"},
		{".eml", "EML 扩展名"},
		{".msg", "MSG 扩展名"},
		{".toml", "TOML 扩展名"},
		{".log", "LOG 扩展名"},
		// 兜底
		{".txt", "TXT 兜底"},
		{".unknown", "未知扩展名"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			pf := getParserByExt(tt.ext)
			if pf == nil {
				t.Fatalf("getParserByExt(%q) 不应为 nil（%s）", tt.ext, tt.description)
			}
		})
	}
}

func TestGetParserByExt_FallbackIsParseText(t *testing.T) {
	pf := getParserByExt(".unknown-ext")
	if pf == nil {
		t.Fatal("未知扩展名应返回兜底 parser")
	}
	// 兜底 parser 应能处理任意文本输入
	doc, err := pf(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("兜底 parser 处理失败: %v", err)
	}
	if doc.Type() != RawDocText {
		t.Fatalf("兜底 parser 应返回 RawDocText, 实际 %q", doc.Type())
	}
}

// ========================= Open / New Integration =========================

func TestOpen_RejectsEmptyPath(t *testing.T) {
	_, err := Open("")
	if err == nil {
		t.Fatal("Open 应拒绝空路径")
	}
}

func TestOpen_RejectsRelativePath(t *testing.T) {
	_, err := Open("relative/path/file.txt")
	if err == nil {
		t.Fatal("Open 应拒绝相对路径")
	}
}

func TestOpen_RejectsDirectory(t *testing.T) {
	absDir, _ := filepath.Abs(testDataDir)
	_, err := Open(absDir)
	if err == nil {
		t.Fatal("Open 应拒绝目录")
	}
}

func TestOpen_RejectsNonExistentFile(t *testing.T) {
	absPath, _ := filepath.Abs(filepath.Join(testDataDir, "nonexistent.txt"))
	_, err := Open(absPath)
	if err == nil {
		t.Fatal("Open 应拒绝不存在的文件")
	}
}

func TestOpen_JSON(t *testing.T) {
	skipIfFileMissing(t, "simple.json")
	absPath, _ := filepath.Abs(filepath.Join(testDataDir, "simple.json"))
	doc, err := Open(absPath)
	if err != nil {
		t.Fatalf("Open JSON 失败: %v", err)
	}
	if doc.Content() == "" {
		t.Fatal("JSON 文档内容不应为空")
	}
	// JSON 归一化为 RawDocData
	if doc.Type() != RawDocData {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
	// 验证 FileName 回填为绝对路径
	if !filepath.IsAbs(doc.FileName()) {
		t.Fatalf("FileName 应为绝对路径, 实际: %q", doc.FileName())
	}
}

func TestOpen_CSV(t *testing.T) {
	skipIfFileMissing(t, "simple.csv")
	absPath, _ := filepath.Abs(filepath.Join(testDataDir, "simple.csv"))
	doc, err := Open(absPath)
	if err != nil {
		t.Fatalf("Open CSV 失败: %v", err)
	}
	if doc.Content() == "" {
		t.Fatal("CSV 文档内容不应为空")
	}
	//CSV 归一化为 RawDocData
	if doc.Type() != RawDocData {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
	if doc.FileName() == "" {
		t.Fatal("FileName 不应为空")
	}
}

func TestOpen_HTML(t *testing.T) {
	skipIfFileMissing(t, "simple.html")
	absPath, _ := filepath.Abs(filepath.Join(testDataDir, "simple.html"))
	doc, err := Open(absPath)
	if err != nil {
		t.Fatalf("Open HTML 失败: %v", err)
	}
	if doc.Content() == "" {
		t.Fatal("HTML 文档内容不应为空")
	}
	// HTML 归一化为 RawDocDoc
	if doc.Type() != RawDocDoc {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}
}

func TestOpen_PDF(t *testing.T) {
	skipIfFileMissing(t, "simple.pdf")
	absPath, _ := filepath.Abs(filepath.Join(testDataDir, "simple.pdf"))
	doc, err := Open(absPath)
	if err != nil {
		t.Fatalf("Open PDF 失败: %v", err)
	}
	if doc.Content() == "" {
		t.Fatal("PDF 文档内容不应为空")
	}
	// PDF 归一化为 RawDocDoc
	if doc.Type() != RawDocDoc {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}
}

func TestOpen_Image(t *testing.T) {
	skipIfFileMissing(t, "simple.jpg")
	absPath, _ := filepath.Abs(filepath.Join(testDataDir, "simple.jpg"))
	doc, err := Open(absPath)
	if err != nil {
		t.Fatalf("Open JPEG 失败: %v", err)
	}
	if doc.Content() == "" {
		t.Fatal("图片文档内容不应为空")
	}
	// 图片归一化为 RawDocImage
	if doc.Type() != RawDocImage {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocImage, doc.Type())
	}
}

func TestNew_HTML(t *testing.T) {
	htmlContent := `<html><head><title>Test</title></head><body><p>Hello</p></body></html>`
	// New 不会解析内容，调用方需保证 content 已归一化。
	// 这里测试 HTML 内容存储，docType 选 RawDocDoc（HTML 归一化后为 document）。
	doc := New(htmlContent, RawDocDoc)
	if doc == nil {
		t.Fatal("New 不应返回 nil")
	}
	if doc.Type() != RawDocDoc {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}
	if doc.Content() == "" {
		t.Fatal("HTML 文档内容不应为空")
	}
	if !strings.Contains(doc.Content(), "Hello") {
		t.Fatal("HTML 文档内容应包含 'Hello'")
	}
}

func TestNew_CSV(t *testing.T) {
	csvContent := "a,b,c\n1,2,3"
	// New 不会解析内容，调用方需保证 content 已归一化。
	// 这里测试 CSV 内容存储，docType 选 RawDocData（CSV 归一化后为 data）。
	doc := New(csvContent, RawDocData)
	if doc == nil {
		t.Fatal("New 不应返回 nil")
	}
	if doc.Type() != RawDocData {
		t.Fatalf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
}

// ========================= Helper =========================

func checkContains(t *testing.T, s, substr, label string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("输出应包含 %s %q", label, substr)
	}
}
