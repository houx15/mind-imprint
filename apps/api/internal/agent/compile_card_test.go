package agent

import (
	"strings"
	"testing"

	"mindimprint/api/internal/cards"
)

func testSpec() cards.Spec {
	return cards.Spec{
		ID:   "question-card",
		Name: "提问卡",
		Steps: []cards.Step{
			{Key: "s1", Title: "step 1", Fields: []cards.Field{
				{Key: "q", Type: "text", Label: "你的问题"},
				{Key: "why", Type: "textarea", Label: "为什么想问"},
			}},
		},
	}
}

func TestCompileCardForCoach_DeclaredFields(t *testing.T) {
	out := CompileCardForCoach(testSpec(), map[string]any{
		"q":   "中国是否让地球更可持续？",
		"why": "  我在纪录片里看到相反的说法  ",
	})
	if !strings.HasPrefix(out, "我刚填完《提问卡》：") {
		t.Fatalf("missing header: %q", out)
	}
	if !strings.Contains(out, "- 你的问题：中国是否让地球更可持续？") {
		t.Fatalf("missing question line: %q", out)
	}
	// value trimmed.
	if !strings.Contains(out, "- 为什么想问：我在纪录片里看到相反的说法") {
		t.Fatalf("missing/untrimmed reason line: %q", out)
	}
}

func TestCompileCardForCoach_EmptyReturnsBlank(t *testing.T) {
	if out := CompileCardForCoach(testSpec(), map[string]any{"q": "", "why": "   "}); out != "" {
		t.Fatalf("empty card should compile to \"\", got %q", out)
	}
	if out := CompileCardForCoach(testSpec(), map[string]any{}); out != "" {
		t.Fatalf("no fields should compile to \"\", got %q", out)
	}
}

func TestCompileCardForCoach_ArraysObjectsAndFallbackKeys(t *testing.T) {
	spec := cards.Spec{ID: "x", Name: "杂卡", Steps: []cards.Step{
		{Key: "s", Fields: []cards.Field{{Key: "tags", Label: "标签"}}},
	}}
	out := CompileCardForCoach(spec, map[string]any{
		"tags": []any{"经济", "环境", ""},
		// a custom-renderer key not declared in steps → generic fallback dump.
		"note": map[string]any{"text": "见 NASA 图", "source": "nasa.gov"},
	})
	if !strings.Contains(out, "- 标签：经济、环境") {
		t.Fatalf("array not joined with 、: %q", out)
	}
	// object flattened (deterministic key order: source, text).
	if !strings.Contains(out, "- note：nasa.gov · 见 NASA 图") {
		t.Fatalf("fallback object dump wrong: %q", out)
	}
}

func TestCompileCardForCoach_NameFallsBackToID(t *testing.T) {
	spec := cards.Spec{ID: "only-id", Steps: []cards.Step{
		{Key: "s", Fields: []cards.Field{{Key: "a", Label: "字段"}}},
	}}
	out := CompileCardForCoach(spec, map[string]any{"a": "值"})
	if !strings.HasPrefix(out, "我刚填完《only-id》：") {
		t.Fatalf("name should fall back to id: %q", out)
	}
}
