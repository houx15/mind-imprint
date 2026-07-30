package api_test

// coach_link_integration_test.go — the link-in-coach bridge end to end through
// the real /coach handler: a URL in the student's turn attaches a linkOffer; a
// URL already in the library does not (dedup). Reuses the coach_proposal_test
// harness (momentReplyProvider/"none" → a benign reply + no card proposal).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostCoach_AttachesLinkOfferForNewURL(t *testing.T) {
	h, cookie, _ := coachProposalHandler(t, "none")
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"forming","user_input":"我看到这篇，觉得有意思 https://nature.com/articles/greening。"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Reply     string `json:"reply"`
		LinkOffer *struct {
			URL string `json:"url"`
		} `json:"linkOffer"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.LinkOffer == nil {
		t.Fatalf("want a linkOffer for a new URL, got none — %s", rr.Body)
	}
	if resp.LinkOffer.URL != "https://nature.com/articles/greening" {
		t.Fatalf("linkOffer.url = %q (trailing punctuation should be trimmed)", resp.LinkOffer.URL)
	}
}

func TestPostCoach_NoLinkOfferWhenNoURL(t *testing.T) {
	h, cookie, _ := coachProposalHandler(t, "none")
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"forming","user_input":"我还没想好研究什么"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), `"linkOffer"`) {
		t.Fatalf("no URL → no linkOffer, got: %s", rr.Body)
	}
}

func TestPostCoach_NoLinkOfferWhenURLAlreadyReference(t *testing.T) {
	h, cookie, _ := coachProposalHandler(t, "none")
	base := "/api/v1/projects/" + seedProjectID

	// The student already added this source to the library.
	rrRef := httptest.NewRecorder()
	h.ServeHTTP(rrRef, withCookie(httptest.NewRequest("POST", base+"/references",
		strings.NewReader(`{"title":"OWID","url":"https://ourworldindata.org/co2"}`)), cookie))
	if rrRef.Code != http.StatusOK && rrRef.Code != http.StatusCreated {
		t.Fatalf("create reference = %d — %s", rrRef.Code, rrRef.Body)
	}

	// Mentioning the same URL in coach must NOT re-offer it.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"forming","user_input":"关于 https://ourworldindata.org/co2 我有个疑问"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), `"linkOffer"`) {
		t.Fatalf("URL already in library → no linkOffer, got: %s", rr.Body)
	}
}
