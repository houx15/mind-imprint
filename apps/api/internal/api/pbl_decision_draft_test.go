package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

func TestDecisionDraftRecoveryAndFinalization(t *testing.T) {
	h, c, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, c)
	base := "/api/v1/pbl/projects/" + pid + "/decisions"
	d := decodeDecision(t, pblPost(t, h, c, base, twoRoads))
	draftURL := base + "/" + d.ID + "/draft"
	body := fmt.Sprintf(`{"revision":0,"draft":{"choiceId":%q,"why":"尚未写完的原因","dropped":{%q:"另一方案的理由"},"adding":true,"mineLabel":"自选方案草稿"}}`, d.Options[0].ID, d.Options[1].ID)
	saved := siteReq(t, h, c, "PUT", draftURL, body)
	if saved.Code != 200 {
		t.Fatal(saved.Body)
	}
	got := siteReq(t, h, c, "GET", draftURL, "")
	if !strings.Contains(got.Body.String(), "尚未写完的原因") || !strings.Contains(got.Body.String(), `"revision":1`) {
		t.Fatal(got.Body)
	}
	decisions := siteReq(t, h, c, "GET", base, "")
	if strings.Contains(decisions.Body.String(), "尚未写完的原因") {
		t.Fatal("draft leaked into final decision")
	}
	if stale := siteReq(t, h, c, "PUT", draftURL, body); stale.Code != http.StatusConflict {
		t.Fatal("stale overwrite accepted", stale.Body)
	}
	other := newProjectViaAPI(t, h, c)
	if cross := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+other+"/decisions/"+d.ID+"/draft", ""); cross.Code != 404 {
		t.Fatal("cross project draft readable")
	}
	for _, invalid := range []string{
		`{"revision":1,"draft":{"choiceId":"not-an-option"}}`,
		`{"revision":1,"draft":{"dropped":{"not-an-option":"reason"}}}`,
		`{"revision":1,"draft":{"why":"ok","unexpected":true}}`,
		`{"draft":{}}`,
	} {
		if r := siteReq(t, h, c, "PUT", draftURL, invalid); r.Code != 400 {
			t.Fatal("invalid draft accepted", r.Body)
		}
	}

	// A concurrent save may win before finalization; either ordering must leave
	// an immutable final decision with no draft, never the other way round.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		r := siteReq(t, h, c, "PUT", draftURL, strings.Replace(body, `"revision":0`, `"revision":1`, 1))
		codes <- r.Code
	}()
	go func() {
		defer wg.Done()
		r := siteReq(t, h, c, "POST", base+"/"+d.ID+"/settle", `{"choice":"先给食堂","why":"他们能调整","whyNot":"班群不能直接调整"}`)
		codes <- r.Code
	}()
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 && code != 409 {
			t.Fatal("unexpected concurrent response", code)
		}
	}
	final := siteReq(t, h, c, "GET", draftURL, "")
	var state struct {
		Draft   map[string]any `json:"draft"`
		Settled bool           `json:"settled"`
	}
	if err := json.Unmarshal(final.Body.Bytes(), &state); err != nil || !state.Settled || len(state.Draft) != 0 {
		t.Fatal("finalization retained draft", final.Body)
	}
	if late := siteReq(t, h, c, "PUT", draftURL, `{"revision":2,"draft":{}}`); late.Code != 409 {
		t.Fatal("late draft overwrote decision", late.Body)
	}
}

func TestDecisionDraftIsPrivateAndNotCoachEvidence(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"请继续比较。"}`), evidenceScript(`{"reply":"按照已确认决定继续。"}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, cookie, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, cookie)
	base := "/api/v1/pbl/projects/" + pid + "/decisions"
	d := decodeDecision(t, pblPost(t, h, cookie, base, twoRoads))
	otherProject := newProjectViaAPI(t, h, cookie)
	decodeDecision(t, pblPost(t, h, cookie, "/api/v1/pbl/projects/"+otherProject+"/decisions", strings.ReplaceAll(twoRoads, "先给食堂", "跨项目不可见候选")))
	url := base + "/" + d.ID + "/draft"
	body := fmt.Sprintf(`{"revision":0,"draft":{"choiceId":%q,"why":"尚未确认的专属草稿标记"}}`, d.Options[0].ID)
	if r := siteReq(t, h, cookie, "PUT", url, body); r.Code != 200 {
		t.Fatal(r.Body)
	}
	other := signInAs(t, pool, createStudent(t, pool, api.SeedSchoolID, "draft-other@demo.local"))
	for _, method := range []string{"GET", "PUT"} {
		if r := siteReq(t, h, other, method, url, body); r.Code != 404 {
			t.Fatalf("other student %s status=%d", method, r.Code)
		}
	}
	turnURL := "/api/v1/pbl/projects/" + pid + "/turn"
	if r := siteReq(t, h, cookie, "POST", turnURL, `{"text":"接下来怎么做"}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	request, _ := json.Marshal(provider.LastRequest)
	if strings.Contains(string(request), "尚未确认的专属草稿标记") {
		t.Fatal("unconfirmed draft became coach evidence")
	}
	if !strings.Contains(string(request), "待定决策") || !strings.Contains(string(request), d.ID) || !strings.Contains(string(request), "先给食堂") {
		t.Fatal("coach cannot see the pending options the student is discussing")
	}
	if strings.Contains(string(request), "跨项目不可见候选") {
		t.Fatal("pending options crossed project boundaries")
	}
	if r := siteReq(t, h, cookie, "POST", base+"/"+d.ID+"/settle", `{"choice":"先给食堂","why":"最终选择依据标记","whyNot":"班群无法直接调整"}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r := siteReq(t, h, cookie, "POST", turnURL, `{"text":"按照我的决定继续"}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	request, _ = json.Marshal(provider.Requests[1])
	if !strings.Contains(string(request), "最终选择依据标记") || strings.Contains(string(request), "尚未确认的专属草稿标记") {
		t.Fatal("final decision and draft were not distinguished in coach input")
	}
}
