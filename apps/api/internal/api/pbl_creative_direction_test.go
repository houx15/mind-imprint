package api_test

import (
	. "mindimprint/api/internal/api"
	"net/http"
	"testing"
)

func TestCreativeDirectionDraftPreservesIntentAndRejectsStaleWrites(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/creative-direction"
	if r := siteReq(t, h, c, "GET", url, ""); r.Code != 200 {
		t.Fatal(r.Body)
	}
	body := `{"revision":0,"document":{"stage":"motifs","feeling":"像植物生长，又有一点科幻","pendingMotif":"  海底","motifs":["温室","飞船"],"suggestions":["大树","星球"]}}`
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 409 {
		t.Fatal("stale overwrite", r.Body)
	}
	got := decodeSite(t, siteReq(t, h, c, "GET", url, ""))
	doc := got["document"].(map[string]any)
	if doc["pendingMotif"] != "  海底" || len(doc["motifs"].([]any)) != 2 {
		t.Fatal("unfinished input lost or silently selected", got)
	}
	if doc["feeling"] != "像植物生长，又有一点科幻" || doc["motifs"].([]any)[0] != "温室" || got["revision"] != float64(1) {
		t.Fatal(got)
	}
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "creative-other@demo.local"))
	for _, method := range []string{"GET", "PUT"} {
		if r := siteReq(t, h, other, method, url, body); r.Code != http.StatusNotFound {
			t.Fatal("foreign draft", r.Code)
		}
	}
	if r := siteReq(t, h, other, "POST", url+"/motifs", `{"revision":1}`); r.Code != 404 {
		t.Fatal("foreign generation", r.Code)
	}
	if r := siteReq(t, h, c, "PUT", url, `{"document":{"stage":"feeling","feeling":"可爱"}}`); r.Code != 400 {
		t.Fatal("missing revision", r.Code)
	}
}

func TestHeroBriefPersistsAndRefinementRequiresScene(t *testing.T) {
	h, c, _, _ := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/creative-direction"
	body := `{"revision":0,"document":{"stage":"hero","feeling":"植物与科幻","motifs":["太空花园"],"suggestions":[],"hero":{"mode":"mixed","scene":"透明温室漂浮在宇宙里","action":"点击花朵打开作品","prompt":"我写的提示词"}}}`
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 200 {
		t.Fatal(r.Body)
	}
	hero := decodeSite(t, siteReq(t, h, c, "GET", url, ""))["document"].(map[string]any)["hero"].(map[string]any)
	if hero["prompt"] != "我写的提示词" || hero["action"] != "点击花朵打开作品" {
		t.Fatal(hero)
	}
	if r := siteReq(t, h, c, "POST", url+"/hero-prompt", `{"revision":0}`); r.Code != 409 {
		t.Fatal("stale generation", r.Body)
	}
	if r := siteReq(t, h, c, "PUT", url, `{"revision":1,"document":{"stage":"hero","feeling":"科幻","hero":{"mode":"image","scene":"","action":"","prompt":""}}}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r := siteReq(t, h, c, "POST", url+"/hero-prompt", `{"revision":2}`); r.Code != 400 {
		t.Fatal("missing scene accepted", r.Body)
	}
}

func TestHeroTrialMustReferenceOwnedVersionAndSurvivesReload(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	var version string
	if err := pool.QueryRow(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html) VALUES($1,1,'{}','<html><body></body></html>') RETURNING id`, id).Scan(&version); err != nil {
		t.Fatal(err)
	}
	url := "/api/v1/pbl/projects/" + id + "/creative-direction"
	body := `{"revision":0,"document":{"stage":"hero","trial":{"versionId":"` + version + `","observation":"Keyboard opens the work dialog"}}}`
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 200 {
		t.Fatal(r.Body)
	}
	got := decodeSite(t, siteReq(t, h, c, "GET", url, ""))["document"].(map[string]any)["trial"].(map[string]any)
	if got["versionId"] != version || got["observation"] != "Keyboard opens the work dialog" {
		t.Fatal(got)
	}
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 409 {
		t.Fatal("stale trial overwrote newer judgment", r.Code)
	}
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "trial-other@demo.local"))
	otherID := decodePblProject(t, siteReq(t, h, other, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	if r := siteReq(t, h, other, "PUT", "/api/v1/pbl/projects/"+otherID+"/creative-direction", body); r.Code != 404 {
		t.Fatal("foreign trial accepted", r.Code, r.Body)
	}
	if r := siteReq(t, h, c, "PUT", url, `{"revision":1,"document":{"stage":"hero","trial":{"versionId":"`+version+`","observation":" "}}}`); r.Code != 400 {
		t.Fatal("blank rationale accepted", r.Code)
	}
}
