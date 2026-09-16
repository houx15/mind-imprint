package interest

import (
	"strings"
	"testing"
)

// digCandidates 是这些用例里那份「阅读库」。名单以外的 slug 一律进不来 ——
// 这正是 2026-09-16 那条「不许推荐不存在的文章」在代码里的形态。
var digCandidates = []LibraryCandidate{
	{Slug: "coral-refuge", Title: "珊瑚的避难所", Reason: "一片四平方公里的海域能不能救一整片珊瑚。"},
	{Slug: "sea-temp", Title: "谁在量海水的温度", Reason: "民间温度计和卫星数据对不上的时候。"},
}

// dig_test —— 「继续深挖」里读代码看不出对错的那几条。

func TestBuildDigPromptCentresOnHerOwnWords(t *testing.T) {
	// 🚨 她的原话是这次调用**唯一真正重要的输入**。少了它们，模型只能围着一个
	// 词泛泛地想 —— 而那正是原型那四个空动词的来源。
	ev := []string{"我读到面积那一段才反应过来，四平方公里其实很小。", "一个避难所不是一个计划。"}
	system, user := BuildDigPrompt("样本代表性", "你现在会先问这个例子能代表多少。", ev, digCandidates)

	if !strings.Contains(system, "think") || !strings.Contains(system, "make") {
		t.Error("system prompt 没有说清四种种子")
	}
	for _, e := range ev {
		if !strings.Contains(user, e) {
			t.Errorf("她的原话没有进 prompt：%q", e)
		}
	}
	if !strings.Contains(user, "样本代表性") {
		t.Error("关键词没有进 prompt")
	}
}

func TestBuildDigPromptSaysSoWhenThereAreNoWords(t *testing.T) {
	// 不该发生（evidence 是 NOT NULL），但如果发生了，说实话比编一句强。
	_, user := BuildDigPrompt("样本代表性", "", nil, digCandidates)
	if !strings.Contains(user, "没有留下原话") {
		t.Errorf("没有原话时应该说出来：\n%s", user)
	}
}

func TestParseDigReplyKeepsOnePerKind(t *testing.T) {
	raw := "```json\n" + `{"seeds":[
	  {"kind":"think","text":"四平方公里凭什么代表一整片海？","why":"你自己写过面积那一段。"},
	  {"kind":"think","text":"重复的一颗","why":"x"},
	  {"kind":"read","slug":"coral-refuge","why":"y"},
	  {"kind":"write","text":"一个避难所不是一个计划","why":"你写过这句。"},
	  {"kind":"make","text":"用一支五十块的温度计记录一个月水温","why":"z"}
	]}` + "\n```"
	got, err := ParseDigReply(raw, digCandidates)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != DigSeedCount {
		t.Fatalf("解析出 %d 颗，want %d：%+v", len(got), DigSeedCount, got)
	}
	seen := map[DigKind]bool{}
	for _, s := range got {
		if seen[s.Kind] {
			t.Errorf("同一种出现了两次：%s", s.Kind)
		}
		seen[s.Kind] = true
	}
}

func TestParseDigReplyDropsUnknownKindAndEmptyText(t *testing.T) {
	// text 会被当作标题直接送进创建接口 —— 空的会建出一个无名的房间。
	raw := `{"seeds":[
	  {"kind":"vibes","text":"随便想想","why":""},
	  {"kind":"read","slug":"   ","why":""},
	  {"kind":"write","text":"一个避难所不是一个计划","why":""}
	]}`
	got, err := ParseDigReply(raw, digCandidates)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Kind != DigWrite {
		t.Fatalf("应该只留下 write 那颗：%+v", got)
	}
}

// 解析失败就没有种子。**绝不摆四个通用动词顶上** —— 那正是这套东西要取代的
// 东西（memory: ai-errors-must-surface-never-fake）。
func TestParseDigReplyErrorsInsteadOfFallingBackToGenericVerbs(t *testing.T) {
	for _, raw := range []string{"", "抱歉，我无法完成。", "{not json", `{"seeds":[]}`} {
		if _, err := ParseDigReply(raw, digCandidates); err == nil {
			t.Errorf("垃圾输入 %q 没有报错", raw)
		}
	}
}

func TestIsDigKind(t *testing.T) {
	for _, ok := range []string{"think", "read", "write", "make"} {
		if !IsDigKind(ok) {
			t.Errorf("%q 应该是一种种子", ok)
		}
	}
	for _, bad := range []string{"", "Think", "ask", "project"} {
		if IsDigKind(bad) {
			t.Errorf("%q 不该被当成种子类型", bad)
		}
	}
}

// 🚨 这是 2026-09-16 那条裁定在代码里的判据。产品负责人的原话：
//
//	> in the interest tree, sometimes we would recommend some papers that don't
//	> exist. we can only recommend readings in our database, if there is not
//	> suitable ones, then we don't recommend. don't fake these articles.
//
// prompt 里当然也写了，但一句 prompt 里的「必须」如果代码里验不了，它就只是
// 一句期望（[[prompt-output-must-be-verifiable-2026-09-03]]）。模型编一个
// slug、或者干脆写一个标题不给 slug，结果都必须一样：这一颗不存在。
func TestParseDigReplyDropsAReadSeedThatIsNotInTheLibrary(t *testing.T) {
	cases := map[string]string{
		"编了一个 slug":     `{"kind":"read","slug":"deep-sea-mining-2031","why":"y"}`,
		"只给了一个标题":       `{"kind":"read","text":"深海采矿的隐性成本：一篇综述","why":"y"}`,
		"slug 看着像但不在库里": `{"kind":"read","slug":"coral-refuge-2","why":"y"}`,
	}
	for name, seed := range cases {
		raw := `{"seeds":[` + seed + `,{"kind":"write","text":"一个避难所不是一个计划","why":""}]}`
		got, err := ParseDigReply(raw, digCandidates)
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		for _, s := range got {
			if s.Kind == DigRead {
				t.Errorf("%s: 库里没有这一篇，却留下了一颗「去读」：%+v", name, s)
			}
		}
	}
}

// 挑中了库里那一篇，text 用的必须是**目录里的真标题**，不是模型写的那句。
// 模型写的那句就是「一篇不存在的论文」进来的那个口子。
func TestParseDigReplyUsesTheCatalogueTitleNotTheModelsOwn(t *testing.T) {
	raw := `{"seeds":[{"kind":"read","slug":"coral-refuge","text":"珊瑚避难所研究进展（2031）","why":"你写过面积那一段。"}]}`
	got, err := ParseDigReply(raw, digCandidates)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 颗，got %+v", got)
	}
	if got[0].LibrarySlug != "coral-refuge" {
		t.Errorf("slug = %q，want coral-refuge", got[0].LibrarySlug)
	}
	if got[0].Text != "珊瑚的避难所" {
		t.Errorf("text = %q，want 目录里的真标题「珊瑚的避难所」", got[0].Text)
	}
}

// 库里一篇候选都没有的时候，prompt 要**说出来**。不说的话模型只会照着示范
// 编一个 slug —— 而那一颗会被解析那一侧丢掉，她看到的是没有理由的三颗。
func TestBuildDigPromptSaysSoWhenTheLibraryHasNothing(t *testing.T) {
	_, user := BuildDigPrompt("样本代表性", "", []string{"一个避难所不是一个计划。"}, nil)
	if !strings.Contains(user, "这次一篇都没有") {
		t.Errorf("候选为空时应该说出来：\n%s", user)
	}
}

// 候选名单本身要进 prompt，否则模型是在猜 slug。
func TestBuildDigPromptCarriesTheCandidateSlugs(t *testing.T) {
	_, user := BuildDigPrompt("珊瑚", "", []string{"一个避难所不是一个计划。"}, digCandidates)
	for _, c := range digCandidates {
		if !strings.Contains(user, c.Slug) {
			t.Errorf("候选 slug 没有进 prompt：%q", c.Slug)
		}
		if !strings.Contains(user, c.Title) {
			t.Errorf("候选标题没有进 prompt：%q", c.Title)
		}
	}
}
