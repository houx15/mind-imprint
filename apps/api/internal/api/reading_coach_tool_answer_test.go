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

// 产品负责人 2026-09-17：「想一想 and 仿写 actually these are things that need
// students' input. how should we do that? put a box there to invite students to
// write and give feedbacks?」
//
// 她在段落工具底下写的那一段交给 印记 要反馈。那是一次旁支练习 —— 清单此刻
// 可能停在别的步骤上。模型很容易把「她交了一段话」读成「这一步的作业交了」，
// 给 done，于是她一个字没写，那一步就过去了。这里用一个**明确说 done** 的桩
// 模型去撞那道判据。
func TestReadingCoach_ToolAnswerNeverAdvancesTheStep(t *testing.T) {
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"reply":"你用上了先场景后原理的写法，「楼下的空调外机一直在吹热风」这一句场景选得准。这一步做完了。","advance":"done","focusBlock":"","card":{"type":"short_text","prompt":"再写一段？"}}`))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	atom := uuid.MustParse(id)
	if _, err := q.ReplaceReadingTasks(context.Background(), sqlc.ReplaceReadingTasksParams{
		AtomID: atom, Positions: []int32{0}, Kinds: []string{"critique"},
		Labels: []string{"你怎么看"}, Details: []string{"作者说的你同意吗？"}, BlockIds: []string{""},
	}); err != nil {
		t.Fatal(err)
	}

	body := map[string]any{
		"text": "",
		"cardAnswer": map[string]string{
			"type":    "block_tool",
			"prompt":  "仿写 · 第2段：先给一个日常场景，再解释背后的原理",
			"choice":  "楼下的空调外机一直在吹热风，所以一楼比别的楼层都热。",
			"blockId": "b2",
		},
	}
	wire, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/coach", bytes.NewReader(wire)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	// 1. 步骤不动。
	tasks, err := q.ListReadingTasks(context.Background(), atom)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Status != "pending" {
		t.Fatalf("她在段落工具底下写了一段，「你怎么看」那一步却被推进了：%+v", tasks)
	}

	// 2. 这一轮是反馈，不再出新卡片。
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if card, ok := resp["coachCard"]; ok && string(card) != "null" {
		t.Errorf("反馈那一轮又递了一张卡：%s", card)
	}

	// 3. 她写的那一段按「她的话」存下来，工具那一行是「印记问」—— 不算她说的。
	msgs, err := q.ListAtomMessages(context.Background(), atom)
	if err != nil {
		t.Fatal(err)
	}
	var mine string
	for _, m := range msgs {
		if m.Role == "student" {
			mine = m.Content
		}
	}
	if !strings.Contains(mine, "> 【印记问】仿写 · 第2段") {
		t.Errorf("工具那一行没有以「印记问」存下来：%q", mine)
	}
	if !strings.Contains(mine, "\n楼下的空调外机一直在吹热风") {
		t.Errorf("她写的那一段没有作为她自己的话存下来：%q", mine)
	}
}
