package api

import (
	"strings"
	"testing"
)

// 语法卡的判据：每一块都要逐字来自那一句，高亮就是拿它去找的。
// 见 reading_block_grammar.go。

const grammarSentence = "These feathers, which likely helped insulate the birds from the cold, might be the key to why hesperornithiforms did not survive."

func TestGrammarCardKeepsVerbatimParts(t *testing.T) {
	body := `{"backbone":"这些羽毛可能是关键",
	  "parts":[
	    {"text":"These feathers","role":"主语","note":"整句在说的东西"},
	    {"text":"which likely helped insulate the birds from the cold","role":"定语从句","note":"修饰 feathers"},
	    {"text":"might be the key","role":"谓语+表语","note":"作者的判断，用了 might"}
	  ],
	  "points":[{"name":"非限制性定语从句","why":"逗号隔开，补充说明","example":"My sister, who lives in Paris, is a doctor.","exampleZh":"我姐姐住在巴黎，是医生。"}],
	  "meaning":"这些可能帮鸟保暖的羽毛，或许正是那一支没熬过大灭绝的原因。"}`
	got, ok := parseGrammarCard(body, grammarSentence)
	if !ok {
		t.Fatal("一张好卡被丢了")
	}
	if len(got.Parts) != 3 {
		t.Errorf("parts = %d, want 3", len(got.Parts))
	}
	for _, p := range got.Parts {
		if !strings.Contains(grammarSentence, p.Text) {
			t.Errorf("留下了一块句子里没有的字：%q", p.Text)
		}
	}
	if len(got.Points) != 1 || got.Points[0].Name != "非限制性定语从句" {
		t.Errorf("points = %+v", got.Points)
	}
}

// 🚨 核不上的那一块丢掉 —— 它在句子上无处可标，而她看到的「这一块」
// 在句子里根本指不出来。
func TestGrammarCardDropsPartsNotInTheSentence(t *testing.T) {
	body := `{"parts":[
	  {"text":"These feathers","role":"主语"},
	  {"text":"these feathers","role":"主语重复但大小写不同"},
	  {"text":"which helped keep the birds warm","role":"定语从句"},
	  {"text":"might be the key","role":"谓语"},
	  {"text":"might be the key","role":"重复"}
	]}`
	got, ok := parseGrammarCard(body, grammarSentence)
	if !ok {
		t.Fatal("还剩两块真的，不该整张丢")
	}
	if len(got.Parts) != 2 {
		t.Fatalf("parts = %+v, want 只剩逐字对得上、且不重复的两块", got.Parts)
	}
	for _, p := range got.Parts {
		if p.Text == "which helped keep the birds warm" {
			t.Error("改写过的那一块被留下了 —— 高亮落不下去")
		}
	}
}

func TestGrammarCardFailsWithFewerThanTwoParts(t *testing.T) {
	for _, body := range []string{
		`{"parts":[{"text":"These feathers","role":"主语"}]}`,
		`{"parts":[{"text":"编的","role":"主语"},{"text":"也是编的","role":"谓语"}]}`,
		`{"parts":[{"text":"These feathers","role":""},{"text":"might be the key","role":""}]}`,
		`不是 JSON`,
	} {
		if _, ok := parseGrammarCard(body, grammarSentence); ok {
			t.Errorf("应该失败（调用点会走失败分支，不把原话当散文渲染）：%s", body)
		}
	}
}

func TestGrammarCardCapsPartsAndPoints(t *testing.T) {
	body := `{"parts":[
	  {"text":"These","role":"a"},{"text":"feathers","role":"b"},{"text":"which","role":"c"},
	  {"text":"likely","role":"d"},{"text":"helped","role":"e"},{"text":"insulate","role":"f"}
	],"points":[{"name":"1"},{"name":"2"},{"name":"3"},{"name":"4"}]}`
	got, ok := parseGrammarCard(body, grammarSentence)
	if !ok {
		t.Fatal("unexpected fail")
	}
	if len(got.Parts) != grammarPartsMax || len(got.Points) != grammarPointsMax {
		t.Errorf("parts=%d points=%d，上限是 %d / %d", len(got.Parts), len(got.Points), grammarPartsMax, grammarPointsMax)
	}
}

// body 那一列存的是纯文字，不带解析器也读得懂。
func TestGrammarCardProseIsReadable(t *testing.T) {
	g := readingGrammar{
		Backbone: "羽毛是关键",
		Parts:    []readingGrammarPart{{Text: "These feathers", Role: "主语"}},
		Points:   []readingGrammarPoint{{Name: "定语从句", Why: "补充说明"}},
		Meaning:  "这些羽毛可能是原因。",
	}
	prose := grammarCardAsProse(g)
	for _, want := range []string{"主干", "These feathers", "定语从句", "这些羽毛可能是原因"} {
		if !strings.Contains(prose, want) {
			t.Errorf("纯文字里少了 %q：%s", want, prose)
		}
	}
	if strings.Contains(prose, "{") {
		t.Errorf("纯文字里混进了 JSON：%s", prose)
	}
}
