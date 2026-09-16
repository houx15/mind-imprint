package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestAudienceDraftPersistsMultipleBoardsAndRejectsStaleWrites(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/audience-board"
	initial := siteReq(t, h, c, "GET", url, "")
	if initial.Code != http.StatusOK || decodeSite(t, initial)["revision"] != float64(0) {
		t.Fatalf("initial: %s", initial.Body)
	}
	body := `{"revision":0,"document":{"step":"interests","activeBoardId":"peer","boards":[{"id":"teacher","role":"老师","person":"美术老师","ageRange":"不确定","interests":["摄影"],"offerings":[]},{"id":"peer","role":"同学","person":"摄影社同学","ageRange":"13至15岁","interests":[],"offerings":[]}]}}`
	saved := siteReq(t, h, c, "PUT", url, body)
	if saved.Code != http.StatusOK || decodeSite(t, saved)["revision"] != float64(1) {
		t.Fatalf("save: %d %s", saved.Code, saved.Body)
	}
	if stale := siteReq(t, h, c, "PUT", url, body); stale.Code != http.StatusConflict {
		t.Fatalf("stale overwrite: %d %s", stale.Code, stale.Body)
	}
	got := decodeSite(t, siteReq(t, h, c, "GET", url, ""))
	doc := got["document"].(map[string]any)
	if doc["activeBoardId"] != "peer" || len(doc["boards"].([]any)) != 2 || got["revision"] != float64(1) {
		t.Fatalf("restore: %+v", got)
	}
	// Saving an incomplete draft must not turn it into a confirmed audience.
	personas := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/personas", "")
	var confirmed []map[string]any
	if personas.Code != http.StatusOK || json.Unmarshal(personas.Body.Bytes(), &confirmed) != nil || len(confirmed) != 0 {
		t.Fatalf("draft leaked to personas: %s", personas.Body)
	}
	otherID := createStudent(t, pool, SeedSchoolID, "other-audience@demo.local")
	other := signInAs(t, pool, otherID)
	for _, method := range []string{"GET", "PUT"} {
		if rec := siteReq(t, h, other, method, url, body); rec.Code != http.StatusNotFound {
			t.Fatalf("foreign %s: %d", method, rec.Code)
		}
	}
	if bad := siteReq(t, h, c, "PUT", url, `{"revision":1,"document":{"step":"roles","boards":[{"id":"a","role":"同学"},{"id":"a","role":"老师"}]}}`); bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid draft: %s", bad.Body)
	}
	if missing := siteReq(t, h, c, "PUT", url, `{"document":{"step":"roles","boards":[]}}`); missing.Code != http.StatusBadRequest {
		t.Fatalf("missing version accepted: %s", missing.Body)
	}
}

func TestAudienceConfirmationPublishesAllBoardsAtomically(t *testing.T) {
	h, c, _, _ := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/audience-board"
	draft := `{"revision":0,"document":{"step":"summary","boards":[{"id":"teacher","role":"老师","person":"美术老师","ageRange":"不确定","interests":["作品过程"],"offerings":["摄影草稿"]},{"id":"peer","role":"同学","person":"摄影社成员","ageRange":"13至15岁","interests":["拍摄地点"],"offerings":["校园路线"]}]}}`
	if r := siteReq(t, h, c, "PUT", url, draft); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r := siteReq(t, h, c, "POST", url+"/confirm", `{"revision":1,"keywords":{"teacher":["摄影"]}}`); r.Code != 400 {
		t.Fatal("partial confirmation accepted", r.Body)
	}
	body := `{"revision":1,"keywords":{"teacher":["摄影","作品过程"],"peer":["摄影","校园路线"]}}`
	if r := siteReq(t, h, c, "POST", url+"/confirm", body); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r := siteReq(t, h, c, "POST", url+"/confirm", body); r.Code != 409 {
		t.Fatal("duplicate confirmation accepted", r.Body)
	}
	r := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/personas", "")
	var rows []map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("lost or duplicated audience: %+v", rows)
	}
	for _, row := range rows {
		if row["chosen"] != true || row["feeling"] != "" {
			t.Fatalf("audience not selected or content conflated with style: %+v", row)
		}
	}
}

func TestAudienceDraftRestoresSummaryAndEditedKeywords(t *testing.T) {
	h, c, _, _ := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/audience-board"
	body := `{"revision":0,"document":{"step":"summary","boards":[{"id":"peer","role":"同学","person":"社团同学","ageRange":"不确定","interests":["摄影"],"offerings":["作品照片"]}],"summary":{"boards":[{"boardId":"peer","keywords":[{"text":"作品","sourceField":"offerings","sourceIndex":0}]}]},"keywords":{"peer":["校园摄影"]}}}`
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 200 {
		t.Fatal(r.Body)
	}
	got := decodeSite(t, siteReq(t, h, c, "GET", url, ""))["document"].(map[string]any)
	if got["keywords"].(map[string]any)["peer"].([]any)[0] != "校园摄影" {
		t.Fatal("student edit lost", got)
	}
	summary := got["summary"].(map[string]any)["boards"].([]any)[0].(map[string]any)
	if summary["keywords"].([]any)[0].(map[string]any)["text"] != "作品" {
		t.Fatal("AI source lost", got)
	}
}

func TestAudienceArchivePersistsWithoutBecomingSelected(t *testing.T) {
	h, c, _, _ := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/audience-board"
	body := `{"revision":0,"document":{"step":"roles","boards":[],"archivedBoards":[{"id":"teacher","role":"teacher","person":"art teacher","ageRange":"unknown","hobbies":["animation"],"interests":["process"],"offerings":["sketchbook"]}]}}`
	if rec := siteReq(t, h, c, "PUT", url, body); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	doc := decodeSite(t, siteReq(t, h, c, "GET", url, ""))["document"].(map[string]any)
	archived := doc["archivedBoards"].([]any)
	if len(doc["boards"].([]any)) != 0 || len(archived) != 1 || archived[0].(map[string]any)["person"] != "art teacher" {
		t.Fatalf("restored: %+v", doc)
	}
	if rec := siteReq(t, h, c, "POST", url+"/confirm", `{"revision":1,"keywords":{}}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("confirmed only archived readers: %d %s", rec.Code, rec.Body)
	}
}
