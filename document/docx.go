package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseDocx reads a .docx file and converts it to Markdown.
// Uses only standard library (archive/zip + encoding/xml).
func ParseDocx(r io.Reader) (*RawDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	mdContent, err := docxToMarkdown(reader)
	if err != nil {
		return nil, err
	}

	// Extract title from core.xml if available
	title := extractDocxTitle(reader)

	return NewRawDoc(mdContent).
		SetValue("title", title), nil
}

// --- internal types for OOXML parsing ---

type xmlRelationships struct {
	XMLName       xml.Name        `xml:"Relationships"`
	Relationship  []xmlRel        `xml:"Relationship"`
}

type xmlRel struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

type xmlNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:"-"`
	Content []byte     `xml:",innerxml"`
	Nodes   []xmlNode  `xml:",any"`
}

func (n *xmlNode) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.Attrs = start.Attr
	type node xmlNode
	return d.DecodeElement((*node)(n), &start)
}

func xmlAttr(attrs []xml.Attr, name string) (string, bool) {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

func xmlEscape(s, set string) string {
	replacer := []string{}
	for _, r := range []rune(set) {
		replacer = append(replacer, string(r), `\`+string(r))
	}
	return strings.NewReplacer(replacer...).Replace(s)
}

// --- docx to markdown core logic (forked from mattn/docx2md) ---

type docxFile struct {
	rels xmlRelationships
	num  xmlNumbering
	zr   *zip.Reader
	list map[string]int
}

type xmlNumbering struct {
	XMLName     xml.Name `xml:"numbering"`
	AbstractNum []struct {
		AbstractNumID string       `xml:"abstractNumId,attr"`
		Lvl          []xmlNumLvl   `xml:"lvl"`
	} `xml:"abstractNum"`
	Num []struct {
		NumID         string `xml:"numId,attr"`
		AbstractNumID struct {
			Val string `xml:"val,attr"`
		} `xml:"abstractNumId"`
	} `xml:"num"`
}

type xmlNumLvl struct {
	Ilvl   string `xml:"ilvl,attr"`
	Start  struct {
		Val string `xml:"val,attr"`
	} `xml:"start"`
	NumFmt struct {
		Val string `xml:"val,attr"`
	} `xml:"numFmt"`
	PPr struct {
		Ind struct {
			Left string `xml:"left,attr"`
		} `xml:"ind"`
	} `xml:"pPr"`
}

func (zf *docxFile) walk(node *xmlNode, w io.Writer) error {
	switch node.XMLName.Local {
	case "hyperlink":
		fmt.Fprint(w, "[")
		var cbuf bytes.Buffer
		for i := range node.Nodes {
			if err := zf.walk(&node.Nodes[i], &cbuf); err != nil {
				return err
			}
		}
		fmt.Fprint(w, xmlEscape(cbuf.String(), "[]"))
		fmt.Fprint(w, "]")
		fmt.Fprint(w, "(")
		if id, ok := xmlAttr(node.Attrs, "id"); ok {
			for _, rel := range zf.rels.Relationship {
				if id == rel.ID {
					fmt.Fprint(w, xmlEscape(rel.Target, "()"))
					break
				}
			}
		}
		fmt.Fprint(w, ")")
	case "t":
		fmt.Fprint(w, string(node.Content))
	case "pPr":
		code := false
		for i := range node.Nodes {
			switch node.Nodes[i].XMLName.Local {
			case "ind":
				if left, ok := xmlAttr(node.Nodes[i].Attrs, "left"); ok {
					if i, err := strconv.Atoi(left); err == nil && i > 0 {
						fmt.Fprint(w, strings.Repeat("  ", i/360))
					}
				}
			case "pStyle":
				if val, ok := xmlAttr(node.Nodes[i].Attrs, "val"); ok {
					if strings.HasPrefix(val, "Heading") {
						if i, err := strconv.Atoi(val[7:]); err == nil && i > 0 {
							fmt.Fprint(w, strings.Repeat("#", i)+" ")
						}
					} else if val == "Code" {
						code = true
					} else {
						if i, err := strconv.Atoi(val); err == nil && i > 0 {
							fmt.Fprint(w, strings.Repeat("#", i)+" ")
						}
					}
				}
			case "numPr":
				numID := ""
				ilvl := ""
				numFmt := ""
				start := 1
				ind := 0
				for j := range node.Nodes[i].Nodes {
					switch node.Nodes[i].Nodes[j].XMLName.Local {
					case "numId":
						if val, ok := xmlAttr(node.Nodes[i].Nodes[j].Attrs, "val"); ok {
							numID = val
						}
					case "ilvl":
						if val, ok := xmlAttr(node.Nodes[i].Nodes[j].Attrs, "val"); ok {
							ilvl = val
						}
					}
				}
				for _, num := range zf.num.Num {
					if numID != num.NumID {
						continue
					}
					for _, abnum := range zf.num.AbstractNum {
						if abnum.AbstractNumID != num.AbstractNumID.Val {
							continue
						}
						for _, ablvl := range abnum.Lvl {
							if ablvl.Ilvl != ilvl {
								continue
							}
							if i, err := strconv.Atoi(ablvl.Start.Val); err == nil {
								start = i
							}
							if i, err := strconv.Atoi(ablvl.PPr.Ind.Left); err == nil {
								ind = i / 360
							}
							numFmt = ablvl.NumFmt.Val
							break
						}
						break
					}
					break
				}

				fmt.Fprint(w, strings.Repeat("  ", ind))
				switch numFmt {
				case "decimal":
					key := fmt.Sprintf("%s:%d", numID, ind)
					cur, ok := zf.list[key]
					if !ok {
						zf.list[key] = start
					} else {
						zf.list[key] = cur + 1
					}
					fmt.Fprintf(w, "%d. ", zf.list[key])
				case "bullet":
					fmt.Fprint(w, "* ")
				}
			}
		}
		if code {
			fmt.Fprint(w, "`")
		}
		for i := range node.Nodes {
			if err := zf.walk(&node.Nodes[i], w); err != nil {
				return err
			}
		}
		if code {
			fmt.Fprint(w, "`")
		}
	case "tbl":
		// table
		var rows [][]string
		for i := range node.Nodes {
			if node.Nodes[i].XMLName.Local != "tr" {
				continue
			}
			var cols []string
			for j := range node.Nodes[i].Nodes {
				if node.Nodes[i].Nodes[j].XMLName.Local != "tc" {
					continue
				}
				var cbuf bytes.Buffer
				if err := zf.walk(&node.Nodes[i].Nodes[j], &cbuf); err != nil {
					return err
				}
				cols = append(cols, strings.Replace(cbuf.String(), "\n", "", -1))
			}
			rows = append(rows, cols)
		}
		if len(rows) == 0 {
			break
		}
		// compute column widths
		maxcol := 0
		for _, cols := range rows {
			if len(cols) > maxcol {
				maxcol = len(cols)
			}
		}
		widths := make([]int, maxcol)
		for _, row := range rows {
			for i, cell := range row {
				if len(cell) > widths[i] {
					widths[i] = len(cell)
				}
			}
		}
		// output table
		for ri, row := range rows {
			if ri == 0 {
				for j := 0; j < maxcol; j++ {
					fmt.Fprint(w, "|")
					fmt.Fprint(w, strings.Repeat(" ", widths[j]))
				}
				fmt.Fprint(w, "|\n")
				for j := 0; j < maxcol; j++ {
					fmt.Fprint(w, "|")
					fmt.Fprint(w, strings.Repeat("-", widths[j]))
				}
				fmt.Fprint(w, "|\n")
			}
			for j := 0; j < maxcol; j++ {
				fmt.Fprint(w, "|")
				if j < len(row) {
					fmt.Fprint(w, xmlEscape(row[j], "|"))
					fmt.Fprint(w, strings.Repeat(" ", widths[j]-len(row[j])))
				} else {
					fmt.Fprint(w, strings.Repeat(" ", widths[j]))
				}
			}
			fmt.Fprint(w, "|\n")
		}
		fmt.Fprint(w, "\n")
	case "r":
		bold := false
		italic := false
		strike := false
		for i := range node.Nodes {
			if node.Nodes[i].XMLName.Local != "rPr" {
				continue
			}
			for j := range node.Nodes[i].Nodes {
				switch node.Nodes[i].Nodes[j].XMLName.Local {
				case "b":
					bold = true
				case "i":
					italic = true
				case "strike":
					strike = true
				}
			}
		}
		if strike {
			fmt.Fprint(w, "~~")
		}
		if bold {
			fmt.Fprint(w, "**")
		}
		if italic {
			fmt.Fprint(w, "*")
		}
		var cbuf bytes.Buffer
		for i := range node.Nodes {
			if err := zf.walk(&node.Nodes[i], &cbuf); err != nil {
				return err
			}
		}
		fmt.Fprint(w, xmlEscape(cbuf.String(), `*~\`))
		if italic {
			fmt.Fprint(w, "*")
		}
		if bold {
			fmt.Fprint(w, "**")
		}
		if strike {
			fmt.Fprint(w, "~~")
		}
	case "p":
		for i := range node.Nodes {
			if err := zf.walk(&node.Nodes[i], w); err != nil {
				return err
			}
		}
		fmt.Fprintln(w)
	case "blip":
		// images embedded in docx - skip inline images for text extraction
	case "Fallback":
	case "txbxContent":
		var cbuf bytes.Buffer
		for i := range node.Nodes {
			if err := zf.walk(&node.Nodes[i], &cbuf); err != nil {
				return err
			}
		}
		fmt.Fprintln(w, "\n```\n"+cbuf.String()+"```")
	default:
		for i := range node.Nodes {
			if err := zf.walk(&node.Nodes[i], w); err != nil {
				return err
			}
		}
	}
	return nil
}

func docxToMarkdown(zr *zip.Reader) (string, error) {
	var rels xmlRelationships
	var num xmlNumbering

	for _, f := range zr.File {
		switch f.Name {
		case "word/_rels/document.xml.rels", "word/_rels/document2.xml.rels":
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			if err := xml.Unmarshal(b, &rels); err != nil {
				return "", err
			}
		case "word/numbering.xml":
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			if err := xml.Unmarshal(b, &num); err != nil {
				return "", err
			}
		}
	}

	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" || f.Name == "word/document2.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", errors.New("invalid docx: word/document.xml not found")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return "", err
	}

	var node xmlNode
	if err := xml.Unmarshal(b, &node); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	zf := &docxFile{
		zr:   zr,
		rels: rels,
		num:  num,
		list: make(map[string]int),
	}
	if err := zf.walk(&node, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// extractDocxTitle tries to read the title from docProps/core.xml
func extractDocxTitle(zr *zip.Reader) string {
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
			// Simple extraction: find <dc:title>...</dc:title>
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
