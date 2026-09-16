package pbl

import (
	"strings"
	"testing"
)

/* ── 路线 ─────────────────────────────────────────────────────────────── */

// 每一步都要说清楚她判断什么。
//
// 这不是一条风格偏好：服务端的 proposePblPlan 会拒掉 decide 为空的步
// （step_without_decision）。一份种下去就过不了自己那道校验的路线，等于把一个
// 只能靠绕过校验才活得下来的东西写进产品里。
//
// 它同时是这份路线不会退化成填表向导的那条线——一步她什么都不判断，那一步就是
// 一屏要她填完点下一步的表。
func TestWebsiteRoutine_EveryStepNamesWhatSheDecides(t *testing.T) {
	steps := WebsiteRoutine()
	if len(steps) != 5 {
		t.Fatalf("路线应该是五步，实际 %d 步", len(steps))
	}
	for i, s := range steps {
		if strings.TrimSpace(s.Decide) == "" {
			t.Errorf("第 %d 步「%s」没说清楚她判断什么", i+1, s.Title)
		}
		if strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.Goal) == "" {
			t.Errorf("第 %d 步缺标题或目标", i+1)
		}
	}
}

func TestWebsiteRoutineOffersStyleImmediatelyAfterAudience(t *testing.T) {
	steps := WebsiteRoutine()
	if steps[0].Tool != "persona" || steps[1].Tool != "creative" || steps[2].Tool != "structure" {
		t.Fatal("homepage onboarding no longer follows audience, creative exploration, content structure")
	}
}

// 路线里点名的工具必须真的在工具箱里。
//
// 🚨 少了这条，改一次工具名就会让 prompt 指着一件不存在的工具——而这类错不会
// 报错，只会让印记递出一件她点开是空白的东西。
func TestWebsiteRoutine_NamesOnlyRealTools(t *testing.T) {
	for _, s := range WebsiteRoutine() {
		if s.Tool == "" {
			continue
		}
		if _, ok := LookupTool(s.Tool); !ok {
			t.Errorf("路线里的「%s」用了工具箱里没有的 %q", s.Title, s.Tool)
		}
	}
}

/* ── prompt ───────────────────────────────────────────────────────────── */

// 主页项目拿到的是路线，不是通用那份「什么时候递哪一件」。
func TestCoachPrompt_WebsiteGetsTheRoutineInsteadOfTheGenericMoments(t *testing.T) {
	p := coachPrompt("website")

	for _, want := range []string{
		"我想让谁，看见我的什么？", // 驱动问题
		"【创作流程】",
		"persona", "sites", "look", // 这个项目独有的三件
		"site_content", // 这个项目独有的产出
	} {
		if !strings.Contains(p, want) {
			t.Errorf("主页项目的 prompt 里少了 %q", want)
		}
	}
	if strings.Contains(p, "【什么时候递哪一件】") {
		t.Error("主页项目不该再带通用那份「什么时候递哪一件」——两份路线会打架")
	}
}

// 🚨 放开的本事只有三样，一样都不能多。
//
// 通用提示里那句禁令（「不要说你能查资料、做图、写网站」）在主页项目里必须放开，
// 否则印记会拒绝做它其实做得到的事。但放开的边界要能验：多放开一样，学生就会
// 收到一句它兑现不了的承诺，而那是这条禁令一开始存在的原因。
func TestCoachPrompt_WebsiteStatesImplementedCapabilities(t *testing.T) {
	p := coachPrompt("website")

	for _, want := range []string{"读他贴进来的网址", "生成图", "把他的页面渲染出来"} {
		if !strings.Contains(p, want) {
			t.Errorf("主页项目里应该放开「%s」", want)
		}
	}
	if strings.Contains(p, "也不要说你能查资料、做图、写网站") {
		t.Error("主页项目里还留着通用那条禁令，印记会拒绝做它其实做得到的事")
	}
	// 边界仍然在：放开的三样之外，那句话还得说。
	if !strings.Contains(p, "后续私有草稿修改不会自动更新公开页面") {
		t.Error("主页项目放开了三样，但没有把边界重新划上")
	}
}

// 普通项目允许外部制作指导，但不能获得主页专用接口或虚构执行状态。
func TestCoachPrompt_OtherProjectsGuideExternalCreation(t *testing.T) {
	p := coachPrompt("")

	if !strings.Contains(p, "【什么时候递哪一件】") {
		t.Error("普通项目丢了「什么时候递哪一件」")
	}
	for _, required := range []string{"外部coding agent", "带回预览、截图或试用记录", "不能把指导搜索说成已经完成搜索"} {
		if !strings.Contains(p, required) {
			t.Errorf("普通项目缺少制作指导边界：%s", required)
		}
	}
	for _, leaked := range []string{"persona", "sites", "site_content", "【五关"} {
		if strings.Contains(p, leaked) {
			t.Errorf("主页项目独有的 %q 漏进了普通项目的 prompt", leaked)
		}
	}
}

/* ── 页面上的字必须是她说过的 ─────────────────────────────────────────── */

// 她没说过的那句，不上页面。
//
// 这是铁律①在代码里的那道闸。prompt 里写「你不替他写正文」只能降低概率；
// 2026-09-03 已经付过一次学费：真模型会把 prompt 里的脚手架当成学生原话返回
// （见 interest.KeepGrounded）。
func TestGroundSiteDraft_DropsWhatSheNeverSaid(t *testing.T) {
	own := "我拆过 41 件旧电器。我想问的是，一件还能修的东西，是谁决定它该被扔的。"

	got, dropped := GroundSiteDraft(SiteDraft{
		Headline: "一件还能修的东西，是谁决定它该被扔的",
		Lead:     "我是一个热爱探索与创造的少年，用双手丈量世界。",
		About:    []string{"我拆过 41 件旧电器", "我的作品曾获市级奖项"},
	}, own)

	if got.Headline == "" {
		t.Error("她原话里的那一句被丢了")
	}
	if got.Lead != "" {
		t.Errorf("模型自己写的那段应该被丢掉，实际留下 %q", got.Lead)
	}
	if len(got.About) != 1 || got.About[0] != "我拆过 41 件旧电器" {
		t.Errorf("关于那一栏应该只剩她真说过的一条，实际 %q", got.About)
	}
	if len(dropped) != 2 {
		t.Errorf("应该报告丢掉了 2 句，实际 %d 句：%q", len(dropped), dropped)
	}
}

// 从她一整段话里摘一句短的，是允许的——那是编排，不是代笔。
func TestGroundSiteDraft_AllowsQuotingAFragmentOfWhatSheSaid(t *testing.T) {
	own := "我在做的事情大概是：把别人扔掉的电器拆开，看看它到底坏在哪里。"

	got, _ := GroundSiteDraft(SiteDraft{Now: "把别人扔掉的电器拆开"}, own)
	if got.Now != "把别人扔掉的电器拆开" {
		t.Errorf("摘她原话里的一段应该留下，实际 %q", got.Now)
	}
}

// 换行和空格不算改写。
func TestGroundSiteDraft_IgnoresRewrappedWhitespace(t *testing.T) {
	own := "我想让\n以后想学修东西的人\n看到这一页。"

	got, _ := GroundSiteDraft(SiteDraft{Role: "以后想学修东西的人"}, own)
	if got.Role == "" {
		t.Error("只是换行不同，不该算她没说过")
	}
}

// 🚨 联系方式永远清空，哪怕模型「引对了」。
//
// 她是未成年人，页面是公开的。留不留联系方式、留哪个，只能由她自己在界面上决定，
// 不能由一次模型输出决定。见 site.go 顶部那段。
func TestGroundSiteDraft_NeverLetsTheModelSetContact(t *testing.T) {
	own := "你可以写 me@example.com。"

	got, _ := GroundSiteDraft(SiteDraft{Contact: "me@example.com"}, own)
	if got.Contact != "" {
		t.Errorf("联系方式这一格模型永远不许写，实际 %q", got.Contact)
	}
}

func TestGroundSiteSections_OnlyStudentTextCanBecomeBody(t *testing.T) {
	got, dropped := GroundSiteDraft(SiteDraft{Sections: []SiteSection{{Key: "a", Title: "AI标题", Body: "我观察了这棵树。"}, {Key: "b", Body: "请写一段邀请说明"}}}, "我观察了这棵树。")
	if len(got.Sections) != 1 || got.Sections[0].Key != "a" || got.Sections[0].Title != "" || len(dropped) != 1 {
		t.Fatalf("got=%+v dropped=%v", got.Sections, dropped)
	}
}
