package api_test

import (
	"encoding/json"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

func TestSiteRefBookmarkDoesNotRequireReadablePageOrModel(t *testing.T) {
	provider := gateway.NewSequenceStubProvider()
	_, c, q, pool := liteHandler(t)
	// No fetcher: a JS-only or unavailable reference must still be collectable.
	h := New(Deps{Queries: q, Pool: pool, Provider: provider, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver()}).Handler()
	pid := newProjectViaAPI(t, h, c)
	base := "/api/v1/pbl/projects/" + pid + "/sites"
	body := `{"url":"https://example.com/interactive-demo#plant"}`
	r := siteReq(t, h, c, "POST", base+"/bookmark", body)
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body)
	}
	var saved struct{ ID, What, Structure, Best, SheSaid string }
	if err := json.Unmarshal(r.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Body.String(), "#plant") {
		t.Fatal("selected effect lost", r.Body)
	}
	if saved.What != "" || saved.Structure != "" || saved.Best != "" || provider.Calls != 0 {
		t.Fatal("bookmark invented an analysis or called model", r.Body)
	}
	note := "尚未体验，想探索鼠标移动能否改变光点方向。"
	data, _ := json.Marshal(map[string]string{"sheSaid": note})
	if r = siteReq(t, h, c, "PATCH", base+"/"+saved.ID, string(data)); r.Code != 200 {
		t.Fatal(r.Body)
	}
	// Re-collecting must not erase the student's judgment or an existing analysis.
	if _, err := pool.Exec(t.Context(), `UPDATE pbl_site_ref SET what='页面文字介绍粒子移动',structure='说明和示例',best='可探索方向映射' WHERE id=$1`, saved.ID); err != nil {
		t.Fatal(err)
	}
	r = siteReq(t, h, c, "POST", base+"/bookmark", body)
	if r.Code != 201 || !strings.Contains(r.Body.String(), note) || !strings.Contains(r.Body.String(), "页面文字介绍粒子移动") {
		t.Fatal(r.Body)
	}
	var count int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM pbl_site_ref WHERE atom_id=$1`, pid).Scan(&count); err != nil || count != 1 {
		t.Fatal("bookmark duplicated", err, count)
	}
	other := createStudent(t, pool, SeedSchoolID, "site-bookmark-other@demo.local")
	if r = siteReq(t, h, signInAs(t, pool, other), "POST", base+"/bookmark", body); r.Code != 404 {
		t.Fatal("foreign project accepted", r.Code, r.Body)
	}
	if r = siteReq(t, h, c, "POST", base+"/bookmark", `{"url":"file:///tmp/demo"}`); r.Code != 400 {
		t.Fatal("non-http URL accepted", r.Body)
	}
	if provider.Calls != 0 {
		t.Fatal("bookmark flow called model")
	}
}
