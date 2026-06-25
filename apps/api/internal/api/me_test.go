package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestMeReturnsSeedUserUnderActAsSeed(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/auth/me", nil))
	if rr.Code != 200 {
		t.Fatalf("me: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{`"display_name":"Phoebe"`, `"school"`, `"classes"`, `"role":"student"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("me body missing %s — %s", want, body)
		}
	}
}
