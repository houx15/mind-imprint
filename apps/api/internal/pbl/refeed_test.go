package pbl

import (
	"strings"
	"testing"
)

// Current source records must follow historical AI claims while a tool-return
// event remains last. Otherwise a stale conclusion can eclipse its evidence.
func TestCurrentEvidenceFollowsHistoricalClaims(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Recent:       []Turn{{Role: "ai", Content: "旧结论：公告栏不是主要渠道"}},
		ToolWork:     []string{"【虚构测试记录，未实地观察】公告栏十分钟没有信息"},
		JustHappened: "完成观察记录",
	})
	history := strings.Index(ctx, "旧结论：公告栏不是主要渠道")
	evidence := strings.Index(ctx, "【虚构测试记录，未实地观察】")
	event := strings.Index(ctx, "【学生刚做完这件事】")
	if history < 0 || evidence <= history || event <= evidence {
		t.Fatalf("source ordering lost: %s", ctx)
	}
}

// 🚨 她在工具里做出来的东西，必须每一轮都回到印记眼前。
//
// 2026-09-02 查出来的：CoachInput 里根本没有装这些的地方，而 system 行又在
// buildPblCoachInput 里被过滤掉了——她在便签板上摆十五分钟，回到对话，印记
// 收到的 prompt 和上一轮**逐字节相同**。AGENTS.md 主线里的「回灌陪练」那个
// 箭头是空的。
//
// 这条测试盯的正是"读代码看不出对错"的那种东西：prompt 少一段，界面上一切
// 正常，编译通过，所有别的测试全绿，只有印记的回话会慢慢变得像没在听。
func TestBuildCoachContext_CarriesHerToolWork(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Idea: "剩饭",
		ToolWork: []string{
			"她把问题定成了：住校生 需要 按饭量打饭，因为 现在只能打固定的一份",
			"她在板上记的实际观察：中午十二点半剩得最多",
		},
	})
	for _, want := range []string{
		"住校生 需要 按饭量打饭",
		"中午十二点半剩得最多",
	} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("她做的东西没进 prompt，缺：%s\n---\n%s", want, ctx)
		}
	}
	// Current-turn precedence is tested separately: carrying historical records
	// must not force the coach to answer an old question instead of the new one.
}

// 一件都没做的时候不要凭空多一段。空标题会让印记以为自己漏看了什么。
func TestBuildCoachContext_NoToolWorkNoSection(t *testing.T) {
	ctx := buildCoachContext(CoachInput{Idea: "剩饭"})
	if strings.Contains(ctx, "已经做出来的东西") {
		t.Fatalf("什么都没做却出现了工具成果那一段：\n%s", ctx)
	}
}
