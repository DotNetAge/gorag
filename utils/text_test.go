package utils

import (
	"strings"
	"testing"
)

func TestCleanNoise(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "HTML tags removed",
			input:    "<p>Hello</p> <div>World</div>",
			expected: "Hello World",
		},
		{
			name:     "HTML entities decoded",
			input:    "Hello&nbsp;World &amp; Go",
			expected: "Hello World & Go",
		},
		{
			name:     "control characters removed",
			input:    "Hello\x00World\x07Test",
			expected: "HelloWorldTest",
		},
		{
			name:     "zero-width spaces normalized",
			input:    "Hello\u200bWorld\u200cTest",
			expected: "Hello World Test",
		},
		{
			name:     "multiple spaces merged",
			input:    "Hello    World   Test",
			expected: "Hello World Test",
		},
		{
			name:     "mixed Chinese and English",
			input:    "<p>你好Hello</p> <div>World世界</div>",
			expected: "你好Hello World世界",
		},
		{
			name:     "tabs and newlines preserved",
			input:    "Hello\tWorld\nTest",
			expected: "Hello World Test",
		},
		{
			name:     "non-breaking space normalized",
			input:    "Hello\u00a0World",
			expected: "Hello World",
		},
		{
			name:     "Chinese with HTML and spaces",
			input:    "<div>中文测试</div>   <span>English</span>",
			expected: "中文测试 English",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanNoise(tt.input)
			if result != tt.expected {
				t.Errorf("CleanNoise(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Markdown image removed",
			input:    "![alt text](https://example.com/image.png)",
			expected: "",
		},
		{
			name:     "Markdown link with text preserved",
			input:    "[Click here](https://example.com)",
			expected: "Click here",
		},
		{
			name:     "HTML link with text preserved",
			input:    "<a href=\"https://example.com\">Visit us</a>",
			expected: "Visit us",
		},
		{
			name:     "bare URL removed",
			input:    "Check https://example.com for details",
			expected: "Check for details",
		},
		{
			name:     "mixed Markdown and HTML links",
			input:    "[Link](https://a.com) <a href=\"https://b.com\">HTML</a>",
			expected: "Link HTML",
		},
		{
			name:     "Chinese text with links",
			input:    "[中文链接](https://example.cn) <a href=\"https://test.com\">英文链接</a>",
			expected: "中文链接 英文链接",
		},
		{
			name:     "multiple URLs removed",
			input:    "Visit https://a.com and https://b.com today",
			expected: "Visit and today",
		},
		{
			name:     "bare URL with path removed",
			input:    "See https://example.com/path/to/page for more",
			expected: "See for more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveLinks(tt.input)
			if result != tt.expected {
				t.Errorf("RemoveLinks(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeParagraphs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple newlines normalized",
			input:    "Para1\n\n\n\nPara2",
			expected: "Para1\n\nPara2",
		},
		{
			name:     "leading tabs removed",
			input:    "\t\tIndented line",
			expected: "Indented line",
		},
		{
			name:     "trailing spaces trimmed per line",
			input:    "Line1   \nLine2   ",
			expected: "Line1\nLine2",
		},
		{
			name:     "mixed Chinese and English paragraphs",
			input:    "中文段落1\n\n\n英文Paragraph\n\n\n混合内容",
			expected: "中文段落1\n\n英文Paragraph\n\n混合内容",
		},
		{
			name:     "tabs and newlines mixed",
			input:    "\tLine1\n\n\n\tLine2",
			expected: "Line1\n\nLine2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeParagraphs(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeParagraphs(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToHalfWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "fullwidth punctuation",
			input:    "！？。，",
			expected: "!?.,", // 正确顺序：！→!, ？→?, 。→., ，→,
		},
		{
			name:     "fullwidth numbers",
			input:    "１２３４５６７８９０",
			expected: "1234567890",
		},
		{
			name:     "fullwidth uppercase letters",
			input:    "ＡＢＣＤＥＦＧＨＩＪ",
			expected: "ABCDEFGHIJ",
		},
		{
			name:     "fullwidth lowercase letters",
			input:    "ａｂｃｄｅｆｇｈｉｊ",
			expected: "abcdefghij",
		},
		{
			name:     "fullwidth space",
			input:    "Hello　World",
			expected: "Hello World",
		},
		{
			name:     "mixed fullwidth and halfwidth",
			input:    "ＡＢＣ123def",
			expected: "ABC123def",
		},
		{
			name:     "Chinese with fullwidth",
			input:    "中文ＡＢＣ和１２３",
			expected: "中文ABC和123",
		},
		{
			name:     "fullwidth symbols",
			input:    "＜＞［］｛｝",
			expected: "<>[]{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToHalfWidth(tt.input)
			if result != tt.expected {
				t.Errorf("ToHalfWidth(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeChinese(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single characters",
			input:    "國中語文書",
			expected: "国中语文书",
		},
		{
			name:     "common words",
			input:    "電腦軟體網路",
			expected: "电脑软件网络",
		},
		{
			name:     "mixed traditional and simplified",
			input:    "這是中文測試軟體",
			expected: "这是中文测试软件",
		},
		{
			name:     "English preserved",
			input:    "Hello World 電腦",
			expected: "Hello World 电脑",
		},
		{
			name:     "numbers and symbols preserved",
			input:    "系統2024年",
			expected: "系统2024年",
		},
		{
			name:     "already simplified",
			input:    "这是一个简单的测试",
			expected: "这是一个简单的测试",
		},
		{
			name:     "technical terms",
			input:    "數據庫網站應用程序",
			expected: "数据库网站应用程序",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeChinese(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeChinese(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveWatermarks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Chinese watermark removed",
			input:    "机密文件内容",
			expected: "文件内容",
		},
		{
			name:     "multiple watermarks removed",
			input:    "内部文件版权所有未经授权",
			expected: "",
		},
		{
			name:     "English watermark case insensitive",
			input:    "CONFIDENTIAL content INTERNAL",
			expected: " content ",
		},
		{
			name:     "mixed watermarks",
			input:    "机密 Internal 严禁传播 Copyright",
			expected: "   ",
		},
		{
			name:     "watermark in sentence",
			input:    "这是一份机密报告，请查收",
			expected: "这是一份报告，请查收",
		},
		{
			name:     "no watermark",
			input:    "正常内容没有水印",
			expected: "正常内容没有水印",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveWatermarks(tt.input)
			if result != tt.expected {
				t.Errorf("RemoveWatermarks(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveLineNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "line numbers with dot",
			input:    "1. First line\n2. Second line\n3. Third line",
			expected: "First line\nSecond line\nThird line",
		},
		{
			name:     "line numbers with parenthesis",
			input:    "1) First\n2) Second\n3) Third",
			expected: "First\nSecond\nThird",
		},
		{
			name:     "no line numbers",
			input:    "Normal text without numbers",
			expected: "Normal text without numbers",
		},
		{
			name:     "mixed content",
			input:    "1. Item one\nSome text\n2. Item two",
			expected: "Item one\nSome text\nItem two",
		},
		{
			name:     "Chinese with numbers",
			input:    "1. 第一项\n2. 第二项\n3. 第三项",
			expected: "第一项\n第二项\n第三项",
		},
		{
			name:     "number not at line start",
			input:    "Text with 1.5 numbers in it",
			expected: "Text with 1.5 numbers in it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveLineNumbers(tt.input)
			if result != tt.expected {
				t.Errorf("RemoveLineNumbers(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDesensitizePII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "ID card masked",
			input:    "身份证号11010519491231002X",
			expected: "身份证号310***********1234",
		},
		{
			name:     "phone number masked",
			input:    "联系电话13812345678",
			expected: "联系电话138****1234",
		},
		{
			name:     "bank card masked",
			input:    "银行卡号6222021234567890123",
			expected: "银行卡号6222****1234",
		},
		{
			name:     "API key masked",
			input:    "API密钥sk1234567890abcdefghijklmnopqrst",
			expected: "API密钥sk-****xxxx",
		},
		{
			name:     "email masked",
			input:    "邮箱test@example.com",
			expected: "邮箱a***@example.com",
		},
		{
			name:     "UUID preserved",
			input:    "UUID: 550e8400-e29b-41d4-a716-446655440000",
			expected: "UUID: 550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "multiple PII",
			input:    "手机13812345678邮箱test@example.com身份证11010519491231002X",
			expected: "手机138****1234邮箱a***@example.com身份证310***********1234",
		},
		{
			name:     "English PII",
			input:    "Phone: 13812345678 Email: test@test.com",
			expected: "Phone: 138****1234 Email: a***@test.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DesensitizePII(tt.input)
			if result != tt.expected {
				t.Errorf("DesensitizePII(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "basic punctuation preserved",
			input:    "Hello!!! World???",
			expected: "Hello!!! World???", // 标点符号被保留
		},
		{
			name:     "multiple spaces merged",
			input:    "Hello    World",
			expected: "Hello World",
		},
		{
			name:     "Chinese and English preserved",
			input:    "你好Hello世界",
			expected: "你好Hello世界", // Clean 保留中英文，不添加空格
		},
		{
			name:     "numbers preserved",
			input:    "Test123Case456",
			expected: "Test123Case456", // Clean 保留连续字母数字
		},
		{
			name:     "basic punctuation preserved",
			input:    "Hello, World! 你好，世界！",
			expected: "Hello, World! 你好，世界！",
		},
		{
			name:     "emoji filtered",
			input:    "Hello 👋 World 🎉",
			expected: "Hello World", // Emoji 被替换为空格，然后空格被合并
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "lowercase and cleaned",
			input:    "HELLO WORLD!!!",
			expected: "hello world!!!", // 标点符号被保留
		},
		{
			name:     "mixed Chinese and English",
			input:    "你好HELLO世界WORLD",
			expected: "你好hello世界world", // 转为小写，中英文都保留
		},
		{
			name:     "with punctuation",
			input:    "Hello, World!",
			expected: "hello, world!",
		},
		{
			name:     "with emoji",
			input:    "Hello 👋 World",
			expected: "hello world", // Emoji 被替换为空格，然后空格被合并
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize(tt.input)
			if result != tt.expected {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "basic English keywords",
			input:    "The quick brown fox jumps over the lazy dog",
			expected: []string{"quick", "brown", "fox", "jumps", "lazy", "dog"}, // "the", "over" 被识别为停用词
		},
		{
			name:     "basic Chinese keywords",
			input:    "这是一个测试关键词提取的中文句子",
			expected: []string{"这是", "一个", "测试", "关键词", "提取", "的", "中文", "句子"}, // GSE 分词结果
		},
		{
			name:     "mixed Chinese and English",
			input:    "Go语言是一种高效的编程语言Python也很好",
			expected: []string{"语言", "是", "一种", "高效", "的", "编程语言", "python", "也", "很", "好"}, // GSE 分词结果
		},
		{
			name:     "with numbers",
			input:    "测试123提取456关键词",
			expected: []string{"测试", "123", "提取", "456", "关键词"},
		},
		{
			name:     "stop words filtered",
			input:    "this is a test document about golang programming",
			expected: []string{"test", "document", "golang", "programming"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractKeywords(tt.input)
			// Check length first
			if len(result) != len(tt.expected) {
				t.Errorf("ExtractKeywords(%q) length = %d, want %d, got %v", tt.input, len(result), len(tt.expected), result)
				return
			}
			// Check content (order may vary due to GSE segmentation)
			resultMap := make(map[string]bool)
			for _, w := range result {
				resultMap[w] = true
			}
			for _, w := range tt.expected {
				if !resultMap[w] {
					t.Errorf("ExtractKeywords(%q) missing expected word %q, got %v", tt.input, w, result)
				}
			}
		})
	}
}

func TestRemoveStopWords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "English stop words removed",
			input:    "This is a test document",
			expected: "test document", // "is", "a" 被识别为停用词
		},
		{
			name:     "Chinese stop words partially removed",
			input:    "这是一个测试的文档",
			expected: "这是 一个 测试 的 文档", // stopwords 库对中文支持有限
		},
		{
			name:     "mixed content",
			input:    "这是一个 test document about golang",
			expected: "这是 一个 test document golang", // "about" 被识别为停用词
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveStopWords(tt.input)
			if result != tt.expected {
				t.Errorf("RemoveStopWords(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCrossLanguageMix tests the functions with mixed Chinese and English content
func TestCrossLanguageMix(t *testing.T) {
	t.Run("CleanNoise mixed", func(t *testing.T) {
		input := "<div>中文内容</div> English <p>content</p> Mixed 混合"
		expected := "中文内容 English content Mixed 混合"
		result := CleanNoise(input)
		if result != expected {
			t.Errorf("CleanNoise mixed content = %q, want %q", result, expected)
		}
	})

	t.Run("RemoveLinks mixed", func(t *testing.T) {
		input := "[中文链接](https://test.com) <a href=\"https://example.com\">English Link</a>"
		expected := "中文链接 English Link"
		result := RemoveLinks(input)
		if result != expected {
			t.Errorf("RemoveLinks mixed content = %q, want %q", result, expected)
		}
	})

	t.Run("Normalize mixed", func(t *testing.T) {
		input := "HELLO 你好 WORLD 世界"
		expected := "hello 你好 world 世界"
		result := Normalize(input)
		if result != expected {
			t.Errorf("Normalize mixed content = %q, want %q", result, expected)
		}
	})

	t.Run("ExtractKeywords mixed", func(t *testing.T) {
		input := "Go语言和Python都是优秀的编程语言"
		result := ExtractKeywords(input)
		if len(result) == 0 {
			t.Errorf("ExtractKeywords mixed content returned empty, expected some keywords")
		}
		// GSE 分词结果可能包含中英文混合词
		t.Logf("ExtractKeywords mixed result: %v", result)
	})

	t.Run("RemoveStopWords mixed", func(t *testing.T) {
		input := "这是一个 test of 混合 content"
		result := RemoveStopWords(input)
		// Should have some content remaining
		if result == "" {
			t.Errorf("RemoveStopWords mixed content returned empty")
		}
	})
}

// TestPerformance checks that functions don't have obvious performance issues
func TestPerformance(t *testing.T) {
	t.Run("CleanNoise large text", func(t *testing.T) {
		// Create a larger text to ensure no obvious performance issues
		input := strings.Repeat("Hello World 你好世界\n", 1000)
		result := CleanNoise(input)
		if len(result) == 0 {
			t.Errorf("CleanNoise large text returned empty")
		}
	})

	t.Run("ExtractKeywords large text", func(t *testing.T) {
		input := strings.Repeat("Go语言是一种强大的编程语言。Python也很好用。", 100)
		result := ExtractKeywords(input)
		if len(result) == 0 {
			t.Errorf("ExtractKeywords large text returned empty")
		}
	})
}

// TestConsistency checks that multiple calls produce consistent results
func TestConsistency(t *testing.T) {
	input := "这是Test文档的Content，包含中文和English混合"

	t.Run("ExtractKeywords consistency", func(t *testing.T) {
		result1 := ExtractKeywords(input)
		result2 := ExtractKeywords(input)
		if len(result1) != len(result2) {
			t.Errorf("ExtractKeywords inconsistent: first call len=%d, second call len=%d", len(result1), len(result2))
		}
	})

	t.Run("Clean consistency", func(t *testing.T) {
		result1 := Clean(input)
		result2 := Clean(input)
		if result1 != result2 {
			t.Errorf("Clean inconsistent: first=%q, second=%q", result1, result2)
		}
	})

	t.Run("Normalize consistency", func(t *testing.T) {
		result1 := Normalize(input)
		result2 := Normalize(input)
		if result1 != result2 {
			t.Errorf("Normalize inconsistent: first=%q, second=%q", result1, result2)
		}
	})
}

// Benchmark tests for performance reference
func BenchmarkCleanNoise(b *testing.B) {
	input := "<p>中文内容</p> English content Mixed 混合" + strings.Repeat(" Test ", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CleanNoise(input)
	}
}

func BenchmarkExtractKeywords(b *testing.B) {
	input := "Go语言是一种强大的编程语言，Python也很好用。JavaScript是前端开发的重要语言。" + strings.Repeat(" testing ", 50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractKeywords(input)
	}
}

func BenchmarkRemoveStopWords(b *testing.B) {
	input := "这是一个测试文档，包含中英文混合内容，用于测试停用词去除功能是否正常工作。"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RemoveStopWords(input)
	}
}
