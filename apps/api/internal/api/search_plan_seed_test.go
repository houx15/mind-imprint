package api_test

// search_plan_seed_test.go — N3e Task 4 (+ Task 6): proves the search-plan
// matrix card's rows are seeded from the student's own preregistration
// directions on first surface (Task 4), and that the seeded-fill-submit path
// completes the card and writes framework_fill (Task 6's acceptance walk).
// Reuses walk_s0_s6_test.go's harness helpers verbatim — no second harness,
// no DB-scan helper; the seeded anchors come back from surfaceWalkCard.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// setupProjectAtS1WithSearchPlan: create -> onboarding -> framing with a
// two-direction search plan. Returns everything the surface helpers need.
// Mirrors TestWalk_S0ToS6_FreshProject's setup (walk_s0_s6_test.go:279-330).
func setupProjectAtS1WithSearchPlan(t *testing.T, dirs [2]string) (h http.Handler, cookie *http.Cookie, pid string, pool *pgxpool.Pool) {
	t.Helper()
	pool = newAPITestPool(t)
	cookie = signInSeed(t, pool)
	h = New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(),
		SpecByID: cards.ByID,
	}).Handler()
	pid = createProjectForTest(t, h, cookie)
	postOK(t, h, cookie, "/api/v1/projects/"+pid+"/onboarding",
		`{"restate":"这道题在问中国是否让地球变得更可持续","weakPicks":[0,2]}`)
	postOK(t, h, cookie, "/api/v1/projects/"+pid+"/framing",
		`{"terms":[{"term":"可持续发展","definition":"资源使用不损害后代满足自身需求的能力这是环境定义"},{"term":"中国角色","definition":"中国政策与产出对全球环境指标造成的净影响这是国家定义"},{"term":"世界","definition":"全球尺度而非仅中国境内的地理与生态范围这是空间定义"}],"answers":["中国的可再生能源投入使全球减排加快"],"searchPlan":["`+dirs[0]+`","`+dirs[1]+`"]}`)
	return h, cookie, pid, pool
}

// postOK is a thin 200-asserting POST helper.
func postOK(t *testing.T, h http.Handler, cookie *http.Cookie, path, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", path, strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST %s = %d; body=%s", path, rec.Code, rec.Body)
	}
}

func TestSearchPlanSeed_OneRowPerDirection(t *testing.T) {
	dirs := [2]string{"查NASA卫星植被数据", "查中国官方碳排放文件"}
	h, cookie, pid, pool := setupProjectAtS1WithSearchPlan(t, dirs)

	// The turn surfaces the search-plan card; surfaceWalkCard returns its anchors.
	_, _, anchors := surfaceWalkCard(t, h, pool, cookie, pid, "search-plan", "我打算开始找证据了")

	got := map[string]string{}
	for _, a := range anchors {
		got[a.Quote] = a.Answer
	}
	if len(got) != 2 || got[dirs[0]] != "" || got[dirs[1]] != "" {
		t.Fatalf("expected two empty-answer seeded rows keyed by direction, got %+v", got)
	}
}

// TestSearchPlan_SurfaceFillComplete is N3e Task 6's acceptance assertion:
// the whole Component A path end-to-end on a fresh funnel project — surface
// (seeded), activate, fill one row fully, submit, complete, and confirm the
// consolidation payload (framework_fill) was written on completion.
func TestSearchPlan_SurfaceFillComplete(t *testing.T) {
	dirs := [2]string{"查NASA卫星植被数据", "查中国官方碳排放文件"}
	h, cookie, pid, pool := setupProjectAtS1WithSearchPlan(t, dirs)

	cid, _, _ := surfaceWalkCard(t, h, pool, cookie, pid, "search-plan", "我打算开始找证据了")
	activateWalkCard(t, h, cookie, pid, cid)

	// Fill one row (min_items:1) fully: one anchor per column, keyed by the
	// direction (row label). Mirrors serialize.ts matrixStateToAnchors.
	filled := []agent.Anchor{
		{Quote: dirs[0], Dimension: "evidence_type", Answer: "官方口径的排放与能源统计", Author: "student"},
		{Quote: dirs[0], Dimension: "blind_spot", Answer: "看不到独立第三方对治理成效的质疑", Author: "student"},
		{Quote: dirs[0], Dimension: "disconfirm", Answer: "只会印证官方叙述，找不到反证", Author: "student"},
	}
	submitWalkCard(t, h, cookie, pid, cid, filled)
	assertCardCompleted(t, pool, cid)

	// R-9: CompleteCard wrote the consolidation payload into framework_fill.
	row, err := sqlc.New(pool).GetCardInstance(context.Background(), mustUUID(cid))
	if err != nil {
		t.Fatalf("GetCardInstance: %v", err)
	}
	if s := strings.TrimSpace(string(row.FrameworkFill)); s == "" || s == "{}" || s == "null" {
		t.Fatalf("expected framework_fill written on completion, got %q", row.FrameworkFill)
	}
}
