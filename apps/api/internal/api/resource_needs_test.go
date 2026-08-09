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

func TestResourceNeeds_GetPut(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// Fresh GET → empty list.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/resource-needs", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET = %d — %s", rr.Code, rr.Body)
	}
	var got struct {
		Needs []struct {
			ID, Text string
			Done     bool
		} `json:"needs"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Needs) != 0 {
		t.Fatalf("fresh needs should be empty: %+v", got)
	}

	// PUT a list — blanks dropped, missing ids minted.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", base+"/resource-needs",
		strings.NewReader(`{"needs":[{"id":"","text":"中国碳排放数据","done":false},{"id":"x","text":"  ","done":false},{"id":"y","text":"NASA 卫星","done":true}]}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT = %d — %s", rr.Code, rr.Body)
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Needs) != 2 {
		t.Fatalf("expected 2 needs (blank dropped), got %+v", got)
	}
	if got.Needs[0].ID == "" {
		t.Fatalf("missing id should be minted: %+v", got.Needs[0])
	}
	if got.Needs[1].Text != "NASA 卫星" || !got.Needs[1].Done {
		t.Fatalf("second need wrong: %+v", got.Needs[1])
	}

	// GET again → persisted.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/resource-needs", nil), cookie))
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Needs) != 2 {
		t.Fatalf("persisted needs wrong: %+v", got)
	}
}
