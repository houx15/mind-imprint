package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedGroundedCard inserts a proposed card that already carries an anchor —
// the state the AI leaves a card in when it grounds the lens on a real
// sentence in her article. seedCard (reading_cards_test.go) leaves anchors at
// their default, which cannot prove anything about protecting them.
func seedGroundedCard(t *testing.T, pool *pgxpool.Pool, atomID, blockID string) string {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`UPDATE atom_card SET status = 'skipped'
		 WHERE atom_id = $1 AND status IN ('proposed','active')`,
		mustUUID(atomID)); err != nil {
		t.Fatalf("retire open cards: %v", err)
	}
	var id string
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status, anchors)
		 VALUES ($1, 'craap', $2, 'proposed',
		         '[{"blockId":"b1","quote":"AI 挑出来的那句原文"}]'::jsonb)
		 RETURNING id::text`,
		mustUUID(atomID), blockID).Scan(&id); err != nil {
		t.Fatalf("seed grounded card: %v", err)
	}
	return id
}

func cardAnchorsJSON(t *testing.T, pool *pgxpool.Pool, cardID string) string {
	t.Helper()
	var raw string
	if err := pool.QueryRow(t.Context(),
		`SELECT anchors::text FROM atom_card WHERE id = $1`, mustUUID(cardID)).Scan(&raw); err != nil {
		t.Fatalf("read anchors: %v", err)
	}
	return raw
}

// TestSubmitCard_EmptyAnchorsDoesNotClearGrounding — a submit carrying
// `anchors: []` must LEAVE the card's AI-grounded anchor alone.
//
// Why this is a real trap and not a hypothetical. The shared CardInstance
// envelope always carries an `anchors` key, empty when the client never
// touched it. Submitting used to treat "present" as "replace", so any client
// forwarding that envelope verbatim — the obvious way to write the next
// card-submitting caller, by copying the previous one — silently destroyed the
// grounding the AI had attached: no error, no symptom, until a student's card
// had lost the sentence it was hanging on.
//
// The lite writing client hit this and worked around it by omitting the key
// entirely. That works, but it rests on every future author knowing that a
// JSON key's PRESENCE rather than its value carries meaning — an invariant no
// type or compiler enforces. So the rule now lives on the server instead.
func TestSubmitCard_EmptyAnchorsDoesNotClearGrounding(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedGroundedCard(t, pool, atomID, "b1")

	before := cardAnchorsJSON(t, pool, cardID)
	if !strings.Contains(before, "AI 挑出来的那句原文") {
		t.Fatalf("precondition: card has no grounding to protect (%s)", before)
	}

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"fieldValues":{"currency":"2024"},"eventTrace":[],"anchors":[]}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	after := cardAnchorsJSON(t, pool, cardID)
	if !strings.Contains(after, "AI 挑出来的那句原文") {
		t.Fatalf("submitting anchors:[] wiped the card's grounding: %s → %s", before, after)
	}
}

// TestSubmitCard_NonEmptyAnchorsStillReplaces — the other half. Re-anchoring
// is a real action (she picks her own sentence instead of the AI's example),
// so the protection above must not become "ignore anchors entirely".
func TestSubmitCard_NonEmptyAnchorsStillReplaces(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)
	cardID := seedGroundedCard(t, pool, atomID, "b1")

	rec := httptest.NewRecorder()
	body := strings.NewReader(
		`{"fieldValues":{"currency":"2024"},"eventTrace":[],"anchors":[{"blockId":"b9","quote":"她自己挑的句子"}]}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+atomID+"/cards/"+cardID+"/submit", body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	after := cardAnchorsJSON(t, pool, cardID)
	if !strings.Contains(after, "她自己挑的句子") {
		t.Fatalf("anchors = %s after a non-empty submit; want her own pick to have replaced the AI's", after)
	}
	if strings.Contains(after, "AI 挑出来的那句原文") {
		t.Fatalf("anchors = %s; her pick should REPLACE the AI's example, not accumulate alongside it", after)
	}
}
