package document

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"strings"

	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

func ParsePDF(r io.Reader) (*RawDocument, error) {
	var mdBuilder strings.Builder
	pageCount := 0
	author := ""
	title := ""
	imgs := make([][]byte, 0)

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

	imageIndex := 0
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

			pageImages, err := ex.ExtractPageImages(&extractor.ImageExtractOptions{})
			if err == nil && pageImages != nil {
				for _, imgMark := range pageImages.Images {
					if imgMark.Image == nil {
						continue
					}
					goImg, err := imgMark.Image.ToGoImage()
					if err != nil {
						continue
					}
					var buf bytes.Buffer
					if err := png.Encode(&buf, goImg); err != nil {
						continue
					}

					imageIndex++
					imgs = append(imgs, buf.Bytes())
					// mediaList = append(mediaList, &BinaryResult{
					// 	Mime: "image/png",
					// 	Data: buf.Bytes(),
					// 	Meta: map[string]any{
					// 		"page":        i,
					// 		"width":       imgMark.Width,
					// 		"height":      imgMark.Height,
					// 		"x":           imgMark.X,
					// 		"y":           imgMark.Y,
					// 		"angle":       imgMark.Angle,
					// 		"image_index": imageIndex,
					// 	},
					// })
				}
			}
		}
	}

	return NewRawDoc(mdBuilder.String()).
		SetValue("title", title).
		SetValue("author", author).
		SetValue("pages", pageCount).
		AddImages(imgs), nil
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
