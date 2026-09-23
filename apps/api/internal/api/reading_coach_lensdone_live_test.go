package api

// reading_coach_lensdone_live_test.go — 把「她刚做完一副透镜」这一轮送去见一次
// **真模型**，用生产用的同一个 system prompt、同一个 prompt builder、同一个
// parser。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveLensDone -v -count=1
//
// 默认跳过，所以常规套件仍然是离线、确定的。
//
// # 🚨 为什么这个文件必须存在
//
// [[prompt-output-must-be-verifiable-2026-09-03]]：stub 测试只证明**解析器读得
// 懂我写的 JSON**，不证明模型真的会那样回。
//
// 这一条不是理论。2026-09-04 线上走查里，`lensDone` 那一轮真的失败了：
//
//	{"error":{"code":"ai_dialogue_failed","message":"AI 响应错误",
//	          "details":"model_unavailable"}}
//	WARN reading coach: reply unparseable  atom_id=2401a94c…
//
// 模型答了，但答出来的东西 `parseReadingCoachReply` 不接。而
// `reading_coach_lensdone_internal_test.go` 里那四个测试全是绿的——它们测的是
// prompt 里有没有那几行字，不是模型会不会照着回。于是学生做完一副透镜、
// 屏幕上弹出一句「AI 响应错误」，正是这一整轮要修掉的那个「没有响应」。
//
// 所以这个测试断言的不是「调用成功了」，而是**产物真的能用**：真模型的回复
// 过得了生产那个 parser，而且过完之后 reply 非空、没有顺手再塞一副透镜或一张
// 卡片（那两条是这一轮的硬规矩）。

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// liveClass resolves the SAME capability class production uses for a given
// call. Anything else would be testing a model production never asks.
//
// Shared by every live prompt test in this package (see also
// writing_english_live_test.go) so none of them can quietly drift onto a
// different model than the one a student actually gets.
func liveClass(t *testing.T, class string) (gateway.Provider, gateway.Resolved) {
	t.Helper()
	if os.Getenv("LIVE_LLM") != "1" {
		t.Skip("set LIVE_LLM=1 to run live prompt checks")
	}
	cfg := config.Config{
		DashScopeKey:  os.Getenv("DASHSCOPE_API_KEY"),
		DeepSeekKey:   os.Getenv("DEEPSEEK_API_KEY"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		ModelChat:     os.Getenv("MODEL_CHAT"),
		ModelFastChat: os.Getenv("MODEL_FAST_CHAT"),
		ModelEval:     os.Getenv("MODEL_EVAL"),
	}
	if cfg.DashScopeKey == "" && cfg.DeepSeekKey == "" && cfg.AnthropicKey == "" {
		t.Skip("no provider key in the environment")
	}
	rs, err := gateway.NewResolvers(cfg)
	if err != nil {
		t.Fatalf("resolvers: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r, rerr := rs.For(class)(ctx)
	if rerr != nil {
		t.Skipf("class %s has no provider here: %v", class, rerr)
	}
	p := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(&http.Client{}),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(&http.Client{}),
	})
	return p, r
}

func liveLensDoneBlocks() []Block {
	return SplitBlocks(strings.Join([]string{
		"点一份外卖，配送费常常是三到六元。很多人以为这笔钱直接进了骑手的口袋。",
		"实际上，配送费先进入平台，再由平台按一套算法分配。骑手拿到的部分与距离、时段、天气和当时的运力缺口都有关。",
		"平台会说，抽成用于调度系统、保险和补贴。但具体比例很少公开，这让外部很难核对这笔账。",
		"一份行业报告称，骑手实际到手约占配送费的六到七成。这个数字来自平台自己提供的数据，独立机构难以验证。",
		"所以问题不只是钱怎么分，而是这笔账由谁来记、谁能核对。",
	}, "\n\n"))
}

// TestLiveLensDoneReplyParses is the whole point of the file: production's
// prompt, production's parser, a real model, three samples.
//
// Three rather than one because a single sample of a generative model tells
// you almost nothing — the 2026-09-02 reasoning probe in models.json makes
// the same point ("single samples swing by a third"). A prompt that parses
// two times out of three is not shippable: the third student gets 「AI 响应
// 错误」 the moment she finishes a lens.
func TestLiveLensDoneReplyParses(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassDialogue)
	blocks := liveLensDoneBlocks()
	// 🚨 带一个 **rethink** 的复核结论（2026-09-17）。这是产品负责人报的那一幕
	// 里最难的一种：复核当着她的面说了「这一句撑不住」，而 印记 只拿到句子和
	// finding，于是照着「先说她哪里选得准」那一节夸了她一句 ——
	// 「句子匹配不通过，但是点击记录发现后，主 ai 又给出了不一样的回答。」
	// 现在结论跟着回灌，这一轮不许跟它相反。
	done := &readingLensDone{
		CardName:      "溯源体检",
		Quote:         "这个数字来自平台自己提供的数据，独立机构难以验证。",
		Finding:       "这句点出数据的来源方就是被审对象，属于自证。",
		Verdict:       "rethink",
		VerdictReason: "这一句说的是数据从哪儿来，还没说这笔钱最后落到谁手上。",
	}
	// A plan whose CURRENT step is the lens step — the realistic shape, and
	// the one whose 「advance: done」 the new rules ask for.
	tasks := liveLensDoneTasks()

	system := buildReadingCoachSystem("zh", "")
	prompt := buildReadingCoachPrompt("一份外卖的配送费，到底付给了谁？", blocks, readingOutline{}, tasks, nil, nil, "", done, "")

	truncated, firstPass := 0, 0
	for i := 0; i < 6; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		start := time.Now()
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: prompt},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: live call failed (%s): %v", i, r.ModelID, err)
		}
		t.Logf("sample %d — %s | %v | out=%d | stop=%q", i, r.ModelID,
			time.Since(start).Round(time.Millisecond), res.Usage.OutputTokens, res.StopReason)

		got, ok := parseReadingCoachReply(res.Text, blocks, "zh", func(string) bool { return true })
		if !ok {
			// 🚨 This is the measurement that put the retry into production.
			// The model writes a good reply and stops at the trailing
			// optional key — with stop_reason "stop", a NORMAL finish:
			//   …"lens":"","card":
			// Roughly 1 sample in 6. Recorded, then retried exactly the way
			// postReadingCoachTurn now does.
			firstPass++
			t.Logf("sample %d: FIRST PASS rejected. stop=%q len=%d\n--- raw tail ---\n%s\n--- end ---",
				i, res.StopReason, len(res.Text), tailOf(res.Text, 140))

			ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Minute)
			res2, err2 := gateway.Collect(ctx2, prov, r, gateway.ChatRequest{
				Messages: []gateway.ChatMessage{
					{Role: gateway.RoleSystem, Content: system},
					{Role: gateway.RoleUser, Content: prompt},
				},
			})
			cancel2()
			if err2 != nil {
				t.Fatalf("sample %d: retry call failed: %v", i, err2)
			}
			got, ok = parseReadingCoachReply(res2.Text, blocks, "zh", func(string) bool { return true })
			if !ok {
				truncated++
				t.Logf("sample %d: RETRY ALSO rejected — this is the case the student sees as 「AI 响应错误」", i)
				continue
			}
			t.Logf("sample %d: retry recovered it", i)
		}
		// 🚨 Run the SAME rule production runs, not an approximation of it.
		// Before this line existed, this test recorded the real finding that
		// the model attaches a card 2 times in 3 despite the prompt forbidding
		// it — which is why enforceLensDoneTurn exists at all.
		rawCard, rawLens := got.Card != nil, got.Lens
		got = enforceLensDoneTurn(got, done)
		if rawCard || rawLens != "" {
			t.Logf("sample %d: model ignored the prose rule (card=%v lens=%q) — dropped in code", i, rawCard, rawLens)
		}
		if strings.TrimSpace(got.Reply) == "" {
			t.Errorf("sample %d: parsed, but reply is empty — she gets a blank turn", i)
		}
		// 这一轮的两条硬规矩：她刚交完一件事，不许马上再塞一件。
		if got.Lens != "" {
			t.Errorf("sample %d: summoned another lens (%q) right after she finished one", i, got.Lens)
		}
		if got.Card != nil {
			t.Errorf("sample %d: handed her a card right after she finished a lens", i)
		}
		// 🚨 整条回复打出来，不截断。这一轮要看的是**它有没有跟复核的结论
		// 对着干**，而那件事没法用一个子串判出来 —— 只能由人读一遍
		// （[[optimizing-a-detector-made-coaching-worse-2026-09-14]]：把一件
		// 教学上的事压成一个探测器，数字会清零而教得更差）。
		t.Logf("sample %d ok — advance=%q\n--- reply ---\n%s\n--- end ---", i, got.Advance, got.Reply)
	}
	t.Logf("RESULT: %d/6 first passes truncated; %d/6 still unusable AFTER one retry", firstPass, truncated)
	// The bar is the one the STUDENT feels: after production's retry, a turn
	// must essentially always land. A first-pass truncation is expected — it
	// is the whole reason the retry exists — so it is logged, not failed.
	if truncated > 0 {
		t.Errorf("%d/6 turns failed even after a retry — she still hits 「AI 响应错误」", truncated)
	}
}

func tailOf(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n:])
}

func liveLensDoneTasks() []sqlc.ReadingTask {
	return []sqlc.ReadingTask{
		{Label: "通读全文", Kind: "read", Status: "done"},
		{Label: "精读重点段落第3段", Kind: "focus_block", Status: "done", BlockID: "b3"},
		{Label: "深入思考", Kind: "lens", Status: "pending"},
		{Label: "总结收获", Kind: "reflect", Status: "pending"},
	}
}
