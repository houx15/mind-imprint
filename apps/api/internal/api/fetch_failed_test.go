package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"mindimprint/api/internal/materialize"
)

// #4 · the paste-fallback carries recovered DOI metadata when present.
func TestFetchFailedError(t *testing.T) {
	// A plain fetch error → generic message, no details.
	e := fetchFailedError(errors.New("boom"))
	if e.Status != http.StatusUnprocessableEntity || e.Code != "fetch_failed" || e.Details != nil {
		t.Fatalf("plain error: status=%d code=%q details=%v", e.Status, e.Code, e.Details)
	}

	// A DOI FetchError carrying metadata → details map + abstract-aware message.
	fe := &materialize.FetchError{Reason: "bad_status", Meta: &materialize.DOIMeta{
		Title: "论文标题", Author: "A B; C D", Year: "2021", Journal: "某期刊", Abstract: "这是摘要",
	}}
	e2 := fetchFailedError(fe)
	d, ok := e2.Details.(map[string]any)
	if !ok {
		t.Fatalf("details type = %T, want map", e2.Details)
	}
	if d["title"] != "论文标题" || d["author"] != "A B; C D" || d["year"] != "2021" || d["journal"] != "某期刊" || d["abstract"] != "这是摘要" {
		t.Fatalf("details wrong: %+v", d)
	}
	if !strings.Contains(e2.Message, "摘要") {
		t.Fatalf("message should mention the abstract: %q", e2.Message)
	}

	// Metadata with a title but no abstract → title-aware message, still details.
	fe2 := &materialize.FetchError{Reason: "unsupported_content", Meta: &materialize.DOIMeta{Title: "只有标题"}}
	e3 := fetchFailedError(fe2)
	if e3.Details == nil || strings.Contains(e3.Message, "摘要") {
		t.Fatalf("title-only: details=%v msg=%q", e3.Details, e3.Message)
	}
}
