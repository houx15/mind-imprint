package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

// Exercise the actual handler and persisted task, not just answeredBoard().
// The former outer advance != "" condition made both existing fallbacks dead.
func TestReadingCoach_EmptyAdvanceStillRunsExistingProgressRules(t *testing.T) {
	for _, c := range []struct {
		name, kind, answerType, want string
		turns                        int
	}{
		{"submitted-label-board", "label", "label_roles", "done", 0},
		{"text-is-not-board-submission", "label", "", "pending", 0},
		{"unfinished-focus", "focus_block", "", "pending", 0},
		{"stalled-focus", "focus_block", "", "done", 6},
		{"stalled-hunt-still-needs-pick", "hunt", "", "pending", 6},
	} {
		t.Run(c.name, func(t *testing.T) {
			h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(`{"reply":"请说明你选择这句话的依据。","advance":"","focusBlock":"b3","card":null}`))
			id := createReadingAtom(t, h, cookie)
			putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
			atom := uuid.MustParse(id)
			_, err := q.ReplaceReadingTasks(context.Background(), sqlc.ReplaceReadingTasksParams{AtomID: atom, Positions: []int32{0}, Kinds: []string{c.kind}, Labels: []string{"分析文章"}, Details: []string{"分析文章"}, BlockIds: []string{"b3"}})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < c.turns; i++ {
				seq, err := q.NextAtomMessageSeq(context.Background(), atom)
				if err != nil {
					t.Fatal(err)
				}
				_, err = q.AppendAtomMessage(context.Background(), sqlc.AppendAtomMessageParams{AtomID: atom, Seq: seq, Role: "ai", Content: "请再说明这句的依据。"})
				if err != nil {
					t.Fatal(err)
				}
			}
			body := map[string]any{"text": "我已经说了我的理由。"}
			if c.answerType != "" {
				body["cardAnswer"] = map[string]string{"type": c.answerType, "prompt": "判断论证成分", "choice": "背景：气象学上把这种现象叫做城市热岛效应。"}
			}
			wire, _ := json.Marshal(body)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/coach", bytes.NewReader(wire)), cookie))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if c.answerType != "" {
				var response map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if card, ok := response["coachCard"]; ok && string(card) != "null" {
					t.Fatal("completed board was replaced by an unsolicited fallback board")
				}
			}
			tasks, err := q.ListReadingTasks(context.Background(), atom)
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != 1 || tasks[0].Status != c.want {
				t.Fatalf("persisted tasks = %+v, want %s", tasks, c.want)
			}
		})
	}
}
