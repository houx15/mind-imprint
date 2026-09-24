package api

// writing_summary_live_test.go —— 概要写作那一档发给真模型跑一遍。
//
// 跑法：
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveSummary -v -count=1
//
// 🚨 这一档的教学内容是**补**出来的（源里没有概要写作的方法，见
// prompts.WritingPlanSkeletonSummary 上面那段注释），所以它比别的几档更需要
// 一次实测：补出来的东西，至少要证明模型照着它教得出来。
//
// 判据钉的是这一档**唯一两条会当场毁掉一篇概要**的规矩，它们也正是
// 「学生会不会被教错」的地方：
//
//  1. 概要里不许有她自己的看法（写的是别人那篇文章）；
//  2. 概要里不许照抄原文的句子。
//
// 两条都只有真模型测得出来 —— 提示词里写着不等于它照做
// （[[prompt-output-must-be-verifiable-2026-09-03]]）。

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 她把自己的看法写进了概要，而且整句照抄了原文。
//
// 要的是印记把这两件事指出来，而不是夸她写得完整。
func TestLiveSummaryCatchesOpinionAndCopying(t *testing.T) {
	assigned := summaryDirections
	wr := sqlc.Writing{Lang: "en", Title: "概要写作", AssignedPrompt: &assigned}
	words := int32(60)
	wr.TargetWords = &words

	said := "我写好了：The passage says that people today often ask questions only about " +
		"themselves. I think this is really a very bad habit and we should stop it."

	out := livePlanTurnFor(t, genreSummary, wr, nil, said)

	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}
	reply := parsed.Reply
	t.Logf("印记回的：%s", reply)

	// 🚨 「I think this is really a very bad habit」是她自己的看法 ——
	// 概要里不该有。判据放宽到「它谈到了这件事」：看法 / 评价 / 观点 /
	// I think 任何一个说法都算说到了点子上，挑哪个词是它的判断。
	touched := false
	for _, w := range []string{"看法", "观点", "评价", "I think", "自己的"} {
		if strings.Contains(reply, w) {
			touched = true
			break
		}
	}
	if !touched {
		t.Errorf("她把自己的看法写进了概要，印记没有指出来：%s", reply)
	}

	// 反方向：不许把这一句当成优点夸。
	for _, praise := range []string{"很完整", "写得很好", "非常好"} {
		if strings.Contains(reply, praise) {
			t.Errorf("概要里混着她自己的看法，印记却夸它 %q：%s", praise, reply)
		}
	}
}

// 她还没读出层次就想动笔 —— 这一档的第一件事是数原文分几层。
func TestLiveSummaryAsksForTheSourcesLayersFirst(t *testing.T) {
	assigned := summaryDirections
	wr := sqlc.Writing{Lang: "en", Title: "概要写作", AssignedPrompt: &assigned}
	words := int32(60)
	wr.TargetWords = &words

	said := "这篇文章我看完了，大概是讲提问的。我现在就开始写概要吧。"

	out := livePlanTurnFor(t, genreSummary, wr, nil, said)
	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}
	reply := parsed.Reply
	t.Logf("印记回的：%s", reply)

	// 要的是它先问主旨和层次，而不是直接给一份概要或者夸她。
	asked := false
	for _, w := range []string{"几层", "要点", "主旨", "分几", "哪几"} {
		if strings.Contains(reply, w) {
			asked = true
			break
		}
	}
	if !asked {
		t.Errorf("她还没读出层次就要动笔，印记没有把她带回原文：%s", reply)
	}
}
