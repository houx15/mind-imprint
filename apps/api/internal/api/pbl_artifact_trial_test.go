package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/pbl"
)

func TestArtifactTrialDraftOwnershipAndSubmission(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"请继续检查"}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	aid := newArtifactViaAPI(t, h, c, pid)
	base := "/api/v1/pbl/projects/" + pid + "/artifacts/" + aid + "/trial-draft"
	doc := pbl.ArtifactTrial{Mode: "not_tested", Version: "草稿唯一标记：未进行试问", Task: "检查提纲用时", Expected: "能判断问题是否过多", Actual: "旧实际结果不得进入未测试计划"}
	save := func(rev int, d pbl.ArtifactTrial) int {
		raw, _ := json.Marshal(map[string]any{"revision": rev, "document": d})
		r := siteReq(t, h, c, "PUT", base, string(raw))
		return r.Code
	}
	if r := siteReq(t, h, c, "GET", base, ""); r.Code != 200 || !strings.Contains(r.Body.String(), `"revision":0`) {
		t.Fatal(r.Body)
	}
	if code := save(0, doc); code != 200 {
		t.Fatal(code)
	}
	if code := save(0, doc); code != 409 {
		t.Fatalf("stale save: %d", code)
	}
	// A different project owned by the same student is not a valid path to this artifact.
	other := newProjectViaAPI(t, h, c)
	otherBase := "/api/v1/pbl/projects/" + other + "/artifacts/" + aid + "/trial-draft"
	otherStudent := createStudent(t, pool, SeedSchoolID, "trial-other@demo.local")
	otherCookie := signInAs(t, pool, otherStudent)
	for _, method := range []string{"GET", "PUT", "POST"} {
		suffix := ""
		body := `{"revision":1,"document":{"mode":"self"}}`
		if method == "POST" {
			suffix = "/submit"
			body = `{"revision":1}`
		}
		if r := siteReq(t, h, c, method, otherBase+suffix, body); r.Code != 404 {
			t.Fatalf("cross-project %s: %d %s", method, r.Code, r.Body)
		}
		if r := siteReq(t, h, otherCookie, method, base+suffix, body); r.Code != 404 {
			t.Fatalf("cross-user %s: %d %s", method, r.Code, r.Body)
		}
	}
	if r := siteReq(t, h, c, "GET", base, ""); !strings.Contains(r.Body.String(), doc.Version) {
		t.Fatal("draft did not round-trip", r.Body)
	}
	keeps := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/keep", "")
	if strings.TrimSpace(keeps.Body.String()) != "[]" {
		t.Fatal("draft appeared as feedback", keeps.Body)
	}
	// Exercise the actual coach/checker input path, not just the list endpoint.
	if r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"请检查当前材料"}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	for _, request := range provider.Requests {
		raw, _ := json.Marshal(request)
		if strings.Contains(string(raw), doc.Version) {
			t.Fatal("unsubmitted draft leaked into model input")
		}
	}
	r := siteReq(t, h, c, "POST", base+"/submit", `{"revision":1}`)
	if r.Code != http.StatusCreated {
		t.Fatal(r.Code, r.Body)
	}
	var submitted struct {
		Entry struct{ ID, Kind, Stage, Body string }
		Draft struct {
			Revision int
			Document pbl.ArtifactTrial
		}
	}
	if err := json.Unmarshal(r.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	if submitted.Entry.Kind != "thought" || submitted.Entry.Stage != "change" || !strings.Contains(submitted.Entry.Body, "未测试，没有实际结果") || strings.Contains(submitted.Entry.Body, doc.Actual) {
		t.Fatal(r.Body)
	}
	if submitted.Draft.Revision != 2 || submitted.Draft.Document != pbl.EmptyArtifactTrial() {
		t.Fatal("draft not cleared atomically", r.Body)
	}
	// A retry must neither duplicate feedback nor erase a new unfinished attempt.
	doc.Version = "下一次未完成的输入"
	if code := save(2, doc); code != 200 {
		t.Fatal(code)
	}
	retry := siteReq(t, h, c, "POST", base+"/submit", `{"revision":1}`)
	if retry.Code != 200 || !strings.Contains(retry.Body.String(), submitted.Entry.ID) || !strings.Contains(retry.Body.String(), doc.Version) {
		t.Fatal("retry lost new draft or entry identity", retry.Body)
	}
	var entries []json.RawMessage
	keeps = siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/keep", "")
	if err := json.Unmarshal(keeps.Body.Bytes(), &entries); err != nil || len(entries) != 1 {
		t.Fatal("retry duplicated entry", keeps.Body)
	}
	doc.Mode = "other"
	doc.Actual = ""
	if code := save(3, doc); code != 200 {
		t.Fatal(code)
	}
	if r := siteReq(t, h, c, "POST", base+"/submit", `{"revision":4}`); r.Code != 400 {
		t.Fatal("missing result accepted", r.Body)
	}
	if r := siteReq(t, h, c, "GET", base, ""); !strings.Contains(r.Body.String(), doc.Version) {
		t.Fatal("failed submission erased draft", r.Body)
	}
	doc.Task = strings.Repeat("字", 1501)
	if code := save(4, doc); code != 400 {
		t.Fatal("oversized draft accepted", code)
	}
}
