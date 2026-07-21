package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecodeUTF16LE(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "简单 ASCII",
			data: []byte{0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00, 0x00, 0x00},
			want: "Hello",
		},
		{
			name: "带空终止符",
			data: []byte{0x41, 0x00, 0x42, 0x00, 0x00, 0x00, 0x43, 0x00},
			want: "AB",
		},
		{
			name: "空字符串",
			data: []byte{0x00, 0x00},
			want: "",
		},
		{
			name: "中文字符",
			// "测试" in UTF-16LE: 测=U+6D4B, 试=U+8BD5
			data: []byte{0x4B, 0x6D, 0xD5, 0x8B, 0x00, 0x00},
			want: "测试",
		},
		{
			name: "奇数字节长度",
			data: []byte{0x48, 0x00, 0x65},
			want: "",
		},
		{
			name: "空数据",
			data: []byte{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeUTF16LE(tt.data)
			if got != tt.want {
				t.Errorf("decodeUTF16LE = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

func TestDecodeFILETIME(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string // RFC3339 字符串；空字符串表示零值
	}{
		{
			name: "零值",
			data: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			want: "",
		},
		{
			name: "字节数过短",
			data: []byte{1, 2, 3},
			want: "",
		},
		{
			name: "2024-01-15 14:30 UTC",
			data: func() []byte {
				t := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
				// 转换为 FILETIME ticks（自 1601-01-01 起的 100ns 计数）
				const unixEpochDiff = 11644473600
				ticks := uint64(t.Unix()+unixEpochDiff) * 10_000_000
				buf := make([]byte, 8)
				buf[0] = byte(ticks)
				buf[1] = byte(ticks >> 8)
				buf[2] = byte(ticks >> 16)
				buf[3] = byte(ticks >> 24)
				buf[4] = byte(ticks >> 32)
				buf[5] = byte(ticks >> 40)
				buf[6] = byte(ticks >> 48)
				buf[7] = byte(ticks >> 56)
				return buf
			}(),
			want: "2024-01-15T14:30:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeFILETIME(tt.data)
			if tt.want == "" {
				if !got.IsZero() {
					t.Errorf("decodeFILETIME = %v, 期望零值时间", got)
				}
			} else {
				if got.Format(time.RFC3339) != tt.want {
					t.Errorf("decodeFILETIME = %s, 期望 %s", got.Format(time.RFC3339), tt.want)
				}
			}
		})
	}
}

func TestStreamKey(t *testing.T) {
	key := streamKey(0x0037, 0x001F)
	expected := "__substg1.0_0037001F"
	if key != expected {
		t.Errorf("streamKey = %q, 期望 %q", key, expected)
	}
}

func TestExtractMSGRecipients(t *testing.T) {
	// 无收件人数据的空 streams
	streams := make(map[string][]byte)
	to, cc := extractMSGRecipients(streams)
	if len(to) != 0 {
		t.Errorf("期望空 to 列表, 实际 %d 项", len(to))
	}
	if len(cc) != 0 {
		t.Errorf("期望空 cc 列表, 实际 %d 项", len(cc))
	}
}

func TestParseMSG_InvalidFile(t *testing.T) {
	// 非 OLE2 数据应返回错误
	_, err := ParseMSG(strings.NewReader("not an ole2 file"))
	if err == nil {
		t.Fatal("无效 MSG 数据应返回错误")
	}
}

func TestParseMSG_WithTestFile(t *testing.T) {
	skipIfMsgFileMissing(t, "simple.msg")
	f := openTestFile(t, "simple.msg")
	defer f.Close()

	doc, err := ParseMSG(f)
	if err != nil {
		t.Fatalf("ParseMSG 失败: %v", err)
	}

	if doc.Content() == "" {
		t.Fatal("ParseMSG 返回空内容")
	}

	// 验证元数据
	meta := doc.Meta()
	if meta["email"] != true {
		t.Error("期望 email 标记为 true")
	}
	t.Logf("MSG from: %v", meta["from"])
	t.Logf("MSG subject: %v", meta["subject"])
	t.Logf("MSG to: %v", meta["to"])
	t.Logf("MSG 输出长度: %d", len(doc.Content()))

	// V2：MSG 归一化为 RawDocData
	if doc.Type() != RawDocData {
		t.Errorf("期望 docType %q, 实际 %q", RawDocData, doc.Type())
	}
}

func skipIfMsgFileMissing(t *testing.T, name string) {
	t.Helper()
	path := filepath.Join(testDataDir, name)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Skipf("跳过：测试文件 %s 不存在", name)
	}
	if err == nil && info.Size() < 1024 {
		t.Skipf("跳过：测试文件 %s 仅 %d 字节（疑似 LFS 占位符）", name, info.Size())
	}
}
