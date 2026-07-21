package document

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"
)

// ParseEML 解析 EML 邮件文件（RFC 822/MIME 格式），归一化为 dataDoc（内容为 JSON 字符串）。
//
// 归一化策略：
//   - 提取发件人、收件人、主题、日期、正文等结构化字段
//   - HTML 正文转为纯文本（去除标签），优先保留 text/plain
//   - 输出 JSON 对象字符串：{"from":"...","to":"...","subject":"...","body":"..."}
//   - 元数据包含 email 标记和邮件头字段
func ParseEML(r io.Reader) (RawDoc, error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, fmt.Errorf("解析 EML 失败: %w", err)
	}

	// 提取元数据
	subject := decodeMIMEHeader(msg.Header.Get("Subject"))
	from := decodeMIMEHeader(msg.Header.Get("From"))
	to := decodeMIMEHeader(msg.Header.Get("To"))
	cc := decodeMIMEHeader(msg.Header.Get("Cc"))
	dateStr := msg.Header.Get("Date")

	// 解析日期为 RFC3339 格式
	dateRFC3339 := ""
	if dateStr != "" {
		if parsedDate, err := mail.ParseDate(dateStr); err == nil {
			dateRFC3339 = parsedDate.Format(time.RFC3339)
		} else {
			dateRFC3339 = dateStr
		}
	}

	// 解析正文
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		mediaType = "text/plain"
	}

	body, err := decodeBody(msg.Body, mediaType, params)
	if err != nil {
		return nil, fmt.Errorf("解码 EML 正文失败: %w", err)
	}

	// 构建 JSON 对象
	emailObj := map[string]any{
		"from":    from,
		"to":      to,
		"subject": subject,
		"date":    dateRFC3339,
		"body":    body,
	}
	if cc != "" {
		emailObj["cc"] = cc
	}

	jsonBytes, err := json.Marshal(emailObj)
	if err != nil {
		return nil, fmt.Errorf("EML 转 JSON 失败: %w", err)
	}

	meta := map[string]any{"email": true}
	if subject != "" {
		meta["subject"] = subject
	}
	if from != "" {
		meta["from"] = from
	}
	if to != "" {
		meta["to"] = to
	}
	if cc != "" {
		meta["cc"] = cc
	}
	if dateStr != "" {
		meta["date"] = dateStr
	}
	return newParsedDoc(string(jsonBytes), meta, RawDocData), nil
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

// decodeBody 递归解析邮件正文，优先 text/plain，HTML 转为纯文本
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
			plainText = decoded
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

// decodeHTMLBody 将 HTML 正文剥离为纯文本
func decodeHTMLBody(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return stripHTMLTags(string(data)), nil
}

// stripHTMLTags 简易 HTML 标签剥离，保留纯文本内容。
func stripHTMLTags(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
			buf.WriteRune(' ')
		default:
			if !inTag {
				buf.WriteRune(r)
			}
		}
	}
	// 折叠多余空白
	return strings.TrimSpace(strings.Join(strings.Fields(buf.String()), " "))
}
