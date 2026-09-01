package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pblPost(t *testing.T, h http.Handler, c *http.Cookie, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", url, strings.NewReader(body)), c))
	return rec
}

func pblGetPlan(t *testing.T, h http.Handler, c *http.Cookie, pid string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/pbl/projects/"+pid+"/plan", nil), c))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET plan = %d; body=%s", rec.Code, rec.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode plan: %v — body=%s", err, rec.Body)
	}
	return out
}

const twoStepPlan = `{"summary":"先弄清楚剩的是什么","reason":"她批准的",
 "steps":[
   {"title":"去食堂看三天","decide":"你判断剩得最多的是哪一类"},
   {"title":"问三个同学","decide":"你决定问谁"}
 ]}`

// seedApprovedPlan proposes and approves a plan, returning the project id.
func seedApprovedPlan(t *testing.T, h http.Handler, c *http.Cookie) string {
	t.Helper()
	pid := newProjectViaAPI(t, h, c)
	rec := pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/plan", twoStepPlan)
	if rec.Code != http.StatusCreated {
		t.Fatalf("propose plan = %d; body=%s", rec.Code, rec.Body)
	}
	var v struct {
		VersionID string `json:"versionId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	rec = pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/plan/approve",
		`{"versionId":"`+v.VersionID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve = %d; body=%s", rec.Code, rec.Body)
	}
	return pid
}

// Nothing runs before she approves: an unapproved version is not the plan.
func TestPblPlan_NothingRunsBeforeApproval(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	if got := pblGetPlan(t, h, cookie, pid)["plan"]; got != nil {
		t.Fatalf("plan = %v before any proposal, want null", got)
	}
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/plan", twoStepPlan); rec.Code != http.StatusCreated {
		t.Fatalf("propose = %d; body=%s", rec.Code, rec.Body)
	}
	if got := pblGetPlan(t, h, cookie, pid)["plan"]; got != nil {
		t.Fatalf("an unapproved version showed as the live plan: %v", got)
	}
}

// 🚨 Every step names what she decides. A hollow plan is regenerated, not shipped.
func TestPblPlan_RefusesAStepWithNoDecision(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/plan",
		`{"summary":"x","steps":[{"title":"去食堂看三天","decide":"  "}]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("step with no decision = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "step_without_decision") {
		t.Fatalf("body = %s, want the step_without_decision code", rec.Body)
	}
}

// 🚨🚨 The invariant this whole slice exists for: a staged structural change
// leaves the live plan untouched. If this ever goes green while the steps moved,
// 隐形重规划 is back.
func TestPblPlan_StagedStructuralChangeDoesNotTouchTheLivePlan(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := seedApprovedPlan(t, h, cookie)

	before, err := json.Marshal(pblGetPlan(t, h, cookie, pid)["plan"])
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/plan/changes",
		`{"kind":"modify","fields":["success_criteria"],
		  "diff":{"remove":["去食堂看三天"],"add":["先问食堂阿姨"]},
		  "evidence":"访谈说法和我们原来的判断不一样"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("stage = %d; body=%s", rec.Code, rec.Body)
	}

	after := pblGetPlan(t, h, cookie, pid)
	afterPlan, err := json.Marshal(after["plan"])
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if string(before) != string(afterPlan) {
		t.Fatalf("the live plan moved on staging:\nbefore=%s\nafter =%s", before, afterPlan)
	}
	// …and it IS waiting for her, visibly.
	pending, _ := after["pending"].([]any)
	if len(pending) != 1 {
		t.Fatalf("pending = %v, want the one staged change", after["pending"])
	}
}

// A change that does not need her is refused rather than parked forever.
func TestPblPlan_NonStructuralChangeIsNotStaged(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := seedApprovedPlan(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/plan/changes",
		`{"kind":"modify","fields":["step_order"],"evidence":"顺序换一下更顺"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("local change staged = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// A proposal with no evidence is one she cannot judge.
func TestPblPlan_ChangeNeedsEvidence(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := seedApprovedPlan(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/plan/changes",
		`{"kind":"modify","fields":["question"],"evidence":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("evidence-free change = %d, want 400", rec.Code)
	}
}

// Plan Check: 「保留原计划」 is a real outcome, and every outcome needs a reason.
func TestPblPlan_ResolveRequiresAReasonAndAcceptsKept(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := seedApprovedPlan(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/plan/changes",
		`{"kind":"modify","fields":["question"],"evidence":"四个新生根本没看到地图"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("stage = %d; body=%s", rec.Code, rec.Body)
	}
	var c struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode change: %v", err)
	}
	url := "/api/v1/pbl/projects/" + pid + "/plan/changes/" + c.ID + "/resolve"

	if rec := pblPost(t, h, cookie, url, `{"resolution":"kept","reason":"  "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("reasonless resolve = %d, want 400", rec.Code)
	}
	if rec := pblPost(t, h, cookie, url, `{"resolution":"whatever","reason":"x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown resolution = %d, want 400", rec.Code)
	}
	if rec := pblPost(t, h, cookie, url,
		`{"resolution":"kept","reason":"访谈只有一个人，先补证据"}`); rec.Code != http.StatusOK {
		t.Fatalf("kept-with-a-reason = %d; body=%s", rec.Code, rec.Body)
	}
	// Resolved once, gone from what is waiting for her.
	if pending, _ := pblGetPlan(t, h, cookie, pid)["pending"].([]any); len(pending) != 0 {
		t.Fatalf("%d change(s) still pending after Plan Check", len(pending))
	}
	// And resolving twice is refused.
	if rec := pblPost(t, h, cookie, url, `{"resolution":"accepted","reason":"再来"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("second resolve = %d, want 400", rec.Code)
	}
}

// Progress applies straight away and never interrupts her.
func TestPblPlan_StepStatusAppliesDirectly(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := seedApprovedPlan(t, h, cookie)

	plan, _ := pblGetPlan(t, h, cookie, pid)["plan"].(map[string]any)
	steps, _ := plan["steps"].([]any)
	if len(steps) != 2 {
		t.Fatalf("steps = %v, want 2", plan["steps"])
	}
	first, _ := steps[0].(map[string]any)
	sid, _ := first["id"].(string)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH",
		"/api/v1/pbl/projects/"+pid+"/plan/steps/"+sid,
		strings.NewReader(`{"status":"awaiting_evidence"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("set status = %d; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH",
		"/api/v1/pbl/projects/"+pid+"/plan/steps/"+sid,
		strings.NewReader(`{"status":"blocked"}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown status = %d, want 400", rec.Code)
	}
}
