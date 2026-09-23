package api

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/litegrade"
)

// 🚨 The stored id must never reach a reader's screen, and whatever is put
// on her screen must survive the trip back through SanitizeProvenance.
//
// This is the other half of the 2026-09-23 blocker: storing the display name
// made SanitizeProvenance non-idempotent, so every teacher save blanked
// 对应毛病. The id is stored now, so something has to resolve it on the way
// out — and what it hands out has to be something the save path still
// recognises.
func TestGradingContentForViewResolvesTheSymptomAndRoundTrips(t *testing.T) {
	stored := []byte(`{"overall":{"grade":"B","comment":"x"},"dimensions":[],"points":[` +
		`{"kind":"issue","quote":null,"text":"t","action":"a","source":"ai","dimension":"内容","symptom":"topic_without_question"},` +
		`{"kind":"good","quote":null,"text":"g","action":null,"source":"ai","dimension":"","symptom":""}]}`)

	view := gradingContentForView(stored, "zh")
	var seen litegrade.Content
	if err := json.Unmarshal(view, &seen); err != nil {
		t.Fatalf("view is not valid content: %v", err)
	}
	want, ok := resolveWritingSymptom("zh", "topic_without_question")
	if !ok {
		t.Fatal("fixture uses an id that is not in the closed table")
	}
	if seen.Points[0].Symptom != want.Name {
		t.Fatalf("the teacher must read a name, got %q", seen.Points[0].Symptom)
	}
	if strings.Contains(string(view), "topic_without_question") {
		t.Fatalf("a raw id must not reach her screen: %s", view)
	}
	if seen.Points[1].Symptom != "" {
		t.Fatalf("a blank symptom must stay blank, got %q", seen.Points[1].Symptom)
	}
	if seen.Points[0].Dimension != "内容" || seen.Points[0].Action == nil || *seen.Points[0].Action != "a" {
		t.Fatalf("nothing else may change: %+v", seen.Points[0])
	}

	// What she sends back is what she was shown — it must canonicalise to
	// the id again, not be blanked.
	back := litegrade.SanitizeProvenance(seen, runTestInput())
	if back.Points[0].Symptom != "topic_without_question" {
		t.Fatalf("the name must round-trip back to the id, got %q", back.Points[0].Symptom)
	}

	// Anything unreadable is passed through untouched rather than dropped —
	// an odd row must not 500 a list.
	if string(gradingContentForView([]byte("not json"), "zh")) != "not json" {
		t.Fatal("an unreadable row must pass through, not 500")
	}
	if len(gradingContentForView(nil, "zh")) != 0 {
		t.Fatal("no content stays no content")
	}
}
