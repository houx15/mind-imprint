package api_test

// coach_stage_test.go — Task 10: every persisted studio coach turn carries the
// studio_state.stage in effect at that moment (chat_message.stage, migration
// 0060), so the evaluation layer can read the per-turn lifecycle arc
// (过程即数据 — "surface" says WHICH room/producer, "stage" says WHERE in the
// project lifecycle). Asserted directly against sqlc's ChatMessage.Stage
// (nullable *string) rather than the wire response, since stage is
// persistence-only metadata the reply DTO does not echo.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestCoachOpening_PersistsTopicDiscussionStage — the opening welcome is the
// studio thread's first turn, before the student has confirmed she's ready
// (postCoachStart is the explicit gate), so it must land tagged with the
// project's starting stage, topic_discussion.
func TestCoachOpening_PersistsTopicDiscussionStage(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach/opening", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach/opening = %d — %s", rr.Code, rr.Body)
	}

	rows := chatMessagesBySurface(t, pool, "studio")
	if len(rows) != 1 {
		t.Fatalf("studio surface turns after opening = %d, want 1 — %+v", len(rows), rows)
	}
	if rows[0].Stage == nil || *rows[0].Stage != string(agent.StageTopicDiscussion) {
		t.Fatalf("opening turn stage = %v, want %q — %+v", stagePtrString(rows[0].Stage), agent.StageTopicDiscussion, rows[0])
	}
}

// TestCoachStart_PersistsProposalFormingStage — the explicit start gate flips
// studio_state.stage from topic_discussion to proposal_forming BEFORE
// persisting the synthetic student turn and the orchestrator's reply, so both
// of start's turns must carry proposal_forming, not the pre-start stage.
func TestCoachStart_PersistsProposalFormingStage(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	openH := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	rrOpen := httptest.NewRecorder()
	openH.ServeHTTP(rrOpen, withCookie(httptest.NewRequest("POST", base+"/coach/opening", nil), cookie))
	if rrOpen.Code != http.StatusOK {
		t.Fatalf("coach/opening = %d — %s", rrOpen.Code, rrOpen.Body)
	}

	startOut := `{"narrate":"我们先想目标","tools":[]}`
	startH := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: orchestratorStubProvider(startOut), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	rr := httptest.NewRecorder()
	startH.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach/start", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach/start = %d — %s", rr.Code, rr.Body)
	}

	rows := chatMessagesBySurface(t, pool, "studio")
	if len(rows) != 3 {
		t.Fatalf("studio surface turns after start = %d, want 3 (opening-assistant + start-user + start-assistant) — %+v", len(rows), rows)
	}
	// The opening turn (first) stays tagged topic_discussion — start does not
	// retroactively rewrite prior turns.
	if rows[0].Stage == nil || *rows[0].Stage != string(agent.StageTopicDiscussion) {
		t.Fatalf("opening turn stage = %v, want %q — %+v", stagePtrString(rows[0].Stage), agent.StageTopicDiscussion, rows[0])
	}
	// start's synthetic student turn + its assistant reply both carry the
	// advanced stage.
	var sawSyntheticUser, sawAssistant bool
	for _, m := range rows[1:] {
		if m.Stage == nil || *m.Stage != string(agent.StageProposalForming) {
			t.Fatalf("start turn stage = %v, want %q — %+v", stagePtrString(m.Stage), agent.StageProposalForming, m)
		}
		if m.Role == "user" && m.Content == "我准备好了，开始吧" {
			sawSyntheticUser = true
		}
		if m.Role == "assistant" {
			sawAssistant = true
		}
	}
	if !sawSyntheticUser || !sawAssistant {
		t.Fatalf("expected start's synthetic student turn + assistant reply among: %+v", rows)
	}
}

// TestPostCoach_PersistsCurrentStudioStateStage — a plain /coach turn (no
// tools emitted) persists both the student turn and the assistant reply
// tagged with the project's CURRENT studio_state.stage at the time of the
// turn — the default topic_discussion for a fresh project.
func TestPostCoach_PersistsCurrentStudioStateStage(t *testing.T) {
	out := `{"narrate":"先说说你的研究问题。","tools":[]}`
	h, cookie, pool := orchestratorHandler(t, out)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我想研究气候变化"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	raw, gerr := sqlc.New(pool).GetStudioState(context.Background(), mustUUID(seedProjectID))
	if gerr != nil {
		t.Fatalf("GetStudioState: %v", gerr)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v — %s", err, raw)
	}

	rows := chatMessagesBySurface(t, pool, "studio")
	if len(rows) != 2 {
		t.Fatalf("studio surface turns = %d, want 2 (student+assistant) — %+v", len(rows), rows)
	}
	for _, m := range rows {
		if m.Stage == nil || *m.Stage != string(st.Stage) {
			t.Fatalf("turn stage = %v, want current studio_state.stage %q — %+v", stagePtrString(m.Stage), st.Stage, m)
		}
	}
}

// chatMessagesBySurface reads every chat_message on the seed project's given
// surface, oldest-first — a thin sqlc-query helper for asserting on the
// `stage` column, which the wire response never echoes.
func chatMessagesBySurface(t *testing.T, pool *pgxpool.Pool, surface string) []sqlc.ChatMessage {
	t.Helper()
	rows, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true}, Surface: &surface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface(%q): %v", surface, err)
	}
	return rows
}

func stagePtrString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
