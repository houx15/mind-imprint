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

func TestShareReportExplicitlyCreatesMinimalShowcaseWithoutPublishingDraft(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := finishedReadingID(t, h, cookie, q, "公开阅读")
	draftBefore := decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", ""))["draft"]
	rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/readings/"+id+"/report/share", `{"addToShowcase":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("share = %d; body=%s", rec.Code, rec.Body)
	}
	shared := decodeSite(t, rec)
	if shared["showcasePublished"] != true || shared["showcaseUrl"] == "" {
		t.Fatalf("share response = %#v", shared)
	}
	state := decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", ""))
	beforeWithoutSelection := draftBefore.(map[string]any)
	afterDraft := state["draft"].(map[string]any)
	beforeSelected := beforeWithoutSelection["selectedWorkIds"]
	afterSelected := afterDraft["selectedWorkIds"]
	delete(beforeWithoutSelection, "selectedWorkIds")
	delete(afterDraft, "selectedWorkIds")
	if showcaseJSON(t, afterDraft) != showcaseJSON(t, beforeWithoutSelection) {
		t.Fatalf("private draft content changed: before=%#v after=%#v", beforeWithoutSelection, afterDraft)
	}
	if len(beforeSelected.([]any)) != 0 || len(afterSelected.([]any)) != 1 || state["revision"] != float64(1) {
		t.Fatalf("draft selection/revision not updated: selected=%#v revision=%#v", afterSelected, state["revision"])
	}
	public := decodeSite(t, siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+tokenOf(shared["showcaseUrl"].(string)), ""))
	config := public["config"].(map[string]any)
	if config["bio"] != "" || config["heroImageKey"] != nil || config["avatarKey"] != nil || len(public["works"].([]any)) != 1 {
		t.Fatalf("minimal public showcase leaked draft or missed work: %#v", public)
	}
}

func TestShareReportAgainRefreshesPublishedWorkPathWithoutDuplicate(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := finishedReadingID(t, h, cookie, q, "重新分享的阅读")
	firstShare := decodeSite(t, siteReq(t, h, cookie, http.MethodPost, "/api/v1/readings/"+id+"/report/share", `{"addToShowcase":true}`))
	showcaseToken := tokenOf(firstShare["showcaseUrl"].(string))
	firstPublic := decodeSite(t, siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+showcaseToken+"/works", ""))
	firstItems := firstPublic["items"].([]any)
	firstPath := firstItems[0].(map[string]any)["publicPath"].(string)

	if rec := shareReportHTTPDelete(t, h, cookie, "readings", id); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d; body=%s", rec.Code, rec.Body)
	}
	secondShare := siteReq(t, h, cookie, http.MethodPost, "/api/v1/readings/"+id+"/report/share", `{"addToShowcase":true}`)
	if secondShare.Code != http.StatusOK {
		t.Fatalf("re-share = %d; body=%s", secondShare.Code, secondShare.Body)
	}
	secondPublic := decodeSite(t, siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+showcaseToken+"/works", ""))
	secondItems := secondPublic["items"].([]any)
	if len(secondItems) != 1 || secondPublic["total"] != float64(1) {
		t.Fatalf("re-shared work duplicated or missing: %#v", secondPublic)
	}
	secondPath := secondItems[0].(map[string]any)["publicPath"].(string)
	if secondPath == firstPath {
		t.Fatalf("re-share retained revoked public path %q", firstPath)
	}
}

func TestShareReportDoesNotPublishShowcaseByDefault(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := finishedReadingID(t, h, cookie, q, "仅分享报告")
	rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/readings/"+id+"/report/share", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("share = %d; body=%s", rec.Code, rec.Body)
	}
	shared := decodeSite(t, rec)
	if shared["showcasePublished"] != false || shared["showcaseUrl"] != "" {
		t.Fatalf("implicit showcase publication: %#v", shared)
	}
}

func TestPublicShowcaseWorksPaginationAndStaleCursor(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	custom := `"selectedWorkIds":["external:a","external:b","external:c","external:d"],"homeWorkLimit":3,"customWorks":[{"id":"a","title":"A","url":"https://example.org/a"},{"id":"b","title":"B","url":"https://example.org/b"},{"id":"c","title":"C","url":"https://example.org/c"},{"id":"d","title":"D","url":"https://example.org/d"}]`
	body := strings.Replace(validShowcase, `"selectedWorkIds":[]`, custom, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", body); rec.Code != 200 {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	pub := decodeSite(t, siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`))
	if pub["hasUnpublishedChanges"] != false {
		t.Fatalf("featured slice incorrectly left publication dirty: %#v", pub)
	}
	token := tokenOf(pub["url"].(string))
	first := decodeSite(t, siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token+"/works?limit=2", ""))
	if first["total"] != float64(4) || len(first["items"].([]any)) != 2 || first["nextCursor"] == "" {
		t.Fatalf("first page = %#v", first)
	}
	second := decodeSite(t, siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token+"/works?limit=2&cursor="+first["nextCursor"].(string), ""))
	if len(second["items"].([]any)) != 2 {
		t.Fatalf("second page = %#v", second)
	}
	if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`); rec.Code != 200 {
		t.Fatalf("republish = %d %s", rec.Code, rec.Body)
	}
	stale := siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token+"/works?limit=2&cursor="+first["nextCursor"].(string), "")
	if stale.Code != http.StatusBadRequest || !strings.Contains(stale.Body.String(), "stale_cursor") {
		t.Fatalf("stale cursor = %d %s", stale.Code, stale.Body)
	}
}

func TestShowcaseFeaturedWorksAppearWithinHomepageLimit(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	custom := `"selectedWorkIds":["external:a","external:b","external:c","external:d"],"featuredWorkIds":["external:d","external:b"],"homeWorkLimit":3,"customWorks":[{"id":"a","title":"A","url":"https://example.org/a"},{"id":"b","title":"B","url":"https://example.org/b"},{"id":"c","title":"C","url":"https://example.org/c"},{"id":"d","title":"D","url":"https://example.org/d"}]`
	body := strings.Replace(validShowcase, `"selectedWorkIds":[]`, custom, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", body); rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	pub := decodeSite(t, siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/publish", `{"expectedRevision":1}`))
	token := tokenOf(pub["url"].(string))
	page := decodeSite(t, siteReq(t, h, nil, http.MethodGet, "/api/v1/public/sites/"+token, ""))
	items := page["works"].([]any)
	if len(items) != 3 || items[0].(map[string]any)["id"] != "external:d" || items[1].(map[string]any)["id"] != "external:b" || items[2].(map[string]any)["id"] != "external:a" {
		t.Fatalf("featured homepage order = %#v", items)
	}
	if page["worksTotal"] != float64(4) {
		t.Fatalf("all works count changed: %#v", page)
	}
}

func TestShowcaseFeaturedWorksMustBeSelected(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	unselected := strings.Replace(validShowcase, `"selectedWorkIds":[]`, `"selectedWorkIds":[],"featuredWorkIds":["missing"]`, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", unselected); rec.Code != http.StatusBadRequest {
		t.Fatalf("unselected focus = %d %s", rec.Code, rec.Body)
	}
	tooMany := strings.Replace(validShowcase, `"selectedWorkIds":[]`, `"selectedWorkIds":["a","b","c"],"featuredWorkIds":["a","b","c"]`, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", tooMany); rec.Code != http.StatusBadRequest {
		t.Fatalf("too many focus works = %d %s", rec.Code, rec.Body)
	}
}

func TestShowcasePublishUsesSnapshotAndRevokePreservesDraft(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	withPrompts := strings.Replace(validShowcase, `"selectedWorkIds":[]`, `"selectedWorkIds":[],"heroImagePrompt":"  my exact hero request  ","avatarImagePrompt":"my exact avatar request"`, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", withPrompts); rec.Code != http.StatusOK {
		t.Fatalf("save = %d; body=%s", rec.Code, rec.Body)
	}
	authDraft := decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", ""))["draft"].(map[string]any)
	if authDraft["heroImagePrompt"] != "  my exact hero request  " || authDraft["avatarImagePrompt"] != "my exact avatar request" {
		t.Fatalf("authenticated prompts not preserved: %#v", authDraft)
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
	if strings.Contains(public.Body.String(), "ImagePrompt") || strings.Contains(public.Body.String(), "exact hero request") || strings.Contains(public.Body.String(), "exact avatar request") {
		t.Fatalf("private image prompt leaked publicly: %s", public.Body)
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

func TestShowcaseVisualEnumsAndBackwardDefaults(t *testing.T) {
	cases := []struct {
		name, field, value string
		want               int
	}{
		{"style", "style", "anime", 200}, {"illustration", "illustration", "robot", 200}, {"font", "font", "handwritten", 200},
		{"bad style", "style", "glitch", 400}, {"bad illustration", "illustration", "stars", 400}, {"bad font", "font", "comic", 400},
		{"minimal", "style", "minimal", 200}, {"about orbit", "aboutLayout", "orbit", 200}, {"portfolio calendar", "portfolioLayout", "calendar", 200},
		{"bad about", "aboutLayout", "spiral", 400}, {"bad portfolio", "portfolioLayout", "grid3d", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, cookie, _, _ := liteHandler(t)
			body := strings.Replace(validShowcase, `"font":"serif"`, `"font":"serif","`+tc.field+`":"`+tc.value+`"`, 1)
			if tc.field == "font" {
				body = strings.Replace(validShowcase, `"font":"serif"`, `"font":"`+tc.value+`"`, 1)
			}
			rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", body)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body)
			}
		})
	}
	h, cookie, _, _ := liteHandler(t)
	draft := decodeSite(t, siteReq(t, h, cookie, http.MethodGet, "/api/v1/pbl/showcase", ""))["draft"].(map[string]any)
	if draft["style"] != "classic" || draft["illustration"] != "none" || draft["aboutLayout"] != "classic" || draft["portfolioLayout"] != "sections" {
		t.Fatalf("defaults = %#v", draft)
	}
}

func TestShowcaseRejectsForeignImagesAndBoundsGenerationBeforeProvider(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	foreign := strings.Replace(validShowcase, `"selectedWorkIds":[]`, `"selectedWorkIds":[],"avatarKey":"users/00000000-0000-0000-0000-000000000001/images/x.png"`, 1)
	if rec := siteReq(t, h, cookie, http.MethodPut, "/api/v1/pbl/showcase", foreign); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign image = %d; body=%s", rec.Code, rec.Body)
	}
	for _, body := range []string{`{"purpose":"hero","prompt":"x"}`, `{"purpose":"banner","prompt":"一张城市与自然的画"}`} {
		if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/images/generate", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid generation = %d; body=%s", rec.Code, rec.Body)
		}
	}
	tooLong := `{"purpose":"hero","prompt":"` + strings.Repeat("画", 2001) + `"}`
	if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/images/generate", tooLong); rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized prompt = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := siteReq(t, h, cookie, http.MethodPost, "/api/v1/pbl/showcase/images/resolve", `{"objectKey":"users/00000000-0000-0000-0000-000000000001/images/x.png"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign image resolve = %d; body=%s", rec.Code, rec.Body)
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
