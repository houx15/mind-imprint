package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// reading_turn_gate_test.go — the gate's two EMPTY-REPLY exits, which are what
// make "check for AI failure BEFORE ApplyReadingGate" the only correct
// ordering. Kept beside reading_turn_test.go rather than inside it because
// this file has exactly one job: stop that ordering from being quietly undone.

// seedCardWith inserts one atom_card in a chosen state — the pacing/ordering
// inputs the gate reads. seedCard (reading_cards_test.go) only ever makes a
// proposed craap; these tests need other ids and other statuses.
func seedCardWith(t *testing.T, pool *pgxpool.Pool, atomID, cardID, blockID, status string) {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status) VALUES ($1, $2, $3, $4)`,
		mustUUID(atomID), cardID, blockID, status); err != nil {
		t.Fatalf("seed card %s/%s: %v", cardID, status, err)
	}
}

// summonScript is a well-formed summon of craap grounded verbatim in b1 — the
// decision the gate is then asked to suppress or soften.
const summonScript = `{"decision":"summon","card_id":"craap",` +
	`"reason":"要不要用 CRAAP 查一下这个十倍是谁统计的？",` +
	`"reply":"你抓到的这个数字确实是全篇的支点。",` +
	`"example_block_id":"b1",` +
	`"example_quote":"中国的太阳能装机量在过去十年增长了十倍。",` +
	`"example_why":"这句给了一个没有出处的数字。"}`

// TestReadingTurn_GateSuppressionIsNotAModelFailure — THE regression guard for
// the subtlest decision in the handler: the AI-failure check runs on the RAW
// decision, BEFORE ApplyReadingGate.
//
// ApplyReadingGate returns the package-level `respond` var — whose Reply is
// EMPTY — on every suppression. So moving the empty-Reply check to after the
// gate would turn every act of restraint into a spurious 502
// ai_dialogue_failed: the student would be told the AI broke precisely when it
// was working as designed. Here an already-open card trips the one-active
// mutex; the turn must still be a normal 200 carrying the model's real answer,
// with no second card minted.
func TestReadingTurn_GateSuppressionIsNotAModelFailure(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(summonScript))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	seedCardWith(t, pool, id, "craap", "b1", "proposed") // one-active mutex

	rec := postTurn(t, h, cookie, id, `{"text":"这个十倍的数字是真的吗"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("suppressed summon = %d, want 200 — a gate suppression is restraint, "+
			"not an AI failure; body=%s", rec.Code, rec.Body)
	}
	var out turnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Decision == "summon" || out.Card != nil {
		t.Fatalf("gate must suppress the summon while a card is open, got %+v", out)
	}
	if out.Reply != "你抓到的这个数字确实是全篇的支点。" {
		t.Fatalf("reply = %q — a downgrade must not swallow the answer the model gave", out.Reply)
	}
	// No SECOND card: the suppressed summon minted nothing.
	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM atom_card WHERE atom_id = $1`, mustUUID(id)).Scan(&n); err != nil {
		t.Fatalf("count cards: %v", err)
	}
	if n != 1 {
		t.Fatalf("cards = %d, want only the seeded one", n)
	}
}

// TestReadingTurn_SoftenedSummonKeepsReplyAndCardID — the gate's OTHER
// empty-Reply exit: within the breathing-room window a summon is SOFTENED to a
// hint, and that hint carries only CardID + Reason (ApplyReadingGate builds a
// fresh ReadingDecision, dropping Reply). So this is the second guard on the
// check-before-the-gate ordering, and the case where forwarding the card id
// matters most — she is being invited to a card the system chose not to open
// for her yet.
func TestReadingTurn_SoftenedSummonKeepsReplyAndCardID(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(summonScript))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	// A just-skipped OTHER card: nothing is open (no mutex) and craap is not in
	// cooldown, but a card was proposed 0 student-turns ago — breathing room.
	seedCardWith(t, pool, id, "sift", "b2", "skipped")

	rec := postTurn(t, h, cookie, id, `{"text":"这个十倍的数字是真的吗"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("softened summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out turnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Decision != "hint" {
		t.Fatalf("decision = %q, want hint (breathing room softens the summon) — %s", out.Decision, rec.Body)
	}
	if out.Card != nil {
		t.Fatalf("a hint proposes no card row, got %+v", out.Card)
	}
	if out.HintCardID == nil || *out.HintCardID != "craap" {
		t.Fatalf("hintCardId = %v, want craap — without it the nudge is a naked "+
			"sentence and the invitation half of 铁律② is lost", out.HintCardID)
	}
	if out.Nudge == "" || out.Reply != "你抓到的这个数字确实是全篇的支点。" {
		t.Fatalf("softened turn = %+v, want the model's reply plus the nudge", out)
	}
}

// TestReadingTurn_HintForwardsTheCardID — the router's own hint path: it is
// told to fill card_id on a hint so the nudge reads as 要不要用它看看. That id
// must reach the client.
func TestReadingTurn_HintForwardsTheCardID(t *testing.T) {
	script := `{"decision":"hint","card_id":"lens-logic",` +
		`"reason":"要不要用逻辑透镜看看这两段之间的推理？",` +
		`"reply":"你其实已经察觉到前后两段没接上。"}`
	h, cookie, id := turnHandler(t, routerStubProvider(script))

	rec := postTurn(t, h, cookie, id, `{"text":"我觉得这两段有点接不上"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("hint turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out turnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Decision != "hint" || out.Card != nil {
		t.Fatalf("turn = %+v, want a hint with no card row", out)
	}
	if out.HintCardID == nil || *out.HintCardID != "lens-logic" {
		t.Fatalf("hintCardId = %v, want lens-logic", out.HintCardID)
	}
	if out.Nudge == "" {
		t.Fatalf("hint must carry the nudge — %s", rec.Body)
	}
}

// TestReadingTurn_RespondCarriesNoHintCardID — the negative: a plain answer
// names no card, so the client renders no invitation.
func TestReadingTurn_RespondCarriesNoHintCardID(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider(`{"decision":"respond","reply":"哪一句让你犹豫？"}`))
	rec := postTurn(t, h, cookie, id, `{"text":"说不上来"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	var out turnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.HintCardID != nil {
		t.Fatalf("respond must name no card, got %v", *out.HintCardID)
	}
}
