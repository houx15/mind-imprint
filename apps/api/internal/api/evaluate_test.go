package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestEvaluateInlineThenGet(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// Seed a tiny transcript so the eval input isn't empty.
	_, _ = q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: task.ID, Role: "user", Content: "hi"})

	evalJSON := `{"scores":[{"dim_id":"D1","level":"L3","note":"n"}],"narrative":"你的思维印记"}`
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: evalJSON},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 50, OutputTokens: 80}},
		{Kind: gateway.EventDone},
	})
	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	idx := map[string]cards.Spec{}
	for _, s := range catalog {
		idx[s.ID] = s
	}
	h := New(Deps{
		Queries:  sqlc.New(pool),
		Pool:     pool,
		Provider: prov,
		EvalResolver: func(_ context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-reasoner", Tier: "flagship"}, nil
		},
		Catalog:  catalog,
		SpecByID: func(id string) (cards.Spec, bool) { s, ok := idx[id]; return s, ok },
	}).Handler()
	cookie := signInSeed(t, pool)

	// POST evaluate → 201 + narrative in body
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/evaluate", nil), cookie))
	if rr.Code != 201 || !strings.Contains(rr.Body.String(), "你的思维印记") {
		t.Fatalf("evaluate: %d %s", rr.Code, rr.Body.String())
	}

	// task is now evaluated
	got, err := q.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != "evaluated" {
		t.Fatalf("task status %s, want 'evaluated'", got.Status)
	}

	// GET evaluation → 200 + model in body
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/"+task.ID.String()+"/evaluation", nil), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"model":"deepseek-reasoner"`) {
		t.Fatalf("get eval: %d %s", rr.Code, rr.Body.String())
	}

	// GET on a task with no evaluation → 404
	other, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "U"})
	if err != nil {
		t.Fatalf("CreateTask other: %v", err)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/"+other.ID.String()+"/evaluation", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("want 404 for no eval, got %d — body: %s", rr.Code, rr.Body.String())
	}
}
