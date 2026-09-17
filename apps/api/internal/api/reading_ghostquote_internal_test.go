package api

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 这一份测的是「话里那句引文到底在不在她屏幕上」。用例全部是产品负责人
// 2026-09-17 截图里的原话 —— 见 reading_ghostquote.go。

func ghostBlocks() []Block {
	return []Block{
		{ID: "b1", Text: "Aid groups are scrambling to help people caught in the war between Israel and Hamas."},
		{ID: "b2", Text: "Israel is a country in the Middle East."},
		{ID: "b3", Text: "The United Nations (U.N.) and aid groups are worried about working in the area. Hundreds of people have been killed. Thousands have been wounded. Aid groups say there are needs both in Gaza and Israel. They are begging to be allowed into Gaza to help Palestinians stuck in the middle of the fighting."},
	}
}

func ghostCard() *coachCard {
	return &coachCard{
		Type:   coachCardLabelRoles,
		Prompt: coachLabelBoardPrompt,
		Options: []coachCardOption{
			{BlockID: "b3", Quote: "Aid groups say there are needs both in Gaza and Israel."},
			{BlockID: "b3", Quote: "They are begging to be allowed into Gaza to help Palestinians stuck in the middle of the fighting."},
			{BlockID: "b1", Quote: "Aid groups are scrambling to help people caught in the war between Israel and Hamas."},
			{BlockID: "b2", Quote: "Israel is a country in the Middle East."},
		},
	}
}

func readGhostCorpus(card *coachCard, msgs []sqlc.AtomMessage) string {
	return readingQuoteCorpus("Aid groups scramble to help as Israel-Hamas war intensifies",
		ghostBlocks(), readingOutline{}, nil, msgs, card, nil)
}

// 截图里那一句。原文是两句（「Hundreds of people have been killed.」
// 「Thousands have been wounded.」），印记 把它们压成一句自己的话再加引号
// 交给她去板上找 —— 板上没有，原文里也没有。
func TestGhostQuoteCatchesASentenceThatIsNotOnTheBoard(t *testing.T) {
	reply := "再看证据那一格。请你把「Hundreds killed, Thousands wounded.」挪到证据格子下面，然后把 begging 那句挪到主张那一格。"
	got := firstGhostQuote(reply, readGhostCorpus(ghostCard(), nil))
	if got == "" {
		t.Fatal("板上没有、原文里也没有的那一句被放过去了 —— 她会在板上找一句不存在的话")
	}
	if !strings.Contains(got, "Hundreds killed") {
		t.Errorf("抓到的是别的句子：%q", got)
	}
}

// 照着板上那一条逐字抄的，一个都不许拦。这是这块板上正常的一轮。
func TestGhostQuoteLetsThroughTheBoardsOwnSentences(t *testing.T) {
	reply := "我们一件一件来。「Aid groups say there are needs both in Gaza and Israel.」放得很准，" +
		"它是援助组织自己说的话。「They are begging to be allowed into Gaza to help Palestinians stuck in the middle of the fighting.」这一句不是证据。"
	if got := firstGhostQuote(reply, readGhostCorpus(ghostCard(), nil)); got != "" {
		t.Errorf("板上就摆着的两句被判成幻引：%q", got)
	}
}

// 原文里的句子，哪怕这张卡片上没摆，也照常放过 —— 印记 指着文章讲是它的本职。
func TestGhostQuoteLetsThroughTheArticlesOwnSentences(t *testing.T) {
	reply := "先回到第 3 段：「Hundreds of people have been killed.」这一句是数字。"
	if got := firstGhostQuote(reply, readGhostCorpus(ghostCard(), nil)); got != "" {
		t.Errorf("原文里的句子被判成幻引：%q", got)
	}
}

// 她自己说过的话也算数：印记 复述她刚说的一句是正常教学。
func TestGhostQuoteLetsThroughHerOwnWords(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		{Role: "student", Content: "我觉得这篇是在讲援助进不去这件事"},
		{Role: "ai", Content: "印记自己说过的话不算数，它正是幻引的来源"},
	}
	reply := "你刚才说「我觉得这篇是在讲援助进不去这件事」，这个判断站得住。"
	if got := firstGhostQuote(reply, readGhostCorpus(ghostCard(), msgs)); got != "" {
		t.Errorf("她自己的话被判成幻引：%q", got)
	}
}

// 🚨 印记 **自己**上一轮说过的那句话不进语料。收进来就是自证：它顺着自己的
// 上文往下引，永远判得过。
func TestGhostQuoteDoesNotTrustItsOwnEarlierWords(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		{Role: "ai", Content: "这一段真正在撑住主张的，是那几个数字："},
	}
	reply := "照我上一轮说的，「这一段真正在撑住主张的，是那几个数字」这句你记住。"
	if got := firstGhostQuote(reply, readGhostCorpus(ghostCard(), msgs)); got == "" {
		t.Error("它引了自己上一轮的话却判过了 —— 语料里混进了 印记 自己说的字")
	}
}

// 她在板上摆完的那一份也算她说过的话（作答原样落在 payload 里）。
func TestGhostQuoteLetsThroughHerBoardPlacement(t *testing.T) {
	payload, err := json.Marshal(coachMessagePayload{Answer: &coachCardAnswer{
		Type:   coachCardLabelRoles,
		Choice: "主张：\nAid groups say there are needs both in Gaza and Israel.\n",
	}})
	if err != nil {
		t.Fatal(err)
	}
	msgs := []sqlc.AtomMessage{{Role: "student", Payload: payload}}
	reply := "你把「Aid groups say there are needs both in Gaza and Israel.」放进了主张。"
	if got := firstGhostQuote(reply, readGhostCorpus(nil, msgs)); got != "" {
		t.Errorf("她自己摆的那一句被判成幻引：%q", got)
	}
}

// 短引号是术语不是引文（「主张」「证据」），一概放过 —— 这条判据在
// internal/quotematch 里，这里守的是阅读室真的靠着它。
func TestGhostQuoteIgnoresBinNames(t *testing.T) {
	reply := "「主张」那一格是作者要你接受的那句话，「证据」那一格放的是数字。"
	if got := firstGhostQuote(reply, readGhostCorpus(ghostCard(), nil)); got != "" {
		t.Errorf("格子名被判成幻引：%q", got)
	}
}

// 清单上那几步的名字和说明她在进度盘上读得到，引它们不算幻引。
func TestGhostQuoteLetsThroughTheChecklistsOwnWords(t *testing.T) {
	tasks := []sqlc.ReadingTask{
		{Label: "通读第1–4段·开篇与类比", Detail: "请通读第1–4段。这几段用宗教家、美术家的创造引出话题。"},
	}
	corpus := readingQuoteCorpus("创造宣言", ghostBlocks(), readingOutline{}, tasks, nil, nil, nil)
	reply := "这一步是「请通读第1–4段。这几段用宗教家、美术家的创造引出话题。」"
	if got := firstGhostQuote(reply, corpus); got != "" {
		t.Errorf("清单自己的说明被判成幻引：%q", got)
	}
}

// 她刚做完的那副透镜，结论也在她屏幕上。
func TestGhostQuoteLetsThroughTheLensVerdict(t *testing.T) {
	done := &readingLensDone{
		CardName:      "溯源体检",
		Quote:         "Israel is a country in the Middle East.",
		Finding:       "这一句只交代背景，不参与说服。",
		VerdictReason: "这一句说的是地理位置，还没说到援助为什么进不去。",
	}
	corpus := readingQuoteCorpus("t", ghostBlocks(), readingOutline{}, nil, nil, nil, done)
	reply := "复核说的是「这一句说的是地理位置，还没说到援助为什么进不去。」，我同意。"
	if got := firstGhostQuote(reply, corpus); got != "" {
		t.Errorf("透镜复核的结论被判成幻引：%q", got)
	}
}
