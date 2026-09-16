package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSiteContentRejectsStaleEditorWithoutLosingLatestCopy(t *testing.T) {
	h, c, _, _ := liteHandler(t)
	if r := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", `{"about":["最初的介绍"]}`); r.Code != http.StatusOK {
		t.Fatalf("seed: %s", r.Body)
	}
	initial := decodeSite(t, siteReq(t, h, c, "GET", "/api/v1/pbl/site", ""))["draft"]
	write := func(about string) int {
		body, _ := json.Marshal(map[string]any{"about": []string{about}, "expectedDraft": initial})
		return siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", string(body)).Code
	}
	if status := write("已保存的新介绍"); status != http.StatusOK {
		t.Fatalf("first save: %d", status)
	}
	if status := write("过期页面的介绍"); status != http.StatusConflict {
		t.Fatalf("stale save: %d", status)
	}
	latest := decodeSite(t, siteReq(t, h, c, "GET", "/api/v1/pbl/site", ""))["draft"].(map[string]any)
	if latest["about"].([]any)[0] != "已保存的新介绍" {
		t.Fatalf("overwritten: %v", latest)
	}
}

func TestSiteEditorNewModuleSurvivesApplyingStructure(t *testing.T) {
	h, c, _, _ := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	initial := decodeSite(t, siteReq(t, h, c, "GET", "/api/v1/pbl/site", ""))["draft"]
	body, _ := json.Marshal(map[string]any{"expectedDraft": initial, "sections": []map[string]any{{"key": "editor-local-id", "title": "温室制作", "body": "我尝试让花朵可以点击。", "depth": 0}}})
	r := siteReq(t, h, c, "PUT", "/api/v1/pbl/site/content", string(body))
	if r.Code != http.StatusOK {
		t.Fatalf("save: %s", r.Body)
	}
	saved := decodeSite(t, r)["draft"].(map[string]any)
	key := saved["sections"].([]any)[0].(map[string]any)["key"]
	if key == "editor-local-id" {
		t.Fatal("module not linked to structure")
	}
	// Applying twice must preserve body/key without creating duplicate modules.
	for i := 0; i < 2; i++ {
		r = siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/site-structure", "")
		if r.Code != http.StatusOK {
			t.Fatalf("apply: %s", r.Body)
		}
		sections := decodeSite(t, r)["draft"].(map[string]any)["sections"].([]any)
		found := 0
		for _, item := range sections {
			s := item.(map[string]any)
			if s["key"] == key {
				found++
				if s["body"] != "我尝试让花朵可以点击。" {
					t.Fatal("lost student body")
				}
			}
		}
		if found != 1 {
			t.Fatalf("module count=%d", found)
		}
	}
}
