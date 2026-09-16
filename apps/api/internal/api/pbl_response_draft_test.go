package api_test

import "testing"

func TestHeroResponseDraftSurvivesWithoutBecomingTrial(t *testing.T) {
	h, c, _, _ := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/creative-direction"
	body := `{"revision":0,"document":{"stage":"hero","responseDrafts":{"6044b2b2-7e91-4b52-8d6d-257afdda92e0":{"feedback":"  请保留这两个空格","observation":"尚未确认","response":"retain","redrawImage":false}}}}`
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 200 {
		t.Fatal(r.Body)
	}
	doc := decodeSite(t, siteReq(t, h, c, "GET", url, ""))["document"].(map[string]any)
	draft := doc["responseDrafts"].(map[string]any)["6044b2b2-7e91-4b52-8d6d-257afdda92e0"].(map[string]any)
	if draft["feedback"] != "  请保留这两个空格" || draft["observation"] != "尚未确认" || doc["trial"] != nil {
		t.Fatal(doc)
	}
	if r := siteReq(t, h, c, "PUT", url, body); r.Code != 409 {
		t.Fatal("stale response overwrote draft", r.Body)
	}
}
