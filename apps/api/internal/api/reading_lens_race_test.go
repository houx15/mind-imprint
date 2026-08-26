package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/gateway"
)

// reading_lens_race_test.go — 一次只开一副透镜, as a fact rather than a habit.
//
// Both write paths that can open a lens (the router's summon in
// postLiteReadingTurn, and the student's own in liteSummonCard) used to be a
// read-then-insert with nothing in between: scan for an open card, call a
// model for a couple of seconds, insert. Two requests could both pass the scan
// and both insert. The damage is not "two cards" — it is that getOpenCard
// returns only the FIRST, so the second is invisible to the student while
// still blocking every future summon. She gets told 先完成当前这副透镜 about a
// lens she cannot see until she reloads.
//
// 0096's partial unique index is the fix; these are its guards.

// raceProvider opens a competing lens WHILE the model call is in flight —
// exactly the window neither handler's own check could cover. Deterministic:
// the insert happens inside Stream, which both handlers call after their scan
// and before their insert. No goroutines, no sleeps, no flake.
type raceProvider struct {
	t      *testing.T
	pool   *pgxpool.Pool
	atomID string
	inner  gateway.Provider
	once   sync.Once
}

func (p *raceProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.once.Do(func() {
		// A DIFFERENT card id, so any assertion about which one survived is
		// unambiguous. context.Background(), not the request's: this insert
		// stands in for another request entirely.
		if _, err := p.pool.Exec(context.Background(),
			`INSERT INTO atom_card (atom_id, card_id, block_id, status) VALUES ($1, 'sift', 'b2', 'proposed')`,
			mustUUID(p.atomID)); err != nil {
			p.t.Errorf("race seed: %v", err)
		}
	})
	return p.inner.Stream(ctx, r, req)
}

// liteRoomWithRacingProvider wires a lite API whose model call opens a
// competing lens mid-flight, with an article already pasted.
func liteRoomWithRacingProvider(t *testing.T, inner gateway.Provider) (http.Handler, *http.Cookie, string, *pgxpool.Pool) {
	t.Helper()
	prov := &raceProvider{t: t, inner: inner}
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	// Both fields are set before any request runs, so Stream always sees them.
	prov.pool = pool
	atomID := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, atomID, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	prov.atomID = atomID
	return h, cookie, atomID, pool
}

func countOpenCards(t *testing.T, pool *pgxpool.Pool, atomID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM atom_card WHERE atom_id = $1 AND status IN ('proposed','active')`,
		mustUUID(atomID)).Scan(&n); err != nil {
		t.Fatalf("count open cards: %v", err)
	}
	return n
}

// TestReadingLensRace_TheDatabaseRefusesASecondOpenLens — the constraint
// itself, independent of any handler. This is the half a future caller cannot
// bypass by forgetting to check, which is why it is a database index and not a
// transaction-local re-read.
func TestReadingLensRace_TheDatabaseRefusesASecondOpenLens(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	atomID := createReadingAtom(t, h, cookie)

	if _, err := pool.Exec(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status) VALUES ($1, 'craap', 'b1', 'proposed')`,
		mustUUID(atomID)); err != nil {
		t.Fatalf("first open card: %v", err)
	}
	_, err := pool.Exec(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status) VALUES ($1, 'sift', 'b2', 'active')`,
		mustUUID(atomID))
	if err == nil {
		t.Fatal("a second OPEN card was accepted — atom_card_one_open_idx is missing or wrong; " +
			"getOpenCard shows only the first, so the second would block every future summon invisibly")
	}
	if !strings.Contains(err.Error(), "atom_card_one_open_idx") {
		t.Fatalf("second open card rejected by the wrong thing: %v", err)
	}

	// The index must NOT stop a reading from using many lenses in sequence —
	// terminal rows are outside its predicate.
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status, submitted_at)
		 VALUES ($1, 'sift', 'b2', 'submitted', now())`, mustUUID(atomID)); err != nil {
		t.Fatalf("a submitted card must never conflict: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO atom_card (atom_id, card_id, block_id, status) VALUES ($1, 'sift', 'b2', 'skipped')`,
		mustUUID(atomID)); err != nil {
		t.Fatalf("a skipped card must never conflict: %v", err)
	}
}

// TestReadingLensRace_SummonLosingTheRaceDeclinesInsteadOfDoubleMinting —
// the student-driven path. Pre-fix this mints a second `proposed` row and
// answers with a card; post-fix it answers with the ordinary busy line, which
// is the truth: a lens IS open, just not the one she asked for.
func TestReadingLensRace_SummonLosingTheRaceDeclinesInsteadOfDoubleMinting(t *testing.T) {
	h, cookie, atomID, pool := liteRoomWithRacingProvider(t, routerStubProvider(exampleScript))

	rec := postSummon(t, h, cookie, atomID, "craap")
	if rec.Code != http.StatusOK {
		t.Fatalf("raced summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card != nil {
		t.Fatalf("raced summon minted a SECOND open card (%+v) — she can only ever see one of them", out.Card)
	}
	if out.Decision != "respond" || !strings.Contains(out.Reply, "先完成") {
		t.Fatalf("raced summon = %+v, want the ordinary 先完成当前这副透镜 decline", out)
	}
	if n := countOpenCards(t, pool, atomID); n != 1 {
		t.Fatalf("open cards = %d, want exactly 1", n)
	}
}

// TestReadingLensRace_TurnLosingTheRaceKeepsHerAnswer — the router-driven
// path. The card is dropped, but her question and the coach's reply must still
// commit: she already paid a flagship call for that answer, and failing the
// whole turn to protect a card she never asked for is the wrong trade.
func TestReadingLensRace_TurnLosingTheRaceKeepsHerAnswer(t *testing.T) {
	h, cookie, atomID, pool := liteRoomWithRacingProvider(t, routerStubProvider(summonScript))

	rec := postTurn(t, h, cookie, atomID, `{"text":"这个十倍的数字是谁统计的？"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("raced turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out turnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Card != nil {
		t.Fatalf("raced turn minted a SECOND open card: %+v", out.Card)
	}
	if out.Reply != "你抓到的这个数字确实是全篇的支点。" {
		t.Fatalf("reply = %q — losing the card must never cost her the answer", out.Reply)
	}
	if n := countOpenCards(t, pool, atomID); n != 1 {
		t.Fatalf("open cards = %d, want exactly 1", n)
	}
	// Both halves of the exchange still committed.
	if msgs := listMessages(t, h, cookie, atomID); len(msgs) != 2 {
		t.Fatalf("messages = %+v, want the student turn and the reply", msgs)
	}
}
