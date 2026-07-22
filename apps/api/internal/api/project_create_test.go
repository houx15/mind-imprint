package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestCreateProject_SeedsOnboardingNodes(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"我的论文","prompt":"讨论社交媒体对青少年注意力的影响"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	// The project exists, owned by the seed user, qualification 0457.
	var qual string
	if err := pool.QueryRow(context.Background(),
		`SELECT qualification FROM project WHERE id=$1`, pid).Scan(&qual); err != nil {
		t.Fatalf("project row missing: %v", err)
	}
	if qual != "0457" {
		t.Errorf("qualification = %q, want 0457", qual)
	}
	// Exactly the three onboarding nodes, correct types + authors.
	rows, err := pool.Query(context.Background(),
		`SELECT type, author FROM graph_node WHERE project_id=$1 ORDER BY type`, pid)
	if err != nil {
		t.Fatalf("query nodes: %v", err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var typ, author string
		if err := rows.Scan(&typ, &author); err != nil {
			t.Fatal(err)
		}
		got[typ] = author
	}
	if got["assignment_brief"] != "imported" || got["rubric_translation"] != "ai" || got["milestone_plan"] != "ai" {
		t.Fatalf("onboarding nodes wrong: %+v", got)
	}
}

// The project title IS her research question — minted at creation so S1's
// read-only banner has a real node behind it and frame_question's
// node_present:research_question item has a producer at all.
func TestCreateProject_MintsResearchQuestion(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	body := strings.NewReader(`{"title":"社交媒体是否影响青少年注意力？","prompt":"讨论社交媒体对青少年注意力的影响"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /projects = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	pid := uuid.MustParse(out.ID)

	var author string
	var rqBody []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT author, body FROM graph_node WHERE project_id=$1 AND type='research_question'`, pid).
		Scan(&author, &rqBody); err != nil {
		t.Fatalf("research_question node missing: %v", err)
	}
	if author != "student" {
		t.Errorf("research_question author = %q, want student", author)
	}
	var got struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rqBody, &got); err != nil {
		t.Fatalf("unmarshal research_question body: %v; raw=%s", err, rqBody)
	}
	if got.Text != "社交媒体是否影响青少年注意力？" {
		t.Errorf("research_question text = %q, want the project title", got.Text)
	}
}

func TestCreateProject_EmptyPromptRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects", strings.NewReader(`{"prompt":""}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty prompt = %d, want 400", rec.Code)
	}
}
