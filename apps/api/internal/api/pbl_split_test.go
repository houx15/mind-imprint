package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	. "mindimprint/api/internal/api"
)

type substepOut struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Owner         string  `json:"owner"`
	Reason        string  `json:"reason"`
	StudentOwner  *string `json:"studentOwner"`
	StudentReason string  `json:"studentReason"`
	ConfirmedAt   *string `json:"confirmedAt"`
}

// firstStepID returns the id of the first step of the approved plan.
func firstStepID(t *testing.T, h http.Handler, c *http.Cookie, pid string) string {
	t.Helper()
	plan := pblGetPlan(t, h, c, pid)
	live, ok := plan["plan"].(map[string]any)
	if !ok {
		t.Fatalf("no live plan: %v", plan)
	}
	steps, ok := live["steps"].([]any)
	if !ok || len(steps) == 0 {
		t.Fatalf("no steps: %v", live)
	}
	first, _ := steps[0].(map[string]any)
	id, _ := first["id"].(string)
	if id == "" {
		t.Fatalf("step has no id: %v", first)
	}
	return id
}

func seedSubsteps(t *testing.T, h http.Handler, c *http.Cookie) (string, []substepOut) {
	t.Helper()
	pid := seedApprovedPlan(t, h, c)
	sid := firstStepID(t, h, c, pid)
	rec := pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/steps/"+sid+"/substeps", `{"substeps":[
	  {"title":"先说清楚要看什么","owner":"both","reason":"这个只有你知道你想看什么"},
	  {"title":"整理三天的数字","owner":"yinji","reason":"重复的活，我来快一些"},
	  {"title":"判断哪一类最多","owner":"student","reason":"这是这一步真正要你判断的东西"}
	]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("propose substeps = %d; body=%s", rec.Code, rec.Body)
	}
	var out []substepOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode substeps: %v — body=%s", err, rec.Body)
	}
	return pid, out
}

// 🚨 改一格就要写一句为什么。允许不写理由地改，等于允许一路点"同意"——那这张
// 卡就只是在帮 AI 领活，而 AI 多领一件，她就少做一件。
func TestPblSplit_ChangingAnAssignmentCostsAReason(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid, subs := seedSubsteps(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/substeps/" + subs[1].ID + "/reassign"

	if rec := pblPost(t, h, cookie, url, `{"owner":"student","reason":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("reassign with no reason = %d, want 400", rec.Code)
	}
	if rec := pblPost(t, h, cookie, url, `{"owner":"谁都行","reason":"想改"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("reassign to an unknown owner = %d, want 400", rec.Code)
	}

	rec := pblPost(t, h, cookie, url, `{"owner":"student","reason":"我想自己数一遍才有感觉"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reassign = %d; body=%s", rec.Code, rec.Body)
	}
	var got substepOut
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.StudentOwner == nil || *got.StudentOwner != "student" {
		t.Fatalf("studentOwner = %v", got.StudentOwner)
	}
	if got.StudentReason == "" {
		t.Fatal("她的理由没存住")
	}
	// 🚨 印记原来提的那一格要留着。「AI 本来想自己做，她拿回去了」是这门课上
	// 最值得记下来的事之一，覆盖掉就没了。
	if got.Owner != "yinji" || got.Reason == "" {
		t.Fatalf("印记原来的分工被覆盖了：owner=%q reason=%q", got.Owner, got.Reason)
	}
}

// 印记自己也要说明为什么这一格归它。没有理由的分工，她没法反对。
func TestPblSplit_YinjiMustSayWhyToo(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := seedApprovedPlan(t, h, cookie)
	sid := firstStepID(t, h, cookie, pid)
	url := "/api/v1/pbl/projects/" + pid + "/steps/" + sid + "/substeps"

	for _, body := range []string{
		`{"substeps":[{"title":"整理数字","owner":"yinji","reason":"  "}]}`,
		`{"substeps":[{"title":"整理数字","owner":"随便谁","reason":"快"}]}`,
		`{"substeps":[{"title":"   ","owner":"yinji","reason":"快"}]}`,
		`{"substeps":[]}`,
	} {
		if rec := pblPost(t, h, cookie, url, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("propose %s = %d, want 400", body, rec.Code)
		}
	}
}

func TestPblSplit_ConfirmAndStatus(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid, subs := seedSubsteps(t, h, cookie)
	base := "/api/v1/pbl/projects/" + pid + "/substeps/" + subs[0].ID

	rec := pblPost(t, h, cookie, base+"/confirm", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm = %d; body=%s", rec.Code, rec.Body)
	}
	var got substepOut
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.ConfirmedAt == nil {
		t.Fatal("认下了却没记住")
	}
	if rec := pblReq(t, h, cookie, "PATCH", base, `{"status":"doing"}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := pblReq(t, h, cookie, "PATCH", base, `{"status":"随便"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, want 400", rec.Code)
	}
}

func TestPblSplit_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid, subs := seedSubsteps(t, h, cookie)

	otherID := createStudent(t, pool, SeedSchoolID, "other-split@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblPost(t, h, other, "/api/v1/pbl/projects/"+pid+"/substeps/"+subs[0].ID+"/reassign",
		`{"owner":"yinji","reason":"我来"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign reassign = %d, want 404", rec.Code)
	}
}
