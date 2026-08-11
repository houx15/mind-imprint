package api_test

// revision_exploration_events_test.go — Task 5 of the revision-recording plan
// (spec 2026-08-11-revision-recording): Mechanism-2 mutation EVENTS for
// exploration-graph actions (leads/edges/dig), plus the material-provenance
// enrichment (every event carries `stage`, and dig gets its own
// `dig_performed`{keyword} event). Mirrors exploration_test.go's own
// harness/seed conventions (same package, same helpers) rather than
// inventing a second one.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// eventCount counts the project's persisted `event` rows of the given type —
// the shared assertion helper for every test in this file.
func eventCount(t *testing.T, pool *pgxpool.Pool, projectID, eventType string) int {
	t.Helper()
	rows, err := sqlc.New(pool).ListEventsByProject(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	n := 0
	for _, r := range rows {
		if r.Type == eventType {
			n++
		}
	}
	return n
}

// eventPayloads returns the decoded payloads of every event of the given
// type, in insertion order — used where a test needs to inspect a field
// (e.g. `stage`, `keyword`), not just count rows.
func eventPayloads(t *testing.T, pool *pgxpool.Pool, projectID, eventType string) []map[string]any {
	t.Helper()
	rows, err := sqlc.New(pool).ListEventsByProject(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	var out []map[string]any
	for _, r := range rows {
		if r.Type != eventType {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			t.Fatalf("decode %s payload: %v — %s", eventType, err, r.Payload)
		}
		out = append(out, p)
	}
	return out
}

// createLeadViaAPI creates a manual top-level lead and returns its id.
func createLeadViaAPI(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, text string) string {
	t.Helper()
	base := "/api/v1/projects/" + projectID
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"`+text+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lead = %d, want 201: %s", rec.Code, rec.Body)
	}
	var out struct {
		Lead struct {
			ID string `json:"id"`
		} `json:"lead"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode created lead: %v — %s", err, rec.Body)
	}
	return out.Lead.ID
}

// deleteLeadViaAPI deletes a lead by id.
func deleteLeadViaAPI(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, leadID string) {
	t.Helper()
	rec := doJSON(t, h, cookie, "DELETE", "/api/v1/projects/"+projectID+"/exploration/leads/"+leadID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete lead = %d, want 204: %s", rec.Code, rec.Body)
	}
}

// TestExploration_EmitsLeadAndEdgeEvents — Task 5 Step 2/3/5: creating then
// deleting a lead over HTTP must each append a Mechanism-2 mutation event
// (lead_added / lead_removed) to the project's event log, on top of whatever
// the handlers already did (the response/DB behavior asserted elsewhere in
// exploration_test.go is unchanged).
func TestExploration_EmitsLeadAndEdgeEvents(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	lid := createLeadViaAPI(t, h, cookie, pid, "sub-question A")
	deleteLeadViaAPI(t, h, cookie, pid, lid)

	if n := eventCount(t, pool, pid, "lead_added"); n != 1 {
		t.Fatalf("lead_added events = %d, want 1", n)
	}
	if n := eventCount(t, pool, pid, "lead_removed"); n != 1 {
		t.Fatalf("lead_removed events = %d, want 1", n)
	}

	added := eventPayloads(t, pool, pid, "lead_added")
	if len(added) != 1 {
		t.Fatalf("lead_added payloads = %+v, want 1", added)
	}
	if added[0]["leadId"] != lid {
		t.Fatalf("lead_added.leadId = %v, want %q", added[0]["leadId"], lid)
	}
	if added[0]["text"] != "sub-question A" {
		t.Fatalf("lead_added.text = %v, want %q", added[0]["text"], "sub-question A")
	}
	if added[0]["origin"] != "manual" {
		t.Fatalf("lead_added.origin = %v, want manual", added[0]["origin"])
	}
	// Material-provenance enrichment: every mutation event carries `stage`
	// (the project's studio stage at emit time, best-effort).
	if _, ok := added[0]["stage"]; !ok {
		t.Fatalf("lead_added payload missing stage: %+v", added[0])
	}

	removed := eventPayloads(t, pool, pid, "lead_removed")
	if len(removed) != 1 {
		t.Fatalf("lead_removed payloads = %+v, want 1", removed)
	}
	if removed[0]["leadId"] != lid {
		t.Fatalf("lead_removed.leadId = %v, want %q", removed[0]["leadId"], lid)
	}
	if removed[0]["text"] != "sub-question A" {
		t.Fatalf("lead_removed.text = %v, want the pre-delete text %q", removed[0]["text"], "sub-question A")
	}
}

// TestQuestionEdge_EmitsEdgeAddedAndRemovedEvents — createQuestionEdge and
// deleteQuestionEdge each append edge_added / edge_removed events, the
// deleted edge's payload carrying the from/to captured BEFORE the delete.
func TestQuestionEdge_EmitsEdgeAddedAndRemovedEvents(t *testing.T) {
	pool := newAPITestPool(t)
	h := libraryTestHandler(pool)
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	fromID := createLeadViaAPI(t, h, cookie, pid, "问题 A")
	toID := createLeadViaAPI(t, h, cookie, pid, "问题 B")

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/edges",
		`{"fromLeadId":"`+fromID+`","toLeadId":"`+toID+`","label":"支持"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create edge = %d, want 201: %s", rec.Code, rec.Body)
	}
	var created struct {
		Edge struct {
			ID string `json:"id"`
		} `json:"edge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created edge: %v — %s", err, rec.Body)
	}
	eid := created.Edge.ID

	if n := eventCount(t, pool, pid, "edge_added"); n != 1 {
		t.Fatalf("edge_added events = %d, want 1", n)
	}
	added := eventPayloads(t, pool, pid, "edge_added")
	if added[0]["from"] != fromID || added[0]["to"] != toID {
		t.Fatalf("edge_added from/to = %v/%v, want %q/%q", added[0]["from"], added[0]["to"], fromID, toID)
	}

	rec = doJSON(t, h, cookie, "DELETE", base+"/exploration/edges/"+eid, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete edge = %d, want 204: %s", rec.Code, rec.Body)
	}
	if n := eventCount(t, pool, pid, "edge_removed"); n != 1 {
		t.Fatalf("edge_removed events = %d, want 1", n)
	}
	removed := eventPayloads(t, pool, pid, "edge_removed")
	if removed[0]["from"] != fromID || removed[0]["to"] != toID {
		t.Fatalf("edge_removed from/to = %v/%v, want %q/%q (pre-delete values)", removed[0]["from"], removed[0]["to"], fromID, toID)
	}
}

// TestExplorationAdoptAttach_EmitsEvents — adoptExploration emits
// lead_adopted, attachExploration emits source_attached.
func TestExplorationAdoptAttach_EmitsEvents(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	parentID := createLeadViaAPI(t, h, cookie, pid, "中国的碳排放到底有多少")

	body := `{"parentLeadId":"` + parentID + `","candidate":{"doi":"10.1/x","title":"Nature Sust 2023","authors":"Li","year":"2023","journal":"Nature Sustainability","abstract":"...","url":"https://n/x"}}`
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/adopt", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("adopt = %d, want 201: %s", rec.Code, rec.Body)
	}
	var adopted struct {
		Lead struct {
			ID string `json:"id"`
		} `json:"lead"`
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &adopted); err != nil {
		t.Fatalf("decode adopt response: %v — %s", err, rec.Body)
	}

	if n := eventCount(t, pool, pid, "lead_adopted"); n != 1 {
		t.Fatalf("lead_adopted events = %d, want 1", n)
	}
	adoptedPayloads := eventPayloads(t, pool, pid, "lead_adopted")
	if adoptedPayloads[0]["leadId"] != adopted.Lead.ID {
		t.Fatalf("lead_adopted.leadId = %v, want %q", adoptedPayloads[0]["leadId"], adopted.Lead.ID)
	}
	if adoptedPayloads[0]["parentLeadId"] != parentID {
		t.Fatalf("lead_adopted.parentLeadId = %v, want %q", adoptedPayloads[0]["parentLeadId"], parentID)
	}

	// attachExploration: a second question lead, then hang an EXISTING
	// reference (the one adopt just created) under it.
	otherQuestion := createLeadViaAPI(t, h, cookie, pid, "另一个问题")
	rec = doJSON(t, h, cookie, "POST", base+"/exploration/attach",
		`{"referenceId":"`+adopted.Reference.ID+`","parentLeadId":"`+otherQuestion+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("attach = %d, want 201: %s", rec.Code, rec.Body)
	}
	if n := eventCount(t, pool, pid, "source_attached"); n != 1 {
		t.Fatalf("source_attached events = %d, want 1", n)
	}
	attachedPayloads := eventPayloads(t, pool, pid, "source_attached")
	if attachedPayloads[0]["referenceId"] != adopted.Reference.ID {
		t.Fatalf("source_attached.referenceId = %v, want %q", attachedPayloads[0]["referenceId"], adopted.Reference.ID)
	}
	if attachedPayloads[0]["parentLeadId"] != otherQuestion {
		t.Fatalf("source_attached.parentLeadId = %v, want %q", attachedPayloads[0]["parentLeadId"], otherQuestion)
	}
}

// TestExplorationDig_EmitsDigPerformedEvent — POST /exploration/dig emits a
// dig_performed{keyword} event even when the search returns zero candidates
// (an empty-result search is still a real search attempt worth recording).
func TestExplorationDig_EmitsDigPerformedEvent(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID,
		Fetcher: fakeFetcher{works: nil}, // zero candidates
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/dig", `{"keyword":"china carbon"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dig = %d, want 200: %s", rec.Code, rec.Body)
	}

	if n := eventCount(t, pool, pid, "dig_performed"); n != 1 {
		t.Fatalf("dig_performed events = %d, want 1 (even with zero candidates)", n)
	}
	payloads := eventPayloads(t, pool, pid, "dig_performed")
	if payloads[0]["keyword"] != "china carbon" {
		t.Fatalf("dig_performed.keyword = %v, want %q", payloads[0]["keyword"], "china carbon")
	}
	if _, ok := payloads[0]["stage"]; !ok {
		t.Fatalf("dig_performed payload missing stage: %+v", payloads[0])
	}
}
