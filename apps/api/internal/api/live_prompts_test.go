package api_test

// live_prompts_test.go — 把这一轮新写的四条 prompt 送去见一次**真模型**。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLivePrompt -v -count=1
//
// 默认跳过，所以常规套件仍然是离线、确定的。
//
// # 为什么这个文件必须存在
//
// 这一轮的每一个 Go 测试都跑在 nil provider 或脚本化的 stub 上，每一张截图都跑在
// mock 的 API 响应上。那些测试证明的是**解析器读得懂我写的 JSON**，而不是
// 「模型真的会那样回」。这两件事之间隔着这个产品最容易翻车的一处：
//
//   🚨 采集与选星都要求 **evidence / 原话逐字摘录**。一个爱转述的模型会返回
//      语义正确、但一个字都对不上的 evidence —— 解析器（正确地）把它全丢掉，
//      于是学生做完五分钟的兴趣测试，看到的是「这次没有长出关键词」。
//      单元测试永远发现不了这件事：它给的是我手写的、必然逐字的样例。
//
// 所以这里断言的不是「调用成功了」，而是**产物真的能用**：解析出来的东西非空、
// evidence 真的出现在输入里、五颗星真的指向存在的候选。

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/news"
)

func liveResolvers(t *testing.T) gateway.Resolvers {
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
	return rs
}

func liveAsk(t *testing.T, rs gateway.Resolvers, class, system, user string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	r, err := rs.For(class)(ctx)
	if err != nil {
		t.Skipf("class %s has no provider here: %v", class, err)
	}
	p := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(&http.Client{}),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(&http.Client{}),
	})
	start := time.Now()
	res, err := gateway.Collect(ctx, p, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		t.Fatalf("live call failed (%s / %s): %v", class, r.ModelID, err)
	}
	t.Logf("%s → %s | %v | in=%d out=%d", class, r.ModelID,
		time.Since(start).Round(time.Millisecond), res.Usage.InputTokens, res.Usage.OutputTokens)
	return res.Text
}

/* ── 1. 采集（阅读 / 写作 / 项目） ──────────────────────────────────────── */

const liveTakeaway = `我本来以为这篇讲的是珊瑚怎么死的，读到一半发现它讲的是那片没死的。
研究者只测了红海北端四平方公里，我读到面积那一段才反应过来，四平方公里其实很小。
如果他们挑的正好是最健康的那一片，那结论就已经被写好了。`

func TestLivePromptHarvest(t *testing.T) {
	rs := liveResolvers(t)
	system, user := interest.BuildHarvestPrompt("reading", "红海北端那片不白化的珊瑚", liveTakeaway)
	raw := liveAsk(t, rs, gateway.ClassCompose, system, user)

	hs, err := interest.ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("解析失败 —— 学生那边的结果就是「一个词都没长出来」：%v\n原始回复：\n%s", err, raw)
	}
	if len(hs) == 0 {
		t.Fatalf("解析成功但零个词。原始回复：\n%s", raw)
	}
	for _, h := range hs {
		t.Logf("  %s (%s) — %s", h.TextZh, h.Field, h.Note)
		t.Logf("    evidence: %q", h.Evidence)
		// 🚨 这是这个文件存在的主要理由。evidence 必须是**逐字摘录**；一个爱
		// 转述的模型会让每一个词都在落库前被丢掉，而学生只看到「没长出词」。
		if !strings.Contains(liveTakeaway, h.Evidence) {
			t.Errorf("evidence 不是原话的逐字摘录 —— 模型在转述。\n  给的是：%q", h.Evidence)
		}
	}
}

/* ── 2. 觉醒协议（兴趣测试） ────────────────────────────────────────────── */

func TestLivePromptQuiz(t *testing.T) {
	rs := liveResolvers(t)
	att := interest.Attempt{
		Navigator: "腹黑军师",
		Work:      "《进击的巨人》里的利威尔",
		Reason:    "他经历了很多痛苦，但在关键时刻依然保持理智，做出自己的选择。",
		Hook:      interest.HookCharacter,
	}.Clean()
	system, user := att.BuildQuizPrompt()
	raw := liveAsk(t, rs, gateway.ClassCompose, system, user)

	hs, err := interest.ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("解析失败 —— 结果页会说「这次没有长出关键词」：%v\n原始回复：\n%s", err, raw)
	}
	if len(hs) == 0 {
		t.Fatalf("零个词。一个学生刚花了五分钟。原始回复：\n%s", raw)
	}
	for _, h := range hs {
		t.Logf("  %s (%s) — %s", h.TextZh, h.Field, h.Note)
		t.Logf("    evidence: %q", h.Evidence)
		if !strings.Contains(att.Reason, h.Evidence) {
			t.Errorf("evidence 不是她原话的逐字摘录：%q", h.Evidence)
		}
	}
}

/* ── 3. 继续深挖 ────────────────────────────────────────────────────────── */

func TestLivePromptDig(t *testing.T) {
	rs := liveResolvers(t)
	system, user := interest.BuildDigPrompt(
		"样本代表性",
		"你反复回到同一个问题：这一小块，凭什么替一大片说话。",
		[]string{
			"我读到面积那一段才反应过来，四平方公里其实很小。",
			"一个避难所不是一个计划。",
		})
	raw := liveAsk(t, rs, gateway.ClassCompose, system, user)

	seeds, err := interest.ParseDigReply(raw)
	if err != nil {
		t.Fatalf("解析失败 —— 抽屉里那一节会是空的：%v\n原始回复：\n%s", err, raw)
	}
	kinds := map[interest.DigKind]bool{}
	for _, s := range seeds {
		kinds[s.Kind] = true
		t.Logf("  [%s] %s", s.Kind, s.Text)
		t.Logf("        why: %s", s.Why)
		// 去读 / 去写 / 去做 的正文会被**直接当标题**送进创建接口。一句话，
		// 不是一段。
		if len([]rune(s.Text)) > 40 {
			t.Errorf("[%s] 正文 %d 字，太长了，它要当标题用：%q", s.Kind, len([]rune(s.Text)), s.Text)
		}
	}
	if len(seeds) < interest.DigSeedCount {
		t.Errorf("只给了 %d 颗种子（四种各一颗才齐），拿到：%v", len(seeds), kinds)
	}
}

/* ── 4. 今日新闻星图 ────────────────────────────────────────────────────── */

// 这一条打**真的 feed 再打真的模型** —— 整条 P4 管线唯一一次端到端。
func TestLivePromptStarmap(t *testing.T) {
	rs := liveResolvers(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := news.NewFetcher().FetchAll(ctx, 72*time.Hour)
	t.Logf("候选池 %d 条", len(pool))
	if len(pool) < news.PlanetCount {
		t.Fatalf("候选池只有 %d 条，凑不满 %d 颗星", len(pool), news.PlanetCount)
	}

	system, user, candidates := news.BuildSelectPrompt(pool)
	raw := liveAsk(t, rs, gateway.ClassDigest, system, user)

	planets, err := news.ParseSelectReply(raw, candidates)
	if err != nil {
		t.Fatalf("解析失败 —— 今天不会有星图：%v\n原始回复：\n%s", err, raw)
	}
	if len(planets) != news.PlanetCount {
		t.Errorf("只挑出 %d 颗，want %d", len(planets), news.PlanetCount)
	}
	fields := map[string]int{}
	for _, p := range planets {
		src := candidates[p.Index]
		fields[p.Field]++
		t.Logf("  [%s] %s", p.Field, p.TitleZh)
		t.Logf("        钩子：%s", p.Hook)
		t.Logf("        摘要：%s", p.Summary)
		t.Logf("        关键词：%s | 学科：%s | 来源：%s", p.Keyword, p.DisciplineID, src.Source)
		// 🚨 模型返回的 index 决定这颗星挂哪个链接、哪个出处。如果 index 和它
		// 自己描述的那条对不上，学生点「读原文」会落到一篇毫不相干的文章上。
		// prompt 要求 titleEn 填原标题，所以这里能对得上号。
		t.Logf("        [%d] 候选原标题：%s", p.Index, src.Title)
		t.Logf("            模型 titleEn：%s", p.TitleEn)
		// 🚨 回查之后，这两个必须是同一条新闻。对不上就说明 anchorByTitle 放行了
		// 一个错误匹配，而学生点「读原文」会落到一篇无关文章上。
		if overlapRatio(p.TitleEn, src.Title) < 0.5 {
			t.Errorf("星球挂错了候选：\n  模型说的是 %q\n  挂上去的是 %q", p.TitleEn, src.Title)
		}
		// 钩子必须是个问题 —— 它是这一屏存在的理由。
		if !strings.HasSuffix(p.Hook, "？") && !strings.HasSuffix(p.Hook, "?") {
			t.Errorf("钩子不是一个问句：%q", p.Hook)
		}
		// titleZh 是**重写**不是翻译，20 字以内（放宽到 28 留点余地）。
		if n := len([]rune(p.TitleZh)); n > 28 {
			t.Errorf("标题 %d 字，太长：%q", n, p.TitleZh)
		}
		if p.Keyword == "" {
			t.Errorf("这颗星没有关键词，收藏它不会往树上加任何东西：%q", p.TitleZh)
		}
	}
	// 五条全挤在一根枝上，这一屏就退化成一个学科的日报了。
	if len(fields) < 2 {
		t.Errorf("五颗星全落在一根主枝上：%v", fields)
	}
	t.Logf("主枝分布：%v", fields)
}


// overlapRatio 复刻 news 包里的词重合度，用来在测试侧独立验证回查结果 ——
// 用被测代码自己的函数去验它自己，等于什么都没验。
func overlapRatio(a, b string) float64 {
	set := func(s string) map[string]bool {
		out := map[string]bool{}
		for _, w := range strings.Fields(strings.ToLower(s)) {
			w = strings.Trim(w, ".,:;!?\"'()[]—-")
			if len([]rune(w)) > 2 {
				out[w] = true
			}
		}
		return out
	}
	x, y := set(a), set(b)
	if len(x) == 0 {
		return 0
	}
	n := 0
	for w := range x {
		if y[w] {
			n++
		}
	}
	return float64(n) / float64(len(x))
}
