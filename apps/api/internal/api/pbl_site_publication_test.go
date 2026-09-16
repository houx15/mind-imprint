package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/pbl"
)

func TestPublishedCodeSiteIsReviewedImmutableAndRevocable(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	project := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	page := pbl.SiteContent{Name: "student", About: []string{"I build paper gardens."}}
	html := "<!doctype html><html><body>I build paper gardens.</body></html>"
	create := func(brief map[string]any, document string) string {
		data, _ := json.Marshal(brief)
		var id string
		if err := pool.QueryRow(context.Background(), "INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,1,$2,$3) RETURNING id", project, data, document).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	hero := create(map[string]any{}, "<html><body>only hero</body></html>")
	complete := create(map[string]any{"pageContent": page, "privateNote": "do not publish this note"}, html)
	publish := func(id string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"versionId": id})
		return siteReq(t, h, c, "POST", "/api/v1/pbl/site/publish", string(body))
	}
	if w := publish(hero); w.Code != 409 {
		t.Fatal("hero-only published", w.Code, w.Body)
	}
	if w := publish(complete); w.Code == 200 {
		t.Fatal("unreviewed version published")
	}
	if w := publish(uuid.NewString()); w.Code != 404 {
		t.Fatal("unknown version accepted", w.Code, w.Body)
	}
	direction := map[string]any{"revision": 0, "document": map[string]any{"stage": "hero", "feeling": "gardens", "motifs": []string{"paper"}, "hero": map[string]string{"mode": "code", "scene": "paper garden", "prompt": "show paper garden"}, "trial": map[string]string{"versionId": complete, "observation": "Checked the complete page."}}}
	body, _ := json.Marshal(direction)
	if w := siteReq(t, h, c, "PUT", "/api/v1/pbl/projects/"+project+"/creative-direction", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w := publish(complete)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var token string
	if err := pool.QueryRow(context.Background(), "SELECT share_token FROM pbl_site WHERE atom_id=$1", project).Scan(&token); err != nil {
		t.Fatal(err)
	}
	public := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	url := "/api/v1/public/sites/" + token
	meta := public(url)
	if meta.Code != 200 || !strings.Contains(meta.Body.String(), `"generated":true`) || strings.Contains(meta.Body.String(), "privateNote") || strings.Contains(meta.Body.String(), complete) {
		t.Fatal("public metadata leaked", meta.Code, meta.Body)
	}
	rendered := public(url + "/render")
	if rendered.Code != 200 || rendered.Body.String() != html || rendered.Header().Get("Content-Security-Policy") != pbl.CodePreviewCSP || rendered.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("wrong public rendering", rendered.Code, rendered.Body)
	}
	// Private drafts and newly generated versions must not mutate this publication.
	_ = create(map[string]any{"pageContent": page}, "<html><body>private newer draft</body></html>")
	if _, err := pool.Exec(context.Background(), "UPDATE pbl_site SET content=$2 WHERE atom_id=$1", project, []byte(`{"headline":"private changed draft"}`)); err != nil {
		t.Fatal(err)
	}
	if current := public(url + "/render"); current.Body.String() != html {
		t.Fatal("private changes leaked", current.Body)
	}
	if w := siteReq(t, h, c, "POST", "/api/v1/pbl/site/publish", ""); w.Code != 409 {
		t.Fatal("implicit template switch accepted")
	}
	if w := siteReq(t, h, c, "DELETE", "/api/v1/pbl/site/publish", ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body)
	}
	if w := public(url); w.Code != 404 {
		t.Fatal("revoked metadata visible")
	}
	if w := public(url + "/render"); w.Code != 404 {
		t.Fatal("revoked code visible")
	}
}
