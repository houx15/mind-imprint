package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

func TestStudentArtifactEditsPreserveHistoryAndRejectStaleWrites(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"开始"}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	aid := newArtifactViaAPI(t, h, c, pid)
	base := "/api/v1/pbl/projects/" + pid + "/artifacts"
	url := base + "/" + aid + "/text"
	beforeCalls := provider.Calls
	body := "中国的碳排放总量全球第一。人均排放低于美国。"
	post := func(text string) string { b, _ := json.Marshal(map[string]string{"body": text}); return string(b) }
	if r := siteReq(t, h, c, "PUT", url, post(body)); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r := siteReq(t, h, c, "PUT", url, post("  ")); r.Code != 400 {
		t.Fatal(r.Body)
	}
	other := createStudent(t, pool, SeedSchoolID, "student-edit-other@demo.local")
	if r := siteReq(t, h, signInAs(t, pool, other), "PUT", url, post("越权")); r.Code != 404 {
		t.Fatalf("foreign edit: %d %s", r.Code, r.Body)
	}
	otherProject := newProjectViaAPI(t, h, c)
	if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/projects/"+otherProject+"/artifacts/"+aid+"/text", post("跨项目")); r.Code != 404 {
		t.Fatalf("cross-project edit: %d", r.Code)
	}
	// Existing approval stays on the source; it must not silently approve new text.
	if r := siteReq(t, h, c, "POST", base+"/"+aid+"/settle", `{"verdict":"kept","why":"已核对"}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, text := range []string{"学生修改甲\n保留换行", "学生修改乙\n保留换行"} {
		wg.Add(1)
		go func(text string) { defer wg.Done(); codes <- siteReq(t, h, c, "PUT", url, post(text)).Code }(text)
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent writes: %v", counts)
	}
	var rows []struct {
		ID         string
		Superseded bool
		Verdict    *string
		Payload    struct {
			Body, PreviousBody, ReplacesArtifactID string
			EditedByStudent                        bool
		}
	}
	r := siteReq(t, h, c, "GET", base, "")
	if err := json.Unmarshal(r.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Payload.Body != body || !rows[0].Superseded || rows[0].Verdict == nil || *rows[0].Verdict != "kept" {
		t.Fatalf("history: %s", r.Body)
	}
	if rows[1].Payload.PreviousBody != body || rows[1].Payload.ReplacesArtifactID != aid || !rows[1].Payload.EditedByStudent || rows[1].Verdict != nil {
		t.Fatalf("new source/status: %s", r.Body)
	}
	if r := siteReq(t, h, c, "PUT", url, post("过期覆盖")); r.Code != 409 {
		t.Fatal(r.Body)
	}
	if provider.Calls != beforeCalls {
		t.Fatalf("student edits called model: %d -> %d", beforeCalls, provider.Calls)
	}
}

func TestStudentArtifactEditRejectsDerivedLayoutsAndNonText(t *testing.T) {
	h, c, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, c)
	for _, fields := range []string{`"kind":"site","payload":{"body":"站点"}`, `"kind":"draft","payload":{"body":"图形","paperLayout":{}}`, `"kind":"spec","payload":{"body":"折页","printLayout":{}}`} {
		r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/artifacts", fmt.Sprintf(`{%s,"title":"测试","guessed":["假设"],"admits":["未测试"]}`, fields))
		if r.Code != http.StatusCreated {
			t.Fatal(r.Body)
		}
		aid := artifactID(t, r.Body.String())
		if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/text", `{"body":"改动"}`); r.Code != 400 {
			t.Fatalf("derived layout accepted: %d %s", r.Code, r.Body)
		}
	}
}
