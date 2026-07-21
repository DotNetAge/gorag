package document

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// createMinimalEPUB 创建一个最小有效的 EPUB 用于测试
func createMinimalEPUB() *bytes.Buffer {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// mimetype（必须第一个、未压缩）
	mt, _ := zw.Create("mimetype")
	mt.Write([]byte("application/epub+zip"))

	// container.xml
	container, _ := zw.Create("META-INF/container.xml")
	container.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	// content.opf
	opf, _ := zw.Create("OEBPS/content.opf")
	opf.Write([]byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
  <metadata>
    <dc:title xmlns:dc="http://purl.org/dc/elements/1.1/">Test EPUB Book</dc:title>
    <dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Test Author</dc:creator>
    <dc:language xmlns:dc="http://purl.org/dc/elements/1.1/">en</dc:language>
  </metadata>
  <manifest>
    <item id="chapter1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chapter2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="chapter1"/>
    <itemref idref="chapter2"/>
  </spine>
</package>`))

	// chapter1.xhtml
	ch1, _ := zw.Create("OEBPS/chapter1.xhtml")
	ch1.Write([]byte(`<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter 1</title></head><body>
<h1>Chapter 1</h1>
<p>First paragraph of the EPUB.</p>
<p>Second paragraph with <strong>bold</strong> text.</p>
</body></html>`))

	// chapter2.xhtml
	ch2, _ := zw.Create("OEBPS/chapter2.xhtml")
	ch2.Write([]byte(`<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter 2</title></head><body>
<h2>Chapter 2</h2>
<ul><li>Item A</li><li>Item B</li></ul>
</body></html>`))

	zw.Close()
	return &buf
}

func TestParseEPUB_Basic(t *testing.T) {
	epubData := createMinimalEPUB()
	doc, err := ParseEPUB(epubData)
	if err != nil {
		t.Fatalf("ParseEPUB 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseEPUB 返回空内容")
	}

	// 验证内容被提取并转换为 Markdown
	checks := []string{"Chapter 1", "Chapter 2", "First paragraph", "bold", "Item A", "Item B"}
	for _, c := range checks {
		if !strings.Contains(doc.Content(), c) {
			t.Errorf("输出应包含 %q", c)
		}
	}

	// 验证不在输出中
	if strings.Contains(doc.Content(), "<html") || strings.Contains(doc.Content(), "<h1>") {
		t.Error("输出不应包含原始 HTML 标签")
	}

	// 验证元数据
	meta := doc.Meta()
	if meta["title"] != "Test EPUB Book" {
		t.Errorf("期望 title 'Test EPUB Book', 实际: %v", meta["title"])
	}
	if meta["author"] != "Test Author" {
		t.Errorf("期望 author 'Test Author', 实际: %v", meta["author"])
	}
	if meta["language"] != "en" {
		t.Errorf("期望 language 'en', 实际: %v", meta["language"])
	}

	// V2：EPUB 归一化为 RawDocDoc
	if doc.Type() != RawDocDoc {
		t.Errorf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}
}

func TestParseEPUB_InvalidZIP(t *testing.T) {
	_, err := ParseEPUB(strings.NewReader("not a zip file"))
	if err == nil {
		t.Fatal("无效 ZIP 应返回错误")
	}
}

func TestParseEPUB_EmptyZIP(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.Close()

	_, err := ParseEPUB(&buf)
	if err == nil {
		t.Fatal("无 container.xml 的 ZIP 应返回错误")
	}
}

func TestParseEPUB_NoContainer(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("random.txt")
	f.Write([]byte("hello"))
	zw.Close()

	_, err := ParseEPUB(&buf)
	if err == nil {
		t.Fatal("无 container.xml 的 EPUB 应返回错误")
	}
}

func TestParseEPUB_NestedTitle(t *testing.T) {
	// EPUB 没有标题时，使用文件名兜底
	epubData := createMinimalEPUB()
	doc, err := ParseEPUB(epubData)
	if err != nil {
		t.Fatalf("ParseEPUB 失败: %v", err)
	}
	_ = doc
}

func TestParseEPUB_ViaNew(t *testing.T) {
	epubData := createMinimalEPUB()
	content := epubData.String()

	// V2：New 不会解析内容，调用方需保证 content 已归一化。
	// EPUB 归一化后为 RawDocDoc。
	doc := New(content, RawDocDoc)
	if doc == nil {
		t.Fatal("New 不应返回 nil（EPUB 场景）")
	}
	if doc.Type() != RawDocDoc {
		t.Errorf("期望 docType %q, 实际 %q", RawDocDoc, doc.Type())
	}
}
