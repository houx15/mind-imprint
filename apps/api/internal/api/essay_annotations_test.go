package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnnotations_DocScoped — slice 4b: the 批注 endpoints are doc-scoped via
// ?doc=. An essay review persists + lists ESSAY 批注; the proposal path (no ?doc)
// is unchanged and doesn't collide.
func TestAnnotations_DocScoped(t *testing.T) {
	h, cookie := annotationsHandler(t, annotationProvider())
	base := "/api/v1/projects/" + seedProjectID

	// Seed both buffers.
	for _, doc := range []string{"proposal", "essay"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", base+"/buffer?doc="+doc, strings.NewReader(`{"content":"草稿。中国一定会成功。"}`)), cookie))
		if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK {
			t.Fatalf("PUT buffer %s = %d — %s", doc, rr.Code, rr.Body)
		}
	}

	// Review the essay → essay 批注.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/proposal-annotations/review?doc=essay", strings.NewReader("")), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("essay review = %d — %s", rr.Code, rr.Body)
	}
	essay := decodeAnnotations(t, rr.Body.Bytes())
	if len(essay) == 0 {
		t.Fatalf("essay review should produce 批注 — %s", rr.Body)
	}

	// GET essay 批注 → the same set.
	rrGet := httptest.NewRecorder()
	h.ServeHTTP(rrGet, withCookie(httptest.NewRequest("GET", base+"/proposal-annotations?doc=essay", nil), cookie))
	if len(decodeAnnotations(t, rrGet.Body.Bytes())) != len(essay) {
		t.Fatalf("GET essay 批注 mismatch")
	}

	// The PROPOSAL 批注 are still empty (essay review didn't touch them).
	rrProp := httptest.NewRecorder()
	h.ServeHTTP(rrProp, withCookie(httptest.NewRequest("GET", base+"/proposal-annotations", nil), cookie))
	if len(decodeAnnotations(t, rrProp.Body.Bytes())) != 0 {
		t.Fatalf("proposal 批注 should be empty (essay review must not collide)")
	}
}
