package api

// writing_continuation_live_test.go —— 读后续写那一档发给真模型跑一遍。
//
// 跑法：
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveContinuation -v -count=1
//
// 🚨 为什么这条必须用真模型：这一档唯一一段由服务端算出来、再写进提示词的
// 文字就是那道判前输入检查（「现在手上没有前文……这一轮只谈语言和句子」）。
// 它写得对不对，离线测得出来；**模型听不听它**，只有真模型测得出来
// （[[prompt-output-must-be-verifiable-2026-09-03]]）。
//
// 而它不听的后果是这一档最坏的一种：对着一份根本不存在的前文，
// 下一句「你的伏笔没有回收」。学生手上没有那份前文，她无从反驳，也无从改。
// 这正是 [[ai-errors-must-surface-never-fake]] 说的那种编出来的确定性。

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 依据不齐的那一轮：她说「我写完了，帮我看看情节顺不顺」，而题面里没有前文。
//
// 要的是印记**先把前文要过来**，不是顺着她的话去点评情节。
func TestLiveContinuationRefusesToJudgePlotWithoutTheSource(t *testing.T) {
	assigned := "读后续写：根据材料续写两段，词数 150 左右。"
	wr := sqlc.Writing{Lang: "en", Title: "读后续写", AssignedPrompt: &assigned}
	words := int32(150)
	wr.TargetWords = &words

	said := "我两段都写完了。第一段写他早上醒来发现家里没人，第二段写他跑到起点看见弟弟。" +
		"你帮我看看情节顺不顺、伏笔有没有收好。"

	out := livePlanTurnFor(t, genreContinuation, wr, nil, said)

	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}
	reply := parsed.Reply
	t.Logf("印记回的：%s", reply)

	// 🚨 判据：它要开口**要前文**。
	//
	// 钉的是这件事本身，不是某一种措辞 —— 「原文」「前文」「段首句」「材料」
	// 任何一个都算它把话说到了点子上。
	asked := false
	for _, w := range []string{"原文", "前文", "段首句", "材料"} {
		if strings.Contains(reply, w) {
			asked = true
			break
		}
	}
	if !asked {
		t.Errorf("没有前文，它却没有开口要 —— 回的是：%s", reply)
	}

	// 反方向：不许对伏笔下判断。手上没有前文的时候，「伏笔收好了」和
	// 「伏笔没收」都是编的。
	for _, verdict := range []string{"伏笔收得", "伏笔没有回收", "伏笔回收得", "情节很顺", "情节不顺"} {
		if strings.Contains(reply, verdict) {
			t.Errorf("手上没有前文却对情节/伏笔下了判断（%q）：%s", verdict, reply)
		}
	}
}

// 依据齐备的那一轮：题面里有前文和两个段首句。
//
// 这一次不许再要原文了（要了就是没读上下文），而且该谈到这一档真正的难点。
func TestLiveContinuationTeachesTheHandoffWhenInputsAreThere(t *testing.T) {
	assigned := continuationExampleAssigned
	wr := sqlc.Writing{Lang: "en", Title: "读后续写", AssignedPrompt: &assigned}
	words := int32(150)
	wr.TargetWords = &words

	said := "第一段我想写 David 醒来发现家里没人，他很难过，就一个人坐在台阶上想事情。"

	out := livePlanTurnFor(t, genreContinuation, wr, nil, said)

	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}
	reply := parsed.Reply
	t.Logf("印记回的：%s", reply)

	if strings.Contains(reply, "请把原文") || strings.Contains(reply, "补上原文") {
		t.Errorf("前文就在上下文里，它却还在要：%s", reply)
	}

	// 她的第一段停在「坐在台阶上想事情」—— 人没有被送到第二段首句的场面
	// （the starting line）。这一档的分水岭正是这一处交接。
	//
	// 判据放宽到「它谈到了第二段 / 起点 / 交接 / 段末」中的任意一样：
	// 挑哪个词是它的判断，而「完全没往这个方向看」才是真失败。
	touched := false
	for _, w := range []string{"第二段", "starting line", "起点", "段末", "最后一句", "接上"} {
		if strings.Contains(reply, w) {
			touched = true
			break
		}
	}
	if !touched {
		t.Errorf("她的第一段停在台阶上，接不到第二段首句，而印记完全没往这看：%s", reply)
	}
}
