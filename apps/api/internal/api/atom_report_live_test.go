package api

// atom_report_live_test.go —— 报告那一次 assess 调用，对真模型跑一遍。
//
// 从 apps/api 跑（env 文件按绝对路径 source，所以不依赖 cwd）：
//
//	set -a; . /Users/houyuxin/08Coding/mind-imprint/.deploy-local/env.prod; set +a
//	LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -run TestLiveReportTurningPoints -v -count=1
//
// ## 为什么非跑不可
//
// stub 测试只证明 `parseReportReply` 读得懂**我自己写的** JSON。2026-09-03 的
// 规矩说得很清楚：新 prompt 上线前要跑一次真模型 —— 真模型把 prompt 里的脚手架
// 当成学生原话返回过。
//
// 这一条断的三件事都是**可验的**，不是「读起来怎么样」：
//
//  1. `turningPoints` 真的回得来（模型没把第四件事当成可选的）；
//  2. 每个 `turn` 都落在喂进去的那个窗口里 —— 编号是这一节全部安全性的支点，
//     越界只能整条丢掉，丢光了这一节就空了；
//  3. `why` 不是空的，而且**不是照抄她那句原话**（prompt 明说了不要复述：她的
//     原话会照原样印在旁边，复述一遍等于同一句话在那一块里说两遍）。
//
// 顺带断金句仍然守着 R4。加【对话记录】那一块最容易出的事，就是模型改从那里引
// 句子 —— 而那些句子不在 corpus 里，一引就会被 validateMoments 全部丢掉。
//
// 不走 `generateReportProse`：那条路要写 llm_call（要数据库）。这里要验的是
// prompt → 解析 → 按编号取正文这一条链子，和记账无关。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestLiveReportTurningPoints(t *testing.T) {
	// `liveClass` 只读 os.Getenv，而 key 住在 apps/api/.env.local（.gitignore
	// 里 `.env.*`，所以它进不了 git，也不会出现在任何一条命令行上）。这一行让
	// 「把 env 摆在文件里」这条路对这个测试也成立 —— cmd/api 和 evalbench
	// 已经是这么做的。
	// 🚨 `go test ./internal/api` 的工作目录是**包目录**，不是 apps/api ——
	// 所以要往上两级去找那个文件。直接写 ".env.local" 会静默找不到，然后
	// liveClass 报「no provider key」，看上去像是 key 没配。
	_ = godotenv.Load("../../.env.local", ".env.local")
	prov, resolved := liveClass(t, gateway.ClassAssess)

	msgs := []sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "我觉得这篇文章想说的是中国的太阳能做得最好。"},
		{Seq: 2, Role: "ai", Content: "文章里有哪一句支持「最好」这个说法吗？"},
		{Seq: 3, Role: "student", Content: "它说装机量世界第一。不过等一下，装机量说的是建了多少，不是实际发了多少电。"},
		{Seq: 4, Role: "ai", Content: "这两个数字确实回答的是两件事。那你原来那句结论要怎么改？"},
		{Seq: 5, Role: "student", Content: "我应该说「建得最多」，不能说「做得最好」。做得好还要看排放有没有降下来。"},
		{Seq: 6, Role: "ai", Content: "那就把这两个数字分开写，各自说清楚它回答了什么。"},
		{Seq: 7, Role: "student", Content: "还有一句「比世界其他地方加起来还多」，后面没标出处，我得自己去查一下。"},
	}

	var corpus reportCorpus
	for _, m := range msgs {
		if m.Role == "student" {
			corpus.add(m.Content, "对话")
		}
	}
	corpus.add("判断一个国家做得怎么样，不能只看它建了多少，还要看排放的走向。", "我的收获")

	pairs := numberedTurns(msgs)
	if len(pairs) != 4 {
		t.Fatalf("fixture built %d turns, want 4", len(pairs))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	res, cerr := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: liteReportSystem},
			{Role: gateway.RoleUser, Content: buildReportPrompt("reading", "中国的太阳能扩张", buildTurnsBlock(pairs), corpus)},
		},
	})
	if cerr != nil {
		t.Fatalf("model call failed: %v", cerr)
	}
	t.Logf("tokens in=%d out=%d", res.Usage.InputTokens, res.Usage.OutputTokens)

	reply, ok := parseReportReply(res.Text)
	if !ok {
		t.Fatalf("unparseable reply:\n%s", res.Text)
	}
	if len(reply.TurningPoints) == 0 {
		t.Fatalf("the model returned no turningPoints at all — the fourth instruction is not landing:\n%s", res.Text)
	}

	points := resolveTurningPoints(reply.TurningPoints, pairs)
	if len(points) == 0 {
		t.Fatalf("every turningPoint was dropped (out of range / duplicate / no reason). raw=%+v", reply.TurningPoints)
	}
	for _, tp := range points {
		if strings.TrimSpace(tp.Why) == "" {
			t.Errorf("turn %d came back with an empty why", tp.Turn)
		}
		if tp.Student != pairs[tp.Turn-1].Student {
			t.Errorf("turn %d's text is not the row's text verbatim:\n got %q\nwant %q", tp.Turn, tp.Student, pairs[tp.Turn-1].Student)
		}
		if strings.Contains(tp.Why, tp.Student) {
			t.Errorf("turn %d's why just repeats her sentence: %q", tp.Turn, tp.Why)
		}
		t.Logf("turn %d · %s", tp.Turn, tp.Why)
	}
	// 模型挑的编号里，有多少是我们只能丢掉的。丢掉本身不是失败（它可能挑了
	// 重复的），但这个数字长期变大就说明 prompt 没把「编号必须真实出现过」讲清楚。
	t.Logf("kept %d of %d picks", len(points), len(reply.TurningPoints))

	for _, m := range validateMoments(reply.Moments, corpus.Text) {
		t.Logf("金句: %s", m.Quote)
	}
	if len(reply.Moments) > 0 && len(validateMoments(reply.Moments, corpus.Text)) == 0 {
		t.Errorf("every 金句 was dropped — the model is quoting from the transcript block instead of her own material:\n%+v", reply.Moments)
	}
}
