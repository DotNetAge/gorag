package document

import "strings"

var funcDict = map[string]ParseFunc{
	".pdf":  ParsePDF,
	".doc":  ParseDocx,
	".docx": ParseDocx,
	".pptx": ParsePPTX,
	".ppt":  ParsePPTX,
	".xlsx": ParseXlsx,
	".csv":  ParseCSV,
	".xls":  ParseXlsx,
	".html": ParseHTML,
	".htm":  ParseHTML,
	".jpg":  ParseImage,
	".jpeg": ParseImage,
	".png":  ParseImage,
}

var mimeDict = map[string]ParseFunc{
	"application/pdf":               ParsePDF,
	"application/msword":            ParseDocx,
	"application/vnd.ms-powerpoint": ParsePPTX,
	"application/vnd.ms-excel":      ParseXlsx,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ParseDocx,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ParsePPTX,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ParseXlsx,
	"text/csv":   ParseCSV,
	"text/html":  ParseHTML,
	"image/jpeg": ParseImage,
	"image/png":  ParseImage,
	"text/plain": ParseText,
}

func getParserByMIME(mime string) ParseFunc {
	if parser, ok := mimeDict[mime]; ok {
		return parser
	}
	// text/* 类型兜底到纯文本
	if strings.HasPrefix(mime, "text/") {
		return ParseText
	}
	return ParseText
}

func getParserByExt(ext string) ParseFunc {
	if parser, ok := funcDict[ext]; ok {
		return parser
	}
	return ParseText
}
