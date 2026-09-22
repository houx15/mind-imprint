package api

// writing_classify_live_test.go —— 同事 2026-09-22 的意见 2 与 3，拿真模型验。
//
// 跑法：
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveWritingClassify -v -count=1
//	LIVE_LLM=1 go test ./internal/api -run TestLiveWritingOpeningAsks -v -count=1
//
// 🚨 为什么必须用真模型：这一轮改的是提示词，而 stub 测试只证明解析器读得懂
// **我自己写的** JSON（[[prompt-output-must-be-verifiable-2026-09-03]]）。
// 意见 2 和 3 说的两件事都只在真模型身上发生：
//
//	2. 「学生发送思路后，AI 自动识别成了中心论点」——她说的是「我得先选择一个
//	   有意思的词」，那是**关于怎么写**的话，不是这篇文章的内容。
//	3. 「黑心商家那个点感觉应该是和分论点并列的一个反面论证，而不是论据。」
//
// 判据都是**可以数出来的**，不是读着像不像：一个是节点条数，一个是 kind 的取值。

import (
	"strings"
	"testing"

	"context"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// 2026 全国一卷：题目要她**先选一个词**，才可能有论点。
const liveWordChoicePrompt = "阅读下面的材料，根据要求写作。词语是表达思想情感的载体，" +
	"也是展现社会生活变化的窗口。当前，世界之变、时代之变、历史之变正以前所未有的方式展开。" +
	"青年是常为新的，在你的成长过程中，你对哪一个词语的理解发生了变化？" +
	"这变化和你的成长有什么关系？请结合自身经历，写一篇文章。"

func liveWordChoiceWriting() sqlc.Writing {
	words := int32(800)
	prompt := liveWordChoicePrompt
	return sqlc.Writing{
		Lang: "zh", Title: "材料作文", TargetWords: &words, AssignedPrompt: &prompt,
	}
}

// 意见 2 的前半：她说的是「怎么写」，图上**一个节点都不该加**。
//
// 原来那一轮把「我得先选择一个有意思的词」摆成了中心论点 —— 于是她的图上
// 第一条就是一句不是主张的话，而下游每一步都拿它当这篇要证明的那句话。
func TestLiveWritingClassifyIgnoresProcessTalk(t *testing.T) {
	wr := liveWordChoiceWriting()
	raw := livePlanTurn(t, wr, nil, "我得先选择一个有意思的词。")

	reply, ok := parseWritingPlanReply(raw)
	if !ok {
		t.Fatalf("解析不了：\n%s", raw)
	}
	if len(reply.Add) != 0 {
		for _, n := range reply.Add {
			t.Errorf("往图上加了一条「%s」（kind=%s）—— 她说的是这次写作要怎么进行，"+
				"不是这篇文章要说的东西", n.Text, n.Kind)
		}
	}
}

// 意见 3：一条从反面立的判断不许被摆成论据。
//
// 判据是 kind 的取值，不是读着像不像：
//   - 论据（evidence / reference）算错 —— 那是同事截图里那个摆法，
//     也是这条测试**唯一**要拦住的事；
//   - 分论点（point）、反方观点（counter）、道理（reasoning）都算对；
//   - 一个节点都不加、reply 里反过来问她这是哪一种，**也算对**（那正是
//     意见 3 后半要的：「请你再想一下，这句真的是分论点吗？」）。
//
// 🚨 `reasoning` 是第一版判据漏掉的，而**漏掉的是我不是模型**。三次里有一次
// 它回的是 reasoning，理由写在 reply 里：「不举具体的事，而是从反面说清
// 『赚钱多不等于成功』……不过它现在还是一句空判断，最好有具体的人或事来撑。」
// 那正是闭表里 `writingKindReasoning` 的定义（「撑住一条分论点的推理，
// 不举具体的事」）。严格说它比 counter 更准：`counter` 是**反方**主张的那一点，
// 而这句话是她自己的主张从反面说一遍。
//
// 这不是为了变绿而放宽判据 —— 要拦的那件事（判断被当成论据）一个字都没松。
// 松掉的是我写错的那份期望。同一个错误 2026-09-21 记过一次
// （[[own-the-whole-product-2026-09-21]] 里那条「我的断言是错的，陪练是对的」）。
func TestLiveWritingClassifyJudgmentIsNotEvidence(t *testing.T) {
	words := int32(800)
	prompt := liveWordChoicePrompt
	wr := sqlc.Writing{Lang: "zh", Title: "成功", TargetWords: &words, AssignedPrompt: &prompt}

	// 同事截到的那张图，照着摆。
	rows := []sqlc.WritingOutline{
		{Text: "我想写「成功」这个词，我对它的理解发生了变化", Kind: writingKindThesis, Depth: 0, Position: 0},
		{Text: "成功的定义太窄，成功的人就太少", Kind: writingKindPoint, Depth: 1, Position: 1},
		{Text: "人人有自己的贡献，平凡尽责也是成功", Kind: writingKindPoint, Depth: 1, Position: 2},
		{Text: "外卖员辛勤付出让人按时吃上饭，是成功", Kind: writingKindReference, Depth: 2, Position: 3},
	}

	raw := livePlanTurn(t, wr, rows, "我还想说，黑心商家哪怕赚很多钱，也是失败。")
	reply, ok := parseWritingPlanReply(raw)
	if !ok {
		t.Fatalf("解析不了：\n%s", raw)
	}

	if len(reply.Add) == 0 {
		// 它选择先问清楚这是哪一种 —— 这是意见 3 后半明确要的做法。
		t.Logf("这一轮没加节点，reply 是：%s", reply.Reply)
		return
	}
	for _, n := range reply.Add {
		switch n.Kind {
		case writingKindCounter, writingKindPoint, writingKindReasoning:
			// 对。三种都是「一句判断」的合理落点，见上面那段。
		case writingKindEvidence, writingKindReference:
			t.Errorf("「%s」被摆成了 %s（%s）—— 它是一句判断，不是一件具体发生过的事。"+
				"这正是同事 2026-09-22 意见 3 里那个摆法。\nreply：%s",
				n.Text, n.Kind, writingKindLabel(n.Kind, ""), reply.Reply)
		default:
			t.Errorf("「%s」被摆成了 %s —— 这一条既不是理由、不是反方、也不是道理，判错了。\nreply：%s",
				n.Text, n.Kind, reply.Reply)
		}
	}
}

// 意见 2 的后半：题目要她先选一个词，开场就该问那个选择，不该问中心论点。
//
//	「上来就让学生总结中心论点好像有时候有点难，得看题而定。
//	  这个题目就是得先选择一个要写的词，才能有论点。」
//
// 判据：开场那句问题里必须出现「词」。这是这道题上唯一可以数出来的那件事 ——
// 她要选的就是一个词，而原来那一轮问的是「如果用一个词概括你最想读者相信的
// 那句话，你会说是什么？」，那是在问主张。
func TestLiveWritingOpeningAsksForTheChoiceFirst(t *testing.T) {
	prov, resolved := liveClass(t, gateway.ClassDialogue)
	wr := liveWordChoiceWriting()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingOpeningSystemFor(wr)},
			{Role: gateway.RoleUser, Content: buildWritingOpeningPrompt(wr, nil)},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("开场：\n%s", res.Text)

	if !strings.Contains(res.Text, "词") {
		t.Errorf("开场一个「词」字都没提 —— 这道题要她先选一个词，选完才有论点。\n%s", res.Text)
	}
	// 「中心论点」这个词在开场里出现，几乎一定意味着它跳过了那个选择。
	if strings.Contains(res.Text, "中心论点") {
		t.Errorf("开场就问中心论点 —— 她还不知道要写哪个词。\n%s", res.Text)
	}
}
