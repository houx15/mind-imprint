package api_test

// coach_plan_tools_test.go — bugs 2 & 7 · the coach's plan-management and
// explore-keyword tools. update_plan lets 印记 mark a plan item done/edit it
// when the student reports progress; note_resource_need drops a keyword into
// the 还需要探索的 box. Both are status tools admitted in the writing statuses.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// TestPostCoach_UpdatePlanCompletesItem — in a writing status, an update_plan
// {op:complete, match:...} tool call flips the matching plan item's column to
// "done" (bug 2: the coach can change a plan item's status when work finishes).
func TestPostCoach_UpdatePlanCompletesItem(t *testing.T) {
	out := `{"narrate":"好，我把这一步标成完成了。","tools":[` +
		`{"name":"update_plan","args":{"op":"complete","match":"溯源"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	q := sqlc.New(pool)
	// Seed a todo plan item to target by title match.
	if _, err := q.CreatePlanItem(context.Background(), sqlc.CreatePlanItemParams{
		ProjectID: mustUUID(seedProjectID), Title: "通读并溯源关键文献", Tag: "read", Col: "todo",
		Stage: "阶段二 · 研究", RefMaterialID: pgtype.UUID{}, StartDay: 3, Days: 4, Position: 0,
	}); err != nil {
		t.Fatalf("CreatePlanItem: %v", err)
	}
	setStudioStage(t, pool, seedProjectID, agent.StageBodyWriting) // essay — update_plan allowed
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我把文献都读完溯源好了"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	items, err := q.ListPlanItems(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("ListPlanItems: %v", err)
	}
	var found bool
	for _, it := range items {
		if strings.Contains(it.Title, "溯源") {
			found = true
			if it.Col != "done" {
				t.Fatalf("expected the matched plan item to be done, got col=%q", it.Col)
			}
		}
	}
	if !found {
		t.Fatalf("seeded plan item not found in %d items", len(items))
	}
}

// TestPostCoach_NoteResourceNeedAddsKeyword — a note_resource_need tool call
// appends a keyword to the 还需要探索的 box (studio_state.ResourceNeeds) and
// flips reply.resourceNeedAdded so the client refetches (bug 7).
func TestPostCoach_NoteResourceNeedAddsKeyword(t *testing.T) {
	out := `{"narrate":"我把这个关键词记进探索清单了。","tools":[` +
		`{"name":"note_resource_need","args":{"text":"中国碳排放总量","why":"可能挑战你的立场"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	setStudioStage(t, pool, seedProjectID, agent.StageBodyWriting) // essay — note_resource_need allowed
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我担心中国碳排放这条反例"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		ResourceNeedAdded bool `json:"resourceNeedAdded"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if !resp.ResourceNeedAdded {
		t.Fatalf("expected resourceNeedAdded=true — %s", rr.Body)
	}

	raw, err := sqlc.New(pool).GetStudioState(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("GetStudioState: %v", err)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v", err)
	}
	var has bool
	for _, n := range st.ResourceNeeds {
		if strings.Contains(n.Text, "中国碳排放总量") {
			has = true
		}
	}
	if !has {
		t.Fatalf("expected the keyword in ResourceNeeds, got %+v", st.ResourceNeeds)
	}
}
