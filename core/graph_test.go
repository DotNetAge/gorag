package core

import "testing"

// TestIsRootLabel 验证文档根节点标签判断：四类根标签返回 true，其余（含 Region）返回 false。
func TestIsRootLabel(t *testing.T) {
	for _, label := range []string{LabelDocument, LabelCode, LabelImage, LabelDataFile} {
		if !IsRootLabel(label) {
			t.Errorf("IsRootLabel(%q) 期望 true", label)
		}
	}
	for _, label := range []string{LabelRegion, "Person", "Organization", "", "DataFileX", "Document2"} {
		if IsRootLabel(label) {
			t.Errorf("IsRootLabel(%q) 期望 false", label)
		}
	}
}

// TestRootLabelsContentType 验证根标签与 content_type 取值同源，避免两处命名漂移。
func TestRootLabelsContentType(t *testing.T) {
	pairs := []struct {
		label string
		ct    string
	}{
		{LabelDocument, ContentTypeDocument},
		{LabelCode, ContentTypeCode},
		{LabelImage, ContentTypeImage},
		{LabelDataFile, ContentTypeDataFile},
	}
	for _, p := range pairs {
		if p.label != p.ct {
			t.Errorf("Label %q 与 ContentType %q 不一致", p.label, p.ct)
		}
	}
}
