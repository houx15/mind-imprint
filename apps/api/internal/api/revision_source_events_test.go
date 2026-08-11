package api_test

// revision_source_events_test.go — Task 6 of the revision-recording plan
// (spec 2026-08-11-revision-recording): Mechanism-2 mutation EVENTS for
// source/reference decisions (add/reclassify/drop). Mirrors Task 5's own
// harness/helper conventions (revision_exploration_events_test.go, same
// package) rather than inventing a second one; reuses its eventCount /
// eventPayloads helpers.

import (
	"encoding/json"
	"net/http"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// createRefViaAPI creates a reference and returns its id.
func createRefViaAPI(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, title, classification string) string {
	t.Helper()
	base := "/api/v1/projects/" + projectID
	body := `{"title":"` + title + `","classification":"` + classification + `"}`
	rec := doJSON(t, h, cookie, "POST", base+"/references", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reference = %d, want 201: %s", rec.Code, rec.Body)
	}
	var out struct {
		Reference struct {
			ID string `json:"id"`
		} `json:"reference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode created reference: %v — %s", err, rec.Body)
	}
	return out.Reference.ID
}

// patchJSON issues an authed PATCH with a JSON body and asserts 200.
func patchJSON(t *testing.T, h http.Handler, cookie *http.Cookie, path, body string) {
	t.Helper()
	rec := doJSON(t, h, cookie, "PATCH", path, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH %s = %d, want 200: %s", path, rec.Code, rec.Body)
	}
}

// deleteJSON issues an authed DELETE and asserts 204.
func deleteJSON(t *testing.T, h http.Handler, cookie *http.Cookie, path string) {
	t.Helper()
	rec := doJSON(t, h, cookie, "DELETE", path, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE %s = %d, want 204: %s", path, rec.Code, rec.Body)
	}
}

// TestSource_EmitsAddDropReclassify — Task 6 Step 2/4: creating a reference,
// reclassifying its triage, then deleting it over HTTP must each append a
// Mechanism-2 mutation event (source_added / source_reclassified /
// source_dropped) to the project's event log.
func TestSource_EmitsAddDropReclassify(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)

	rid := createRefViaAPI(t, h, cookie, pid, "NASA greening", "期刊论文")
	patchJSON(t, h, cookie, "/api/v1/projects/"+pid+"/references/"+rid+"/triage", `{"triage":"yellow"}`)
	deleteJSON(t, h, cookie, "/api/v1/projects/"+pid+"/references/"+rid)

	if n := eventCount(t, pool, pid, "source_added"); n != 1 {
		t.Fatalf("source_added events = %d, want 1", n)
	}
	if n := eventCount(t, pool, pid, "source_reclassified"); n != 1 {
		t.Fatalf("source_reclassified events = %d, want 1", n)
	}
	if n := eventCount(t, pool, pid, "source_dropped"); n != 1 {
		t.Fatalf("source_dropped events = %d, want 1", n)
	}

	added := eventPayloads(t, pool, pid, "source_added")
	if added[0]["referenceId"] != rid {
		t.Fatalf("source_added.referenceId = %v, want %q", added[0]["referenceId"], rid)
	}
	if added[0]["title"] != "NASA greening" {
		t.Fatalf("source_added.title = %v, want %q", added[0]["title"], "NASA greening")
	}
	if added[0]["classification"] != "期刊论文" {
		t.Fatalf("source_added.classification = %v, want %q", added[0]["classification"], "期刊论文")
	}
	if added[0]["provenance"] != "manual" {
		t.Fatalf("source_added.provenance = %v, want manual", added[0]["provenance"])
	}
	if _, ok := added[0]["stage"]; !ok {
		t.Fatalf("source_added payload missing stage: %+v", added[0])
	}

	reclassified := eventPayloads(t, pool, pid, "source_reclassified")
	if reclassified[0]["referenceId"] != rid {
		t.Fatalf("source_reclassified.referenceId = %v, want %q", reclassified[0]["referenceId"], rid)
	}
	if reclassified[0]["field"] != "triage" {
		t.Fatalf("source_reclassified.field = %v, want triage", reclassified[0]["field"])
	}
	if reclassified[0]["before"] != "" {
		t.Fatalf("source_reclassified.before = %v, want empty (unset triage)", reclassified[0]["before"])
	}
	if reclassified[0]["after"] != "yellow" {
		t.Fatalf("source_reclassified.after = %v, want yellow", reclassified[0]["after"])
	}

	dropped := eventPayloads(t, pool, pid, "source_dropped")
	if dropped[0]["referenceId"] != rid {
		t.Fatalf("source_dropped.referenceId = %v, want %q", dropped[0]["referenceId"], rid)
	}
	if dropped[0]["title"] != "NASA greening" {
		t.Fatalf("source_dropped.title = %v, want the pre-delete title %q", dropped[0]["title"], "NASA greening")
	}
}

// TestSourceReclassify_DecisionAndClassificationViaPatch — patchReference
// only emits source_reclassified when decision/classification actually
// change; other-field patches (e.g. author) must not emit.
func TestSourceReclassify_DecisionAndClassificationViaPatch(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rid := createRefViaAPI(t, h, cookie, pid, "Global Greening", "报告")

	// Author-only patch: no decision/classification touched, no emit.
	patchJSON(t, h, cookie, base+"/references/"+rid, `{"author":"NASA"}`)
	if n := eventCount(t, pool, pid, "source_reclassified"); n != 0 {
		t.Fatalf("source_reclassified events after author-only patch = %d, want 0", n)
	}

	// Decision set.
	patchJSON(t, h, cookie, base+"/references/"+rid, `{"decision":"use"}`)
	if n := eventCount(t, pool, pid, "source_reclassified"); n != 1 {
		t.Fatalf("source_reclassified events after decision patch = %d, want 1", n)
	}

	// Classification changed.
	patchJSON(t, h, cookie, base+"/references/"+rid, `{"classification":"期刊论文"}`)
	if n := eventCount(t, pool, pid, "source_reclassified"); n != 2 {
		t.Fatalf("source_reclassified events after classification patch = %d, want 2", n)
	}

	payloads := eventPayloads(t, pool, pid, "source_reclassified")
	if payloads[0]["field"] != "decision" || payloads[0]["before"] != nil || payloads[0]["after"] != "use" {
		t.Fatalf("decision reclassify payload = %+v", payloads[0])
	}
	if payloads[1]["field"] != "classification" || payloads[1]["before"] != "报告" || payloads[1]["after"] != "期刊论文" {
		t.Fatalf("classification reclassify payload = %+v", payloads[1])
	}
}
