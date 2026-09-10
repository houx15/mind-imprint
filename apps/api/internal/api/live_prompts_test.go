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
	"unicode"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/materialize"
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
	t.Logf("%s → %s | %v | in=%d out=%d | stop=%q", class, r.ModelID,
		time.Since(start).Round(time.Millisecond), res.Usage.InputTokens, res.Usage.OutputTokens,
		res.StopReason)
	return res.Text
}

// liveAskJSON 和 liveAsk 一样，只是**带上多轮来回，并且要求 JSON 模式** ——
// 星图那条路生产上就是这么发的，测试少一个字段就不是同一件事了。
func liveAskJSON(t *testing.T, rs gateway.Resolvers, class, system string, turns []news.Turn) (string, error) {
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
	msgs := []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: system}}
	for _, tn := range turns {
		msgs = append(msgs, gateway.ChatMessage{Role: tn.Role, Content: tn.Content})
	}
	start := time.Now()
	res, err := gateway.Collect(ctx, p, r, gateway.ChatRequest{
		Messages:       msgs,
		ResponseFormat: gateway.ResponseFormatJSONObject,
	})
	if err != nil {
		return "", err
	}
	t.Logf("     %s → %s | %v | in=%d out=%d | stop=%q", class, r.ModelID,
		time.Since(start).Round(time.Millisecond), res.Usage.InputTokens, res.Usage.OutputTokens,
		res.StopReason)
	return res.Text, nil
}

/* ── 1. 采集（阅读 / 写作 / 项目） ──────────────────────────────────────── */

const liveTakeaway = `我本来以为这篇讲的是珊瑚怎么死的，读到一半发现它讲的是那片没死的。
研究者只测了红海北端四平方公里，我读到面积那一段才反应过来，四平方公里其实很小。
如果他们挑的正好是最健康的那一片，那结论就已经被写好了。`

func TestLivePromptHarvest(t *testing.T) {
	rs := liveResolvers(t)
	system, user := interest.BuildHarvestPrompt("reading", "红海北端那片不白化的珊瑚", liveTakeaway)
	raw := liveAsk(t, rs, gateway.ClassDigest, system, user)

	hs, err := interest.ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("解析失败 —— 学生那边的结果就是「一个词都没长出来」：%v\n原始回复：\n%s", err, raw)
	}
	if len(hs) == 0 {
		t.Fatalf("解析成功但零个词。原始回复：\n%s", raw)
	}
	for _, h := range hs {
		it, known := interests.ByID(h.InterestID)
		if !known {
			// ParseHarvestReply 已经挡了；这里只是让日志说得出名字。
			t.Fatalf("解析器放过了一个表外 id：%q", h.InterestID)
		}
		t.Logf("  %s (%s / %s) — %s", it.Zh, it.ID, it.Field, h.Note)
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
	raw := liveAsk(t, rs, gateway.ClassDigest, system, user)

	hs, err := interest.ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("解析失败 —— 结果页会说「这次没有长出关键词」：%v\n原始回复：\n%s", err, raw)
	}
	if len(hs) == 0 {
		t.Fatalf("零个词。一个学生刚花了五分钟。原始回复：\n%s", raw)
	}
	for _, h := range hs {
		it, known := interests.ByID(h.InterestID)
		if !known {
			// ParseHarvestReply 已经挡了；这里只是让日志说得出名字。
			t.Fatalf("解析器放过了一个表外 id：%q", h.InterestID)
		}
		t.Logf("  %s (%s / %s) — %s", it.Zh, it.ID, it.Field, h.Note)
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
	raw := liveAsk(t, rs, gateway.ClassDigest, system, user)

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
	if len(planets) < news.PlanetCount {
		t.Errorf("只挑出 %d 条，want %d（多挑的那几条是给「写」那一步丢的余量）", len(planets), news.SelectCount())
	}
	fields := map[string]int{}
	withInterest := 0
	for _, p := range planets {
		src := candidates[p.Index]
		fields[p.Field]++
		t.Logf("  [%s] %s", p.Field, src.Title)
		t.Logf("        领域：%s | 学科：%s | 来源：%s", p.InterestID, p.DisciplineID, src.Source)
		// 领域是闭表 id，解析器已经把表外的清成空串了。空串合法（这条新闻挑不出
		// 领域，收藏它不长词），但一个非空却查不到的 id 说明解析器漏了。
		if p.InterestID != "" && !interests.Exists(p.InterestID) {
			t.Errorf("星球挂了一个表外的领域 id：%q", p.InterestID)
		}
		// 🚨 挂哪个链接、哪个出处，全看回查对没对上。对不上，学生点「读原文」
		// 会落到一篇毫不相干的文章上。
		if overlapRatio(p.TitleEn, src.Title) < 0.5 {
			t.Errorf("星球挂错了候选：\n  模型说的是 %q\n  挂上去的是 %q", p.TitleEn, src.Title)
		}
		if p.InterestID != "" {
			withInterest++
		}
	}
	// 空领域是合法的（那颗星收藏了不长词），但**全空**说明模型根本没在用那张候选
	// 表 —— 那时整张星图对树来说是死的。
	if withInterest < 3 {
		t.Errorf("%d 条里只有 %d 条挑出了领域，收藏它们大多不会往树上加东西", len(planets), withInterest)
	}
	// 全挤在一根枝上，这一屏就退化成一个学科的日报了。
	if len(fields) < 2 {
		t.Errorf("挑出来的全落在一根主枝上：%v", fields)
	}
	t.Logf("主枝分布：%v", fields)
}

// TestLivePromptPlanetCopy 是「照着正文写」那一步见真模型。
//
// 🚨 **这个用例才是 2026-09-10 那次改动的验收。** 它断言的不是「调用成功了」，
// 而是那句 evidence 真的能在正文里查到 —— 一个爱转述的模型会交回一句语义正确、
// 却一个字都对不上的「原话」，而校验（正确地）把整条丢掉。真发生的话，线上的
// 症状是星图天天凑不满五颗，而所有单元测试都是绿的：它们喂的是我手写的、必然
// 逐字的样例。
//
// 这里也顺带量出**正文抓得到的比例**。抓不到就退回 feed 那 400 字导语，而导语
// 常常是半句话 —— 上一版编出「特例」那个争议，根子就在这里。
func TestLivePromptPlanetCopy(t *testing.T) {
	rs := liveResolvers(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := news.NewFetcher().FetchAll(ctx, 72*time.Hour)
	if len(pool) < news.PlanetCount {
		t.Fatalf("候选池只有 %d 条", len(pool))
	}
	if len(pool) > news.PlanetCount {
		pool = pool[:news.PlanetCount]
	}

	fetcher := materialize.NewFetcher()
	fromArticle, passed, retried := 0, 0, 0
	for _, src := range pool {
		fctx, fcancel := context.WithTimeout(ctx, 15*time.Second)
		_, text, _, ferr := fetcher.FetchReadable(fctx, src.Link)
		fcancel()
		ground := news.GroundText(text, src)
		if ferr == nil && len([]rune(text)) >= 600 {
			fromArticle++
		}
		t.Logf("── %s", src.Title)
		t.Logf("   来源 %s | 抓到正文 %d 字 | 实际用作原文 %d 字",
			src.Source, len([]rune(text)), len([]rune(ground)))

		// 🚨 跑的是**生产那段循环**（news.WriteOne：写一次，验不过带着理由重写
		// 一次）。测试自己再实现一遍重试，等于生产那段没被测过。
		firstShot := true
		w, perr := news.WriteOne(src, ground, func(system string, turns []news.Turn) (string, error) {
			if len(turns) > 1 {
				firstShot = false
			}
			return liveAskJSON(t, rs, gateway.ClassDigest, system, turns)
		})
		if !firstShot {
			retried++
			t.Log("   ↻ 第一版没过，带着理由重写了一次")
		}
		if perr != nil {
			// 不是 Fatal：选星那一步多挑了几条正是为了这个。但记下来 —— 通不过
			// 的比例高了，说明 prompt 或者哪道校验该改。
			t.Errorf("   ✗ 两次都没通过校验：%v", perr)
			continue
		}
		passed++
		t.Logf("   ✓ 标题：%s", w.TitleZh)
		t.Logf("     摘要：%s", w.Summary)
		t.Logf("     它想问你：%s", w.Hook)
		t.Logf("     出自原文：%s", w.Evidence)

		// 🚨 用测试自己的判据再验一遍那句原话，不用被测代码的 squash ——
		// 用被测代码自己的函数去验它自己，等于什么都没验。
		if !containsLoosely(ground, w.Evidence) {
			t.Errorf("     evidence 在正文里查不到，校验却放行了：%q", w.Evidence)
		}
		if !strings.HasSuffix(w.Hook, "？") && !strings.HasSuffix(w.Hook, "?") {
			t.Errorf("     钩子不是一个问句：%q", w.Hook)
		}
	}
	t.Logf("正文抓得到 %d / %d；照原文写成 %d / %d（其中 %d 条是重写一次之后才过的）",
		fromArticle, len(pool), passed, len(pool), retried)
	if passed < news.PlanetCount-news.WriteMargin {
		t.Errorf("只有 %d 条写成了，凑不满一屏（余量 %d）", passed, news.WriteMargin)
	}
}

// containsLoosely 是测试侧独立实现的「这句话在不在那段文本里」：两边都只留
// 字母数字与汉字，再看子串。
func containsLoosely(haystack, needle string) bool {
	strip := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(unicode.ToLower(r))
			}
		}
		return b.String()
	}
	return strings.Contains(strip(haystack), strip(needle))
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
