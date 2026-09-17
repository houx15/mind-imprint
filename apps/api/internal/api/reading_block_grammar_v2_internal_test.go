package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// 语法卡第二版：句法（从句 / 句子成分）· 词法 · 时态（产品负责人 2026-09-17 晚些）。
// 句子沿用 reading_block_grammar_internal_test.go 的 grammarSentence。

const v2Card = `{
  "clauses":[
    {"text":"which likely helped insulate the birds from the cold","label":"定语从句","note":"修饰 These feathers"},
    {"text":"why hesperornithiforms did not survive","label":"宾语从句","note":"作介词 to 的宾语"}
  ],
  "parts":[
    {"text":"These feathers","label":"主语"},
    {"text":"might be","label":"谓语"},
    {"text":"the key to why hesperornithiforms did not survive","label":"表语"}
  ],
  "words":[
    {"text":"likely","label":"副词","note":"表示很可能"},
    {"text":"insulate","label":"动词原形","note":"help 后面省略 to"}
  ],
  "tenses":[
    {"text":"might be","label":"情态动词表推测","note":"作者没有把话说死","example":"It might rain later.","exampleZh":"晚些可能下雨。"},
    {"text":"did not survive","label":"一般过去时","note":"说的是过去那次大灭绝"}
  ],
  "meaning":"这些可能帮鸟保暖的羽毛，或许正是那一支没熬过大灭绝的原因。"
}`

func TestGrammarCardV2KeepsAllThreeLayers(t *testing.T) {
	got, ok := parseGrammarCard(v2Card, grammarSentence)
	if !ok {
		t.Fatal("一张完整的第二版卡片被丢了")
	}
	if len(got.Clauses) != 2 || got.Clauses[0].Label != "定语从句" {
		t.Errorf("clauses = %+v", got.Clauses)
	}
	if len(got.Parts) != 3 || got.Parts[0].Label != "主语" {
		t.Errorf("parts = %+v", got.Parts)
	}
	if len(got.Words) != 2 || got.Words[0].Text != "likely" {
		t.Errorf("words = %+v", got.Words)
	}
	if len(got.Tenses) != 2 || got.Tenses[0].Example == "" {
		t.Errorf("tenses = %+v", got.Tenses)
	}
	for _, layer := range [][]readingGrammarSpan{got.Clauses, got.Parts, got.Words, got.Tenses} {
		for _, s := range layer {
			if !strings.Contains(grammarSentence, s.Text) {
				t.Errorf("留下了一段句子里没有的字：%q", s.Text)
			}
		}
	}
}

// 🚨 主句不由模型给。模型把整句当成一个「从句」交上来，那一条要丢掉 —— 否则
// 界面上整句都是从句的颜色，主句一个字都没有。
func TestGrammarCardDropsAClauseThatIsTheWholeSentence(t *testing.T) {
	whole, _ := json.Marshal(grammarSentence)
	body := `{"clauses":[{"text":` + string(whole) + `,"label":"主句"},
	  {"text":"which likely helped insulate the birds from the cold","label":"定语从句"}],
	  "parts":[{"text":"These feathers","label":"主语"},{"text":"might be","label":"谓语"}]}`
	got, ok := parseGrammarCard(body, grammarSentence)
	if !ok {
		t.Fatal("unexpected fail")
	}
	if len(got.Clauses) != 1 || got.Clauses[0].Label != "定语从句" {
		t.Errorf("整句那一条没被丢掉：%+v", got.Clauses)
	}
}

// 套在另一个从句里面的从句画不出来（一层只有一种底色），丢掉里面那个。
func TestGrammarCardDropsNestedClauses(t *testing.T) {
	body := `{"clauses":[
	  {"text":"which likely helped insulate the birds from the cold","label":"定语从句"},
	  {"text":"helped insulate the birds","label":"不该出现"}],
	  "parts":[{"text":"These feathers","label":"主语"},{"text":"might be","label":"谓语"}]}`
	got, _ := parseGrammarCard(body, grammarSentence)
	if len(got.Clauses) != 1 {
		t.Errorf("套在里面的那一条没被丢掉：%+v", got.Clauses)
	}
}

// 简单句没有从句、时态也普通：只要主语和谓语在，卡片就成立。
func TestGrammarCardSimpleSentenceNeedsOnlyParts(t *testing.T) {
	sentence := "The analysis revealed two smaller feathers."
	body := `{"clauses":[],"parts":[{"text":"The analysis","label":"主语"},{"text":"revealed","label":"谓语"},{"text":"two smaller feathers","label":"宾语"}],"tenses":[]}`
	got, ok := parseGrammarCard(body, sentence)
	if !ok || len(got.Parts) != 3 || len(got.Clauses) != 0 {
		t.Errorf("简单句的卡片没留下来：ok=%v %+v", ok, got)
	}
}

// 词法那一层同样逐字核对、大小写敏感：还原成原形的词在句子里找不到。
func TestGrammarCardWordsMustBeVerbatim(t *testing.T) {
	body := `{"parts":[{"text":"These feathers","label":"主语"},{"text":"might be","label":"谓语"}],
	  "words":[{"text":"help","label":"动词"},{"text":"Likely","label":"副词"},{"text":"likely","label":"副词"}]}`
	got, _ := parseGrammarCard(body, grammarSentence)
	// help 是 helped 的一部分 —— 逐字确实出现了，这一条留着是对的（界面标的就是那几个字母）。
	var texts []string
	for _, w := range got.Words {
		texts = append(texts, w.Text)
	}
	joined := strings.Join(texts, ",")
	if strings.Contains(joined, "Likely") || !strings.Contains(joined, "likely") {
		t.Errorf("words = %v —— 大写的 Likely 不在句子里，小写的 likely 在", texts)
	}
}

func TestGrammarCardTrimsEdgeCommas(t *testing.T) {
	body := `{"parts":[{"text":"These feathers","label":"主语"},
	  {"text":", which likely helped insulate the birds from the cold,","label":"定语"}]}`
	got, ok := parseGrammarCard(body, grammarSentence)
	if !ok || got.Parts[1].Text != "which likely helped insulate the birds from the cold" {
		t.Errorf("两头的逗号没去掉：ok=%v %+v", ok, got.Parts)
	}
}

// 实测（线上 2026-09-17）：并列句里的 and 被标成「插入语」。
func TestGrammarCardDropsABareConjunction(t *testing.T) {
	sentence := "Plants were unable to photosynthesize, and many animals starved to death."
	body := `{"parts":[{"text":"Plants","label":"主语"},{"text":"were unable to photosynthesize","label":"谓语"},
	  {"text":"and","label":"插入语"},{"text":"many animals","label":"主语"},{"text":"starved to death","label":"谓语"}]}`
	got, ok := parseGrammarCard(body, sentence)
	if !ok || len(got.Parts) != 4 {
		t.Fatalf("ok=%v parts=%+v", ok, got.Parts)
	}
	for _, p := range got.Parts {
		if p.Text == "and" {
			t.Errorf("单独一个连词被当成了成分")
		}
	}
}
