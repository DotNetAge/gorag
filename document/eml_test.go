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
		t.Fatalf("ParseEML failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseEML returned empty text")
	}

	// 验证元数据
	if doc.Meta["subject"] != "Test Email Subject" {
		t.Errorf("Expected subject 'Test Email Subject', got: %v", doc.Meta["subject"])
	}
	if doc.Meta["from"] != "sender@example.com" {
		t.Errorf("Expected from 'sender@example.com', got: %v", doc.Meta["from"])
	}
	if doc.Meta["to"] != "recipient@example.com" {
		t.Errorf("Expected to 'recipient@example.com', got: %v", doc.Meta["to"])
	}
	if doc.Meta["email"] != true {
		t.Error("Expected email flag to be true")
	}

	// 验证正文内容存在于 Markdown 输出中
	if !strings.Contains(doc.Text, "Hello") {
		t.Error("Output should contain email body text starting with Hello")
	}
	if !strings.Contains(doc.Text, "From:") {
		t.Error("Output should contain From header")
	}
}

func TestParseEML_HTMLBody(t *testing.T) {
	doc, err := ParseEML(strings.NewReader(createEMLWithHTML()))
	if err != nil {
		t.Fatalf("ParseEML failed for HTML body: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseEML returned empty text for HTML body")
	}

	// HTML 应该被转为 Markdown
	if !strings.Contains(doc.Text, "HTML Email") {
		t.Error("Output should contain heading text from HTML")
	}
	if strings.Contains(doc.Text, "<h1>") || strings.Contains(doc.Text, "<html>") {
		t.Error("Output should not contain raw HTML tags")
	}
}

func TestParseEML_Multipart(t *testing.T) {
	doc, err := ParseEML(strings.NewReader(createMultipartEML()))
	if err != nil {
		t.Fatalf("ParseEML failed for multipart: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseEML returned empty text for multipart")
	}

	// multipart/alternative 应该优先返回 text/plain
	if !strings.Contains(doc.Text, "Plain text version") {
		t.Error("Output should contain the plain text version")
	}
}

func TestParseEML_EmptyInput(t *testing.T) {
	_, err := ParseEML(strings.NewReader(""))
	if err == nil {
		t.Fatal("Expected error for empty EML input")
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
		t.Fatalf("ParseEML failed for MIME-encoded headers: %v", err)
	}

	if doc.Meta["subject"] != "测试邮件" {
		t.Errorf("Expected decoded subject '测试邮件', got: %q", doc.Meta["subject"])
	}
}

func TestParseEML_ViaNew(t *testing.T) {
	emlContent := createMinimalEML()
	doc := New(emlContent, "message/rfc822")
	if doc == nil {
		t.Fatal("New should not return nil for EML")
	}
	if doc.GetMimeType() != "message/rfc822" {
		t.Errorf("Expected MIME 'message/rfc822', got: %q", doc.GetMimeType())
	}
}
