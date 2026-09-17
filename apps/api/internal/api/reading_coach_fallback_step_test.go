package api_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// 2026-09-17 入口走查（分级阅读库那篇 PRO/CON）：在「先预测」那一步，印记说
// 「写在下面这张卡上」，却没给卡；兜底拼出来的是一块「关键主张 / 证据」板，
// 格子里是标题和署名。话要她写，卡要她摆 —— 第一批反馈里的第 2 条。
// 标注板只属于「拆开作者的论证」那一步；别的步骤上兜底按话里的动词给卡。
func TestCoachFallbackOnPredictStepIsAWritingCardNotABoard(t *testing.T) {
	const reply = `{"reply":"你猜的是数字，不是理由。说理由：电费涨了，它凭什么说这全是数据中心的锅？写在下面这张卡上。","advance":"","focusBlock":"","card":null}`
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	if _, err := q.ReplaceReadingTasks(context.Background(), sqlc.ReplaceReadingTasksParams{
		AtomID:    uuid.MustParse(id),
		Positions: []int32{0, 1, 2},
		Kinds:     []string{"predict", "read", "label"},
		Labels:    []string{"先预测", "通读第1–3段·现象", "拆开作者的论证"},
		Details:   []string{"只看标题猜一猜。", "请通读第1–3段。", ""},
		BlockIds:  []string{"", "b1", ""},
	}); err != nil {
		t.Fatal(err)
	}

	out := coachTurnRaw(t, coachTurn(t, h, cookie, id, "我猜他们会说太费电了"))
	card, ok := out["coachCard"].(map[string]any)
	if !ok {
		t.Fatalf("话里说了「下面这张卡」，屏幕上却没有卡：%s", rec2s(out))
	}
	if card["type"] != "short_text" {
		t.Errorf("先预测那一步兜底给的是 %v，要的是 short_text（话里请她写）：%s", card["type"], rec2s(out))
	}
}

// 反过来那一半（同一天第二轮走查）：精读那一步印记说「把每一句拖到它该在的角色里」，
// 板被上一版收成「只在标注步建」之后，屏幕上一块板都没有。话里说的是板，就兜一块板。
func TestCoachFallbackOnFocusStepBuildsTheBoardItPromised(t *testing.T) {
	const reply = `{"reply":"第3段是整篇的关键。下面这张卡上有几句话，把每一句拖到它该在的角色里。","advance":"","focusBlock":"b3","card":null}`
	h, cookie, q, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	if _, err := q.ReplaceReadingTasks(context.Background(), sqlc.ReplaceReadingTasksParams{
		AtomID:    uuid.MustParse(id),
		Positions: []int32{0, 1},
		Kinds:     []string{"focus_block", "label"},
		Labels:    []string{"精读重点段落第3段", "拆开作者的论证"},
		Details:   []string{"", ""},
		BlockIds:  []string{"b3", ""},
	}); err != nil {
		t.Fatal(err)
	}
	out := coachTurnRaw(t, coachTurn(t, h, cookie, id, "我读完第3段了"))
	card, ok := out["coachCard"].(map[string]any)
	if !ok || card["type"] != "label_roles" {
		t.Fatalf("话里请她把句子拖进角色，屏幕上却是 %v：%s", out["coachCard"], rec2s(out))
	}
}
