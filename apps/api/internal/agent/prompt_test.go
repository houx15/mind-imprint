package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mindimprint/api/internal/cards"
)

func TestBuildCatalogTextFormatsOneEntry(t *testing.T) {
	catalog := []cards.Spec{
		{
			ID:               "sift_craap",
			Category:         "信息素养",
			Name:             "SIFT×CRAAP 信息核查",
			Purpose:          "先横向找更多来源(SIFT)，必要时再纵向深挖单一材料(CRAAP)",
			TriggerCondition: "学生准备直接采信或引用一个网络来源，但还没核查出处",
			InteractionType:  "步骤引导卡",
		},
	}
	got := BuildCatalogText(catalog)
	want := "【信息素养】\n" +
		"· sift_craap｜SIFT×CRAAP 信息核查（步骤引导卡）\n" +
		"   何时用：学生准备直接采信或引用一个网络来源，但还没核查出处\n" +
		"   能帮他：先横向找更多来源(SIFT)，必要时再纵向深挖单一材料(CRAAP)"
	if got != want {
		t.Fatalf("catalog text mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestBuildCatalogTextNoKindWhenEmpty(t *testing.T) {
	catalog := []cards.Spec{
		{ID: "x", Category: "C", Name: "N", Purpose: "P", TriggerCondition: "T"},
	}
	got := BuildCatalogText(catalog)
	if strings.Contains(got, "（）") {
		t.Fatalf("empty interaction_type must not render parens: %q", got)
	}
	want := "【C】\n· x｜N\n   何时用：T\n   能帮他：P"
	if got != want {
		t.Fatalf("mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestSummonCardToolEnumIsCatalogIDs(t *testing.T) {
	catalog := []cards.Spec{{ID: "a"}, {ID: "b"}}
	tool := SummonCardTool(catalog)
	if tool.Name != "summon_card" {
		t.Fatalf("name = %q", tool.Name)
	}
	props, _ := tool.Parameters["properties"].(map[string]any)
	cardID, _ := props["card_id"].(map[string]any)
	enum, _ := cardID["enum"].([]string)
	if len(enum) != 2 || enum[0] != "a" || enum[1] != "b" {
		t.Fatalf("enum = %v", enum)
	}
	req, _ := tool.Parameters["required"].([]string)
	if len(req) != 3 {
		t.Fatalf("required = %v, want 3 entries", req)
	}
}

func TestBuildSystemPromptMatchesGolden(t *testing.T) {
	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	got := BuildSystemPrompt(catalog)
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "system_prompt_full.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(wantBytes)
	if got != want {
		t.Fatalf("system prompt drifted from TS golden.\nfirst diff context — got len=%d want len=%d", len(got), len(want))
	}
}
