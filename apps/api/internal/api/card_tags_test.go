package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestCardTags_GetPut(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	get := func() map[string]string {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/card-tags", nil), cookie))
		var out struct {
			Tags map[string]string `json:"tags"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out.Tags
	}
	put := func(body string) int {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", base+"/card-tags", strings.NewReader(body)), cookie))
		return rr.Code
	}

	if len(get()) != 0 {
		t.Fatalf("fresh tags should be empty")
	}
	if put(`{"key":"understanding","status":"green"}`) != http.StatusOK {
		t.Fatal("set green failed")
	}
	if put(`{"key":"thesis","status":"bad"}`) != http.StatusBadRequest {
		t.Fatal("invalid status should 400")
	}
	tags := get()
	if tags["understanding"] != "green" {
		t.Fatalf("persisted tags wrong: %+v", tags)
	}
	// clearing a tag
	if put(`{"key":"understanding","status":""}`) != http.StatusOK {
		t.Fatal("clear failed")
	}
	if _, ok := get()["understanding"]; ok {
		t.Fatal("tag should be cleared")
	}
}
