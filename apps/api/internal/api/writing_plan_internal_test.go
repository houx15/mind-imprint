package api

// writing_plan_internal_test.go — white-box tests for writing_plan.go that
// need package-internal access: the raw system-prompt string (unexported
// constant) and rootInsertPosition (unexported helper).
//
// TestWritingPlanSystem_TeachesWholePieceJudgment is the RED-PHASE fix for
// B0: TestPlanTurn_AcceptsATopLevelOpeningBesideTheThesis (writing_plan_test.go,
// package api_test) exercises insertPlanNode/insertPlanNode's storage layer
// through a scripted stub — it passed even before the prompt changed, because
// nothing in storage ever forbade a second depth-0 node. This test instead
// pins the actual prompt text, so it genuinely fails without the change B0
// makes.

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

func TestWritingPlanSystem_TeachesWholePieceJudgment(t *testing.T) {
	for _, want := range []string{
		"最上层不止中心论点",
		"不超过 200 字",
	} {
		if !strings.Contains(writingPlanSystem, want) {
			t.Fatalf("writingPlanSystem is missing %q", want)
		}
	}
	if strings.Contains(writingPlanSystem, "不超过 120 字") {
		t.Fatalf("writingPlanSystem still carries the old 120-字 cap")
	}
}

// TestWritingPlanSystem_NamesOnlyRealMethods enforces the 🔑 rule stated in
// writing_plan.go and writing_guide.go: a method name written in PROSE inside a
// prompt must exist verbatim in the vocabulary library. The prompt turns around
// and tells the model 「只能用这里的名字，别造新词」, so a demonstration sentence
// naming something the library does not have teaches it to invent terms in the
// same breath as forbidding it. This test is the reason the 2026-08-28 rename
// could not quietly leave 『正反』 behind.
func TestWritingPlanSystem_NamesOnlyRealMethods(t *testing.T) {
	known := map[string]bool{}
	for _, m := range vocab.All() {
		known[m.Name] = true
		if m.FormalName != "" {
			known[m.FormalName] = true
		}
	}
	// Every 『…』 inside these prompts is a method name by convention.
	for _, prompt := range []struct {
		what string
		text string
	}{
		{"writingPlanSystem", writingPlanSystem},
		{"writingGuideTeachingRules", writingGuideTeachingRules},
		{"writingOpeningSystem", writingOpeningSystem},
	} {
		for _, quoted := range bracketed(prompt.text) {
			if !known[quoted] {
				t.Errorf("%s names 『%s』, which is not a name or formal_name in packages/contracts/vocab/methods.json", prompt.what, quoted)
			}
		}
	}
	// Guard against this test passing vacuously if the quoting convention
	// changes and bracketed stops finding anything.
	if n := len(bracketed(writingPlanSystem)); n < 3 {
		t.Fatalf("found only %d 『』-quoted method names in writingPlanSystem — the prompt or the quoting convention changed, and this test is no longer checking anything", n)
	}
	// Examples need not list a fixed menu on every turn. The selected
	// professional term must still resolve through the shared vocabulary.
	if !known["并列论证"] || !strings.Contains(writingGuideTeachingRules, "并列论证") {
		t.Fatal("guide example must use a registered method name")
	}
	// The retired name must be gone everywhere, including the setup prompt.
	for _, prompt := range []string{writingPlanSystem, writingGuideTeachingRules, writingOpeningSystem, writingGuideSystem, writingGuideBatchSystem} {
		if strings.Contains(prompt, "正反") {
			t.Errorf("a prompt still names 正反, which is no longer any method's name (point_contrast is 比一比 / 对比论证)")
		}
	}
}

// bracketed pulls out every 『…』 span, the convention these prompts use to
// quote a method name.
func bracketed(s string) []string {
	var out []string
	rest := s
	for {
		i := strings.Index(rest, "『")
		if i < 0 {
			return out
		}
		rest = rest[i+len("『"):]
		j := strings.Index(rest, "』")
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j+len("』"):]
	}
}

func TestRootInsertPosition(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{Text: "不该一刀切禁手机", Role: "中心论点", Depth: 0, Position: 0},
		{Text: "理由一", Role: "一条理由", Depth: 1, Position: 1},
	}

	cases := []struct {
		name string
		role string
		want int32
	}{
		{"opening role goes first", "开头", 0},
		{"opening synonym goes first", "钩子式开头", 0},
		{"closing role appends", "结尾", int32(len(rows))},
		{"thesis role appends", "中心论点", int32(len(rows))},
		{"unrecognised role appends", "反方会说的话", int32(len(rows))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rootInsertPosition(c.role, rows); got != c.want {
				t.Fatalf("rootInsertPosition(%q, rows) = %d, want %d", c.role, got, c.want)
			}
		})
	}
}
