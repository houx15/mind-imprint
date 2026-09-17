package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

// 答过的卡片改答案（产品负责人 2026-09-17）。服务端要做的两件事：
// 转写里多一行「> 【她改了答案】」（带 > —— 不是她说的话），payload 里带着
// revised，前端据此把这一份挂回原来那张卡上。
func TestReadingCoach_RevisedAnswerIsMarked(t *testing.T) {
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"reply":"你把答案从第二句换成了第一句，这一句确实更直接。请接着看下一段。","advance":"","focusBlock":"","card":null}`))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	if _, err := q.ReplaceReadingTasks(context.Background(), sqlc.ReplaceReadingTasksParams{
		AtomID: uuid.MustParse(id), Positions: []int32{0}, Kinds: []string{"critique"},
		Labels: []string{"你怎么看"}, Details: []string{"作者说的你同意吗？"}, BlockIds: []string{""},
	}); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"text": "",
		"cardAnswer": map[string]any{
			"type": "short_text", "prompt": "用一句话说说你的判断。",
			"choice": "我改主意了：证据不够。", "revised": true,
		},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/coach", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	msgs, err := q.ListAtomMessages(context.Background(), uuid.MustParse(id))
	if err != nil {
		t.Fatal(err)
	}
	var content string
	var payload map[string]map[string]any
	for _, m := range msgs {
		if m.Role == "student" {
			content = m.Content
			_ = json.Unmarshal(m.Payload, &payload)
		}
	}
	if !strings.HasPrefix(content, "> 【她改了答案】\n") {
		t.Errorf("转写里没有标出这是改过的答案：%q", content)
	}
	if !strings.Contains(content, "\n我改主意了：证据不够。") {
		t.Errorf("她改后的那句没有作为她的话存下来：%q", content)
	}
	if payload["answer"]["revised"] != true {
		t.Errorf("payload 里没带 revised —— 前端没法把它挂回原来那张卡：%v", payload)
	}
}
