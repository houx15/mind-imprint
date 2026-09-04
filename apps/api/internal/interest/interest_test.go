package interest

import (
	"fmt"
	"strings"
	"testing"
)

/* ── merge ─────────────────────────────────────────────────────────────── */

func TestStrengthLaddersBySourceCount(t *testing.T) {
	for _, c := range []struct{ sources, want int }{
		{0, 1}, {1, 1}, {2, 2}, {3, 3}, {4, 3}, {5, 4}, {7, 4}, {8, 5}, {40, 5},
	} {
		if got := Strength(c.sources); got != c.want {
			t.Errorf("Strength(%d) = %d, want %d", c.sources, got, c.want)
		}
	}
}

// 强度是读数，不是等级 —— 它必须始终落在 1..5，否则界面上的 STR n/5 就撒谎了。
func TestStrengthIsAlwaysInRange(t *testing.T) {
	for n := -5; n < 200; n++ {
		if got := Strength(n); got < 1 || got > 5 {
			t.Fatalf("Strength(%d) = %d, 越界", n, got)
		}
	}
}

func TestSameKeywordIgnoresPunctuationAndCase(t *testing.T) {
	if !SameKeyword("例外与代表性", "例外 与 代表性") {
		t.Error("空格不该让同一个词变成两个")
	}
	if !SameKeyword("Sampling", "sampling") {
		t.Error("大小写不该让同一个词变成两个")
	}
	if SameKeyword("气候与海洋", "记忆怎么形成") {
		t.Error("两个不同的词被判成了同一个")
	}
	if SameKeyword("", "") {
		t.Error("两个空串不该被判成同一个词")
	}
}

/* ── harvest ───────────────────────────────────────────────────────────── */

func TestHarvestPromptNamesWhatKindOfThingItIs(t *testing.T) {
	_, user := BuildHarvestPrompt("reading", "红海北端那片不白化的珊瑚", "正文……")
	if !strings.Contains(user, "读完") {
		t.Error("prompt 没说这是一次阅读——什么算她的原话，读和写是不一样的")
	}
	if !strings.Contains(user, "红海北端那片不白化的珊瑚") {
		t.Error("标题没进 prompt")
	}
}

// 候选清单必须真的进 system，否则模型是在凭空猜 id。
func TestHarvestPromptCarriesTheCatalogue(t *testing.T) {
	system, _ := BuildHarvestPrompt("reading", "标题", "正文")
	for _, want := range []string{"(games)", "(finance)", "(tailoring)"} {
		if !strings.Contains(system, want) {
			t.Errorf("候选清单里没有 %s —— 模型选不到它", want)
		}
	}
	if !strings.Contains(system, "不能自己造词") {
		t.Error("system 没说这是一张闭表")
	}
}

// 树的断言全靠这条：没带她原话的词直接丢掉。
func TestParseHarvestDropsKeywordsWithoutEvidence(t *testing.T) {
	raw := `{"keywords":[
	  {"id":"games","note":"你现在会先问这个例子能代表多少","evidence":"我读到面积那一段才反应过来它有多小"},
	  {"id":"climate","note":"n","evidence":""}]}`
	got, err := ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(got) != 1 || got[0].InterestID != "games" {
		t.Fatalf("没带证据的词没被丢掉：%+v", got)
	}
}

// 🚨 闭表那条不变量的执行点。模型很会造出看起来合理的 id —— esports 像个领域，
// coral-reefs 像个话题，两个都不在表里。靠 prompt 劝它不要造是在期望模型守
// 规矩；这条测试守的是规矩本身成立。
func TestParseHarvestDropsIDsThatAreNotInTheTable(t *testing.T) {
	raw := `{"keywords":[
	  {"id":"esports","note":"n","evidence":"我每天都看比赛录像"},
	  {"id":"coral-reefs","note":"n","evidence":"珊瑚那一段我读了三遍"},
	  {"id":"","note":"n","evidence":"这也是我写下的一句话"},
	  {"id":"games","note":"n","evidence":"抽卡明明知道是坑我还是想抽"}]}`
	got, err := ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(got) != 1 || got[0].InterestID != "games" {
		t.Fatalf("表外的 id 被收下了：%+v", got)
	}
}

// 模型偷懒时会拿「有」「是的」凑非空检查。
func TestParseHarvestDropsTooShortEvidence(t *testing.T) {
	raw := `{"keywords":[{"id":"climate","note":"n","evidence":"有"}]}`
	got, _ := ParseHarvestReply(raw)
	if len(got) != 0 {
		t.Fatalf("敷衍的证据被收下了：%+v", got)
	}
}

// 一篇长出八个词，一周后那棵树就是一丛灌木。
func TestParseHarvestCapsAtThree(t *testing.T) {
	ids := []string{"games", "finance", "climate", "tailoring", "film", "poetry", "sleep", "cities"}
	var b strings.Builder
	b.WriteString(`{"keywords":[`)
	for i, id := range ids {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":%q,"note":"n","evidence":"她写下的一句原话"}`, id)
	}
	b.WriteString(`]}`)
	got, err := ParseHarvestReply(b.String())
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
}

func TestParseHarvestDedupesWithinOneRound(t *testing.T) {
	raw := `{"keywords":[
	  {"id":"games","note":"n","evidence":"她写下的一句原话"},
	  {"id":"games","note":"n","evidence":"另外一句原话"}]}`
	got, _ := ParseHarvestReply(raw)
	if len(got) != 1 {
		t.Fatalf("同一个词被收了两次：%+v", got)
	}
}

func TestParseHarvestErrorsOnGarbage(t *testing.T) {
	for _, raw := range []string{"", "抱歉，我没能理解", "```\n\n```"} {
		if _, err := ParseHarvestReply(raw); err == nil {
			t.Errorf("ParseHarvestReply(%q) 没报错，调用方会以为采集成功了", raw)
		}
	}
}

func TestParseHarvestEmptyIsNotAnError(t *testing.T) {
	got, err := ParseHarvestReply(`{"keywords":[]}`)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

/* ── KeepGrounded ───────────────────────────────────────────────────────── */

// 🚨 这一组守的是 2026-09-03 由**真模型实测**抓出来的一个 bug：模型把 prompt 里
// 的脚手架文字（「她喜欢的是：《进击的巨人》里的利威尔」）当作她的原话返回。
// 它长度合格、语义通顺、能过解析器的每一道检查 —— 然后被挂在她树上，标签写着
// 「你自己写的」。那是一句她无从反驳的谎话。
func TestKeepGroundedDropsPromptScaffoldingQuotedBackAsHerWords(t *testing.T) {
	own := "《进击的巨人》里的利威尔\n他经历了很多痛苦，但在关键时刻依然保持理智，做出自己的选择。"
	got := KeepGrounded([]Harvested{
		{InterestID: "purpose", Evidence: "他经历了很多痛苦，但在关键时刻依然保持理智，做出自己的选择。"},
		{InterestID: "emotions", Evidence: "她喜欢的是：《进击的巨人》里的利威尔"},
	}, own)

	if len(got) != 1 {
		t.Fatalf("留下了 %d 个，want 1：%+v", len(got), got)
	}
	if got[0].InterestID != "purpose" {
		t.Errorf("留错了：%q", got[0].InterestID)
	}
}

func TestKeepGroundedAcceptsWhatSheActuallyTyped(t *testing.T) {
	// 作品名也是她敲进去的，摘它是合法的。
	own := "《进击的巨人》里的利威尔\n他在关键时刻依然保持理智。"
	got := KeepGrounded([]Harvested{{Evidence: "《进击的巨人》里的利威尔"}}, own)
	if len(got) != 1 {
		t.Error("她自己敲的作品名被当成不是她的了")
	}
}

func TestKeepGroundedIgnoresRewrappedWhitespace(t *testing.T) {
	// 模型经常重新换行，那不算转述。
	own := "我读到面积那一段才反应过来，\n四平方公里其实很小。"
	got := KeepGrounded([]Harvested{
		{Evidence: "我读到面积那一段才反应过来， 四平方公里其实很小。"},
	}, own)
	if len(got) != 1 {
		t.Error("只是换行不同就被判成不是原话了")
	}
}

func TestKeepGroundedDropsParaphrase(t *testing.T) {
	own := "我读到面积那一段才反应过来，四平方公里其实很小。"
	got := KeepGrounded([]Harvested{{Evidence: "她意识到取样面积很小"}}, own)
	if len(got) != 0 {
		t.Errorf("转述被当成了原话：%+v", got)
	}
}
