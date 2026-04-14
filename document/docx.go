package document

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	docx "github.com/unidoc/unioffice/document"
)

func ParseDocx(r io.Reader) (*RawDocument, error) {
	var mdBuilder strings.Builder

	// 读取所有内容到字节缓冲区
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	doc, err := docx.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	defer doc.Close()

	paras := doc.Paragraphs()
	tables := doc.Tables()

	paraMap := make(map[uintptr]docx.Paragraph, len(paras))
	for _, p := range paras {
		paraMap[reflect.ValueOf(p.X()).Pointer()] = p
	}
	tableMap := make(map[uintptr]docx.Table, len(tables))
	for _, t := range tables {
		tableMap[reflect.ValueOf(t.X()).Pointer()] = t
	}

	extracted := doc.ExtractText()
	processedParas := make(map[uintptr]bool)
	processedTables := make(map[uintptr]bool)

	for _, item := range extracted.Items {
		if item.TableInfo != nil && item.TableInfo.Table != nil {
			tblPtr := reflect.ValueOf(item.TableInfo.Table).Pointer()
			if !processedTables[tblPtr] {
				processedTables[tblPtr] = true
				if tbl, ok := tableMap[tblPtr]; ok {
					mdBuilder.WriteString(tableToMarkdown(tbl))
					mdBuilder.WriteString("\n")
				}
			}
			continue
		}

		if item.Paragraph == nil {
			continue
		}

		paraPtr := reflect.ValueOf(item.Paragraph).Pointer()
		if processedParas[paraPtr] {
			continue
		}
		processedParas[paraPtr] = true

		if para, ok := paraMap[paraPtr]; ok {
			mdBuilder.WriteString(paragraphToMarkdown(para))
		}
	}

	// Create a new document from the markdown content
	rawDoc := NewRawDoc(mdBuilder.String())

	for _, img := range doc.Images {
		rawDoc.AddImage(*img.Data())
	}

	return rawDoc.SetValue("title", doc.CoreProperties.Title()).
		SetValue("last_modified_by", doc.CoreProperties.LastModifiedBy()), nil
}

func paragraphToMarkdown(para docx.Paragraph) string {
	var textBuilder strings.Builder
	styleName := para.Style()

	text := runsToText(para.Runs())
	if text == "" {
		return ""
	}

	switch styleName {
	case "Heading1", "标题 1", "heading 1":
		textBuilder.WriteString(fmt.Sprintf("# %s\n\n", text))
	case "Heading2", "标题 2", "heading 2":
		textBuilder.WriteString(fmt.Sprintf("## %s\n\n", text))
	case "Heading3", "标题 3", "heading 3":
		textBuilder.WriteString(fmt.Sprintf("### %s\n\n", text))
	case "Heading4", "标题 4", "heading 4":
		textBuilder.WriteString(fmt.Sprintf("#### %s\n\n", text))
	default:
		if strings.HasPrefix(text, "\t") || strings.HasPrefix(text, "    ") {
			textBuilder.WriteString(text + "\n")
		} else {
			textBuilder.WriteString(text + "\n\n")
		}
	}

	return textBuilder.String()
}

func runsToText(runs []docx.Run) string {
	var result strings.Builder
	for _, run := range runs {
		text := run.Text()
		if text == "" {
			continue
		}
		props := run.Properties()
		bold := props.IsBold()
		italic := props.IsItalic()
		if bold && italic {
			result.WriteString(fmt.Sprintf("***%s***", text))
		} else if bold {
			result.WriteString(fmt.Sprintf("**%s**", text))
		} else if italic {
			result.WriteString(fmt.Sprintf("*%s*", text))
		} else {
			result.WriteString(text)
		}
	}
	return result.String()
}

func tableToMarkdown(tbl docx.Table) string {
	var builder strings.Builder
	rows := tbl.Rows()

	if len(rows) == 0 {
		return ""
	}

	for i, row := range rows {
		cells := row.Cells()
		rowTexts := make([]string, len(cells))
		for j, cell := range cells {
			cellText := ""
			for _, para := range cell.Paragraphs() {
				cellText += runsToText(para.Runs())
			}
			rowTexts[j] = strings.TrimSpace(strings.ReplaceAll(cellText, "|", "\\|"))
		}
		builder.WriteString("| " + strings.Join(rowTexts, " | ") + " |\n")

		if i == 0 {
			separators := make([]string, len(cells))
			for k := range separators {
				separators[k] = "---"
			}
			builder.WriteString("| " + strings.Join(separators, " | ") + " |\n")
		}
	}

	return builder.String()
}
