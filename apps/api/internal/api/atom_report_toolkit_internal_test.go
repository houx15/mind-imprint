package api

import (
	"encoding/json"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 阅读报告上「段落工具」那一节。全是确定性的，所以全都测得到。

func toolNote(t *testing.T, tool, block, subject string, data any) sqlc.ReadingBlockNote {
	t.Helper()
	n := sqlc.ReadingBlockNote{Tool: tool, BlockID: block, Subject: subject, Body: "x"}
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		n.Data = b
	}
	return n
}

func toolAnswerMsg(t *testing.T, typ, prompt, choice string) sqlc.AtomMessage {
	t.Helper()
	b, err := json.Marshal(coachMessagePayload{Answer: &coachCardAnswer{Type: typ, Prompt: prompt, Choice: choice}})
	if err != nil {
		t.Fatal(err)
	}
	return sqlc.AtomMessage{Role: "student", Payload: b}
}

func TestReportToolkitCountsParagraphsPerTool(t *testing.T) {
	notes := []sqlc.ReadingBlockNote{
		toolNote(t, "translate", "b1", "", nil),
		toolNote(t, "translate", "b3", "", nil),
		// 同一段用了两次（两个句子）只算一段。
		toolNote(t, "grammar", "b2", "句子一。", nil),
		toolNote(t, "grammar", "b2", "句子二。", nil),
		// 并掉的「把握度」：目录里没有名字，不摆。
		toolNote(t, "hedge", "b4", "", nil),
	}
	got := buildReportToolkit("en", notes, nil)
	if got == nil {
		t.Fatal("她用过工具，这一节不该是空的")
	}
	want := map[string]int{"翻译": 2, "语法": 1}
	if len(got.Tools) != len(want) {
		t.Fatalf("tools = %+v", got.Tools)
	}
	for _, u := range got.Tools {
		if want[u.Label] != u.Blocks {
			t.Errorf("%s 用了 %d 段，want %d", u.Label, u.Blocks, want[u.Label])
		}
	}
	// 顺序跟工具目录走：翻译在语法前面。
	if got.Tools[0].Label != "翻译" {
		t.Errorf("顺序不对：%+v", got.Tools)
	}
}

func TestReportToolkitCollectsWordsFromBothWordTools(t *testing.T) {
	notes := []sqlc.ReadingBlockNote{
		toolNote(t, "vocabulary", "b1", "", map[string]any{"words": []readingWord{
			{Term: "scramble", Meaning: "争先恐后"}, {Term: "intensify", Meaning: "加剧"},
		}}),
		// 她自己查的那个词，大小写不同但是同一个 —— 只留一张。
		toolNote(t, "lookup", "b2", "Scramble", map[string]any{"words": []readingWord{
			{Term: "Scramble", Meaning: "争先恐后"},
		}}),
		toolNote(t, "lookup", "b2", "coprolite", map[string]any{"words": []readingWord{
			{Term: "coprolite", Meaning: "粪化石"},
		}}),
	}
	got := buildReportToolkit("en", notes, nil)
	if got == nil || len(got.Words) != 3 {
		t.Fatalf("words = %+v, want scramble / intensify / coprolite", got)
	}
}

func TestReportToolkitCapsWords(t *testing.T) {
	words := make([]readingWord, 0, 20)
	for i := 0; i < 20; i++ {
		words = append(words, readingWord{Term: "w" + itoaSmall(i), Meaning: "m"})
	}
	got := buildReportToolkit("en", []sqlc.ReadingBlockNote{toolNote(t, "vocabulary", "b1", "", map[string]any{"words": words})}, nil)
	if len(got.Words) != reportToolkitWordsMax {
		t.Errorf("words = %d, want the cap %d", len(got.Words), reportToolkitWordsMax)
	}
}

func TestReportToolkitGrammarKeepsTheSentenceAndPointNames(t *testing.T) {
	g := readingGrammar{
		Parts:  []readingGrammarPart{{Text: "These feathers", Role: "主语"}, {Text: "might be", Role: "谓语"}},
		Points: []readingGrammarPoint{{Name: "定语从句"}, {Name: "情态动词表推测"}},
	}
	notes := []sqlc.ReadingBlockNote{toolNote(t, "grammar", "b2", "These feathers might be the key.", map[string]any{"grammar": g})}
	got := buildReportToolkit("en", notes, nil)
	if got == nil || len(got.Grammar) != 1 {
		t.Fatalf("grammar = %+v", got)
	}
	if got.Grammar[0].Sentence != "These feathers might be the key." || len(got.Grammar[0].Points) != 2 {
		t.Errorf("grammar[0] = %+v", got.Grammar[0])
	}
}

// 🚨 writings 里只有她写的那一段，工具那一行单独放 —— 界面据此标清谁说的。
// 别的卡片回答（板、选句）不进这一节。
func TestReportToolkitWritingsAreOnlyHerToolAnswers(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		toolAnswerMsg(t, blockToolAnswerType, "仿写 · 第2段：先说大家以为的，再用实际上翻过来", "很多人以为饭钱都给了厨师。实际上……"),
		toolAnswerMsg(t, blockToolAnswerType, "想一想 · 第4段：这个数字可信吗？", "不太可信。"),
		toolAnswerMsg(t, coachCardLabelRoles, "分析下列句子", "关键主张：\n某句。"),
		toolAnswerMsg(t, blockToolAnswerType, "仿写 · 第3段", "   "),
		{Role: "student", Content: "普通的一句话"},
		{Role: "ai", Content: "印记说的"},
	}
	got := buildReportToolkit("zh", nil, msgs)
	if got == nil || len(got.Writings) != 2 {
		t.Fatalf("writings = %+v, want 两段", got)
	}
	if got.Writings[0].Tool != "仿写" || got.Writings[0].Text != "很多人以为饭钱都给了厨师。实际上……" {
		t.Errorf("writings[0] = %+v", got.Writings[0])
	}
	if got.Writings[1].Tool != "想一想" {
		t.Errorf("writings[1] = %+v", got.Writings[1])
	}
}

// 什么都没做就是 nil：这一节整个不显示，不是一个空框。
func TestReportToolkitIsNilWhenNothingWasDone(t *testing.T) {
	if got := buildReportToolkit("en", nil, []sqlc.AtomMessage{{Role: "student", Content: "hi"}}); got != nil {
		t.Errorf("toolkit = %+v, want nil", got)
	}
}
