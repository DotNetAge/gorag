package document

// funcDict 扩展名 → ParseFunc 映射表。
//
// 基于扩展名查找解析器；未知扩展名兜底到 ParseText。
//
// 归一化策略：
//   - 文档类（pdf/docx/html/epub/pptx）→ Markdown
//   - 数据类（csv/xlsx/json/yaml/xml/eml/msg/toml/log）→ JSON
//   - 文本类（txt/代码/md 等）→ 原文
//   - 图片类（jpg/png/gif/webp/bmp/tiff）→ Base64
var funcDict = map[string]ParseFunc{
	// 文档类 → Markdown
	".pdf":  ParsePDF,
	".doc":  ParseDocx,
	".docx": ParseDocx,
	".pptx": ParsePPTX,
	".ppt":  ParsePPTX,
	".html": ParseHTML,
	".htm":  ParseHTML,
	".epub": ParseEPUB,

	// 数据类 → JSON
	".csv":  ParseCSV,
	".xls":  ParseXlsx,
	".xlsx": ParseXlsx,
	".json": ParseJSON,
	".yml":  ParseYAML,
	".yaml": ParseYAML,
	".xml":  ParseXML,
	".eml":  ParseEML,
	".msg":  ParseMSG,
	".toml": ParseTOML,
	".log":  ParseLog,

	// 图片类 → Base64
	".jpg":  ParseImage,
	".jpeg": ParseImage,
	".png":  ParseImage,
	".gif":  ParseImage,
	".webp": ParseImage,
	".bmp":  ParseImage,
	".tiff": ParseImage,
	".tif":  ParseImage,
}

// getParserByExt 按扩展名查找 ParseFunc，未命中时返回 ParseText 兜底。
func getParserByExt(ext string) ParseFunc {
	if parser, ok := funcDict[ext]; ok {
		return parser
	}
	return ParseText
}
