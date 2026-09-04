package interest

import (
	"strings"
	"testing"

	"mindimprint/api/internal/disciplines"
)

// quiz_test —— 守着觉醒协议里「读代码看不出对错」的那几处。
//
// 这里不测屏幕、不测文案。测的是三件会静默出错的事：透镜连的是不是真学科、
// 一次作答被收拾成什么样、以及**什么时候不该花那次模型调用**。

func TestHookLensesAreRealDisciplines(t *testing.T) {
	// 🚨 这条是这个文件里最重要的一条。hookLenses 写的是字符串 id，打错一个字母
	// 不会有任何编译错误，只会让结果页少一片透镜 —— 而少一片透镜没有人会发现。
	for _, h := range []Hook{HookCharacter, HookCraft, HookSociety} {
		lenses := HookLenses(h)
		if len(lenses) == 0 {
			t.Fatalf("hook %q 一片透镜都没有", h)
		}
		for _, d := range lenses {
			if _, ok := disciplines.ByID(d.ID); !ok {
				t.Errorf("hook %q 连到了一个不存在的学科 %q", h, d.ID)
			}
			if d.Zh == "" || d.Asks == "" {
				t.Errorf("hook %q 的学科 %q 内容是空的", h, d.ID)
			}
		}
	}
}

func TestHookLensesCrossFields(t *testing.T) {
	// 一个钩子只连一根主枝，等于在她刚说完自己喜欢什么的那一刻就把她框死在
	// 一个方向上。每个钩子至少要跨两根枝。
	for _, h := range []Hook{HookCharacter, HookCraft, HookSociety} {
		fields := map[string]bool{}
		for _, d := range HookLenses(h) {
			fields[d.Field] = true
		}
		if len(fields) < 2 {
			t.Errorf("hook %q 的透镜全都落在一根主枝上：%v", h, fields)
		}
	}
}

func TestUnknownHookHasNoLenses(t *testing.T) {
	// 没选钩子就不该被配上三片学科 —— 那是替她做了她没做的选择。
	if got := HookLenses(Hook("")); got != nil {
		t.Errorf("空钩子应该没有透镜，得到 %v", got)
	}
	if got := HookLenses(Hook("vibes")); got != nil {
		t.Errorf("未知钩子应该没有透镜，得到 %v", got)
	}
}

func TestIsHook(t *testing.T) {
	for _, ok := range []string{"character", "craft", "society"} {
		if !IsHook(ok) {
			t.Errorf("%q 应该是一个钩子", ok)
		}
	}
	for _, bad := range []string{"", "Character", "psychology", "society "} {
		if IsHook(bad) {
			t.Errorf("%q 不该被当成钩子", bad)
		}
	}
}

func TestCleanTrimsAndTruncates(t *testing.T) {
	a := Attempt{
		Navigator: "  资深向导  ",
		Work:      "  " + strings.Repeat("长", 60) + "  ",
		Reason:    strings.Repeat("理", 300),
		Hook:      HookCraft,
	}.Clean()

	if a.Navigator != "资深向导" {
		t.Errorf("navigator 没有去掉空白：%q", a.Navigator)
	}
	if n := len([]rune(a.Work)); n != maxWorkRunes {
		t.Errorf("work 应该截到 %d 个字符，得到 %d", maxWorkRunes, n)
	}
	if n := len([]rune(a.Reason)); n != maxReasonRunes {
		t.Errorf("reason 应该截到 %d 个字符，得到 %d", maxReasonRunes, n)
	}
}

func TestCleanDropsUnknownHookButKeepsTheAttempt(t *testing.T) {
	// 一次乱填的作答仍然是数据。Clean 丢掉认不出的钩子，但**不丢掉这次作答**。
	a := Attempt{Work: "流浪地球", Reason: "他一直在算那个轨道", Hook: Hook("magic")}.Clean()
	if a.Hook != "" {
		t.Errorf("认不出的钩子应该被清空，得到 %q", a.Hook)
	}
	if a.Work == "" || a.Reason == "" {
		t.Error("Clean 不该把她填过的东西一起丢掉")
	}
}

func TestCleanFloorsNegativeAttempts(t *testing.T) {
	if got := (Attempt{ChallengeAttempts: -3}).Clean().ChallengeAttempts; got != 0 {
		t.Errorf("错误次数不该是负数，得到 %d", got)
	}
}

func TestShouldHarvestNeedsARealSentence(t *testing.T) {
	// 「很帅」里没有可摘的原话。发那次调用只有两种结果：空手而归，或者开始编。
	// 两种都不值得花那次钱，而第二种还会往她树上挂一句她无从反驳的假话。
	thin := []string{"", "很帅", "好看", "喜欢", "  帅  "}
	for _, r := range thin {
		if (Attempt{Reason: r}).Clean().ShouldHarvest() {
			t.Errorf("%q 不该触发采集", r)
		}
	}
	fat := "他经历了很多痛苦，但在关键时刻依然保持理智，做出自己的选择。"
	if !(Attempt{Reason: fat}).Clean().ShouldHarvest() {
		t.Error("一段真的理由应该触发采集")
	}
}

func TestShouldHarvestIgnoresTheWorkTitle(t *testing.T) {
	// 作品名不是兴趣信号：千万人喜欢同一部作品，理由各不相同，而理由才是那个人。
	// 所以一个只填了作品、没写理由的作答，不该因为作品名很长就被当成有信号。
	a := Attempt{Work: "《进击的巨人》里的利威尔·阿克曼", Reason: "帅"}.Clean()
	if a.ShouldHarvest() {
		t.Error("只有作品名、没有理由时不该采集")
	}
}

func TestBuildQuizPromptCarriesHerOwnWords(t *testing.T) {
	a := Attempt{
		Work:   "《进击的巨人》里的利威尔",
		Reason: "他经历了很多痛苦，但在关键时刻依然保持理智。",
		Hook:   HookCharacter,
	}.Clean()
	system, user := a.BuildQuizPrompt()

	// 同一段 system prompt —— 测试和阅读采集必须对「什么算一个好领域」有完全
	// 相同的看法，否则同一棵树上会挂着两种质量的词。BuildHarvestPrompt 的
	// system 与 kind 无关，所以直接比。
	wantSystem, _ := BuildHarvestPrompt("reading", "", "")
	if system != wantSystem {
		t.Error("测试没有复用采集的 system prompt")
	}
	if !strings.Contains(user, a.Reason) {
		t.Error("她自己写的那段没有进 prompt —— evidence 就该从这里摘")
	}
	if !strings.Contains(user, a.Work) {
		t.Error("作品没有进 prompt")
	}
	// 钩子作为线索出现（透镜的中文名），帮模型判断同一段话里哪一层是她的兴趣。
	if !strings.Contains(user, "发展心理学") {
		t.Errorf("钩子的方向没有进 prompt：\n%s", user)
	}
}

func TestBuildQuizPromptWithoutAHook(t *testing.T) {
	// 没选钩子也要能拼出 prompt —— 她的原话本身就够采了。
	a := Attempt{Work: "流浪地球", Reason: "他们真的去算那条轨道能不能成立。"}.Clean()
	_, user := a.BuildQuizPrompt()
	if strings.Contains(user, "追问方向") {
		t.Error("没选钩子时不该凭空写一个方向进去")
	}
	if !strings.Contains(user, a.Reason) {
		t.Error("她的原话必须在 prompt 里")
	}
}
