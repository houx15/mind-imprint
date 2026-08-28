package api_test

// writing_deepen_isolation_test.go — handler-level coverage for 深入一层
// (writing_deepen.go): POST /outline/{oid}/deepen, GET the same route back,
// and the isolation guarantee migration 0102 exists for. This file is
// package api_test (unlike writing_deepen_test.go's white-box
// buildDeepenBrief test) because it needs the shared HTTP test harness
// (liteHandlerWithProvider, createWritingAtomHTTP, putWritingOutlineHTTP,
// withCookie, writingTextStubProvider) that lives there — Go does not allow
// one file to be both the internal and the external test package for the
// same directory, so the two tests the brief describes live in two files.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type deepenTurnReply struct {
	Reply string `json:"reply"`
}

func postWritingDeepenTurn(t *testing.T, h http.Handler, cookie *http.Cookie, id, oid, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/outline/"+oid+"/deepen", strings.NewReader(body)), cookie))
	return rec
}

func getWritingDeepenThread(t *testing.T, h http.Handler, cookie *http.Cookie, id, oid string) []writingTurnMessage {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET",
		"/api/v1/writings/"+id+"/outline/"+oid+"/deepen", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET deepen thread = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Messages []writingTurnMessage `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode deepen thread: %v — body=%s", err, rec.Body)
	}
	return out.Messages
}

func seedTwoBlockOutline(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []writingOutlineItem {
	t.Helper()
	putBody := `{"outline":[{"role":"中心论点","text":"该种，但要先定谁长期养","depth":0},{"role":"一条理由","text":"维护年年花钱","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, putBody); rec.Code != http.StatusOK {
		t.Fatalf("put outline = %d; body=%s", rec.Code, rec.Body)
	}
	return getWritingOutlineHTTP(t, h, cookie, id)
}

// TestDeepenTurn_DoesNotEnterTheRoomThread — a deepen turn must be invisible
// to the room's own thread — the top risk of migration 0102 expressed at the
// endpoint rather than at the query.
func TestDeepenTurn_DoesNotEnterTheRoomThread(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("这笔钱谁出，是物业还是市政，得先弄清楚。"))
	id := createWritingAtomHTTP(t, h, cookie, "城市该不该大规模种行道树")
	rows := seedTwoBlockOutline(t, h, cookie, id)
	oid := rows[1].ID

	beforeRoom := listWritingMessages(t, h, cookie, id)

	rec := postWritingDeepenTurn(t, h, cookie, id, oid, `{"text":"这笔钱谁出？"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST deepen = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out deepenTurnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode deepen reply: %v — body=%s", err, rec.Body)
	}
	if out.Reply != "这笔钱谁出，是物业还是市政，得先弄清楚。" {
		t.Fatalf("reply = %q, want the model's reply verbatim", out.Reply)
	}

	// The room's own thread (GET /writings/{id}/messages) must be UNCHANGED —
	// neither her deepen turn nor the sub-agent's reply may leak into it.
	afterRoom := listWritingMessages(t, h, cookie, id)
	if len(afterRoom) != len(beforeRoom) {
		t.Fatalf("room thread grew from %d to %d messages; a deepen turn leaked in: %+v", len(beforeRoom), len(afterRoom), afterRoom)
	}
	for _, m := range afterRoom {
		if strings.Contains(m.Content, "这笔钱谁出") {
			t.Fatalf("deepen turn found in the room's own thread: %+v", afterRoom)
		}
	}

	// It DOES land in the block's own thread.
	thread := getWritingDeepenThread(t, h, cookie, id, oid)
	if len(thread) != 2 {
		t.Fatalf("block thread = %d messages, want 2 (student + ai): %+v", len(thread), thread)
	}
	if thread[0].Role != "student" || thread[0].Content != "这笔钱谁出？" {
		t.Fatalf("block thread[0] = %+v, want her turn", thread[0])
	}
	if thread[1].Role != "ai" || thread[1].Content != out.Reply {
		t.Fatalf("block thread[1] = %+v, want the ai reply", thread[1])
	}
}

// TestDeepenTurn_TwoBlocksHaveSeparateThreads — a second block's drawer must
// not see the first block's conversation; each block_id is its own thread.
func TestDeepenTurn_TwoBlocksHaveSeparateThreads(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("先说说维护谁来管？"))
	id := createWritingAtomHTTP(t, h, cookie, "城市该不该大规模种行道树")
	rows := seedTwoBlockOutline(t, h, cookie, id)
	oidA, oidB := rows[0].ID, rows[1].ID

	if rec := postWritingDeepenTurn(t, h, cookie, id, oidA, `{"text":"论点要不要更明确？"}`); rec.Code != http.StatusOK {
		t.Fatalf("POST deepen block A = %d; body=%s", rec.Code, rec.Body)
	}

	threadB := getWritingDeepenThread(t, h, cookie, id, oidB)
	if len(threadB) != 0 {
		t.Fatalf("block B thread = %+v, want empty — block A's turn must not leak across blocks", threadB)
	}
}

// TestDeepenTurn_UnknownBlockIs404 — an outline id that does not belong to
// this atom (or does not exist at all) is scoped exactly like guideWritingBlock:
// 404, never a leak of another writing's block.
func TestDeepenTurn_UnknownBlockIs404(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("不会用到"))
	id := createWritingAtomHTTP(t, h, cookie, "城市该不该大规模种行道树")

	rec := postWritingDeepenTurn(t, h, cookie, id, "00000000-0000-0000-0000-000000000000", `{"text":"你好"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST deepen on unknown block = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
