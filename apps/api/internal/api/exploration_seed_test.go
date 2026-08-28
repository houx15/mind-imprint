package api_test

// exploration_seed_test.go — bug report 2026-08-28 §3/§4. The 兔子洞地图 must
// never open with zero question nodes: with none there is nothing to hang a
// source under, the placement picker has no targets, and a student who
// collected papers before 印记 proposed a question is dead-locked in 未归类.
// GET /exploration seeds the project's own title as the first root question.

import (
	"encoding/json"
	"testing"
)

type seedLeadsResp struct {
	Leads []struct {
		ID           string  `json:"id"`
		Text         string  `json:"text"`
		Status       string  `json:"status"`
		Origin       string  `json:"origin"`
		ParentLeadID *string `json:"parentLeadId"`
	} `json:"leads"`
}

func TestGetExploration_SeedsRootQuestionFromTitle(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rec := doJSON(t, h, cookie, "GET", "/api/v1/projects/"+pid+"/exploration", "")
	if rec.Code != 200 {
		t.Fatalf("GET exploration = %d, want 200: %s", rec.Code, rec.Body)
	}
	var out seedLeadsResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode exploration: %v — %s", err, rec.Body)
	}
	if len(out.Leads) != 1 {
		t.Fatalf("leads = %d, want exactly 1 seeded root question: %s", len(out.Leads), rec.Body)
	}
	seeded := out.Leads[0]
	// createProjectForTest posts {"title":"T"}.
	if seeded.Text != "T" {
		t.Errorf("seeded text = %q, want the project title", seeded.Text)
	}
	if seeded.ParentLeadID != nil {
		t.Errorf("seeded lead must be a ROOT (parentLeadId null), got %v", *seeded.ParentLeadID)
	}
	if seeded.Status != "open" {
		t.Errorf("seeded status = %q, want open", seeded.Status)
	}

	// Idempotent: a second load must not stack another copy.
	rec = doJSON(t, h, cookie, "GET", "/api/v1/projects/"+pid+"/exploration", "")
	var again seedLeadsResp
	if err := json.Unmarshal(rec.Body.Bytes(), &again); err != nil {
		t.Fatalf("decode second exploration: %v — %s", err, rec.Body)
	}
	if len(again.Leads) != 1 || again.Leads[0].ID != seeded.ID {
		t.Fatalf("second GET must reuse the same seeded lead, got %d leads: %s", len(again.Leads), rec.Body)
	}
}

// Once the student has any lead of her own, the seed must stay out of the way —
// including after she deletes the seeded one and creates her own.
func TestGetExploration_DoesNotSeedOverExistingLeads(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"我自己的问题"}`)
	if rec.Code != 201 {
		t.Fatalf("create lead = %d, want 201: %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	var out seedLeadsResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode exploration: %v — %s", err, rec.Body)
	}
	if len(out.Leads) != 1 || out.Leads[0].Text != "我自己的问题" {
		t.Fatalf("existing leads must be left alone, got %s", rec.Body)
	}
}

// The seed decides the map's OPENING state, once. A student who later empties
// her map has done that on purpose — the title node must not come back on the
// next load (the map still offers 「＋ 新建问题」, so she is never stranded).
func TestGetExploration_DoesNotReseedAnEmptiedMap(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "GET", base+"/exploration", "")
	var out seedLeadsResp
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode exploration: %v — %s", err, rec.Body)
	}
	if len(out.Leads) != 1 {
		t.Fatalf("first load should seed one root question, got %s", rec.Body)
	}

	if rec = doJSON(t, h, cookie, "DELETE", base+"/exploration/leads/"+out.Leads[0].ID, ""); rec.Code != 204 {
		t.Fatalf("delete seeded lead = %d, want 204: %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, h, cookie, "GET", base+"/exploration", "")
	var after seedLeadsResp
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatalf("decode exploration after delete: %v — %s", err, rec.Body)
	}
	if len(after.Leads) != 0 {
		t.Fatalf("an emptied map must stay empty, got %s", rec.Body)
	}
}
