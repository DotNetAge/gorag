package document

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

// ParsePDF 解析 PDF 文件并转换为 docDoc（内容为 Markdown）。
// 元数据包含 title/author/pages（若存在），不提取嵌入图片附件。
func ParsePDF(r io.Reader) (RawDoc, error) {
	var mdBuilder strings.Builder
	pageCount := 0
	author := ""
	title := ""

	// 读取所有内容
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	pdfReader, err := model.NewPdfReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	pdfInfo, err := pdfReader.GetPdfInfo()
	if err == nil && pdfInfo != nil {
		if pdfInfo.Author != nil {
			author = pdfInfo.Author.Decoded()
		}
		if pdfInfo.Title != nil {
			title = pdfInfo.Title.Decoded()
		}
	}

	pageCount, err = pdfReader.GetNumPages()
	if err != nil {
		return nil, err
	}

	if title != "" {
		mdBuilder.WriteString(fmt.Sprintf("# %s\n\n", title))
	}

	for i := 1; i <= pageCount; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			continue
		}

		mdBuilder.WriteString(fmt.Sprintf("\n---\n\n## Page %d\n\n", i))

		ex, err := extractor.New(page)
		if err == nil {
			pageText, err := ex.ExtractText()
			if err == nil && pageText != "" {
				mdBuilder.WriteString(textToMarkdown(pageText))
			}
			// 不提取嵌入图片附件
		}
	}

	meta := map[string]any{
		"pages": pageCount,
	}
	if title != "" {
		meta["title"] = title
	}
	if author != "" {
		meta["author"] = author
	}
	return newParsedDoc(mdBuilder.String(), meta, RawDocDoc), nil
}

func textToMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	var result strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result.WriteString("\n")
		} else {
			result.WriteString(trimmed + "\n")
		}
	}

	return result.String() + "\n"
}
