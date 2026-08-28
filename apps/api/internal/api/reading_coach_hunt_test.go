package api_test

// reading_coach_hunt_test.go — F3 (final review): a hunt step must not
// settle on an assertion.
//
// The ruling: advance:"done" on a hunt step (readingTaskKind "hunt") requires
// a valid pick — a literal quote of a real paragraph — seen either THIS turn
// or in the IMMEDIATELY PRECEDING student turn. Below, "seed" tests skip the
// five preliminary advancing turns every real routine would need to reach the
// (always-last) hunt step, by writing a single pending hunt task directly —
// postReadingCoachTurn only plans when ListReadingTasks comes back empty, so
// a seeded non-empty list is picked up exactly like a real one.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// seedReadingHuntTask replaces the atom's whole task list with a single
// pending hunt step.
func seedReadingHuntTask(t *testing.T, q *sqlc.Queries, id string) {
	t.Helper()
	if _, err := q.ReplaceReadingTasks(context.Background(), sqlc.ReplaceReadingTasksParams{
		AtomID:    uuid.MustParse(id),
		Positions: []int32{0},
		Kinds:     []string{"hunt"},
		Labels:    []string{"回去找一句"},
		Details:   []string{"在文章里点出最能撑住作者观点的那一句。"},
		BlockIds:  []string{""},
	}); err != nil {
		t.Fatalf("seed hunt task: %v", err)
	}
}

type readingPickJSON struct {
	BlockID string `json:"blockId"`
	Quote   string `json:"quote"`
}

// coachTurnWithPicks is coachTurn (reading_coach_test.go) plus structured
// picks — the field a real pointed-at quote rides in, alongside the same
// blockquote-inlined text ReadingCoachPanel.tsx's send() always sends too.
func coachTurnWithPicks(t *testing.T, h http.Handler, cookie *http.Cookie, id, text string, picks []readingPickJSON) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{"text": text, "picks": picks})
	if err != nil {
		t.Fatalf("marshal picks body: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("POST", "/api/v1/readings/"+id+"/coach", bytes.NewReader(body)), cookie))
	return rec
}

// huntScript is a reply script the reader can drop advance/reply text into —
// same shape writingTextStubProvider (writing_turn_test.go) builds, kept
// local because these tests need to script TWO DIFFERENT turns via
// gateway.NewSequenceStubProvider, which writingTextStubProvider's
// single-script StubProvider cannot do.
func huntScript(reply string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 90, OutputTokens: 30}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

const huntDoneScript = `{"reply":"好，接住了。","advance":"done","focusBlock":""}`

// b3's literal text in zhArticle (reading_plan_test.go) — reused verbatim so
// every pick below survives validateReadingPicks.
const huntQuoteB3 = "气象学上把这种现象叫做城市热岛效应。"

// TestReadingCoach_HuntStaysPutWithoutAPoint — she types an assertion
// ("我觉得是第三段那句") and points at nothing. Even though the model returns
// advance:"done", the hunt step must NOT settle: an answer given by
// assertion is exactly the thing the hunt replaced, and it must not sneak
// back in just because the model agreed with her.
func TestReadingCoach_HuntStaysPutWithoutAPoint(t *testing.T) {
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(huntDoneScript))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	seedReadingHuntTask(t, q, id)

	out := decodeCoachTurn(t, coachTurn(t, h, cookie, id, "我觉得是第三段那句。"))
	for _, task := range out.Tasks {
		if task.Kind == "hunt" && task.Status != "pending" {
			t.Fatalf("hunt step settled on an assertion, no point ever seen: %+v", task)
		}
	}
	if out.Finished {
		t.Fatalf("the reading finished on an assertion, not a point")
	}
}

// TestReadingCoach_HuntSettlesWhenSheActuallyPoints — a valid pick THIS turn
// is enough on its own: the guard is a floor under "done", not an extra hoop
// on top of a real point.
func TestReadingCoach_HuntSettlesWhenSheActuallyPoints(t *testing.T) {
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(huntDoneScript))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	seedReadingHuntTask(t, q, id)

	out := decodeCoachTurn(t, coachTurnWithPicks(t, h, cookie, id,
		"> "+huntQuoteB3+"\n\n就是这句",
		[]readingPickJSON{{BlockID: "b3", Quote: huntQuoteB3}}))
	var settled bool
	for _, task := range out.Tasks {
		if task.Kind == "hunt" && task.Status == "done" {
			settled = true
		}
	}
	if !settled {
		t.Fatalf("hunt step did not settle despite a valid pick this turn: %+v", out.Tasks)
	}
}

// TestReadingCoach_HuntSettlesOnTheLookbackTurn — she points on turn N; the
// coach legitimately settles on turn N+1, once she has moved on to plain talk
// with no picks of her own. The guard must read the PRECEDING student turn's
// quoted lines, not only this turn's picks — otherwise a coach replying one
// beat late would read her real point as an unsupported assertion.
//
// Turn N's script deliberately answers advance:"" (not "done") so nothing
// settles from turn N itself; only turn N+1's lookback is under test.
func TestReadingCoach_HuntSettlesOnTheLookbackTurn(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		huntScript(`{"reply":"我看到了，再说说它撑不撑得住。","advance":"","focusBlock":""}`),
		huntScript(huntDoneScript),
	)
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	seedReadingHuntTask(t, q, id)

	// Turn N: she points. The script does not advance, so this alone proves
	// nothing about the lookback — it only seeds the transcript.
	first := decodeCoachTurn(t, coachTurnWithPicks(t, h, cookie, id,
		"> "+huntQuoteB3+"\n\n是这句吗",
		[]readingPickJSON{{BlockID: "b3", Quote: huntQuoteB3}}))
	for _, task := range first.Tasks {
		if task.Kind == "hunt" && task.Status != "pending" {
			t.Fatalf("setup: hunt settled on turn N already — turn N+1 cannot test the lookback: %+v", task)
		}
	}

	// Turn N+1: no picks this turn, plain talk only.
	second := decodeCoachTurn(t, coachTurn(t, h, cookie, id, "好的"))
	var settled bool
	for _, task := range second.Tasks {
		if task.Kind == "hunt" && task.Status == "done" {
			settled = true
		}
	}
	if !settled {
		t.Fatalf("hunt step did not settle on the lookback turn despite turn N's point: %+v", second.Tasks)
	}
}

// TestReadingCoach_HuntSkipIsNeverGuarded — 铁律②: she can always decline a
// step by saying so. The guard applies ONLY to advance:"done"; "skipped"
// must go through completely untouched, with no picks at all — a guard that
// trapped her on the hunt would be a serious violation of that ruling.
func TestReadingCoach_HuntSkipIsNeverGuarded(t *testing.T) {
	const skipping = `{"reply":"行，那这步跳过。","advance":"skipped","focusBlock":""}`
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(skipping))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	seedReadingHuntTask(t, q, id)

	out := decodeCoachTurn(t, coachTurn(t, h, cookie, id, "这一步我不想找了，跳过吧。"))
	var skipped bool
	for _, task := range out.Tasks {
		if task.Kind == "hunt" && task.Status == "skipped" {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("skipping a hunt step with no point at all was blocked; tasks=%+v", out.Tasks)
	}
}
