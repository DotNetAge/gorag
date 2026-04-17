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

// ParsePPTX reads a .pptx file and converts it to Markdown.
// Uses only standard library (archive/zip + encoding/xml).
func ParsePPTX(r io.Reader) (*RawDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	// Find all slide XML files, sorted by slide number
	var slideFiles []*zip.File
	for _, f := range zr.File {
		if slidePattern.MatchString(f.Name) {
			slideFiles = append(slideFiles, f)
		}
	}

	// Sort by slide number extracted from filename
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

	return NewRawDoc(mdBuilder.String()).
		SetValue("title", title).
		SetValue("slide_count", len(slideFiles)), nil
}

// parsePptxSlide extracts text from a single slide XML file.
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
		// Determine if this shape is a title
		isTitle := false
		if sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil {
			phType := sp.NvSpPr.NvPr.Ph.Type
			if phType == "title" || phType == "ctrTitle" {
				isTitle = true
			}
		}

		// Extract text from text runs
		var textParts []string
		for _, txBody := range sp.TxBody {
			for _, p := range txBody.P {
				var runText strings.Builder
				for _, r := range p.R {
					runText.WriteString(r.T)
				}
				// Fallback: also check <a:t> directly in paragraphs
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
			// Check if it looks like a bullet point
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

// --- PPTX XML types ---

type pptxSlide struct {
	Shapes []pptxShape `xml:"p:sp"`
	// Also handle group shapes
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
			T string `xml:"a:t"` // direct text in paragraph (fallback)
		} `xml:"a:p"`
	} `xml:"txBody"`
}

// extractPptxTitle tries to read the title from docProps/core.xml
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
	// Extract slide number from filename like "ppt/slides/slide1.xml"
	extractNum := func(name string) int {
		// Find the last number in the path
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
