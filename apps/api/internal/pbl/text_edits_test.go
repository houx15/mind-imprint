package pbl

import (
	"strings"
	"testing"
)

func TestTextEditFailureIdentifiesRepairTarget(t *testing.T) {
	for _, tc := range []struct {
		edit TextEdit
		want string
	}{
		{TextEdit{"", "x"}, "第2处修改缺少原文"},
		{TextEdit{"a", "a"}, "第2处修改与原文相同"},
		{TextEdit{"missing", "x"}, "第2处修改原文在原始正文中匹配0次"},
		{TextEdit{"b", "x"}, "第2处修改原文在原始正文中匹配2次"},
	} {
		result, err := ApplyTextEdits("abb", []TextEdit{{"a", "new"}, tc.edit})
		if result != "" || err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("invalid edit must fail atomically with repair location: %q, %v", result, err)
		}
	}
}

func TestTextEditsPreserveUntouchedBytes(t *testing.T) {
	body := "# 原标题\n\n- 旧规则\n- 保留规则\n\n| 原话 | 理解 |\n| --- | --- |\n| | |\n"
	got, err := ApplyTextEdits(body, []TextEdit{{Old: "- 旧规则\n", New: ""}, {Old: "保留规则", New: "新规则"}})
	want := "# 原标题\n\n- 新规则\n\n| 原话 | 理解 |\n| --- | --- |\n| | |\n"
	if err != nil || got != want {
		t.Fatalf("got %q, %v", got, err)
	}
}
func TestTextEditsRejectAmbiguityOverlapAndRetargeting(t *testing.T) {
	for _, tc := range []struct {
		body  string
		edits []TextEdit
	}{
		{"重复重复", []TextEdit{{"重复", "新"}}},
		{"abc", []TextEdit{{"ab", "x"}, {"bc", "y"}}},
		{"abc", []TextEdit{{"a", "x"}, {"x", "z"}}},
		{"abc", []TextEdit{{"abc", ""}}},
		{"abc", []TextEdit{{"", "x"}}},
	} {
		if _, err := ApplyTextEdits(tc.body, tc.edits); err == nil {
			t.Fatalf("accepted invalid edits: %+v", tc)
		}
	}
}
