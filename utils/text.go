package utils

import (
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/go-ego/gse"
	"github.com/zoomio/stopwords"
)

// GSE 分词器全局实例（延迟初始化以避免进口内存）
var (
	gseSeg     *gse.Segmenter
	gseSegOnce sync.Once
)

// getGseSegmenter 获取 GSE 分词器单例
func getGseSegmenter() *gse.Segmenter {
	gseSegOnce.Do(func() {
		gseSeg = &gse.Segmenter{}
		gseSeg.LoadDict()
	})
	return gseSeg
}

// 预编译正则表达式（避免每次调用重新编译）
var (
	// CleanNoise
	reControlChars = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`) // 排除 \t \n \r
	reMultiSpace   = regexp.MustCompile(`\s+`)

	// RemoveLinks
	reMarkdownImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)
	reMarkdownLink  = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	reBareURL      = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

	// NormalizeParagraphs
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
	reLeadingTab   = regexp.MustCompile(`(?m)^\t+`)

	// RemoveLineNumbers - 更精确的行号匹配（需要后面跟着非数字内容）
	reLineNumbers = regexp.MustCompile(`(?m)^\s*\d+[\.\)]\s+`)

	// DesensitizePII
	reIDCard   = regexp.MustCompile(`\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)
	rePhone    = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	reBankCard = regexp.MustCompile(`\b(?:62|4|5)\d{14,17}\b`) // 银行卡通常以 62/4/5 开头
	reAPIKey   = regexp.MustCompile(`\b(?:sk-|pk-|api_)?[A-Za-z0-9]{32,}\b`)
	reEmail    = regexp.MustCompile(`\b([\w.+-]+)@([\w.-]+\.[a-zA-Z]{2,})\b`)

	// Clean - 保留中文、字母、数字、空白和基本标点
	reSpecialChars = regexp.MustCompile(`[^\p{Han}\p{L}\p{N}\s.,;!?:'"()。，、！？；：（）""'']`)
)

// 繁简转换映射表（扩展版）
var traditionalToSimplified = map[string]string{
	// 常见词汇（长词优先）
	"系統": "系统", "資料": "资料", "處理": "处理", "資訊": "信息",
	"電腦": "电脑", "軟體": "软件", "硬體": "硬件", "網路": "网络",
	"網站": "网站", "數據": "数据", "程序": "程序", "設計": "设计",
	"開發": "开发", "測試": "测试", "維護": "维护", "部署": "部署",
	"服務": "服务", "應用": "应用", "功能": "功能", "界面": "界面",
	"視窗": "视窗", "文件": "文件", "夾": "夹", "檔案": "档案",
	"數據庫": "数据库", "連結": "链接", "連接": "连接",
	"設定": "设定", "配置": "配置", "選項": "选项", "參數": "参数",
	"變量": "变量", "常量": "常量", "函數": "函数", "方法": "方法",
	"類別": "类别", "對象": "对象", "實例": "实例", "屬性": "属性",
	"字段": "字段", "記錄": "记录", "查詢": "查询", "結果": "结果",
	"輸出": "输出", "輸入": "输入", "打印": "打印", "顯示": "显示",
	"頁面": "页面", "視圖": "视图", "模板": "模板", "樣式": "样式",
	"腳本": "脚本", "代碼": "代码", "編程": "编程", "算法": "算法",
	"結構": "结构", "架構": "架构", "模組": "模块", "組件": "组件",
	"框架": "框架", "環境": "环境", "平台": "平台", "工具": "工具",
	"技術": "技术", "方案": "方案", "問題": "问题", "解決": "解决",
	"優化": "优化", "性能": "性能", "效率": "效率", "質量": "质量",
	"調試": "调试", "錯誤": "错误", "異常": "异常",
	"日誌": "日志", "監控": "监控", "警報": "警报", "通知": "通知",
	"帳戶": "账户", "用戶": "用户", "權限": "权限", "安全": "安全",
	"加密": "加密", "解密": "解密", "認證": "认证", "授權": "授权",
	"登錄": "登录", "註冊": "注册", "賬號": "账号", "密碼": "密码",
	// 常见字
	"國": "国", "中": "中", "文": "文", "語": "语", "言": "言",
	"書": "书", "學": "学", "習": "习", "業": "业", "務": "务",
	"機": "机", "會": "会", "時": "时", "間": "间", "地": "地",
	"區": "区", "域": "域", "門": "门", "窗": "窗", "戶": "户",
	"車": "车", "馬": "马", "鳥": "鸟", "魚": "鱼", "蟲": "虫",
	"龍": "龙", "鳳": "凤", "飛": "飞", "雲": "云", "風": "风",
	"雨": "雨", "雪": "雪", "電": "电", "氣": "气", "水": "水",
	"火": "火", "土": "土", "金": "金", "木": "木", "石": "石",
	"山": "山", "河": "河", "海": "海", "湖": "湖", "島": "岛",
	"腦": "脑", "網": "网", "訊": "讯", "頭": "头", "殼": "壳",
	"見": "见", "視": "视", "許": "许", "號": "号",
	"來": "来", "裡": "里", "認": "认", "識": "识",
	"開": "开", "東": "东", "西": "西", "南": "南",
	"北": "北", "萬": "万", "與": "与", "義": "义",
	// 补充缺失的字符
	"這": "这", "為": "为", "過": "过", "個": "个", "們": "们",
	"將": "将", "對": "对", "於": "于", "說": "说",
	"請": "请", "問": "问", "長": "长", "短": "短",
	"新": "新", "舊": "旧", "難": "难", "易": "易",
	"大": "大", "小": "小", "多": "多", "少": "少",
	"好": "好", "壞": "坏", "真": "真", "假": "假",
	"空": "空", "滿": "满", "紅": "红", "藍": "蓝",
	"黃": "黄", "綠": "绿", "黑": "黑", "白": "白",
	"動": "动", "靜": "静", "快": "快", "慢": "慢",
	"重": "重", "輕": "轻", "軟": "软", "硬": "硬",
	"深": "深", "淺": "浅", "遠": "远", "近": "近",
	"高": "高", "低": "低",
}

// CleanNoise 去除噪音字符。
// 升级说明：使用 GSE FilterHtml 增强 HTML 标签过滤能力，
// 保留原有的 HTML 实体解码、控制字符和空白字符处理逻辑。
func CleanNoise(text string) string {
	if text == "" {
		return ""
	}

	// 先解码 HTML 实体
	text = decodeHTMLEntities(text)

	// 使用 GSE 过滤 HTML 标签
	text = gse.FilterHtml(text)

	// 删除控制字符（保留 \t \n \r）
	text = reControlChars.ReplaceAllString(text, "")

	// 规范化空白字符（零宽空格、不间断空格等 → 普通空格）
	var result strings.Builder
	result.Grow(len(text))
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			result.WriteRune(' ')
		case r == '\u200b', r == '\u200c', r == '\u200d', r == '\ufeff', r == '\u00a0':
			result.WriteRune(' ')
		default:
			result.WriteRune(r)
		}
	}
	text = result.String()

	// 合并连续空格
	text = reMultiSpace.ReplaceAllString(text, " ")

	// Trim首尾空白
	return strings.TrimSpace(text)
}

// RemoveLinks 去除链接。
// 升级说明：使用 GSE FilterHtml 移除 HTML 标签和链接，保留 Markdown 链接和裸露 URL 的处理。
func RemoveLinks(text string) string {
	if text == "" {
		return ""
	}

	// 使用 GSE 过滤 HTML 标签
	text = gse.FilterHtml(text)

	// 去除 Markdown 图片
	text = reMarkdownImage.ReplaceAllString(text, "")

	// 去除 Markdown 链接（保留链接文字）
	text = reMarkdownLink.ReplaceAllString(text, "$1")

	// 去除裸露 URL
	text = reBareURL.ReplaceAllString(text, "")

	// 合并多余空格（GSE FilterHtml 可能留下双空格）
	text = reMultiSpace.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// decodeHTMLEntities 解码HTML实体。
func decodeHTMLEntities(text string) string {
	entities := map[string]string{
		"&nbsp;":   " ",
		"&lt;":     "<",
		"&gt;":     ">",
		"&amp;":    "&",
		"&quot;":   `"`,
		"&apos;":   "'",
		"&hellip;": "…",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&copy;":   "©",
		"&reg;":    "®",
		"&trade;":  "™",
	}

	for entity, char := range entities {
		text = strings.ReplaceAll(text, entity, char)
	}

	return text
}

// NormalizeParagraphs 规范化段落。
// 注意：此函数保留原有实现，GSE 无直接对应的段落结构化处理功能。
func NormalizeParagraphs(text string) string {
	if text == "" {
		return ""
	}

	// 合并多余换行（超过2个换行 → 2个换行）
	text = reMultiNewline.ReplaceAllString(text, "\n\n")

	// 去除行首行尾空格
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	// 规范化缩进（去除行首 Tab）
	text = reLeadingTab.ReplaceAllString(text, "")

	return text
}

// ToHalfWidth 全角半角转换。
// 注意：GSE 未提供全角半角转换功能，此函数保留原有实现。
// GSE 主要提供分词、停用词过滤和文本清洗功能。
func ToHalfWidth(text string) string {
	if text == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		switch {
		case r == '\u3000': // 全角空格
			result.WriteRune(' ')
		case r >= '\uFF01' && r <= '\uFF5E': // 全角 ASCII 扩展字符 ! ~ ~
			result.WriteRune(r - 0xFEE0)
		case r >= '\uFF10' && r <= '\uFF19': // 全角数字 ０-９
			result.WriteRune(r - 0xFEE0)
		case r >= '\uFF21' && r <= '\uFF3A': // 全角大写字母 Ａ-Ｚ
			result.WriteRune(r - 0xFEE0)
		case r >= '\uFF41' && r <= '\uFF5A': // 全角小写字母 ａ-ｚ
			result.WriteRune(r - 0xFEE0)
		case r == '\u3002': // 全角句号 。
			result.WriteRune('.')
		case r == '\u3001': // 全角逗号 ，
			result.WriteRune(',')
		case r == '\uFF08': // 全角左括号 （
			result.WriteRune('(')
		case r == '\uFF09': // 全角右括号 ）
			result.WriteRune(')')
		case r == '\u300C': // 全角左引号 「
			result.WriteRune('"')
		case r == '\u300D': // 全角右引号 」
			result.WriteRune('"')
		case r == '\u300E': // 全角左引号 『
			result.WriteRune('\'')
		case r == '\u300F': // 全角右引号 』
			result.WriteRune('\'')
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// NormalizeChinese 繁简转换（扩展版）。
// 注意：GSE 未提供繁简转换功能，此函数保留原有实现。
// 未来可探索使用 GSE 分词结合外部繁简转换库（如 gocc）实现更全面的转换。
func NormalizeChinese(text string) string {
	if text == "" {
		return ""
	}

	// 构建有序的替换对列表（按长度降序）
	type pair struct{ trad, simp string}
	pairs := make([]pair, 0, len(traditionalToSimplified))
	for trad, simp := range traditionalToSimplified {
		pairs = append(pairs, pair{trad, simp})
	}
	// 按长度降序排序
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if len(pairs[j].trad) > len(pairs[i].trad) {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// 依次替换
	for _, p := range pairs {
		text = strings.ReplaceAll(text, p.trad, p.simp)
	}

	return text
}

// RemoveWatermarks 去除水印。
// 注意：此函数保留原有实现，水印检测依赖领域关键词匹配。
// 未来可探索利用 GSE 分词增强关键词匹配准确性。
func RemoveWatermarks(text string) string {
	if text == "" {
		return ""
	}

	// 常见水印关键词（支持正则）
	watermarks := []struct {
		pattern string
		isRegex bool
	}{
		{"机密", false},
		{"内部文件", false},
		{"版权所有", false},
		{"未经授权", false},
		{"严禁传播", false},
		{"(?i)CONFIDENTIAL", true},
		{"(?i)INTERNAL", true},
		{"(?i)COPYRIGHT", true},
		{"(?i)UNAUTHORIZED", true},
		{"(?i)DO NOT DISTRIBUTE", true},
	}

	for _, wm := range watermarks {
		if wm.isRegex {
			re := regexp.MustCompile(wm.pattern)
			text = re.ReplaceAllString(text, "")
		} else {
			text = strings.ReplaceAll(text, wm.pattern, "")
		}
	}

	return text
}

// RemoveLineNumbers 去除代码行号。
// 注意：此函数保留原有实现，行号检测需要特定的正则模式匹配。
func RemoveLineNumbers(text string) string {
	if text == "" {
		return ""
	}

	// 只匹配 "数字. " 或 "数字) " 格式的行号
	text = reLineNumbers.ReplaceAllString(text, "")

	return text
}

// DesensitizePII 隐私脱敏。
// 注意：此函数保留原有实现，PII 脱敏需要严格的正则规则匹配。
// GSE 分词可用于辅助识别潜在实体，但正则规则更为精确可靠。
func DesensitizePII(text string) string {
	if text == "" {
		return ""
	}

	// 身份证号脱敏（18位，符合身份证号规则）
	text = reIDCard.ReplaceAllString(text, "310***********1234")

	// 手机号脱敏（11位，以1开头）
	text = rePhone.ReplaceAllString(text, "138****1234")

	// 银行卡号脱敏（16-19位，以常见银行卡前缀开头）
	text = reBankCard.ReplaceAllString(text, "6222****1234")

	// API密钥脱敏（32位以上字母数字，可选前缀）
	text = reAPIKey.ReplaceAllStringFunc(text, func(match string) string {
		// 避免误伤 UUID 和普通长字符串
		if strings.Contains(match, "-") && len(match) == 36 {
			return match // 可能是 UUID，保留
		}
		return "sk-****xxxx"
	})

	// 邮箱脱敏
	text = reEmail.ReplaceAllString(text, "a***@$2")

	return text
}

// Clean 基础清洗：去特殊符号、多余空格、统一格式。
// 升级说明：使用正则表达式保留中文、字母、数字和基本标点符号，
// 合并多余空格，返回清洗后的文本。
func Clean(s string) string {
	if s == "" {
		return ""
	}

	// 去特殊字符（保留常用标点）
	s = reSpecialChars.ReplaceAllString(s, " ")

	// 合并空格
	s = reMultiSpace.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

// Normalize 归一化：小写+清理。
// 升级说明：使用 GSE FilterEmoji 删除 emoji，然后转换为小写并清洗。
func Normalize(s string) string {
	if s == "" {
		return ""
	}
	// 使用 GSE 过滤 emoji
	s = gse.FilterEmoji(s)
	s = strings.ToLower(s)
	return Clean(s)
}

// ExtractKeywords 提取关键词。
// 升级说明：使用 GSE 的分词替代原有的空白分割，
// 结合 zoomio/stopwords 进行停用词过滤，比原有方法更准确。
func ExtractKeywords(s string) []string {
	if s == "" {
		return nil
	}

	seg := getGseSegmenter()
	stopWords := stopwords.Setup()

	// 使用 GSE 分词
	words := seg.Cut(s, true)

	// 过滤停用词和短词
	var kw []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		// 过滤停用词
		if stopWords.IsStopWord(w) {
			continue
		}
		// 过滤单字
		if len(w) <= 1 {
			continue
		}
		kw = append(kw, w)
	}

	return kw
}

// RemoveStopWords 去除停用词。
// 升级说明：使用 GSE 分词后过滤停用词，比原有方法更准确。
// GSE 的 CutStop 本身不过滤停用词，需要手动过滤。
func RemoveStopWords(s string) string {
	if s == "" {
		return ""
	}

	seg := getGseSegmenter()
	stopWords := stopwords.Setup()

	// 使用 GSE 分词
	words := seg.Cut(s, true)

	// 手动过滤停用词
	var filtered []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if stopWords.IsStopWord(w) {
			continue
		}
		filtered = append(filtered, w)
	}

	// 合并空格并修剪
	result := strings.Join(filtered, " ")
	result = reMultiSpace.ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}
