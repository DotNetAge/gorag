package document

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/unidoc/unioffice/presentation"
)

func ParsePPTX(r io.Reader) (*RawDocument, error) {
	var mdBuilder strings.Builder
	mediaList := [][]byte{} // []*BinaryResult{}
	slideCount := 0
	title := ""

	// 读取所有内容
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	ppt, err := presentation.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	defer ppt.Close()

	slides := ppt.Slides()
	slideCount = len(slides)

	title = ppt.CoreProperties.Title()
	if title != "" {
		mdBuilder.WriteString(fmt.Sprintf("# %s\n\n", title))
	}

	imageIndex := 0
	for i, slide := range slides {
		if i > 0 {
			mdBuilder.WriteString("\n---\n\n")
		}
		mdBuilder.WriteString(fmt.Sprintf("## Slide %d\n\n", i+1))

		slideText := slide.ExtractText()
		if slideText != nil && len(slideText.Items) > 0 {
			mdBuilder.WriteString(extractSlideTextToMarkdown(slideText, i+1))
		}
	}

	for _, img := range ppt.Images {
		data := img.Data()
		if data == nil || len(*data) == 0 {
			continue
		}

		imageIndex++
		// imageBase64 := base64.StdEncoding.EncodeToString(*data)
		// mdBuilder.WriteString(fmt.Sprintf("\n![Image %d](data:image/png;base64,%s)\n\n", imageIndex, imageBase64))
		mediaList = append(mediaList, *data)
		// mediaList = append(mediaList, &BinaryResult{
		// 	Data: *data,
		// 	Meta: map[string]any{
		// 		"image_index": imageIndex,
		// 	},
		// })
	}

	// return &Result{
	// 	Content: mdBuilder.String(),
	// 	Mime:    "text/markdown",
	// 	Meta: map[string]any{
	// 		"slide_count": slideCount,
	// 		"title":       title,
	// 	}}, mediaList, nil

	return NewRawDoc(mdBuilder.String()).
		SetValue("title", title).
		SetValue("slide_count", slideCount).
		AddImages(mediaList), nil
}

func extractSlideTextToMarkdown(slideText *presentation.SlideText, _ int) string {
	if slideText == nil || len(slideText.Items) == 0 {
		return ""
	}

	var builder strings.Builder
	processedTexts := make(map[string]bool)

	for _, item := range slideText.Items {
		if item.Text == "" || processedTexts[item.Text] {
			continue
		}
		processedTexts[item.Text] = true

		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}

		isTitle := false
		if item.Shape != nil && item.Shape.NvSpPr != nil && item.Shape.NvSpPr.NvPr != nil {
			if ph := item.Shape.NvSpPr.NvPr.Ph; ph != nil {
				phType := string(ph.TypeAttr)
				if phType == "title" || phType == "ctrTitle" {
					isTitle = true
				}
			}
		}

		if isTitle {
			builder.WriteString(fmt.Sprintf("### %s\n\n", text))
		} else if strings.HasPrefix(text, "•") || strings.HasPrefix(text, "-") {
			builder.WriteString(fmt.Sprintf("%s\n", text))
		} else {
			builder.WriteString(fmt.Sprintf("%s\n\n", text))
		}
	}

	return builder.String()
}
