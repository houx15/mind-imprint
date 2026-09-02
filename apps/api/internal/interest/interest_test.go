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

/* ── router · T1 / T2 ──────────────────────────────────────────────────── */

func TestT1AliasHitIsFreeAndCertain(t *testing.T) {
	got := RouteCheap("代表性", nil, nil)
	if len(got) != 1 || got[0].DisciplineID != "statistical-inference" {
		t.Fatalf("别名没命中：%+v", got)
	}
	if got[0].How != HowAlias || got[0].Confidence != 1.0 {
		t.Errorf("want alias/1.0, got %s/%v", got[0].How, got[0].Confidence)
	}
	if got[0].Rationale == "" {
		t.Error("没给理由——界面要说得出凭什么这么放")
	}
}

func TestT2InheritsFromAKeywordSharingTwoSources(t *testing.T) {
	known := []Known{{
		KeywordNorm:   "例外与代表性",
		DisciplineIDs: []string{"statistical-inference"},
		SourceRefs:    []string{"r-coral", "w-coral", "n-0829"},
	}}
	got := RouteCheap("样本量够不够", []string{"r-coral", "w-coral"}, known)
	if len(got) != 1 || got[0].How != HowCooccur {
		t.Fatalf("共现没继承：%+v", got)
	}
	if got[0].DisciplineID != "statistical-inference" {
		t.Errorf("继承到了别的学科：%s", got[0].DisciplineID)
	}
	if got[0].Confidence != 0.6 {
		t.Errorf("want 0.6（推断出来的，不该和别名一样自信），got %v", got[0].Confidence)
	}
}

// 只共享一条来源不够 —— 一篇文章里同时出现的两个词经常毫无关系。
func TestT2NeedsTwoSharedSources(t *testing.T) {
	known := []Known{{
		KeywordNorm:   "例外与代表性",
		DisciplineIDs: []string{"statistical-inference"},
		SourceRefs:    []string{"r-coral"},
	}}
	if got := RouteCheap("一个完全无关的词", []string{"r-coral"}, known); got != nil {
		t.Fatalf("一条共享来源就继承了：%+v", got)
	}
}

// 返回 nil 是「该花那次模型调用了」的信号。空切片和 nil 在这里不是一回事。
func TestRouteCheapReturnsNilOnMiss(t *testing.T) {
	if got := RouteCheap("一个谁也不认识的词", nil, nil); got != nil {
		t.Fatalf("want nil so the caller knows to spend an LLM call, got %+v", got)
	}
}

// 数据里留下的旧学科 id 不该凭空变成一门学科。
func TestT2SkipsUnknownDisciplineIDs(t *testing.T) {
	known := []Known{{
		KeywordNorm:   "老词",
		DisciplineIDs: []string{"a-discipline-we-deleted"},
		SourceRefs:    []string{"r-1", "r-2"},
	}}
	if got := RouteCheap("新词", []string{"r-1", "r-2"}, known); len(got) != 0 {
		t.Fatalf("不存在的学科被继承了：%+v", got)
	}
}

func TestT2DoesNotInheritFromItself(t *testing.T) {
	known := []Known{{
		KeywordNorm:   Norm("代表性抽样"),
		DisciplineIDs: []string{"statistical-inference"},
		SourceRefs:    []string{"r-1", "r-2"},
	}}
	got := RouteCheap("代表性抽样", []string{"r-1", "r-2"}, known)
	for _, r := range got {
		if r.How == HowCooccur {
			t.Fatal("一个词从它自己身上继承了学科")
		}
	}
}

/* ── router · T3 ───────────────────────────────────────────────────────── */

func TestRoutePromptCarriesOnlyThatFieldsSix(t *testing.T) {
	_, user := BuildRoutePrompt("样本量", "我去查了那片珊瑚有多大", "formal")
	if !strings.Contains(user, "statistical-inference") {
		t.Error("同枝的候选没进 prompt")
	}
	if strings.Contains(user, "art-history") {
		t.Error("别的主枝漏进了候选——prompt 该只带六门")
	}
	if !strings.Contains(user, "我去查了那片珊瑚有多大") {
		t.Error("她的原话没进 prompt，模型就只能猜词面")
	}
	if strings.Count(user, "\n- ") != 6 {
		t.Errorf("候选不是六条，是 %d 条", strings.Count(user, "\n- "))
	}
}

// 采集器出 bug 给了个野生 field 时，退回全表好过让这个词永远悬着。
func TestRoutePromptFallsBackToAllOnUnknownField(t *testing.T) {
	_, user := BuildRoutePrompt("词", "话", "magic")
	if strings.Count(user, "\n- ") != 42 {
		t.Errorf("未知主枝没有退回全表，只给了 %d 条候选", strings.Count(user, "\n- "))
	}
}

func TestParseRouteReplyToleratesCodeFences(t *testing.T) {
	raw := "这是我的判断：\n```json\n{\"routes\":[{\"id\":\"statistical-inference\",\"why\":\"她在问代表性\"}]}\n```"
	got, err := ParseRouteReply(raw, "formal")
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(got) != 1 || got[0].DisciplineID != "statistical-inference" {
		t.Fatalf("got %+v", got)
	}
	if got[0].How != HowLLM {
		t.Errorf("want how=llm, got %s", got[0].How)
	}
	if got[0].Rationale != "她在问代表性" {
		t.Errorf("理由没带出来：%q", got[0].Rationale)
	}
}

// 模型发明了一门学科，绝不能落库。
func TestParseRouteReplyDropsUnknownIDs(t *testing.T) {
	got, err := ParseRouteReply(`{"routes":[{"id":"astrology","why":"星象"}]}`, "formal")
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("不存在的 id 被收下了：%+v", got)
	}
}

// 候选表里没有的学科出现在回话里，就是幻觉，即使那门学科真的存在。
func TestParseRouteReplyDropsOffFieldIDs(t *testing.T) {
	got, _ := ParseRouteReply(`{"routes":[{"id":"art-history","why":"…"}]}`, "formal")
	if len(got) != 0 {
		t.Fatalf("别的主枝的学科被收下了：%+v", got)
	}
}

func TestParseRouteReplyCapsAtTwo(t *testing.T) {
	raw := `{"routes":[
	  {"id":"statistical-inference","why":"a"},
	  {"id":"probability","why":"b"},
	  {"id":"calculus","why":"c"}]}`
	got, _ := ParseRouteReply(raw, "formal")
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
}

// 解析不了就报错，让调用方什么也不写 —— 绝不返回一个像样的假路由。
func TestParseRouteReplyErrorsOnGarbage(t *testing.T) {
	for _, raw := range []string{"", "模型今天不想说话", "   "} {
		if _, err := ParseRouteReply(raw, "formal"); err == nil {
			t.Errorf("ParseRouteReply(%q) 没报错，调用方会以为路由成功了", raw)
		}
	}
}

// 合法回话但一门都不合适 → 空切片 + nil。这和「解析失败」必须分得开。
func TestParseRouteReplyEmptyIsNotAnError(t *testing.T) {
	got, err := ParseRouteReply(`{"routes":[]}`, "formal")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
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

// 树的断言全靠这条：没带她原话的词直接丢掉。
func TestParseHarvestDropsKeywordsWithoutEvidence(t *testing.T) {
	raw := `{"keywords":[
	  {"zh":"例外与代表性","en":"Exception vs representative","field":"formal",
	   "note":"你现在会先问这个例子能代表多少","evidence":"我读到面积那一段才反应过来它有多小"},
	  {"zh":"气候","en":"Climate","field":"science","note":"n","evidence":""}]}`
	got, err := ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(got) != 1 || got[0].TextZh != "例外与代表性" {
		t.Fatalf("没带证据的词没被丢掉：%+v", got)
	}
}

// 模型偷懒时会拿「有」「是的」凑非空检查。
func TestParseHarvestDropsTooShortEvidence(t *testing.T) {
	raw := `{"keywords":[{"zh":"气候","en":"Climate","field":"science","note":"n","evidence":"有"}]}`
	got, _ := ParseHarvestReply(raw)
	if len(got) != 0 {
		t.Fatalf("敷衍的证据被收下了：%+v", got)
	}
}

func TestParseHarvestRejectsUnknownField(t *testing.T) {
	raw := `{"keywords":[{"zh":"占星","en":"Astrology","field":"magic","note":"n","evidence":"我一直很好奇星座"}]}`
	got, _ := ParseHarvestReply(raw)
	if len(got) != 0 {
		t.Fatalf("野生 field 被收下了：%+v", got)
	}
}

// 一篇长出八个词，一周后那棵树就是一丛灌木。
func TestParseHarvestCapsAtThree(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"keywords":[`)
	for i := 0; i < 8; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"zh":"关键词%d","en":"w%d","field":"science","note":"n","evidence":"她写下的一句原话"}`, i, i)
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
	  {"zh":"例外与代表性","en":"a","field":"formal","note":"n","evidence":"她写下的一句原话"},
	  {"zh":"例外 与 代表性","en":"b","field":"formal","note":"n","evidence":"另外一句原话"}]}`
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
