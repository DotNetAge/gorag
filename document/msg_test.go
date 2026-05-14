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
			name: "simple ascii",
			data: []byte{0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00, 0x00, 0x00},
			want: "Hello",
		},
		{
			name: "with null terminator",
			data: []byte{0x41, 0x00, 0x42, 0x00, 0x00, 0x00, 0x43, 0x00},
			want: "AB",
		},
		{
			name: "empty",
			data: []byte{0x00, 0x00},
			want: "",
		},
		{
			name: "chinese characters",
			// "测试" in UTF-16LE: 测=U+6D4B, 试=U+8BD5
			data: []byte{0x4B, 0x6D, 0xD5, 0x8B, 0x00, 0x00},
			want: "测试",
		},
		{
			name: "odd length bytes",
			data: []byte{0x48, 0x00, 0x65},
			want: "",
		},
		{
			name: "empty data",
			data: []byte{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeUTF16LE(tt.data)
			if got != tt.want {
				t.Errorf("decodeUTF16LE = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeFILETIME(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string // RFC3339 string or empty for zero
	}{
		{
			name: "zero",
			data: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			want: "",
		},
		{
			name: "too short",
			data: []byte{1, 2, 3},
			want: "",
		},
		{
			name: "jan 1 2024",
			// FILETIME for 2024-01-01 00:00:00 UTC
			// Ticks since 1601-01-01: (2024-1601)*365.2425*86400*10000000 ≈ 133456608000000000
			// Let me just use a known value
			data: func() []byte {
				t := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
				// Convert to FILETIME ticks (100-ns intervals since 1601-01-01)
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
					t.Errorf("decodeFILETIME = %v, want zero time", got)
				}
			} else {
				if got.Format(time.RFC3339) != tt.want {
					t.Errorf("decodeFILETIME = %s, want %s", got.Format(time.RFC3339), tt.want)
				}
			}
		})
	}
}

func TestStreamKey(t *testing.T) {
	key := streamKey(0x0037, 0x001F)
	expected := "__substg1.0_0037001F"
	if key != expected {
		t.Errorf("streamKey = %q, want %q", key, expected)
	}
}

func TestExtractMSGRecipients(t *testing.T) {
	// 无收件人数据的空 streams
	streams := make(map[string][]byte)
	to, cc := extractMSGRecipients(streams)
	if len(to) != 0 {
		t.Errorf("Expected empty to list, got %d items", len(to))
	}
	if len(cc) != 0 {
		t.Errorf("Expected empty cc list, got %d items", len(cc))
	}
}

func TestParseMSG_InvalidFile(t *testing.T) {
	// 非 OLE2 数据应返回错误
	_, err := ParseMSG(strings.NewReader("not an ole2 file"))
	if err == nil {
		t.Fatal("Expected error for invalid MSG data")
	}
}

func TestParseMSG_WithTestFile(t *testing.T) {
	skipIfMsgFileMissing(t, "simple.msg")
	f := openTestFile(t, "simple.msg")
	defer f.Close()

	doc, err := ParseMSG(f)
	if err != nil {
		t.Fatalf("ParseMSG failed: %v", err)
	}

	if doc.Text == "" {
		t.Fatal("ParseMSG returned empty text")
	}

	// 验证元数据
	if doc.Meta["email"] != true {
		t.Error("Expected email flag to be true")
	}
	t.Logf("MSG from: %v", doc.Meta["from"])
	t.Logf("MSG subject: %v", doc.Meta["subject"])
	t.Logf("MSG to: %v", doc.Meta["to"])
	t.Logf("MSG output length: %d", len(doc.Text))
}

func skipIfMsgFileMissing(t *testing.T, name string) {
	t.Helper()
	path := filepath.Join(testDataDir, name)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Skipf("Skipping: test file %s not found", name)
	}
	if err == nil && info.Size() < 1024 {
		t.Skipf("Skipping: test file %s is only %d bytes (possible LFS placeholder)", name, info.Size())
	}
}
