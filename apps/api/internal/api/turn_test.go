package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/cards"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestTurnStreamsCardThenDone(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	idx := map[string]cards.Spec{}
	for _, s := range catalog {
		idx[s.ID] = s
	}

	// Stub proposes sift_craap then ends.
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "先一起核查来源"},
		{Kind: gateway.EventToolUse, ToolUse: &gateway.StreamToolUse{ID: "tc1", Name: "summon_card", ArgsJSON: `{"card_id":"sift_craap","reason":"r","nudge_text":"要不要一起溯源？"}`}},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopToolCall},
	})
	h := New(Deps{
		Queries:  sqlc.New(pool),
		Pool:     pool,
		Provider: prov,
		ChatResolver: func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "chaperone"}, nil
		},
		Catalog:  catalog,
		SpecByID: func(id string) (cards.Spec, bool) { s, ok := idx[id]; return s, ok },
	}).Handler()
	cookie := signInSeed(t, pool)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/turn", strings.NewReader(`{"user_input":"我想引用这篇公众号文章"}`)), cookie))
	if rr.Code != 200 {
		t.Fatalf("turn: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	bodyStr := rr.Body.String()
	for _, want := range []string{"event: text", "event: card", `"card_id":"sift_craap"`, "event: done"} {
		if !strings.Contains(bodyStr, want) {
			t.Fatalf("missing %q in SSE:\n%s", want, bodyStr)
		}
	}
	// A proposed card_instance was persisted.
	crds, err := q.ListCardsByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListCardsByTask: %v", err)
	}
	if len(crds) != 1 || crds[0].Status != "proposed" {
		t.Fatalf("card not persisted: %+v", crds)
	}
	// The SSE card event must carry a non-empty card_instance_id that matches
	// the persisted row — so the client can submit the envelope back by id.
	if !strings.Contains(bodyStr, `"card_instance_id":"`) {
		t.Fatalf(`missing "card_instance_id" key in SSE body:\n%s`, bodyStr)
	}
	if !strings.Contains(bodyStr, crds[0].ID.String()) {
		t.Fatalf("card_instance_id %q not found in SSE body:\n%s", crds[0].ID.String(), bodyStr)
	}
}

func TestTurnEmptyInputIsContinuation(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// seed a prior user message so history isn't empty (realistic continuation)
	_, _ = q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: task.ID, Role: "user", Content: "hi"})

	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	idx := map[string]cards.Spec{}
	for _, s := range catalog {
		idx[s.ID] = s
	}
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "继续"},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	h := New(Deps{
		Queries:  sqlc.New(pool),
		Pool:     pool,
		Provider: prov,
		ChatResolver: func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "chaperone"}, nil
		},
		Catalog:  catalog,
		SpecByID: func(id string) (cards.Spec, bool) { s, ok := idx[id]; return s, ok },
	}).Handler()
	cookie := signInSeed(t, pool)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/turn", strings.NewReader(`{"user_input":""}`)), cookie))
	if rr.Code != 200 {
		t.Fatalf("continuation: want 200, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("want done event, got %s", rr.Body.String())
	}
}
