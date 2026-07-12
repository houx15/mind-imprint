package api_test

// projectcards_test.go — Task 4: POST /api/v1/projects/{id}/cards/{cid}/activate
// and .../skip, the project-scoped card lifecycle endpoints that sit beside
// postProjectTurn's card surfacing (Task 5c-2). Uses the real testcontainers
// Postgres + the seeded demo project (00000000-0000-0000-0000-000000000101,
// owned by Phoebe) so loadOwnedProjectCard's membership scan exercises real
// rows, not a fixture.

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// createProjectCardForTest inserts a card_instance scoped to the seeded demo
// project (…0101) / task (…0100) via CreateProjectCardInstance, and returns
// its id as a string for building request paths. Uses the "craap" spec
// (not "sift_craap") — Task 5's submit test needs a card whose spec
// actually declares completion predicates + graph_effects so a fully
// answered submission can mint an evidence node; "sift_craap" declares
// neither.
func createProjectCardForTest(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	q := sqlc.New(pool)
	ci, err := q.CreateProjectCardInstance(t.Context(), sqlc.CreateProjectCardInstanceParams{
		TaskID:    mustUUID("00000000-0000-0000-0000-000000000100"),
		ProjectID: pgtype.UUID{Bytes: mustUUID("00000000-0000-0000-0000-000000000101"), Valid: true},
		CardID:    "craap",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("createProjectCardForTest: %v", err)
	}
	return ci.ID.String()
}

// craapCompleteAnchors builds a fully-satisfying CRAAP []Anchor JSON: the
// five CRAAP tags (currency/relevance/authority/accuracy/purpose) each with
// a non-empty AI-authored Answer, plus a student-authored risk_note — the
// exact shape craap.json's completion predicates
// (every_tag_present + field_written_by risk_note/student) require. Every
// anchor is keyed to the seeded material 00000000-0000-0000-0000-000000000110
// (migration 0018, project …0101) so CompleteCard's anchoredMaterialID +
// GraphEffects have a material to promote into an evidence node.
func craapCompleteAnchors(t *testing.T, pool *pgxpool.Pool, cid string) string {
	t.Helper()
	_ = pool // seeded material id is a stable fixture (migration 0018); no live lookup needed
	_ = cid
	materialID := "00000000-0000-0000-0000-000000000110"
	anchors := []agent.Anchor{
		{ID: "a0", MaterialID: materialID, Dimension: "currency", Author: "ai", Answer: "2024年发布，数据较新"},
		{ID: "a1", MaterialID: materialID, Dimension: "relevance", Author: "ai", Answer: "直接支持中国可持续论点"},
		{ID: "a2", MaterialID: materialID, Dimension: "authority", Author: "ai", Answer: "NASA地球观测团队发布，具备权威性"},
		{ID: "a3", MaterialID: materialID, Dimension: "accuracy", Author: "ai", Answer: "数据可在Nature Sustainability交叉核对"},
		{ID: "a4", MaterialID: materialID, Dimension: "purpose", Author: "ai", Answer: "科普告知性质，非商业推广"},
		{ID: "a5", MaterialID: materialID, Dimension: "risk_note", Author: "student", Answer: "仍需留意样本口径是否一致"},
	}
	b, err := json.Marshal(anchors)
	if err != nil {
		t.Fatalf("marshal craap anchors: %v", err)
	}
	return string(b)
}

func TestProjectCardActivateSkip(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	cid := createProjectCardForTest(t, pool)
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/activate", nil), cookie))
	if rr.Code != 204 {
		t.Fatalf("activate: %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/skip", strings.NewReader(`{"event_trace":[{"kind":"skip","at":"2026-07-12T00:00:00Z"}]}`)), cookie))
	if rr.Code != 204 {
		t.Fatalf("skip: %d — %s", rr.Code, rr.Body.String())
	}

	// 404 for a random (non-project) card id.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/"+uuid.NewString()+"/activate", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign card: %d", rr.Code)
	}
}

// TestProjectCardSubmit — Task 5: POST .../cards/{cid}/submit persists the
// filled envelope, runs CompleteCard (which mints an evidence node once the
// CRAAP completion predicates hold), and refeeds one RunAgentStep over SSE.
func TestProjectCardSubmit(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	cid := createProjectCardForTest(t, pool) // craap on a seeded material, status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	q := sqlc.New(pool)

	// Snapshot graph_nodes BEFORE the submit — the seed migration (0018)
	// already plants a pre-existing type=evidence node in this project (the
	// 5b orphan-evidence fixture, id …0144, body {"text":...}). Asserting
	// "any evidence node exists" would pass even if CompleteCard mints
	// nothing, so we must isolate the nodes that are NEW as of this submit.
	before, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (before): %v", err)
	}
	beforeIDs := make(map[uuid.UUID]bool, len(before))
	for _, n := range before {
		beforeIDs[n.ID] = true
	}

	// Fully-satisfying CRAAP anchors: all 5 tags answered + a student risk_note.
	anchors := craapCompleteAnchors(t, pool, cid) // helper builds anchors keyed to the card's material
	body := `{"field_values":{"final_verdict":"存疑"},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":` + anchors + `}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	// status completed
	got, _ := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if got.Status != "completed" {
		t.Fatalf("status = %q", got.Status)
	}

	// A NEW evidence node was minted by this submit — not merely the
	// pre-seeded one. Delta by identity (id not present before the submit),
	// then belt-and-suspenders on body shape: GraphEffects mints
	// Body: {"source_quality": {...}} (card_effects.go), which the seed
	// node's {"text": "..."} body does not have.
	after, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (after): %v", err)
	}
	var newEvidence *sqlc.GraphNode
	for i, n := range after {
		if beforeIDs[n.ID] {
			continue // pre-existing node (e.g. the seed's orphan evidence fixture)
		}
		if n.Type == "evidence" {
			newEvidence = &after[i]
			break
		}
	}
	if newEvidence == nil {
		t.Fatalf("no NEW evidence node minted by submit (found %d nodes before, %d after)", len(before), len(after))
	}
	var decodedBody map[string]json.RawMessage
	if err := json.Unmarshal(newEvidence.Body, &decodedBody); err != nil {
		t.Fatalf("unmarshal minted evidence node body: %v — %s", err, newEvidence.Body)
	}
	if _, ok := decodedBody["source_quality"]; !ok {
		t.Fatalf("minted evidence node body missing source_quality (CRAAP GraphEffects shape): %s", newEvidence.Body)
	}
}

// TestProjectCardSubmit_FormPathDoesNotComplete — the re-scope gate (whole-
// branch-review, Slice 5c-2): a submit via the form path — anchors: [] as
// envelopeReducer actually produces today, since the schema field renderer
// only ever fills field_values, never anchors (that bridge is deferred to
// Slice 6's material-annotation surface) — must NOT flip the card to
// "completed" and must NOT mint a new evidence node. Before this fix,
// submitProjectCard set status="completed" unconditionally regardless of
// whether CompleteCard's predicates held, permanently marking every
// form-only CRAAP submit "done" with no evidence and no resubmit path.
func TestProjectCardSubmit_FormPathDoesNotComplete(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	cid := createProjectCardForTest(t, pool) // craap on a seeded material, status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	q := sqlc.New(pool)

	before, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (before): %v", err)
	}
	beforeIDs := make(map[uuid.UUID]bool, len(before))
	for _, n := range before {
		beforeIDs[n.ID] = true
	}

	// Mirrors what envelopeReducer actually produces for a form-only CRAAP
	// fill today: no anchors at all.
	body := `{"field_values":{"final_verdict":"存疑"},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":[]}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}

	// status stays active — NOT completed — so the student can resubmit.
	got, _ := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if got.Status != "active" {
		t.Fatalf("status = %q, want active (non-satisfying submit must not complete)", got.Status)
	}

	// No NEW evidence node minted.
	after, err := q.ListGraphNodesByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ListGraphNodesByProject (after): %v", err)
	}
	for _, n := range after {
		if beforeIDs[n.ID] {
			continue
		}
		if n.Type == "evidence" {
			t.Fatalf("unexpected NEW evidence node minted by a non-satisfying (anchors:[]) submit: %+v", n)
		}
	}
}
