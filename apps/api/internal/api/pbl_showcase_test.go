package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const validShowcase = `{"draft":{"name":"小雨","bio":"我关注城市与自然的关系。","tagline":"把观察做成作品","interests":["城市","生态"],"layout":"studio","palette":"forest","font":"serif","writingStyle":"cards","readingStyle":"shelf","sectionOrder":["project","writing","reading"],"selectedWorkIds":[]},"expectedRevision":0}`

func TestShowcaseWorksWithoutHomepageProjectAndChecksRevision(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	initial := siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", "")
	if initial.Code != http.StatusOK {
		t.Fatalf("get showcase = %d; body=%s", initial.Code, initial.Body)
	}
	got := decodeSite(t, initial)
	if got["revision"] != float64(0) || got["published"] != false {
		t.Fatalf("initial state = %#v", got)
	}
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", validShowcase); rec.Code != http.StatusOK {
		t.Fatalf("save = %d; body=%s", rec.Code, rec.Body)
	}
	stale := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", validShowcase)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale save = %d, want 409; body=%s", stale.Code, stale.Body)
	}
}

func TestShowcasePublishUsesSnapshotAndRevokePreservesDraft(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", validShowcase); rec.Code != http.StatusOK {
		t.Fatalf("save = %d; body=%s", rec.Code, rec.Body)
	}
	pub := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`)
	if pub.Code != http.StatusOK {
		t.Fatalf("publish = %d; body=%s", pub.Code, pub.Body)
	}
	state := decodeSite(t, pub)
	if state["published"] != true || state["url"] == "" || state["hasUnpublishedChanges"] != false {
		t.Fatalf("published state = %#v", state)
	}
	token := tokenOf(state["url"].(string))
	public := siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token, "")
	if public.Code != http.StatusOK || !strings.Contains(public.Body.String(), `"showcase":true`) || !strings.Contains(public.Body.String(), `"layout":"studio"`) {
		t.Fatalf("public showcase = %d; body=%s", public.Code, public.Body)
	}

	changed := strings.Replace(validShowcase, `"expectedRevision":0`, `"expectedRevision":1`, 1)
	changed = strings.Replace(changed, `"palette":"forest"`, `"palette":"ocean"`, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", changed); rec.Code != http.StatusOK {
		t.Fatalf("save changed = %d; body=%s", rec.Code, rec.Body)
	}
	public = siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token, "")
	if strings.Contains(public.Body.String(), `"palette":"ocean"`) || !strings.Contains(public.Body.String(), `"palette":"forest"`) {
		t.Fatalf("save changed public snapshot: %s", public.Body)
	}

	revoked := siteReq(t, h, cookie, http.MethodDelete, "/api/v1/pbl/showcase/publish", "")
	if revoked.Code != http.StatusOK || decodeSite(t, revoked)["published"] != false {
		t.Fatalf("revoke = %d; body=%s", revoked.Code, revoked.Body)
	}
	if rec := siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("old token after revoke = %d; body=%s", rec.Code, rec.Body)
	}
}

func TestShowcaseOnlyOffersSharedWorkAndRevocationRemovesIt(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	privateID := finishedReadingID(t, h, cookie, q, "私人阅读")
	state := decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", ""))
	if strings.Contains(showcaseJSON(t, state["availableWorks"]), "私人阅读") {
		t.Fatal("private reading appeared in availableWorks")
	}
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", strings.Replace(validShowcase, `"selectedWorkIds":[]`, `"selectedWorkIds":["`+privateID+`"]`, 1)); rec.Code != http.StatusBadRequest {
		t.Fatalf("private selection = %d; body=%s", rec.Code, rec.Body)
	}

	share := shareReportHTTP(t, h, cookie, "readings", privateID)
	if share.Code != http.StatusOK {
		t.Fatalf("share reading = %d; body=%s", share.Code, share.Body)
	}
	state = decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", ""))
	works := state["availableWorks"].([]any)
	if len(works) != 1 {
		t.Fatalf("available works = %#v", works)
	}
	workID := works[0].(map[string]any)["id"].(string)
	body := strings.Replace(validShowcase, `"selectedWorkIds":[]`, `"selectedWorkIds":["`+workID+`"]`, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", body); rec.Code != 200 {
		t.Fatalf("save selected = %d; body=%s", rec.Code, rec.Body)
	}
	pub := decodeSite(t, siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`))
	token := tokenOf(pub["url"].(string))
	if rec := siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token, ""); !strings.Contains(rec.Body.String(), "私人阅读") {
		t.Fatalf("shared work absent: %s", rec.Body)
	}
	if rec := shareReportHTTPDelete(t, h, cookie, "readings", privateID); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke work = %d; body=%s", rec.Code, rec.Body)
	}
	public := siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token, "")
	if strings.Contains(public.Body.String(), "私人阅读") || strings.Contains(public.Body.String(), workID) {
		t.Fatalf("revoked work leaked: %s", public.Body)
	}
}

func TestShowcasePreservesLegacyOwnerStableURLAndRejectsLegacyPublisher(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	project := decodePblProject(t, siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/site/project", ""))["id"].(string)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", validShowcase); rec.Code != 200 {
		t.Fatalf("save = %d; body=%s", rec.Code, rec.Body)
	}
	first := decodeSite(t, siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`))
	url := first["url"].(string)
	legacy := decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/site", ""))
	if legacy["projectId"] != project {
		t.Fatalf("legacy owner changed: %v want %s", legacy["projectId"], project)
	}
	if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/site/publish", ""); rec.Code != http.StatusConflict {
		t.Fatalf("legacy publish = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/site/publish", `{"versionId":"bad"}`); rec.Code != http.StatusConflict {
		t.Fatalf("legacy code publish = %d; body=%s", rec.Code, rec.Body)
	}
	if again := decodeSite(t, siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`)); again["url"] != url {
		t.Fatalf("republish changed live URL: %v != %s", again["url"], url)
	}
	if rec := siteReq(t, h, cookie, http.MethodDelete, "/api/v1/pbl/showcase/publish", ""); rec.Code != 200 {
		t.Fatalf("revoke = %d", rec.Code)
	}
	if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/site/publish", ""); rec.Code != http.StatusConflict {
		t.Fatalf("legacy publish after revoke = %d", rec.Code)
	}
	if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`); rec.Code != 200 {
		t.Fatalf("showcase republish after revoke = %d; body=%s", rec.Code, rec.Body)
	}
}

func TestShowcaseConcurrentSaveAndPublishNeverLoseSnapshot(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", validShowcase); rec.Code != 200 {
		t.Fatalf("save = %d", rec.Code)
	}
	changed := strings.Replace(strings.Replace(validShowcase, `"expectedRevision":0`, `"expectedRevision":1`, 1), `"palette":"forest"`, `"palette":"ocean"`, 1)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	bodies := make(chan string, 2)
	for _, req := range []struct{ method, body string }{{http.MethodPut, changed}, {http.MethodPost, `{"expectedRevision":1}`}} {
		wg.Add(1)
		go func(method, body string) {
			defer wg.Done()
			rec := siteReq(t, h, cookie, method, map[bool]string{true: "/api/v1/pbl/showcase/publish", false: "/api/v1/pbl/showcase"}[method == http.MethodPost], body)
			codes <- rec.Code
			bodies <- rec.Body.String()
		}(req.method, req.body)
	}
	wg.Wait()
	close(codes)
	close(bodies)
	for code := range codes {
		if code != 200 && code != 409 {
			t.Fatalf("concurrent status = %d; bodies=%v", code, bodies)
		}
	}
	state := decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", ""))
	if state["revision"] != float64(2) {
		t.Fatalf("revision = %v", state["revision"])
	}
	if state["published"] == true {
		token := tokenOf(state["url"].(string))
		public := siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token, "")
		if !strings.Contains(public.Body.String(), `"palette":"forest"`) {
			t.Fatalf("published snapshot lost: %s", public.Body)
		}
	}
}

func showcaseJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func shareReportHTTPDelete(t *testing.T, h http.Handler, cookie *http.Cookie, base, id string) *httptest.ResponseRecorder {
	t.Helper()
	return siteReq(t, h, cookie, http.MethodDelete, "/api/v1/"+base+"/"+id+"/report/share", "")
}
