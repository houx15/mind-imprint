package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestEssaySubmission_TrackWalk(t *testing.T) {
	h, cookie, pool := proposalTrackHandler(t, guideCardProvider())
	q := sqlc.New(pool)
	seedStatementProject(t, q) // EssayTrack stage=statement, 2 subs
	base := "/api/v1/projects/" + seedProjectID

	req := func(method, path, body string) map[string]any {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest(method, base+path, strings.NewReader(body)), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s %s = %d — %s", method, path, rr.Code, rr.Body)
		}
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}

	// Fresh GET: not started, total = 4 (引言/结论/成文/润色), current = sub:intro, no card.
	got := req("GET", "/essay-submission", "")
	if got["started"] != false || got["total"].(float64) != 4 || got["key"] != "sub:intro" {
		t.Fatalf("fresh submission wrong: %+v", got)
	}
	if got["card"] != nil {
		t.Fatalf("no card before started")
	}

	// Start → started, index 0, card generated.
	got = req("POST", "/essay-submission/start", "")
	if got["started"] != true || got["card"] == nil {
		t.Fatalf("after start expected started + card: %+v", got)
	}

	// Advance → 结论.
	got = req("POST", "/essay-submission/advance", `{"dir":"next"}`)
	if got["key"] != "sub:conclusion" {
		t.Fatalf("after next expected sub:conclusion, got %+v", got)
	}

	// Advance to the last step (润色), clamped.
	req("POST", "/essay-submission/advance", `{"dir":"next"}`)
	got = req("POST", "/essay-submission/advance", `{"dir":"next"}`)
	if got["key"] != "sub:polish" || got["index"].(float64) != 3 {
		t.Fatalf("expected sub:polish index 3, got %+v", got)
	}
}
