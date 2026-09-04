package api_test

// pbl_coach_live_test.go —— 把陪练这条 prompt 上新加的两块送去见一次真模型。
//
//	set -a; . apps/api/.env.local; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveCoach -v -count=1
//
// 默认跳过，常规套件仍然是离线、确定的。
//
// # 为什么这两条必须见真模型
//
// 上面那些 stub 测试证明的是**解析器读得懂我写的 JSON**，以及**那段字进了
// prompt**。它们证明不了模型读到那段字之后真的改了行为——而这两块新加的东西，
// 价值全在行为上：
//
//   - 她答不上来的时候，印记该给例子，而不是换个侧面再问一遍。
//   - 上一轮那件工具被撤掉了，印记不该再说「我给你了」；要么这一轮和产出一起
//     给，要么直说还没有。
//
// 第二条能硬断言：这一轮如果又递 review，那 produce 必须同时是一份 artifact。
// 第一条只能断言「一个问号」这条硬规矩，其余打印出来由人读。
// 见 [[prompt-output-must-be-verifiable-2026-09-03]]。

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/pbl"
)

func liveCoach(t *testing.T, in pbl.CoachInput) pbl.CoachOutput {
	t.Helper()
	rs := liveResolvers(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	r, err := rs.For(gateway.ClassDialogue)(ctx)
	if err != nil {
		t.Skipf("dialogue 档在这里没有通道：%v", err)
	}
	p := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(&http.Client{}),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(&http.Client{}),
	})
	start := time.Now()
	out, usage, err := pbl.Coach(ctx, p, r, in)
	if err != nil {
		t.Fatalf("陪练这一轮失败（%s）：%v", r.ModelID, err)
	}
	t.Logf("dialogue → %s | %v | in=%d out=%d", r.ModelID,
		time.Since(start).Round(time.Millisecond), usage.InputTokens, usage.OutputTokens)
	return out
}

// 她连着三轮说不上来，还自己开口要了例子。这一轮印记该给她能接住的东西。
func TestLiveCoachGivesExamplesWhenSheIsStuck(t *testing.T) {
	out := liveCoach(t, pbl.CoachInput{
		Idea: "我想做一个自己的主页，但是我不知道要做什么",
		Kind: "website",
		Recent: []pbl.Turn{
			{Role: "ai", Content: "这个主页你想给谁看？"},
			{Role: "student", Content: "不知道"},
			{Role: "ai", Content: "那换个角度，最近有谁问过你在忙什么吗？"},
			{Role: "student", Content: "没想过"},
			{Role: "ai", Content: "那你希望别人看完这一页，记住你哪一点？"},
			{Role: "student", Content: "能给我几个选项选一下吗"},
		},
		Stuck:        3,
		AskedForHelp: true,
	})

	t.Logf("印记说：\n%s", out.Reply)
	if out.Hook != "" {
		t.Logf("钩子：%s", out.Hook)
	}
	// 铁律③：一轮一个问题。给例子最容易破的就是这条——三个例子写成三个问句。
	if n := strings.Count(out.Reply, "？") + strings.Count(out.Reply, "?"); n > 1 {
		t.Errorf("这一轮有 %d 个问号，铁律③是一次只问一个：\n%s", n, out.Reply)
	}
	// 她开口要例子，这一轮总得有点具体的东西给她。空泛的一句"你再想想"就是
	// 走查里那个学生放弃的原因。
	if len([]rune(strings.TrimSpace(out.Reply))) < 30 {
		t.Errorf("她要例子，回来的只有一句话：%q", out.Reply)
	}
}

// 🚨 上一轮那件工具被撤掉了。这一轮不能再说「我给你了」。
//
// 硬断言：要递 review，就必须同一轮做出 artifact。这正是闸要的那个形状——
// 两个一起给，或者两个都不给。
func TestLiveCoachStopsClaimingADroppedTool(t *testing.T) {
	out := liveCoach(t, pbl.CoachInput{
		Idea: "我想做一个自己的主页",
		Kind: "website",
		Recent: []pbl.Turn{
			{Role: "student", Content: "你先帮我写一版首页的文案，我来看看"},
			{Role: "ai", Content: "我把审核助手给你了，你点开逐条看。"},
			{Role: "student", Content: "审核助手在哪，我没看到"},
		},
		ToolDropped: "「审核助手」这件工具**没有**出现在她屏幕上。你说过要给她，但同一轮" +
			"没有做出可审的成果，那个界面打开是一块白板，所以服务端把它撤掉了。" +
			"她现在找不到这张卡。\n" +
			"这一轮只有两条路：把可审的成果和这件工具**一起**给（produce 和 tool 同一轮），" +
			"或者直说这件东西还没有。不要再说你已经把它给她了。",
	})

	t.Logf("印记说：\n%s", out.Reply)
	t.Logf("tool=%q produce=%v", out.Tool, out.Produce != nil)
	if out.Tool == "review" {
		if out.Produce == nil || out.Produce.Kind != "artifact" {
			t.Errorf("又递了一次审核助手，却还是没有可审的成果——闸会再撤一次，"+
				"她还是看不到那张卡：produce=%v", out.Produce)
		}
	}
	// 它可以选择这一轮先不给，但不能继续说那张卡已经在她那儿了。
	for _, lie := range []string{"我给你了", "已经给你", "已经递给你", "已经在你"} {
		if strings.Contains(out.Reply, lie) {
			t.Errorf("上文明说那张卡没出现，它还在说「%s」：\n%s", lie, out.Reply)
		}
	}
}
