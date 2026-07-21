package document

import (
	"strings"
	"testing"
)

func createMinimalEML() string {
	return `From: sender@example.com
To: recipient@example.com
Subject: Test Email Subject
Date: Mon, 15 Jan 2024 14:30:00 +0000
MIME-Version: 1.0
Content-Type: text/plain; charset="utf-8"

Hello,

This is a test email body for EML parsing.

Regards,
Test Sender`
}

func createEMLWithHTML() string {
	return `From: html-sender@example.com
To: html-recipient@example.com
Subject: HTML Email
Date: Tue, 16 Jan 2024 10:00:00 +0000
MIME-Version: 1.0
Content-Type: text/html; charset="utf-8"

<html><body>
<h1>HTML Email</h1>
<p>This is an <strong>HTML</strong> email body.</p>
</body></html>`
}

func createMultipartEML() string {
	return `From: multi@example.com
To: multi-recipient@example.com
Subject: Multipart Email
Date: Wed, 17 Jan 2024 08:00:00 +0000
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="boundary123"

--boundary123
Content-Type: text/plain; charset="utf-8"

Plain text version.

--boundary123
Content-Type: text/html; charset="utf-8"

<html><body><h1>HTML</h1><p>HTML version.</p></body></html>

--boundary123--`

}

func TestParseEML_Basic(t *testing.T) {
	doc, err := ParseEML(strings.NewReader(createMinimalEML()))
	if err != nil {
		t.Fatalf("ParseEML 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseEML 返回空内容")
	}

	// 验证元数据
	meta := doc.Meta()
	if meta["subject"] != "Test Email Subject" {
		t.Errorf("期望 subject 'Test Email Subject', 实际: %v", meta["subject"])
	}
	if meta["from"] != "sender@example.com" {
		t.Errorf("期望 from 'sender@example.com', 实际: %v", meta["from"])
	}
	if meta["to"] != "recipient@example.com" {
		t.Errorf("期望 to 'recipient@example.com', 实际: %v", meta["to"])
	}
	if meta["email"] != true {
		t.Error("期望 email 标记为 true")
	}

	// V2：EML 归一化为 JSON，验证正文与发件人字段均存在于 JSON 输出中
	if !strings.Contains(doc.Content(), "Hello") {
		t.Error("输出应包含以 Hello 开头的邮件正文")
	}
	if !strings.Contains(doc.Content(), `"from":`) {
		t.Error("输出应包含 'from' JSON key")
	}

	// V2：EML 归一化为 RawDocData
	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
}

func TestParseEML_HTMLBody(t *testing.T) {
	doc, err := ParseEML(strings.NewReader(createEMLWithHTML()))
	if err != nil {
		t.Fatalf("ParseEML 处理 HTML 正文失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseEML 处理 HTML 正文返回空内容")
	}

	// HTML 标签应被剥离为纯文本
	if !strings.Contains(doc.Content(), "HTML Email") {
		t.Error("输出应包含 HTML 中的标题文本")
	}
	if strings.Contains(doc.Content(), "<h1>") || strings.Contains(doc.Content(), "<html>") {
		t.Error("输出不应包含原始 HTML 标签")
	}
}

func TestParseEML_Multipart(t *testing.T) {
	doc, err := ParseEML(strings.NewReader(createMultipartEML()))
	if err != nil {
		t.Fatalf("ParseEML 处理 multipart 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseEML 处理 multipart 返回空内容")
	}

	// multipart/alternative 应该优先返回 text/plain
	if !strings.Contains(doc.Content(), "Plain text version") {
		t.Error("输出应包含纯文本版本")
	}
}

func TestParseEML_EmptyInput(t *testing.T) {
	_, err := ParseEML(strings.NewReader(""))
	if err == nil {
		t.Fatal("空 EML 输入应返回错误")
	}
}

func TestParseEML_MIMEHeaderEncoding(t *testing.T) {
	eml := `From: =?utf-8?B?5Lic5L2T55Sf?= <user@example.com>
To: recipient@example.com
Subject: =?utf-8?B?5rWL6K+V6YKu5Lu2?=
Date: Thu, 18 Jan 2024 12:00:00 +0000
Content-Type: text/plain; charset="utf-8"

Body content`

	doc, err := ParseEML(strings.NewReader(eml))
	if err != nil {
		t.Fatalf("ParseEML 处理 MIME 编码头失败: %v", err)
	}

	if doc.Meta()["subject"] != "测试邮件" {
		t.Errorf("期望解码后 subject '测试邮件', 实际: %q", doc.Meta()["subject"])
	}
}

func TestParseEML_ViaNew(t *testing.T) {
	emlContent := createMinimalEML()
	// V2：New 不会解析内容，调用方需保证 content 已归一化。
	// EML 归一化后为 RawDocData。
	doc := New(emlContent, RawDocData)
	if doc == nil {
		t.Fatal("New 不应返回 nil（EML 场景）")
	}
	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
}
