package api_test

// reading_coach_help_test.go —— 她按「给点提示」的那一轮，到底发生了什么。
//
// 黑盒，因为要钉住的是**离开进程的那份 JSON** 和**打给模型的次数**：
//
//  1. 她屏幕上那张卡不许被换掉（响应里不能有 coachCard）；
//  2. 这一步不许推进（tasks 原样）；
//  3. 这一轮**只打一次模型**。
//
// 第 3 条是 2026-09-20 线上走查量出来的：三次提示里有两次被判
// cardRejectDeadTurn（「这一轮什么都没给她做」）⇒ 每次提示白花一次模型调用，
// 而重来那一次收到的指令正好和「不要再发新卡片」顶上。判据读代码看不出来，
// 只有数调用次数才问得出口。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// 第一轮：递一张要她自己写的卡（开放题）。
const helpTurnOpensACard = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
  "reply":"先看第3段。","advance":"","focusBlock":"b3",
  "card":{"type":"short_text","prompt":"用你自己的话说说，作者为什么先讲柏油路？"}}`

// 提示那一轮模型的反应：又写了一张卡，而且顺手把这一步判成做完了。
// 两件事都不许发生。
const helpTurnTriesToReplaceTheCard = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
  "reply":"看第3段讲吸热和放热的那半句。","advance":"done","focusBlock":"b3",
  "card":{"type":"choose_span","prompt":"哪一句在讲白天和夜里的差别？","options":[
    {"blockId":"b3","quote":"城市里的柏油路和水泥墙白天大量吸热"},
    {"blockId":"b5","quote":"是把灰色的屋顶改成绿色的"}]}}`

// coachOnlyProvider —— 只数**陪练**那几次调用，并按顺序回答它们。
//
// 🚨 SequenceStubProvider 在这里不够用：第一次陪练请求会先把读法排出来
// （planReadingTasks 走同一个 Provider，而且它自己也会重来一次），于是「第几个
// 脚本」和「第几轮对话」对不上 —— 第一版测试就是这么绿的，它断言的那张卡其实
// 来自下一个脚本。按**请求内容**认：陪练那条 system 是独一份的。
type coachOnlyProvider struct {
	replies []string
	Calls   int // 只数陪练
}

func (p *coachOnlyProvider) Stream(ctx context.Context, _ gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	// 排读法那次用同一份 JSON：它带着 routineKey 和 steps，正是排读法要的形状
	// （陪练那一份多出来的键被忽略）。这里要的只是「别让排读法失败」。
	text := p.replies[0]
	isCoach := false
	for _, m := range req.Messages {
		if m.Role == gateway.RoleSystem && strings.Contains(m.Content, "你是「印记」，带一名中学生读文章") {
			isCoach = true
		}
	}
	if isCoach {
		i := p.Calls
		if i >= len(p.replies) {
			i = len(p.replies) - 1
		}
		p.Calls++
		text = p.replies[i]
	}
	out := make(chan gateway.StreamEvent, 3)
	go func() {
		defer close(out)
		for _, ev := range []gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: text},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 40, OutputTokens: 20}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		} {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out, nil
}

func coachScripts(outs ...string) *coachOnlyProvider {
	return &coachOnlyProvider{replies: outs}
}

// 🚨 产品负责人 2026-09-20 报的第 1 条：「可以就一张卡片一直点提示一下，使得论文
// 阅读流程卡住，无法进行下一步」。提示轮换掉她手上那张卡，她写了一半的草稿跟着
// 一起没了，而清单一步都不动。
func TestReadingCoach_HelpTurnKeepsHerCardAndDoesNotAdvance(t *testing.T) {
	prov := coachScripts(helpTurnOpensACard, helpTurnTriesToReplaceTheCard)
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	opened := coachTurn(t, h, cookie, id, "")
	if _, ok := coachTurnRaw(t, opened)["coachCard"].(map[string]any); !ok {
		t.Fatalf("第一轮那张卡没到她屏幕上：%s", opened.Body)
	}
	before := decodeCoachTurn(t, opened)
	callsBefore := prov.Calls

	// 🚨 这一轮模型**两件事都做了**：又写了一张卡，还把这一步判成 done。
	// 两件都不许落地。
	hint := coachTurn(t, h, cookie, id, "给点提示")
	got := coachTurnRaw(t, hint)

	if card, present := got["coachCard"]; present {
		t.Errorf("提示那一轮换掉了她手上那张卡：%v", card)
	}
	if got["reply"] == "" {
		t.Error("提示那一轮一句话都没有")
	}
	after := decodeCoachTurn(t, hint)
	if len(after.Tasks) != len(before.Tasks) {
		t.Fatalf("任务数变了：%d → %d", len(before.Tasks), len(after.Tasks))
	}
	for i := range after.Tasks {
		if after.Tasks[i].Status != before.Tasks[i].Status {
			t.Errorf("第 %d 步被一次提示推走了：%q → %q", i, before.Tasks[i].Status, after.Tasks[i].Status)
		}
	}
	// 🚨 只打一次模型。多出来的那一次就是 cardRejectDeadTurn 白买的重试
	// （2026-09-20 线上三次提示里有两次这样）。
	if spent := prov.Calls - callsBefore; spent != 1 {
		t.Errorf("提示那一轮打了 %d 次模型，应该只打 1 次（不重试）", spent)
	}
}

// 提示那一轮**没有**新卡（正常的提示就该是这样）：一句线索，句尾没有问句、
// 也没有「请…」—— 于是它会被判 cardRejectDeadTurn，而那个判断在这里是假的：
// 她屏幕上那张卡还开着，事情多得很。
const helpTurnIsJustAClue = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
  "reply":"线索在第3段的前半句，它讲的是白天。","advance":"","focusBlock":"b3"}`

// 🚨 2026-09-20 线上走查量到的那一条：三次提示里有两次判了 cardRejectDeadTurn，
// 于是每次提示白花一次模型调用（阅读陪练占一次阅读成本的 83%），而重来那一次
// 收到的指令是「结尾要么给一张卡片，要么明确请她做一件事」—— 正好和「不要再发
// 新卡片」顶上。读代码看不出来，只有数调用次数才问得出口。
func TestReadingCoach_APlainHintDoesNotBuyARetry(t *testing.T) {
	prov := coachScripts(helpTurnOpensACard, helpTurnIsJustAClue)
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	opened := coachTurn(t, h, cookie, id, "")
	if _, ok := coachTurnRaw(t, opened)["coachCard"].(map[string]any); !ok {
		t.Fatalf("第一轮那张卡没到她屏幕上：%s", opened.Body)
	}
	callsBefore := prov.Calls

	hint := coachTurn(t, h, cookie, id, "给点提示")
	if got := coachTurnRaw(t, hint); got["reply"] == "" {
		t.Fatalf("提示那一轮一句话都没有：%s", hint.Body)
	}
	if spent := prov.Calls - callsBefore; spent != 1 {
		t.Errorf("一句普通的提示打了 %d 次模型，应该只打 1 次", spent)
	}

	// 🚨 而且不许留下「你递出去的东西没到她屏幕上」那条理由 —— 下一轮会当面
	// 告诉模型「不要再提这张卡」，而那张卡她正看着。
	for _, m := range listAiPayloads(t, h, cookie, id) {
		if why, _ := m["dropped"].(string); why != "" {
			t.Errorf("提示那一轮留下了一条丢卡理由：%q", why)
		}
	}
}

// listAiPayloads —— 转写里每条 印记 消息的 payload（没有 payload 的跳过）。
func listAiPayloads(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/readings/"+id+"/messages", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("messages = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var listed struct {
		Messages []struct {
			Role    string          `json:"role"`
			Payload json.RawMessage `json:"payload"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode messages: %v — body=%s", err, rec.Body)
	}
	var out []map[string]any
	for _, m := range listed.Messages {
		if m.Role != "ai" || len(m.Payload) == 0 {
			continue
		}
		var env map[string]any
		if err := json.Unmarshal(m.Payload, &env); err != nil {
			t.Fatalf("payload is not JSON: %v", err)
		}
		out = append(out, env)
	}
	return out
}
