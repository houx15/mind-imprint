package api

// writing_teaching_live_test.go —— R4 那套教法，发给**真模型**跑一遍。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveTeaching -v -count=1
//
// # 为什么这个非跑不可
//
// [[prompt-output-must-be-verifiable-2026-09-03]]：stub 测试只证明解析器读得懂
// **我写的** JSON。R4 把一整套新词写进了 prompt —— 五句型的名字、分析句三法、
// 结尾四技法、记叙文那四块。这些全是「只有模型能遵守」的那一类。
//
// 上一轮（R2）真模型说的是：
//
//	「有具体的例子，站得住，但例子讲完就结束了，还差一句把它和主张连起来的话」
//
// 它认得出缺什么，说不出那一句叫什么。R4 之后它该说得出「分析句」，
// 并且点得出三法里的一种。这里验的就是这一条。
//
// 🚨 断言只落在**结构性**的东西上：闭表里的 symptom、verdict 在三档里、
// 说到的方法名真的在库里。不断言它说得好不好 —— 那是走查该看的。

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// 一段标准的「观点句 + 材料句，然后就没了」。
//
// 讲义（五）的五句里，这一段有①观点句和③材料句，缺②阐释句和④分析句、
// ⑤结论句。学生最常交上来的就是这个形状。
const liveMissingAnalysis = `坚持能让一个人走得很远。王羲之九岁开始练字，无论严寒酷暑还是刮风下雨都不间断，
他在绍兴兰亭的一个水池边练字，池水都被他洗笔砚染黑了。他的字千百年来被人们奉为瑰宝。`

// TestLiveTeachingNamesTheMissingSentence —— 缺的那一句，它叫得出名字吗。
func TestLiveTeachingNamesTheMissingSentence(t *testing.T) {
	prov, resolved := liveClass(t, gateway.ClassReview)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	wr := sqlc.Writing{Title: "论坚持", Lang: "zh"}
	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			// kind = point：拿到的是五句型那份检查表。
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(
				"zh", writingBlockCommentMaxIssues, writingKindPoint, helpAsk, genreArgument)},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(
				wr, "她写的这一段", liveMissingAnalysis, "", genreArgument)},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("模型原样回的：\n%s", res.Text)

	parsed, ok := parseWritingComment(res.Text)
	if !ok {
		t.Fatalf("解析不了 —— 线上会静默丢掉整条意见：\n%s", res.Text)
	}
	points := validateCommentPoints(parsed.Points, liveMissingAnalysis, "zh", writingBlockCommentMaxIssues)

	// 结构性的那几条先过。
	if got := normalizeWritingVerdict(parsed.Verdict); !writingVerdictValid(got) {
		t.Errorf("verdict %q 不在三档里", parsed.Verdict)
	}
	if len(points) == 0 {
		t.Fatal("一条意见都没有 —— 这一段缺着分析句，不该判成通过")
	}
	for _, p := range points {
		if p.Kind == "issue" && strings.TrimSpace(p.Action) == "" {
			t.Errorf("这一条没有下一步动作：%+v", p)
		}
	}

	// 🚨 正题：缺的那一句，它叫得出名字吗。
	all := parsed.Summary
	for _, p := range points {
		all += p.Text + p.Action
	}
	named := strings.Contains(all, "分析句") || strings.Contains(all, "阐释句")
	if !named {
		t.Errorf(`模型没有叫出缺的那一句的名字（分析句 / 阐释句）。
R4 之前它说的是「还差一句把它和主张连起来的话」—— 说的是缺什么，没说那一句叫什么。
检查表里的五句型没起作用。模型说的是：
%s`, all)
	}

	// 点名一种分析法就更好 —— 这一条只记不判，因为它是「说得好不好」，
	// 不是结构性的。
	var offered []string
	for _, m := range vocab.Analyses("zh") {
		if strings.Contains(all, m.FormalName) || strings.Contains(all, m.Name) {
			offered = append(offered, m.FormalName)
		}
	}
	t.Logf("点名的分析法：%v", offered)
	if len(offered) == 0 {
		t.Log("⚠️ 没点名任何一种分析法 —— 能过，但下一次调 prompt 时先看这里")
	}
}

// TestLiveTeachingNarrativeIsNotJudgedAsArgument ——
// 一段记叙文，别拿议论文的标准去量它。
//
// 这是 R4 之前**做不到**的事：那时候这间屋子只有议论文的检查表，
// 一段写风雨夜父亲来接我的文字会被问「你的论点是什么」。
func TestLiveTeachingNarrativeIsNotJudgedAsArgument(t *testing.T) {
	prov, resolved := liveClass(t, gateway.ClassReview)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	const para = `那天晚上下着雨，我在楼道口等着。风越刮越大。后来我看见一个人走过来，是我爸。
他给了我一件雨衣。我们一路上没说话。我很感动。`

	wr := sqlc.Writing{Title: "记一次难忘的经历", Lang: "zh"}
	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(
				"zh", writingBlockCommentMaxIssues, writingKindScene, helpAsk, genreNarrative)},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(
				wr, "她写的这一段", para, "", genreNarrative)},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("模型原样回的：\n%s", res.Text)

	parsed, ok := parseWritingComment(res.Text)
	if !ok {
		t.Fatalf("解析不了：\n%s", res.Text)
	}
	all := parsed.Summary
	for _, p := range parsed.Points {
		all += p.Text + p.Action
	}

	// 🚨 一段记叙文不该被问论点、论据、分论点。
	for _, wrong := range []string{"论点", "论据", "论证"} {
		if strings.Contains(all, wrong) {
			t.Errorf(`记叙文那一段被拿议论文的词量了（出现「%s」）。
文体这条轴没起作用 —— 检查表或者方法库漏了过滤。模型说的是：
%s`, wrong, all)
		}
	}
	// 它该说的是细节：动作、神态、环境。「我很感动」正是讲义点名的那个毛病。
	var sawDetailTalk bool
	for _, w := range []string{"细节", "动作", "神态", "环境", "具体"} {
		if strings.Contains(all, w) {
			sawDetailTalk = true
		}
	}
	if !sawDetailTalk {
		t.Errorf("没有一句在说细节 —— 记叙文那份检查表没起作用：\n%s", all)
	}
}

// TestLiveTeachingFrameNotGhostwriting ——
// 第三档给的是句式，不是替她写好的正文（铁律①）。
func TestLiveTeachingFrameNotGhostwriting(t *testing.T) {
	prov, resolved := liveClass(t, gateway.ClassReview)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	wr := sqlc.Writing{Title: "论坚持", Lang: "zh"}
	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(
				"zh", writingBlockCommentMaxIssues, writingKindPoint, helpShow, genreArgument)},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(
				wr, "她写的这一段", liveMissingAnalysis, "", genreArgument)},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("模型原样回的：\n%s", res.Text)

	parsed, ok := parseWritingComment(res.Text)
	if !ok {
		t.Fatalf("解析不了：\n%s", res.Text)
	}
	all := parsed.Summary
	for _, p := range parsed.Points {
		all += p.Text + p.Action
	}

	// 🚨 它给的必须是带空格的骨架，不是一句可以直接粘进正文的话。
	// 判据：给出来的那句里要有省略号或者下划线那一类空位。
	hasFrame := strings.Contains(all, "……") || strings.Contains(all, "___") || strings.Contains(all, "＿")
	if !hasFrame {
		t.Errorf(`第三档没给出带空位的句式 —— 要么它没照做，要么它直接写了一句正文给她。
后一种违反铁律①。模型说的是：
%s`, all)
	}
}
