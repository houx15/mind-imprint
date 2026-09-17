package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// 同事 2026-09-17 的阅读模块 PRD：报道有自己的板（事实 / 引述 / 解释）和一条
// 时间线（排序板）。这两条走的是真 handler，因为格子是在存进转写之前才换的。

const reportArticle = `暴雨后的城东

周五夜里，暴雨抵达城东，河水在两小时内漫过了堤岸。

市政府在周三就发布了撤离通知，周四起全区学校停课。

“我们不会离开这里。”一位住在河边的老人对记者说。

到了周日，城东大部分家庭恢复了供电。`

func setupReportReading(t *testing.T, reply string, kinds, labels []string) (http.Handler, *http.Cookie, string, *sqlc.Queries) {
	t.Helper()
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "暴雨后的城东", reportArticle)
	ctx := context.Background()
	pos := make([]int32, len(kinds))
	empty := make([]string, len(kinds))
	for i := range kinds {
		pos[i] = int32(i)
	}
	if _, err := q.ReplaceReadingTasks(ctx, sqlc.ReplaceReadingTasksParams{
		AtomID: uuid.MustParse(id), Positions: pos, Kinds: kinds,
		Labels: labels, Details: empty, BlockIds: empty,
	}); err != nil {
		t.Fatal(err)
	}
	outline, _ := json.Marshal(map[string]any{"oneLine": "暴雨之后城东怎么样了", "genre": "report"})
	if _, err := q.UpdateReadingSourceOutline(ctx, sqlc.UpdateReadingSourceOutlineParams{
		AtomID: uuid.MustParse(id), Outline: outline,
	}); err != nil {
		t.Fatal(err)
	}
	return h, cookie, id, q
}

func TestReportLabelBoardUsesReportBins(t *testing.T) {
	const reply = `{"reply":"这几句来源不一样。","advance":"","focusBlock":"b2","card":{"type":"label_roles","prompt":"判断下列句子哪句是主张，哪句是证据。","binSet":"basic","options":[{"blockId":"b2","quote":"周五夜里，暴雨抵达城东"},{"blockId":"b4","quote":"“我们不会离开这里。”一位住在河边的老人对记者说。"}]}}`
	h, cookie, id, _ := setupReportReading(t, reply, []string{"label"}, []string{"分清事实与说法"})
	out := coachTurnRaw(t, coachTurn(t, h, cookie, id, "我读完了"))
	card, ok := out["coachCard"].(map[string]any)
	if !ok {
		t.Fatalf("报道上的标注板没发出来：%s", rec2s(out))
	}
	labels, _ := json.Marshal(card["labels"])
	if string(labels) != `["事实","引述","解释"]` {
		t.Errorf("格子 = %s，要的是事实/引述/解释", labels)
	}
	if strings.Contains(card["prompt"].(string), "主张") {
		t.Errorf("题目还在说议论文的格子：%v", card["prompt"])
	}
}

func TestSequenceStepGetsAnOrderBoardAndAdvancesWhenSubmitted(t *testing.T) {
	// 模型只说话、没给板 —— 这一步的全部内容就是那块板，服务端要兜。
	const reply = `{"reply":"这篇报道先讲了结果。","advance":"","focusBlock":"","card":null}`
	h, cookie, id, q := setupReportReading(t, reply,
		[]string{"sequence", "hunt"}, []string{"排出事件时间线", "找出关键句"})
	out := coachTurnRaw(t, coachTurn(t, h, cookie, id, "好的"))
	card, ok := out["coachCard"].(map[string]any)
	if !ok || card["type"] != "order_events" {
		t.Fatalf("排序步上没有排序板：%s", rec2s(out))
	}
	opts, _ := card["options"].([]any)
	if len(opts) < 3 {
		t.Fatalf("排序板只有 %d 件事", len(opts))
	}

	// 她排好交上来：这一步做完。
	choice := "第1：\n市政府在周三就发布了撤离通知，周四起全区学校停课。\n第2：\n周五夜里，暴雨抵达城东，河水在两小时内漫过了堤岸。\n第3：\n到了周日，城东大部分家庭恢复了供电。"
	body := `{"text":"","cardAnswer":{"type":"order_events","prompt":` + strconv.Quote(card["prompt"].(string)) +
		`,"choice":` + strconv.Quote(choice) + `}}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("POST", "/api/v1/readings/"+id+"/coach", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("交板失败：%d %s", rec.Code, rec.Body)
	}
	tasks, err := q.ListReadingTasks(context.Background(), uuid.MustParse(id))
	if err != nil {
		t.Fatal(err)
	}
	if tasks[0].Status != "done" {
		t.Errorf("排好交上来之后排序步 = %s，要 done", tasks[0].Status)
	}
}

// 议论文上模型给的排序板不发 —— 议论文那条路不多出任何东西。
func TestOrderBoardDroppedOnArgument(t *testing.T) {
	const reply = `{"reply":"我们看看几件事的先后。","advance":"","focusBlock":"","card":{"type":"order_events","prompt":"请排列下列事件。","options":[{"blockId":"b2","quote":"夏天的傍晚，如果你从市中心骑车回到郊区的家，会明显感到凉快下来。"},{"blockId":"b3","quote":"气象学上把这种现象叫做城市热岛效应。"},{"blockId":"b4","quote":"绿地和水面是相反的力量。"}]}}`
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	if _, err := q.ReplaceReadingTasks(context.Background(), sqlc.ReplaceReadingTasksParams{
		AtomID: uuid.MustParse(id), Positions: []int32{0}, Kinds: []string{"critique"},
		Labels: []string{"你怎么看"}, Details: []string{""}, BlockIds: []string{""},
	}); err != nil {
		t.Fatal(err)
	}
	out := coachTurnRaw(t, coachTurn(t, h, cookie, id, "我觉得有道理"))
	if card, ok := out["coachCard"].(map[string]any); ok && card["type"] == "order_events" {
		t.Errorf("议论文上发出了排序板：%s", rec2s(out))
	}
}
