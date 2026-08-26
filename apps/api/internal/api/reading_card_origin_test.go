package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// reading_card_origin_test.go — 铁律④: the record is EVIDENCE, and a report is
// generated from it.
//
// Before atom_card.origin (0097), a lens the AI's router proposed and a lens
// the student picked herself out of the 透镜库 wrote byte-identical rows: same
// card_id, same status='proposed', same anchors. That erased precisely the
// autonomy signal the P2 report exists to measure — and it is not
// reconstructible after the fact, so every reading done before the column
// existed is permanently ambiguous.
//
// These two tests pin the two creation sites against each other. They are
// deliberately a matched pair: a fix that hard-codes one value would pass one
// and fail the other.

// getCardsJSON GETs the card list for one reading. The COLUMN is what a later
// report reads; this is how today's client sees the same fact.
func getCardsJSON(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/cards", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET cards = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	return rec
}

// TestSummonCard_RecordsStudentOrigin — she opened the 透镜库 and chose this
// lens herself. That is the whole autonomy signal; it must be on the row.
func TestSummonCard_RecordsStudentOrigin(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(exampleScript))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}

	rec := postSummon(t, h, cookie, id, "craap")
	if rec.Code != http.StatusOK {
		t.Fatalf("summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card == nil {
		t.Fatalf("summon reply = %+v, want a card", out)
	}

	var origin string
	if err := pool.QueryRow(t.Context(),
		`SELECT origin FROM atom_card WHERE id = $1`, mustUUID(out.Card.ID)).Scan(&origin); err != nil {
		t.Fatalf("read origin: %v", err)
	}
	if origin != "student" {
		t.Fatalf("atom_card.origin = %q after a 透镜库 summon, want %q — a lens she chose "+
			"herself and one the AI proposed must not be the same row", origin, "student")
	}

	// …and it has to be visible on the wire too, or the client can never show
	// her which of her lenses were her own idea.
	var listed struct {
		Cards []struct {
			ID     string `json:"id"`
			Origin string `json:"origin"`
		} `json:"cards"`
	}
	lrec := getCardsJSON(t, h, cookie, id)
	if err := json.Unmarshal(lrec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode cards: %v — body=%s", err, lrec.Body)
	}
	if len(listed.Cards) != 1 || listed.Cards[0].Origin != "student" {
		t.Fatalf("cardDTO origin = %+v, want one card with origin \"student\"", listed.Cards)
	}
}

// TestReadingTurn_RecordsRouterOrigin — the other half of the pair: a card the
// coach turn's router proposed on its own initiative is 'router'.
func TestReadingTurn_RecordsRouterOrigin(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(summonScript))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}

	rec := postTurn(t, h, cookie, id, `{"text":"这个十倍的数字是真的吗"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var origin string
	if err := pool.QueryRow(t.Context(),
		`SELECT origin FROM atom_card WHERE atom_id = $1`, mustUUID(id)).Scan(&origin); err != nil {
		t.Fatalf("read origin: %v", err)
	}
	if origin != "router" {
		t.Fatalf("atom_card.origin = %q after a router summon, want %q", origin, "router")
	}
}
