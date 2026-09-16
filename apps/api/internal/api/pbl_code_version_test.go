package api_test

import (
	"context"
	"encoding/json"
	. "mindimprint/api/internal/api"
	"strings"
	"testing"
)

func TestCodeRevisionRejectsIncompleteFeedbackAndForeignParent(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id
	brief := `{"revision":0,"document":{"stage":"hero","feeling":"科幻","motifs":["温室"],"hero":{"mode":"code","scene":"太空温室","prompt":"画出温室"}}}`
	if r := siteReq(t, h, c, "PUT", url+"/creative-direction", brief); r.Code != 200 {
		t.Fatal(r.Body)
	}
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "revision-other@demo.local"))
	otherID := decodePblProject(t, siteReq(t, h, other, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	var foreign string
	if err := pool.QueryRow(context.Background(), "INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,1,'{}','<html><body></body></html>') RETURNING id", otherID).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		base, feedback string
		status         int
	}{
		{"", "放大花朵", 400}, {foreign, " ", 400}, {foreign, strings.Repeat("花", 2001), 400}, {"bad-id", "放大", 400}, {foreign, "放大花朵", 404},
	} {
		body, _ := json.Marshal(map[string]any{"revision": 1, "baseVersion": tc.base, "feedback": tc.feedback})
		if r := siteReq(t, h, c, "POST", url+"/code-versions", string(body)); r.Code != tc.status {
			t.Fatalf("got %d want %d: %s", r.Code, tc.status, r.Body)
		}
	}
	if _, err := pool.Exec(context.Background(), "INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html,parent_version_id,feedback) VALUES($1,1,'{}','<html><body></body></html>',$2,'放大')", id, foreign); err == nil {
		t.Fatal("database accepted parent from another project")
	}
}

func TestCodePreviewOwnershipHeadersAndVersionHistory(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	var version string
	html := "<!doctype html><html><body><button onclick=\"this.textContent='changed'\">test</button></body></html>"
	if err := pool.QueryRow(context.Background(), "INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,1,'{}',$2) RETURNING id", id, html).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,2,'{"hero":{"mode":"mixed","prompt":"PRIVATE_PROMPT"},"heroImage":{"key":"PRIVATE_ASSET"}}',$2)`, id, html); err != nil {
		t.Fatal(err)
	}
	url := "/api/v1/pbl/projects/" + id + "/code-versions/" + version + "/preview"
	r := siteReq(t, h, c, "GET", url, "")
	if r.Code != 200 || r.Body.String() != html {
		t.Fatal(r.Code, r.Body)
	}
	csp := r.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"sandbox allow-scripts", "default-src 'none'", "connect-src 'none'", "form-action 'none'", "base-uri 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Fatal("missing isolation", csp)
		}
	}
	if strings.Contains(csp, "allow-same-origin") || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe preview headers")
	}
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "code-other@demo.local"))
	if r := siteReq(t, h, other, "GET", url, ""); r.Code != 404 {
		t.Fatal("foreign preview", r.Code)
	}
	list := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/code-versions", "")
	if strings.Count(list.Body.String(), "brief_revision") != 2 || strings.Contains(list.Body.String(), "<html") {
		t.Fatal("history lost or listing leaked code", list.Body)
	}
	var versions []map[string]any
	if err := json.Unmarshal(list.Body.Bytes(), &versions); err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0]["mode"] != "mixed" || versions[1]["mode"] != nil {
		t.Fatal("version mode must come from its snapshot, legacy mode stays unknown", list.Body)
	}
	if strings.Contains(list.Body.String(), "PRIVATE_") {
		t.Fatal("brief internals leaked", list.Body)
	}
}
