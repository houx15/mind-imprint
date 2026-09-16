package liteworkspace

import (
	"os"
	"strings"
	"testing"
)

func TestLabels(t *testing.T) {
	for in, want := range map[string]string{"reading": "阅读", "writing": "写作", "project": "项目"} {
		if got := KindLabel(in); got != want {
			t.Fatalf("KindLabel(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"library": "分级阅读库", "url": "链接", "text": "正文", "file": "上传文件", "personalized": "个性化",
	} {
		if got := SourceLabel(in); got != want {
			t.Fatalf("SourceLabel(%q) = %q, want %q", in, got, want)
		}
	}
	three := 3
	if got := TierLabel(&three); got != "进阶" {
		t.Fatalf("TierLabel(3) = %q, want 进阶", got)
	}
	if got := TierLabel(nil); got != "按学生水平" {
		t.Fatalf("TierLabel(nil) = %q, want 按学生水平", got)
	}
}

// TestLabelsMatchTheWebTables pins the Go label tables to the source text of
// the three tables the teacher actually looks at: KIND_OPTIONS/SOURCE_OPTIONS
// in labels.ts (the single copy shared by AssignmentForm.tsx and
// StudentViewPreview.tsx as of Task 6's fix round) and TIER_NAMES in
// assignmentLogic.ts. A word changed on one side without the other would
// otherwise only show up as a teacher pointing out that the AI's card summary
// and the form disagree.
func TestLabelsMatchTheWebTables(t *testing.T) {
	labels, err := os.ReadFile("../../../lite-web/src/teacher/labels.ts")
	if err != nil {
		t.Fatalf("read labels.ts: %v", err)
	}
	labelsSrc := string(labels)
	for value, label := range kindLabels {
		want := `value: "` + value + `", label: "` + label + `"`
		if !strings.Contains(labelsSrc, want) {
			t.Fatalf("labels.ts no longer has %q — KindLabel(%q) drifted from KIND_OPTIONS", want, value)
		}
	}
	for value, label := range sourceLabels {
		want := `value: "` + value + `", label: "` + label + `"`
		if !strings.Contains(labelsSrc, want) {
			t.Fatalf("labels.ts no longer has %q — SourceLabel(%q) drifted from SOURCE_OPTIONS", want, value)
		}
	}

	logic, err := os.ReadFile("../../../lite-web/src/teacher/assignmentLogic.ts")
	if err != nil {
		t.Fatalf("read assignmentLogic.ts: %v", err)
	}
	logicSrc := string(logic)
	for _, name := range tierNames {
		if !strings.Contains(logicSrc, `"`+name+`"`) {
			t.Fatalf("assignmentLogic.ts no longer mentions %q — tierNames drifted from TIER_NAMES", name)
		}
	}
}

func TestReplaceSlugs(t *testing.T) {
	titles := map[string]string{"biden-creates-climate-corps": "美国气候队"}
	lookup := func(s string) (string, bool) { v, ok := titles[s]; return v, ok }

	got := ReplaceSlugs("文章：biden-creates-climate-corps", lookup)
	if want := "文章：《美国气候队》"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// A hyphenated word that is not a known slug is left alone.
	if got := ReplaceSlugs("well-known 方法", lookup); got != "well-known 方法" {
		t.Fatalf("touched a non-slug: %q", got)
	}

	// A slug already quoted is not wrapped a second time.
	got = ReplaceSlugs("材料：《biden-creates-climate-corps》", lookup)
	if want := "材料：《美国气候队》"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Two occurrences in the same text both get replaced.
	got = ReplaceSlugs("biden-creates-climate-corps 和 biden-creates-climate-corps", lookup)
	if want := "《美国气候队》 和 《美国气候队》"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
