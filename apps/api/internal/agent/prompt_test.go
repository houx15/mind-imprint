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
			ID:               "craap",
			Category:         "信息素养",
			Name:             "SIFT×CRAAP 信息核查",
			Purpose:          "先横向找更多来源(SIFT)，必要时再纵向深挖单一材料(CRAAP)",
			TriggerCondition: "学生准备直接采信或引用一个网络来源，但还没核查出处",
			InteractionType:  "步骤引导卡",
		},
	}
	got := BuildCatalogText(catalog)
	want := "【信息素养】\n" +
		"· craap｜SIFT×CRAAP 信息核查（步骤引导卡）\n" +
		"   何时用：学生准备直接采信或引用一个网络来源，但还没核查出处\n" +
		"   用途：先横向找更多来源(SIFT)，必要时再纵向深挖单一材料(CRAAP)"
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
	want := "【C】\n· x｜N\n   何时用：T\n   用途：P"
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

func TestBuildMaterialContextQualifiesBlockIDs(t *testing.T) {
	materials := []Material{
		{ID: "mat-a", Title: "NASA 观测", Blocks: []MaterialBlock{{ID: "b0", Text: "叶面积指数上升。"}}},
		{ID: "mat-b", Title: "BP 统计", Blocks: []MaterialBlock{{ID: "b0", Text: "煤炭消费仍在上升。"}}},
	}
	got := BuildMaterialContext(materials)
	if strings.Count(got, "[b0]") > 0 {
		t.Error("bare [b0] labels collide across materials — ids must be alias-qualified")
	}
	for _, want := range []string{"[m0:b0]", "[m1:b0]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing alias-qualified label %s in:\n%s", want, got)
		}
	}
	// The real material id must never leak into the prompt label — that is
	// exactly the 36-char-UUID defect (I2) the index alias exists to avoid:
	// on the studio path a real material id IS a UUID, not a short string
	// like "mat-a", so a bare-id label would cost ~37 characters per block
	// line and give a beginner nothing short to echo back correctly.
	if strings.Contains(got, "mat-a:") || strings.Contains(got, "mat-b:") {
		t.Error("prompt label leaked the real material id instead of the short index alias")
	}
}

// TestMaterialAliasMatchesL1InstructionExample pins the exact gap that let
// I2 through: BuildMaterialContext's rendered label and buildAnchorPrompt's
// L1 instruction example must describe the SAME format, even when the real
// material id is a 36-char UUID (the studio path) rather than a short
// literal (the course path's "m0"). Before the alias fix, the rendered label
// was "[<uuid>:b0]" while the instruction's own example still showed
// "m0:b0" — a mismatch a golden-fixture-free test never caught because
// every existing test's material id happened to already be short.
func TestMaterialAliasMatchesL1InstructionExample(t *testing.T) {
	materials := []Material{
		{ID: "3f2a91c4-1111-2222-3333-446655440000", Title: "长 UUID 材料", Blocks: []MaterialBlock{{ID: "b0", Text: "t"}}},
	}
	ctx := BuildMaterialContext(materials)
	if !strings.Contains(ctx, "[m0:b0]") {
		t.Fatalf("expected alias label [m0:b0] regardless of the real (UUID) material id, got:\n%s", ctx)
	}
	if strings.Contains(ctx, "3f2a91c4") {
		t.Fatalf("real material UUID leaked into the rendered label:\n%s", ctx)
	}
	instr := buildAnchorPrompt(cards.Spec{Name: "长 UUID 材料"}, GuidanceL1)
	if !strings.Contains(instr, `"block_id":"m0:b0"`) {
		t.Fatalf("L1 instruction example drifted from the alias format the prompt actually renders:\n%s", instr)
	}
}

func TestBuildSystemPromptMatchesGolden(t *testing.T) {
	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	got := BuildSystemPrompt(catalog)
	// The Go builder is the sole authoritative producer of this golden. The old
	// TS generator (apps/web/scripts/gen-go-fixtures.ts) is dead — its source
	// modules (apps/web/src/agent/*) were removed on 2026-06-25. Regenerate with:
	//   UPDATE_GOLDEN=1 CGO_ENABLED=0 go test ./internal/agent/ -run TestBuildSystemPromptMatchesGolden
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(filepath.Join("testdata", "system_prompt_full.txt"), []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "system_prompt_full.txt"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(wantBytes)
	if got != want {
		t.Fatalf("system prompt differs from reviewed golden.\nfirst diff context — got len=%d want len=%d", len(got), len(want))
	}
}
