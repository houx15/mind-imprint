package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestReviewDraftAnnotations_Parses(t *testing.T) {
	prov := stubProviderText(`{"annotations":[` +
		`{"level":"paper","nature":"good","quote":"","locator":"","note":"整体结构清晰"},` +
		`{"level":"sentence","nature":"problem","quote":"中国一定会成功","locator":"第2段","note":"这是断言，缺证据支撑"}` +
		`]}`)
	out, usage, err := ReviewDraftAnnotations(context.Background(), prov, gateway.Resolved{Provider: "stub"}, DraftAnnotationInput{
		Title: "中国是否让地球更可持续", Draft: "……",
	})
	if err != nil {
		t.Fatalf("ReviewDraftAnnotations err = %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 annotations, got %d: %+v", len(out), out)
	}
	if out[0].Level != "paper" || out[0].Nature != "good" {
		t.Fatalf("paper item wrong: %+v", out[0])
	}
	if out[1].Level != "sentence" || out[1].Nature != "problem" || out[1].Quote != "中国一定会成功" {
		t.Fatalf("sentence item wrong: %+v", out[1])
	}
	if usage.OutputTokens == 0 {
		t.Error("usage should be reported for metering")
	}
}

func TestReviewDraftAnnotations_ParsesFenced(t *testing.T) {
	prov := stubProviderText("```json\n{\"annotations\":[{\"level\":\"paragraph\",\"nature\":\"suggest\",\"quote\":\"\",\"locator\":\"第1段\",\"note\":\"可以先交代背景\"}]}\n```")
	out, _, err := ReviewDraftAnnotations(context.Background(), prov, gateway.Resolved{Provider: "stub"}, DraftAnnotationInput{Draft: "x"})
	if err != nil {
		t.Fatalf("fenced err = %v", err)
	}
	if len(out) != 1 || out[0].Level != "paragraph" {
		t.Fatalf("out = %+v", out)
	}
}

func TestReviewDraftAnnotations_DropsEmptyNoteAndClamps(t *testing.T) {
	// 1 empty-note (dropped) + 12 valid → clamped to 10.
	items := `{"level":"sentence","nature":"suggest","quote":"q","locator":"第1段","note":""}`
	for i := 0; i < 12; i++ {
		items += `,{"level":"sentence","nature":"suggest","quote":"q","locator":"第1段","note":"具体建议"}`
	}
	prov := stubProviderText(`{"annotations":[` + items + `]}`)
	out, _, err := ReviewDraftAnnotations(context.Background(), prov, gateway.Resolved{Provider: "stub"}, DraftAnnotationInput{Draft: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 10 {
		t.Fatalf("want 10 (empty-note dropped, clamped), got %d", len(out))
	}
}

func TestReviewDraftAnnotations_GarbageErrors(t *testing.T) {
	prov := stubProviderText("我觉得写得不错。")
	if _, _, err := ReviewDraftAnnotations(context.Background(), prov, gateway.Resolved{Provider: "stub"}, DraftAnnotationInput{Draft: "x"}); err == nil {
		t.Fatal("expected error on an unparseable reply (caller degrades to empty)")
	}
}
