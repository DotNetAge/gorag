package document

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf16"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/richardlehane/mscfb"
)

// MSG property tag constants (lower 16 bits = tag, upper 16 bits = type)
const (
	propSubject       = 0x0037 // PidTagSubject
	propNormalSubject = 0x0E1D // PidTagNormalizedSubject
	propSenderName    = 0x0C1A // PidTagSenderName
	propSenderEmail   = 0x0C1F // PidTagSenderEmailAddress
	propSentOn        = 0x0E04 // PidTagClientSubmitTime
	propBody          = 0x007D // PidTagBody (plain text)
	propBodyHTML      = 0x1013 // PidTagBodyHtml
	propRecipType     = 0x0C15 // PidTagRecipientType (1=To, 2=CC, 3=BCC)
	propRecipName     = 0x3001 // PidTagDisplayName
	propRecipEmail    = 0x39FE // PidTagEmailAddress

	typeUnicode = 0x001F // PT_UNICODE (UTF-16LE string)
	typeBinary  = 0x0102 // PT_BINARY
	typeInt     = 0x0003 // PT_LONG (int32)
)

// ParseMSG 解析 Outlook MSG 文件（OLE2 复合文档格式）。
// 提取发件人、主题、日期、收件人等元信息，将正文转为 Markdown。
func ParseMSG(r io.Reader) (*RawDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read msg: %w", err)
	}

	oleDoc, err := mscfb.New(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid msg (ole2): %w", err)
	}

	// 遍历所有流，按名称收集
	streams := make(map[string][]byte)
	for {
		entry, err := oleDoc.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		streamData, err := io.ReadAll(oleDoc)
		if err != nil {
			continue
		}
		streams[entry.Name] = streamData
	}

	// 提取发件人
	senderName := readMSGUnicode(streams, streamKey(propSenderName, typeUnicode))
	senderEmail := readMSGUnicode(streams, streamKey(propSenderEmail, typeUnicode))
	var from string
	switch {
	case senderName != "" && senderEmail != "":
		from = fmt.Sprintf("%s <%s>", senderName, senderEmail)
	case senderName != "":
		from = senderName
	case senderEmail != "":
		from = senderEmail
	}

	// 提取主题
	subject := readMSGUnicode(streams, streamKey(propSubject, typeUnicode))
	if subject == "" {
		subject = readMSGUnicode(streams, streamKey(propNormalSubject, typeUnicode))
	}

	// 提取日期
	dateStr := ""
	if rawDate, ok := streams[streamKey(propSentOn, 0x0040)]; ok && len(rawDate) >= 8 {
		if t := decodeFILETIME(rawDate); !t.IsZero() {
			dateStr = t.Format(time.RFC3339)
		}
	}

	// 提取收件人
	toList, ccList := extractMSGRecipients(streams)

	// 提取正文
	body := readMSGUnicode(streams, streamKey(propBody, typeUnicode))
	isHTML := false
	if body == "" {
		if htmlData, ok := streams[streamKey(propBodyHTML, typeBinary)]; ok && len(htmlData) > 0 {
			body = string(htmlData)
			isHTML = true
		}
	}
	if body == "" {
		body = readMSGUnicode(streams, streamKey(propBodyHTML, typeUnicode))
		isHTML = body != ""
	}

	if isHTML {
		converter := md.NewConverter("", true, &md.Options{HeadingStyle: "atx"})
		if mdBody, err := converter.ConvertString(body); err == nil {
			body = mdBody
		}
	}

	// 构建 Markdown 文档
	var mdBuilder strings.Builder
	mdBuilder.WriteString(fmt.Sprintf("**From:** %s\n", from))
	mdBuilder.WriteString(fmt.Sprintf("**Subject:** %s\n", subject))
	if len(toList) > 0 {
		mdBuilder.WriteString(fmt.Sprintf("**To:** %s\n", strings.Join(toList, ", ")))
	}
	if len(ccList) > 0 {
		mdBuilder.WriteString(fmt.Sprintf("**Cc:** %s\n", strings.Join(ccList, ", ")))
	}
	if dateStr != "" {
		mdBuilder.WriteString(fmt.Sprintf("**Date:** %s\n", dateStr))
	}
	mdBuilder.WriteString("\n---\n\n")
	mdBuilder.WriteString(body)

	rawDoc := NewRawDoc(strings.TrimSpace(mdBuilder.String()))
	if from != "" {
		rawDoc.SetValue("from", from)
	}
	if subject != "" {
		rawDoc.SetValue("subject", subject)
	}
	if len(toList) > 0 {
		rawDoc.SetValue("to", strings.Join(toList, "; "))
	}
	if len(ccList) > 0 {
		rawDoc.SetValue("cc", strings.Join(ccList, "; "))
	}
	if dateStr != "" {
		rawDoc.SetValue("date", dateStr)
	}
	rawDoc.SetValue("email", true)

	return rawDoc, nil
}

// streamKey 构建 MSG 流名称：__substg1.0_XXXXYYYY
func streamKey(tag, typ uint16) string {
	return fmt.Sprintf("__substg1.0_%04X%04X", tag, typ)
}

// readMSGUnicode 从 streams 中读取 UTF-16LE Unicode 字符串
func readMSGUnicode(streams map[string][]byte, name string) string {
	data, ok := streams[name]
	if !ok || len(data) < 2 {
		return ""
	}
	return decodeUTF16LE(data)
}

// decodeUTF16LE 将 UTF-16LE 字节解码为 Go 字符串（遇空终止符截断）
func decodeUTF16LE(data []byte) string {
	if len(data)%2 != 0 {
		return ""
	}
	u16 := make([]uint16, len(data)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	// 截断到空终止符
	nullIdx := len(u16)
	for i, c := range u16 {
		if c == 0 {
			nullIdx = i
			break
		}
	}
	return string(utf16.Decode(u16[:nullIdx]))
}

// decodeFILETIME 解码 Windows FILETIME（1601-01-01 以来的 100ns 间隔数）
func decodeFILETIME(data []byte) time.Time {
	if len(data) < 8 {
		return time.Time{}
	}
	ticks := binary.LittleEndian.Uint64(data)
	if ticks == 0 {
		return time.Time{}
	}
	// Windows FILETIME epoch: January 1, 1601
	// Unix epoch: January 1, 1970
	// Difference: 369 years + leap days = 11644473600 seconds
	const unixEpochDiff = 11644473600
	unixSecs := int64(ticks/10_000_000) - unixEpochDiff
	return time.Unix(unixSecs, 0).UTC()
}

// extractMSGRecipients 从 streams 中提取收件人列表
// 收件人信息存储在 __recip_version1.0_00000000 子存储中，
// 其条目名称为 "recip_{index}" 等形式。这里搜索所有含收件人属性的流。
func extractMSGRecipients(streams map[string][]byte) (to, cc []string) {
	// 将所有条目按前缀分组
	recipPrefix := "__recip_version1.0_00000000"
	var recipNames, recipEmails []string
	var recipTypes []int32

	for name, data := range streams {
		if !strings.Contains(name, recipPrefix) {
			continue
		}
		// 提取属性标签（流名末尾的 XXXXYYYY）
		// 格式: .../__substg1.0_XXXXYYYY
		if !strings.Contains(name, "__substg1.0_") {
			continue
		}
		tagStr := name[len(name)-8:]
		if len(tagStr) != 8 {
			continue
		}

		var tag, typ uint16
		if _, err := fmt.Sscanf(tagStr, "%04X%04X", &tag, &typ); err != nil {
			continue
		}

		switch {
		case tag == propRecipName && typ == typeUnicode:
			recipNames = append(recipNames, decodeUTF16LE(data))
		case tag == propRecipEmail && typ == typeUnicode:
			recipEmails = append(recipEmails, decodeUTF16LE(data))
		case tag == propRecipType && typ == typeInt && len(data) >= 4:
			recipTypes = append(recipTypes, int32(binary.LittleEndian.Uint32(data)))
		}
	}

	// 合并收件人信息
	// 注意：多个收件人时，属性按收件人索引交错存储。
	// 我们采用简化处理：按类型分组成 to/cc
	for i, rtype := range recipTypes {
		var name, email string
		if i < len(recipNames) {
			name = recipNames[i]
		}
		if i < len(recipEmails) {
			email = recipEmails[i]
		}
		addr := name
		if name != "" && email != "" {
			addr = fmt.Sprintf("%s <%s>", name, email)
		} else if email != "" {
			addr = email
		}
		if addr == "" {
			continue
		}
		switch rtype {
		case 1: // MAPI_TO
			to = append(to, addr)
		case 2: // MAPI_CC
			cc = append(cc, addr)
		}
	}

	return
}
