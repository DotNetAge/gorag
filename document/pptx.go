package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var slidePattern = regexp.MustCompile(`ppt/slides/slide\d+\.xml$`)

// ParsePPTX 读取 .pptx 文件并转换为 docDoc（内容为 Markdown）。
// 仅使用标准库（archive/zip + encoding/xml）。
// 元数据包含 title 和 slide_count。
func ParsePPTX(r io.Reader) (RawDoc, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	// 查找所有 slide XML 文件，按 slide 序号排序
	var slideFiles []*zip.File
	for _, f := range zr.File {
		if slidePattern.MatchString(f.Name) {
			slideFiles = append(slideFiles, f)
		}
	}

	// 按文件名中提取的序号排序
	sortSlides(slideFiles)

	title := extractPptxTitle(zr)

	var mdBuilder strings.Builder
	if title != "" {
		mdBuilder.WriteString(fmt.Sprintf("# %s\n\n", title))
	}

	for i, sf := range slideFiles {
		if i > 0 {
			mdBuilder.WriteString("\n---\n\n")
		}
		mdBuilder.WriteString(fmt.Sprintf("## Slide %d\n\n", i+1))

		slideMd, err := parsePptxSlide(sf)
		if err != nil {
			continue
		}
		mdBuilder.WriteString(slideMd)
	}

	meta := map[string]any{
		"slide_count": len(slideFiles),
	}
	if title != "" {
		meta["title"] = title
	}
	return newParsedDoc(mdBuilder.String(), meta, RawDocDoc), nil
}

// parsePptxSlide 从单个 slide XML 文件中提取文本
func parsePptxSlide(sf *zip.File) (string, error) {
	rc, err := sf.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	var spSlide pptxSlide
	if err := xml.Unmarshal(b, &spSlide); err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, sp := range spSlide.Shapes {
		// 判断该 shape 是否为标题
		isTitle := false
		if sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil {
			phType := sp.NvSpPr.NvPr.Ph.Type
			if phType == "title" || phType == "ctrTitle" {
				isTitle = true
			}
		}

		// 从文本 run 中提取内容
		var textParts []string
		for _, txBody := range sp.TxBody {
			for _, p := range txBody.P {
				var runText strings.Builder
				for _, r := range p.R {
					runText.WriteString(r.T)
				}
				// 兜底：直接检查段落内的 <a:t>
				if runText.Len() == 0 && p.T != "" {
					runText.WriteString(p.T)
				}
				text := strings.TrimSpace(runText.String())
				if text != "" {
					textParts = append(textParts, text)
				}
			}
		}

		if len(textParts) == 0 {
			continue
		}

		joined := strings.Join(textParts, " ")
		joined = strings.TrimSpace(joined)
		if joined == "" {
			continue
		}

		if isTitle {
			builder.WriteString(fmt.Sprintf("### %s\n\n", joined))
		} else {
			// 判断是否为项目符号
			for _, part := range textParts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				if strings.HasPrefix(part, "•") || strings.HasPrefix(part, "-") || strings.HasPrefix(part, "\u2022") {
					builder.WriteString(fmt.Sprintf("%s\n", part))
				} else {
					builder.WriteString(fmt.Sprintf("%s\n\n", part))
				}
			}
		}
	}

	return builder.String(), nil
}

// --- PPTX XML 类型定义 ---

type pptxSlide struct {
	Shapes []pptxShape `xml:"p:sp"`
	// 同时处理 group shapes
	GspShapes []struct {
		Shapes []pptxShape `xml:"p:sp"`
	} `xml:"p:grpSp"`
}

type pptxShape struct {
	NvSpPr *struct {
		NvPr *struct {
			Ph *struct {
				Type string `xml:"type,attr"`
			} `xml:"ph"`
		} `xml:"nvPr"`
	} `xml:"nvSpPr"`
	TxBody []struct {
		P []struct {
			R []struct {
				T string `xml:"t"`
			} `xml:"a:r"`
			T string `xml:"a:t"` // 段落内直接文本（兜底）
		} `xml:"a:p"`
	} `xml:"txBody"`
}

// extractPptxTitle 尝试从 docProps/core.xml 中读取文档标题
func extractPptxTitle(zr *zip.Reader) string {
	for _, f := range zr.File {
		if f.Name == "docProps/core.xml" {
			rc, err := f.Open()
			if err != nil {
				return ""
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return ""
			}
			start := bytes.Index(b, []byte("<dc:title>"))
			if start == -1 {
				start = bytes.Index(b, []byte("<dcterms:title>"))
				if start == -1 {
					return ""
				}
				start += len("<dcterms:title>")
				end := bytes.Index(b[start:], []byte("</dcterms:title>"))
				if end == -1 {
					return ""
				}
				return string(bytes.TrimSpace(b[start : start+end]))
			}
			start += len("<dc:title>")
			end := bytes.Index(b[start:], []byte("</dc:title>"))
			if end == -1 {
				return ""
			}
			return string(bytes.TrimSpace(b[start : start+end]))
		}
	}
	return ""
}

func sortSlides(files []*zip.File) {
	// 从文件名（如 "ppt/slides/slide1.xml"）中提取 slide 序号
	extractNum := func(name string) int {
		// 查找路径中最后一段数字
		n := 0
		for i := len(name) - 1; i >= 0; i-- {
			c := name[i]
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else if n > 0 {
				break
			}
		}
		return n
	}
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if extractNum(files[i].Name) > extractNum(files[j].Name) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}
