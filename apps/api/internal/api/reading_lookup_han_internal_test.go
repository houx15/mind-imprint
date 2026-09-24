package api

import (
	"reflect"
	"strings"
	"testing"
)

// 🚨 **中文查词以前一次都不可能成功。**
//
// lookupTokens 只认 a–z，所以 lookupTokens("蓑") 是空切片，而 lookupCardFor
// 开头就是「want 为空 → 返回 nil」。任何 Subject == "word" 的中文工具都会走进
// 「没有一张卡讲她点的那个词」那条失败分支，返回 model_unavailable。
//
// 之所以一直没暴露：查词（lookup）是 Lang "en"，中文那边从来没有词一级的工具。
// 产品负责人 2026-09-24：「I think we need to make chinese lookup different
// from english.」—— 要做这件事，先得让这条判据认识汉字。
func TestLookupTokensSplitsHanCharByChar(t *testing.T) {
	cases := map[string][]string{
		"蓑":    {"蓑"},
		"蓑笠":   {"蓑", "笠"},
		"俄而":   {"俄", "而"},
		"未若":   {"未", "若"},
		"孤舟蓑笠翁": {"孤", "舟", "蓑", "笠", "翁"},
		// 标点不是 token，和拉丁那边一样。
		"白雪，纷纷": {"白", "雪", "纷", "纷"},
	}
	for in, want := range cases {
		if got := lookupTokens(in); !reflect.DeepEqual(got, want) {
			t.Errorf("lookupTokens(%q) = %v，要的是 %v", in, got, want)
		}
	}
}

// 英文那一侧**一个 token 都不许变** —— 查词是线上已经在用的一件工具，
// 而它的匹配判据就建在这个函数上。
func TestLookupTokensEnglishUnchanged(t *testing.T) {
	cases := map[string][]string{
		"prolonged":          {"prolonged"},
		"Prolonged":          {"prolonged"},
		"well-being":         {"well-being"},
		"don't":              {"don't"},
		"starved to death":   {"starved", "to", "death"},
		"the U.S. economy":   {"the", "u", "s", "economy"},
		"1990":               nil,
		"":                   nil,
		"  spaced   out    ": {"spaced", "out"},
	}
	for in, want := range cases {
		got := lookupTokens(in)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("lookupTokens(%q) = %v，要的是 %v", in, got, want)
		}
	}
}

// 混排：一句中文里夹一个英文词，两边都要切得出来。
func TestLookupTokensMixed(t *testing.T) {
	got := lookupTokens("用 AI 写的诗")
	want := []string{"用", "ai", "写", "的", "诗"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lookupTokens 混排 = %v，要的是 %v", got, want)
	}
}

// 🚨 真正要守的是**这一层**：她点的那个字，要能在讲整个词的那张卡上认出来。
//
// 这就是查词在英文那边的判据（点 starved，卡片讲 starved to death 算对），
// 现在它对中文也必须成立：点「蓑」，卡片讲「蓑笠」算对。
func TestLookupCardForFindsHanWordInsidePhrase(t *testing.T) {
	cards := []readingWord{
		{Term: "孤舟", Meaning: "一条小船"},
		{Term: "蓑笠", Meaning: "草编的雨衣和斗笠"},
	}
	for _, tapped := range []string{"蓑", "笠", "蓑笠"} {
		got := lookupCardFor(cards, tapped)
		if got == nil {
			t.Fatalf("点「%s」没有对上任何一张卡 —— 这一件工具会整件失败", tapped)
		}
		if got.Term != "蓑笠" {
			t.Errorf("点「%s」对上的是 %q，要的是「蓑笠」", tapped, got.Term)
		}
	}
	// 反方向：这一段里没有的字对不上，仍然是 nil。
	if lookupCardFor(cards, "雪") != nil {
		t.Error("点一个卡片上没有的字，却对上了一张卡")
	}
}

// 🚨 顺序要紧：「笠蓑」不是「蓑笠」。判据是「连着出现」，不是「都出现过」。
func TestLookupCardForHanRequiresContiguousOrder(t *testing.T) {
	cards := []readingWord{{Term: "蓑笠", Meaning: "草编的雨衣和斗笠"}}
	if lookupCardFor(cards, "笠蓑") != nil {
		t.Error("「笠蓑」对上了「蓑笠」—— 判据退化成了「这几个字都出现过」")
	}
}

// 中文的词卡照旧要回段落里逐字核对。这一条守的是 parseWordCards 对中文一样有效
// —— 它用的是 strings.Index，本来就不分语言，但没有测试钉住这件事。
func TestParseWordCardsWorksOnClassicalChinese(t *testing.T) {
	para := "俄而雪骤，公欣然曰：「白雪纷纷何所似？」"
	body := `{"words":[{"term":"俄而","pos":"副词","meaning":"不久，一会儿"},` +
		`{"term":"骤","pos":"动词","meaning":"急，猛"},` +
		`{"term":"子曰","pos":"动词","meaning":"这三个字不在这一段里"}]}`
	got, ok := parseWordCards(body, para)
	if !ok {
		t.Fatal("一张中文词卡都没活下来")
	}
	if len(got) != 2 {
		t.Fatalf("留下了 %d 张卡，要的是 2 张（第三张不在段落里，该丢）", len(got))
	}
	for _, w := range got {
		if !strings.Contains(para, w.Term) {
			t.Errorf("卡片 %q 不在段落里却留下来了 —— 荧光笔无处可落", w.Term)
		}
	}
}
