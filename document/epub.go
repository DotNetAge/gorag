package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

// ParseEPUB 解析 EPUB 电子书，将内容提取为 Markdown 格式。
// EPUB 是 ZIP 容器，包含 OPF 元数据文件和 XHTML 内容文件。
// 使用已有的 html-to-markdown 库将 XHTML 转为 Markdown。
func ParseEPUB(r io.Reader) (*RawDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read epub: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid epub (zip): %w", err)
	}

	// 1. 读取 META-INF/container.xml 找到 OPF 文件路径
	opfPath, err := findOPFPath(zr)
	if err != nil {
		return nil, err
	}
	opfDir := filepath.Dir(opfPath)

	// 2. 读取 OPF 获取元数据和内容清单
	opf, err := parseOPF(zr, opfPath)
	if err != nil {
		return nil, err
	}

	// 3. 按 spine 顺序读取内容文件，转换为 Markdown
	content, images, err := readSpineItems(zr, opf, opfDir)
	if err != nil {
		return nil, err
	}

	doc := NewRawDoc(strings.TrimSpace(content))
	if opf.Title != "" {
		doc.SetValue("title", opf.Title)
	}
	if opf.Creator != "" {
		doc.SetValue("author", opf.Creator)
	}
	if opf.Language != "" {
		doc.SetValue("language", opf.Language)
	}
	if len(images) > 0 {
		doc.AddImages(images)
	}

	return doc, nil
}

// findOPFPath 从 container.xml 中读取 OPF 文件的路径
func findOPFPath(zr *zip.Reader) (string, error) {
	containerData, err := readZipFile(zr, "META-INF/container.xml")
	if err != nil {
		return "", fmt.Errorf("META-INF/container.xml not found: %w", err)
	}

	// 剥离 XML 命名空间后解析
	clean := stripXMLNamespaces(containerData)

	var container struct {
		XMLName   xml.Name `xml:"container"`
		RootFiles struct {
			RootFile []struct {
				FullPath  string `xml:"full-path,attr"`
				MediaType string `xml:"media-type,attr"`
			} `xml:"rootfile"`
		} `xml:"rootfiles"`
	}
	if err := xml.Unmarshal(clean, &container); err != nil {
		return "", fmt.Errorf("parse container.xml: %w", err)
	}
	if len(container.RootFiles.RootFile) == 0 {
		return "", fmt.Errorf("invalid epub: no rootfile in container.xml")
	}
	return container.RootFiles.RootFile[0].FullPath, nil
}

// opfMetadata EPUB OPF 文件解析结果
type opfMetadata struct {
	Title    string
	Creator  string
	Language string
	Spine    []string   // spine 顺序的 manifest item ID
	Manifest map[string]manifestItem // id → item
}

type manifestItem struct {
	Href      string
	MediaType string
}

// parseOPF 解析 OPF 文件
func parseOPF(zr *zip.Reader, opfPath string) (*opfMetadata, error) {
	opfData, err := readZipFile(zr, opfPath)
	if err != nil {
		return nil, fmt.Errorf("OPF file %s not found: %w", opfPath, err)
	}

	// 剥离命名空间后统一解析
	clean := stripXMLNamespaces(opfData)

	var opf struct {
		XMLName  xml.Name `xml:"package"`
		Metadata struct {
			Title    string `xml:"title"`
			Creator  string `xml:"creator"`
			Language string `xml:"language"`
		} `xml:"metadata"`
		Manifest struct {
			Items []struct {
				ID        string `xml:"id,attr"`
				Href      string `xml:"href,attr"`
				MediaType string `xml:"media-type,attr"`
			} `xml:"item"`
		} `xml:"manifest"`
		Spine struct {
			ItemRefs []struct {
				IDRef string `xml:"idref,attr"`
			} `xml:"itemref"`
		} `xml:"spine"`
	}

	if err := xml.Unmarshal(clean, &opf); err != nil {
		return nil, fmt.Errorf("parse OPF: %w", err)
	}

	meta := &opfMetadata{
		Title:    opf.Metadata.Title,
		Creator:  opf.Metadata.Creator,
		Language: opf.Metadata.Language,
		Manifest: make(map[string]manifestItem),
	}

	for _, item := range opf.Manifest.Items {
		meta.Manifest[item.ID] = manifestItem{
			Href:      item.Href,
			MediaType: item.MediaType,
		}
	}

	for _, ref := range opf.Spine.ItemRefs {
		meta.Spine = append(meta.Spine, ref.IDRef)
	}

	return meta, nil
}

// readSpineItems 按 spine 顺序读取 XHTML 内容并转换为 Markdown
func readSpineItems(zr *zip.Reader, opf *opfMetadata, opfDir string) (string, [][]byte, error) {
	converter := md.NewConverter("", true, &md.Options{HeadingStyle: "atx"})
	var mdBuilder strings.Builder
	var images [][]byte

	for _, id := range opf.Spine {
		item, ok := opf.Manifest[id]
		if !ok {
			continue
		}

		contentPath := filepath.ToSlash(filepath.Join(opfDir, item.Href))
		data, err := readZipFile(zr, contentPath)
		if err != nil {
			continue
		}

		// 跳过非 XHTML/HTML 文件（CSS、图片等）
		if !isContentFile(item.MediaType) {
			continue
		}

		markdown, err := converter.ConvertString(string(data))
		if err != nil {
			// 转换失败时提取纯文本
			markdown = extractPlainText(string(data))
		}

		mdBuilder.WriteString(markdown)
		mdBuilder.WriteString("\n\n")
	}

	return mdBuilder.String(), images, nil
}

// isContentFile 判断是否为需要转换的内容文件
func isContentFile(mediaType string) bool {
	switch mediaType {
	case "application/xhtml+xml",
		"text/html",
		"application/html+xml",
		"text/xml",
		"application/xml":
		return true
	}
	return false
}

// extractPlainText 从 HTML 中提取纯文本（转换失败时的兜底方案）
func extractPlainText(html string) string {
	// 移除所有标签
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	// 解码常见实体
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	// 压缩空白
	re = regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// stripXMLNamespaces 剥离 XML 命名空间声明和前綴，简化解析
func stripXMLNamespaces(data []byte) []byte {
	// 移除 xmlns="..." 默认命名空间声明
	re := regexp.MustCompile(`\s+xmlns\s*=\s*"[^"]*"`)
	data = re.ReplaceAll(data, nil)
	// 移除 xmlns:prefix="..." 前缀命名空间声明
	re = regexp.MustCompile(`\s+xmlns:[a-zA-Z0-9_.-]+\s*=\s*"[^"]*"`)
	data = re.ReplaceAll(data, nil)
	// 移除元素前缀: <prefix:tag → <tag
	re = regexp.MustCompile(`<([a-zA-Z_][a-zA-Z0-9_.-]*):([a-zA-Z_][a-zA-Z0-9_.-]*)([/>\s])`)
	data = re.ReplaceAll(data, []byte(`<$2$3`))
	// 移除闭合标签前缀: </prefix:tag → </tag
	re = regexp.MustCompile(`</([a-zA-Z_][a-zA-Z0-9_.-]*):([a-zA-Z_][a-zA-Z0-9_.-]*)\s*>`)
	data = re.ReplaceAll(data, []byte(`</$2>`))
	// 移除属性前缀: prefix:attr=" → attr="
	re = regexp.MustCompile(`\s+[a-zA-Z_][a-zA-Z0-9_.-]*:([a-zA-Z_][a-zA-Z0-9_.-]*)=`)
	data = re.ReplaceAll(data, []byte(` $1=`))
	return data
}

// readZipFile 从 ZIP reader 中读取指定路径的文件内容
func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file not found: %s", name)
}
