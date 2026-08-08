package api_test

// coach_history_page_test.go — Task 4: GET /coach/history moves from
// load-everything to cursor pagination (composite (created_at, id) cursor,
// newest-first fetch reversed to oldest→newest for display) plus a
// digest-fed recap on the first page when there's more history hiding behind
// it. recap must NEVER cost a spend — it only reads the stored
// conversation_digest row, never calls the model.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// seedStudioThread ensures a chat_thread exists for the seeded project (owned
// by SeedUserID) and returns its id, creating one directly via SQL if absent
// — mirroring what agent.getOrCreateThread does lazily on a real coach turn.
func seedStudioThread(t *testing.T, pool *pgxpool.Pool, projectID string) string {
	t.Helper()
	var threadID string
	err := pool.QueryRow(context.Background(),
		`SELECT id::text FROM chat_thread WHERE seeded_project_id = $1 LIMIT 1`,
		mustUUID(projectID)).Scan(&threadID)
	if err == nil {
		return threadID
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO chat_thread (user_id, seeded_project_id) VALUES ($1, $2) RETURNING id::text`,
		SeedUserID, mustUUID(projectID)).Scan(&threadID); err != nil {
		t.Fatalf("insert chat_thread: %v", err)
	}
	return threadID
}

// seedStudioTurns inserts n chat_message rows on the project's studio surface
// with EXPLICIT, strictly-increasing created_at (base + i seconds) so
// pagination ordering is deterministic — real now() writes can tie at
// whatever the DB's clock resolution is, which would make "messages[0] is
// older than messages[n-1]" assertions flaky.
func seedStudioTurns(t *testing.T, pool *pgxpool.Pool, projectID string, n int) {
	t.Helper()
	threadID := seedStudioThread(t, pool, projectID)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		content := fmt.Sprintf("turn-%02d", i)
		createdAt := base.Add(time.Duration(i) * time.Second)
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO chat_message (thread_id, role, content, modality, surface, attachments, created_at)
			 VALUES ($1, $2, $3, 'text', 'studio', '[]', $4)`,
			threadID, role, content, createdAt); err != nil {
			t.Fatalf("seed studio turn %d: %v", i, err)
		}
	}
}

type coachHistoryPageResp struct {
	Messages []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"messages"`
	HasMore    bool    `json:"hasMore"`
	Recap      *string `json:"recap"`
	NextCursor *string `json:"nextCursor"`
}

func getCoachHistoryPage(t *testing.T, h http.Handler, cookie *http.Cookie, base, qs string) coachHistoryPageResp {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/coach/history?"+qs, nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("history %s = %d — %s", qs, rr.Code, rr.Body)
	}
	var resp coachHistoryPageResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode history %s: %v — %s", qs, err, rr.Body)
	}
	return resp
}

// TestCoachHistory_Pagination — 25 seeded studio turns (turn-00 oldest ..
// turn-24 newest). The first page (no `before`) is the NEWEST 20 — chat-style
// "land at the bottom of the conversation" — returned oldest→newest within
// the page (turn-05 .. turn-24), hasMore true, nextCursor set to page the
// remaining older history. Following `before` returns the remaining 5 oldest
// (turn-00 .. turn-04), hasMore false, no cursor, and (per spec) no recap on a
// `before` page even if a digest row exists.
func TestCoachHistory_Pagination(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	seedStudioTurns(t, pool, seedProjectID, 25)

	page1 := getCoachHistoryPage(t, h, cookie, base, "surface=studio&limit=20")
	if len(page1.Messages) != 20 {
		t.Fatalf("page1 messages = %d, want 20", len(page1.Messages))
	}
	if page1.Messages[0].Text != "turn-05" {
		t.Fatalf("page1 messages[0] = %q, want turn-05 (oldest of the newest-20 page)", page1.Messages[0].Text)
	}
	if page1.Messages[19].Text != "turn-24" {
		t.Fatalf("page1 messages[19] = %q, want turn-24 (newest)", page1.Messages[19].Text)
	}
	if !page1.HasMore {
		t.Fatalf("page1 hasMore = false, want true")
	}
	if page1.NextCursor == nil {
		t.Fatalf("page1 nextCursor = nil, want set")
	}
	// No digest row yet → recap must be null even though hasMore is true.
	if page1.Recap != nil {
		t.Fatalf("page1 recap = %q, want nil (no digest row yet)", *page1.Recap)
	}

	page2 := getCoachHistoryPage(t, h, cookie, base, "surface=studio&limit=20&before="+*page1.NextCursor)
	if len(page2.Messages) != 5 {
		t.Fatalf("page2 messages = %d, want 5", len(page2.Messages))
	}
	if page2.Messages[0].Text != "turn-00" || page2.Messages[4].Text != "turn-04" {
		t.Fatalf("page2 messages = %+v, want turn-00..turn-04", page2.Messages)
	}
	if page2.HasMore {
		t.Fatalf("page2 hasMore = true, want false")
	}
	if page2.NextCursor != nil {
		t.Fatalf("page2 nextCursor = %q, want nil", *page2.NextCursor)
	}
	if page2.Recap != nil {
		t.Fatalf("page2 recap = %q, want nil (before-page never carries recap)", *page2.Recap)
	}
}

// TestCoachHistory_RecapFromDigest — with a conversation_digest row present,
// the first page (hasMore true) surfaces its prose as recap; without one,
// recap stays null. Never spends: this only reads the stored digest row.
func TestCoachHistory_RecapFromDigest(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	seedStudioTurns(t, pool, seedProjectID, 25)

	// No digest row → recap null on the first page despite hasMore.
	before := getCoachHistoryPage(t, h, cookie, base, "surface=studio&limit=20")
	if !before.HasMore {
		t.Fatalf("hasMore = false, want true (25 seeded, limit 20)")
	}
	if before.Recap != nil {
		t.Fatalf("recap before digest = %q, want nil", *before.Recap)
	}

	const prose = "学生正在为「中国是否让地球更可持续」搭建论证，已确认研究问题与三条资源线索。"
	if err := q.UpsertConversationDigest(context.Background(), sqlc.UpsertConversationDigestParams{
		ProjectID:   mustUUID(seedProjectID),
		Prose:       prose,
		TurnsFolded: 5,
	}); err != nil {
		t.Fatalf("upsert digest: %v", err)
	}

	after := getCoachHistoryPage(t, h, cookie, base, "surface=studio&limit=20")
	if after.Recap == nil || *after.Recap != prose {
		t.Fatalf("recap after digest = %v, want %q", after.Recap, prose)
	}
}

// TestCoachHistory_BadCursor — a malformed `before` value is a 400, not a
// panic or a silent empty page.
func TestCoachHistory_BadCursor(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/coach/history?surface=studio&before=notbase64!", nil), cookie))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor = %d, want 400 — %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "invalid_cursor") {
		t.Fatalf("bad cursor body = %s, want invalid_cursor code", rr.Body)
	}
}
