package document

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

// ParseEML 解析 EML 邮件文件（RFC 822/MIME 格式）。
// 提取发件人、收件人、主题、日期等元信息，将正文转为 Markdown。
func ParseEML(r io.Reader) (*RawDocument, error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, fmt.Errorf("parse EML: %w", err)
	}

	// 提取元数据
	subject := decodeMIMEHeader(msg.Header.Get("Subject"))
	from := decodeMIMEHeader(msg.Header.Get("From"))
	to := decodeMIMEHeader(msg.Header.Get("To"))
	cc := decodeMIMEHeader(msg.Header.Get("Cc"))
	dateStr := msg.Header.Get("Date")

	// 构建文档内容：头部 + 正文
	var mdBuilder strings.Builder

	// 邮件头作为 Markdown 元信息写入
	mdBuilder.WriteString(fmt.Sprintf("**From:** %s\n", from))
	mdBuilder.WriteString(fmt.Sprintf("**To:** %s\n", to))
	if cc != "" {
		mdBuilder.WriteString(fmt.Sprintf("**Cc:** %s\n", cc))
	}
	mdBuilder.WriteString(fmt.Sprintf("**Subject:** %s\n", subject))
	if dateStr != "" {
		if parsedDate, err := mail.ParseDate(dateStr); err == nil {
			mdBuilder.WriteString(fmt.Sprintf("**Date:** %s\n", parsedDate.Format(time.RFC3339)))
		} else {
			mdBuilder.WriteString(fmt.Sprintf("**Date:** %s\n", dateStr))
		}
	}
	mdBuilder.WriteString("\n---\n\n")

	// 解析正文
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		mediaType = "text/plain"
	}

	body, err := decodeBody(msg.Body, mediaType, params)
	if err != nil {
		return nil, fmt.Errorf("decode EML body: %w", err)
	}

	mdBuilder.WriteString(body)

	doc := NewRawDoc(strings.TrimSpace(mdBuilder.String()))
	if subject != "" {
		doc.SetValue("subject", subject)
	}
	if from != "" {
		doc.SetValue("from", from)
	}
	if to != "" {
		doc.SetValue("to", to)
	}
	if cc != "" {
		doc.SetValue("cc", cc)
	}
	if dateStr != "" {
		doc.SetValue("date", dateStr)
	}
	doc.SetValue("email", true)

	return doc, nil
}

// decodeMIMEHeader 解码 MIME 编码的邮件头（支持 =?charset?encoding?text?= 格式）
func decodeMIMEHeader(s string) string {
	if s == "" {
		return ""
	}
	decoded, err := (&mime.WordDecoder{}).DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// decodeBody 递归解析邮件正文，优先 text/plain，HTML 转为 Markdown
func decodeBody(body io.Reader, mediaType string, params map[string]string) (string, error) {
	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		return decodeMultipart(body, params["boundary"])
	case mediaType == "text/plain":
		return decodeTextBody(body)
	case mediaType == "text/html":
		return decodeHTMLBody(body)
	default:
		// 未知类型尝试按纯文本读取
		return decodeTextBody(body)
	}
}

// decodeMultipart 递归解析 multipart 正文，优先取 text/plain
func decodeMultipart(body io.Reader, boundary string) (string, error) {
	if boundary == "" {
		data, _ := io.ReadAll(body)
		return string(data), nil
	}

	mr := multipart.NewReader(body, boundary)
	var plainText, htmlText string

	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}

		partMediaType, partParams, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			continue
		}

		decoded, err := decodeBody(part, partMediaType, partParams)
		if err != nil {
			continue
		}

		switch {
		case strings.HasPrefix(partMediaType, "multipart/"):
			// 递归处理嵌套 multipart
			plainText = decoded
			if strings.HasPrefix(partMediaType, "multipart/alternative") {
				plainText = decoded
			}
		case partMediaType == "text/plain":
			plainText = decoded
		case partMediaType == "text/html" && plainText == "":
			htmlText = decoded
		}
	}

	if plainText != "" {
		return plainText, nil
	}
	if htmlText != "" {
		return htmlText, nil
	}
	return "", nil
}

// decodeTextBody 读取纯文本正文
func decodeTextBody(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// decodeHTMLBody 将 HTML 正文转换为 Markdown
func decodeHTMLBody(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	converter := md.NewConverter("", true, &md.Options{HeadingStyle: "atx"})
	markdown, err := converter.ConvertString(string(data))
	if err != nil {
		// 转换失败时返回纯文本
		return string(data), nil
	}
	return markdown, nil
}
