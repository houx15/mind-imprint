package api_test

import (
	"net/http"
	"strings"
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
