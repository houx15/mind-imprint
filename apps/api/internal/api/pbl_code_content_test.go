package api_test

import (
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/pbl"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCodeCompositionUsesSavedPageContent(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"html":"<html><body>Saved introduction</body></html>"}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_creative_direction(atom_id,revision,document) VALUES($1,1,'{"stage":"hero","feeling":"space","motifs":["garden"],"hero":{"mode":"code","scene":"garden","prompt":"Make the garden"}}')`, id); err != nil {
		t.Fatal(err)
	}
	var base string
	if err := pool.QueryRow(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,1,'{}','<html><body>Existing hero</body></html>') RETURNING id`, id).Scan(&base); err != nil {
		t.Fatal(err)
	}
	url := "/api/v1/pbl/projects/" + id + "/code-versions"
	body := `{"revision":1,"baseVersion":"` + base + `","feedback":"Add saved content","includeContent":true}`
	if r := siteReq(t, h, c, "POST", url, body); r.Code != 400 {
		t.Fatal("empty content accepted", r.Code, r.Body)
	}
	if provider.Calls != 0 {
		t.Fatal("empty content consumed generation")
	}
	if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"about":["Saved introduction"]}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	r := siteReq(t, h, c, "POST", url, body)
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body)
	}
	input, _ := json.Marshal(provider.LastRequest)
	if !strings.Contains(string(input), "Saved introduction") || !strings.Contains(string(input), "Existing hero") {
		t.Fatal("missing real content or base code")
	}
	var result struct{ ID string }
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var snapshot string
	if err := pool.QueryRow(t.Context(), `SELECT brief->'pageContent'->'about'->>0 FROM pbl_code_version WHERE id=$1`, result.ID).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot != "Saved introduction" {
		t.Fatal("lost source snapshot", snapshot)
	}
}

func TestCodeCompositionCanUseOnlySavedProcess(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"html":"<html><body><h2>修改意见</h2><p>Make flowers larger.</p><h2>试用判断</h2><p>Keyboard works. Touch untested.</p></body></html>"}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	var base string
	if err := pool.QueryRow(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html,feedback) VALUES($1,1,'{}','<html><body>Existing hero</body></html>','Make flowers larger.') RETURNING id`, id).Scan(&base); err != nil {
		t.Fatal(err)
	}
	doc := `{"stage":"hero","feeling":"space","motifs":["garden"],"hero":{"mode":"code","scene":"garden","prompt":"Make a garden"},"includeProcess":true,"includeComparison":true,"trial":{"versionId":"` + base + `","observation":"Keyboard works. Touch untested."}}`
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_creative_direction(atom_id,revision,document) VALUES($1,1,$2)`, id, doc); err != nil {
		t.Fatal(err)
	}
	request := `{"revision":1,"baseVersion":"` + base + `","feedback":"Add saved process","includeContent":true}`
	endpoint := "/api/v1/pbl/projects/" + id + "/code-versions"
	if r := siteReq(t, h, c, "POST", endpoint, request); r.Code != 400 || provider.Calls != 0 {
		t.Fatal("comparison without parent must fail before model call", r.Code, r.Body)
	}
	var before string
	if err := pool.QueryRow(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,1,'{}','<html><body>Small flowers</body></html>') RETURNING id`, id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE pbl_code_version SET parent_version_id=$1 WHERE id=$2`, before, base); err != nil {
		t.Fatal(err)
	}
	r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/code-versions", `{"revision":1,"baseVersion":"`+base+`","feedback":"Add saved process","includeContent":true}`)
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body)
	}
	var result struct{ ID string }
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var snapshot string
	if err := pool.QueryRow(t.Context(), `SELECT brief->'pageContent'->'sections'->0->>'body' FROM pbl_code_version WHERE id=$1`, result.ID).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snapshot, "Make flowers larger.") || !strings.Contains(snapshot, "Keyboard works. Touch untested.") {
		t.Fatal("lost recorded input", snapshot)
	}
	var pair struct{ BeforeVersionID, AfterVersionID, Feedback, Observation string }
	var pairJSON []byte
	if err := pool.QueryRow(t.Context(), `SELECT brief->'processComparison' FROM pbl_code_version WHERE id=$1`, result.ID).Scan(&pairJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(pairJSON, &pair); err != nil {
		t.Fatal(err)
	}
	if pair.BeforeVersionID != before || pair.AfterVersionID != base || pair.Feedback != "Make flowers larger." || pair.Observation != "Keyboard works. Touch untested." {
		t.Fatal("wrong comparison snapshot", pair)
	}
	// The process is the beginner's actual work. It can be published only
	// after the newly composed page, not just its earlier hero, is reviewed.
	publishBody := `{"versionId":"` + result.ID + `"}`
	if r := siteReq(t, h, c, "POST", "/api/v1/pbl/site/publish", publishBody); r.Code != 409 {
		t.Fatal("unreviewed composition should be blocked", r.Code, r.Body)
	}
	doc = strings.Replace(doc, `"versionId":"`+base+`"`, `"versionId":"`+result.ID+`"`, 1)
	if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/projects/"+id+"/creative-direction", `{"revision":1,"document":`+doc+`}`); r.Code != 200 {
		t.Fatal("retain composed page", r.Code, r.Body)
	}
	var still []byte
	if err := pool.QueryRow(t.Context(), `SELECT brief->'processComparison' FROM pbl_code_version WHERE id=$1`, result.ID).Scan(&still); err != nil {
		t.Fatal(err)
	}
	if string(still) != string(pairJSON) {
		t.Fatal("retaining a new version changed the recorded pair")
	}
	if r := siteReq(t, h, c, "POST", "/api/v1/pbl/site/publish", publishBody); r.Code != 200 {
		t.Fatal("reviewed process-only page should publish", r.Code, r.Body)
	}
	var published string
	if err := pool.QueryRow(t.Context(), `SELECT version_id FROM pbl_site_publication WHERE atom_id=$1`, id).Scan(&published); err != nil || published != result.ID {
		t.Fatal("wrong published process version", published, err)
	}
	var token string
	if err := pool.QueryRow(t.Context(), `SELECT share_token FROM pbl_site WHERE atom_id=$1`, id).Scan(&token); err != nil {
		t.Fatal(err)
	}
	public := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	url := "/api/v1/public/sites/" + token
	meta := public(url)
	if meta.Code != 200 || !strings.Contains(meta.Body.String(), "Make flowers larger.") || strings.Contains(meta.Body.String(), before) || strings.Contains(meta.Body.String(), base) {
		t.Fatal("wrong public process metadata", meta.Body)
	}
	var metadata struct{ RenderKey string }
	if err := json.Unmarshal(meta.Body.Bytes(), &metadata); err != nil || metadata.RenderKey == "" {
		t.Fatal("missing render key", err)
	}
	for _, tc := range []struct{ side, body string }{{"before", "Small flowers"}, {"after", "Existing hero"}} {
		path := url + "/render?publication=" + metadata.RenderKey + "&comparison=" + tc.side
		w := public(path)
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.body) || w.Header().Get("Content-Security-Policy") != pbl.CodePreviewCSP {
			t.Fatal("wrong comparison", tc.side, w.Code, w.Body)
		}
		private := siteReq(t, h, c, "GET", endpoint+"/"+result.ID+"/preview?comparison="+tc.side, "")
		if private.Code != 200 || private.Body.String() != w.Body.String() {
			t.Fatal("owner/public mismatch", private.Code)
		}
	}
	for _, suffix := range []string{"?comparison=" + before, "?comparison=before&publication=" + base} {
		if w := public(url + "/render" + suffix); w.Code != 404 {
			t.Fatal("arbitrary side or stale publication accepted", w.Code)
		}
	}
	if w := public("/api/v1/pbl/projects/" + id + "/code-versions/" + before + "/preview"); w.Code == 200 {
		t.Fatal("private history exposed")
	}
	if w := siteReq(t, h, c, "DELETE", "/api/v1/pbl/site/publish", ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	for _, suffix := range []string{"", "/render?comparison=before", "/render?comparison=after"} {
		if w := public(url + suffix); w.Code != 404 {
			t.Fatal("revoked comparison visible", w.Code)
		}
	}
}

func TestCodeRevisionKeepsSelectedVersionsContentSnapshot(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"html":"<html><body>Original student content</body></html>"}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_creative_direction(atom_id,revision,document) VALUES($1,1,'{"stage":"hero","feeling":"space","motifs":["garden"],"hero":{"mode":"code","scene":"garden","prompt":"Make a garden"}}')`, id); err != nil {
		t.Fatal(err)
	}
	var base string
	if err := pool.QueryRow(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,1,'{"pageContent":{"about":["Original student content"]}}','<html><body>Original student content</body></html>') RETURNING id`, id).Scan(&base); err != nil {
		t.Fatal(err)
	}
	if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"about":["Later unrelated content"]}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/code-versions", `{"revision":1,"baseVersion":"`+base+`","feedback":"Make the animation slower"}`)
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body)
	}
	input, _ := json.Marshal(provider.LastRequest)
	if strings.Contains(string(input), "Later unrelated content") || !strings.Contains(string(input), "pageContent") {
		t.Fatal("selected snapshot was lost or replaced")
	}
	var result struct{ ID string }
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	var text, parent string
	if err := pool.QueryRow(t.Context(), `SELECT brief->'pageContent'->'about'->>0,parent_version_id FROM pbl_code_version WHERE id=$1`, result.ID).Scan(&text, &parent); err != nil {
		t.Fatal(err)
	}
	if text != "Original student content" || parent != base {
		t.Fatal(text, parent)
	}
}
