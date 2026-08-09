package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestReviewDraftAnnotations_DocNoun(t *testing.T) {
	if annotationDocNoun("essay") != "论文正文" {
		t.Fatalf("essay noun = %q", annotationDocNoun("essay"))
	}
	if annotationDocNoun("") != "研究提案" || annotationDocNoun("proposal") != "研究提案" {
		t.Fatalf("proposal noun wrong")
	}
	// The essay prompt names 论文正文, not 研究提案.
	sys := draftAnnotationSystemFor(annotationDocNoun("essay"))
	if !strings.Contains(sys, "论文正文") || strings.Contains(sys, "研究提案") {
		t.Fatalf("essay prompt should say 论文正文 not 研究提案")
	}
}

func TestReviewDraftAnnotations_EssayDocParses(t *testing.T) {
	prov := stubProviderText(`{"annotations":[{"level":"paper","nature":"good","quote":"","locator":"","note":"论证清晰"}]}`)
	out, _, err := ReviewDraftAnnotations(context.Background(), prov, gateway.Resolved{Provider: "stub"}, DraftAnnotationInput{
		Title: "T", Draft: "正文……", Doc: "essay",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1, got %d", len(out))
	}
}
