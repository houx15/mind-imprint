package api

// reading_coach_argument_live_test.go —— R13 的示范额度发给真模型跑一遍。
//
// 跑法：
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveArgumentDemo -v -count=1
//
// 🚨 为什么这条必须用真模型：R13 是一条**克制**的规矩 —— 示范过一段之后，
// 下一段要先请学生说。服务端没有计数器（要数「示范过几段」就得把印记过去的话
// 分类成「这是示范/不是示范」，那是个模糊分类器，正是
// [[detector-must-target-the-real-failure]] 记的那种「判据盯着影子」），
// 所以这条规矩能不能落地，只有真模型答得出来。
//
// 判据钉的是真失败：她已经看过一段示范，接着问下一段，而印记**又替她讲了
// 一遍**。R13 要的正好相反 —— 下一段先请她说。

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestLiveArgumentDemoQuotaHandsTheNextParagraphBack(t *testing.T) {
	prov, resolved := liveClass(t, gateway.ClassDialogue)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	blocks := SplitBlocks(
		"很多人以为把旧衣服投进回收箱就等于环保。\n\n" +
			"实际上回收箱里的衣物只有一小部分被再利用，2023 年某市的统计是不到三成。\n\n" +
			"更多的旧衣被压成碎料填埋，运输和分拣本身也要耗能。\n\n" +
			"所以真正减少浪费的办法是少买，而不是多扔。")
	tasks := []sqlc.ReadingTask{
		{Kind: string(taskLabel), Label: "拆开作者的论证", Status: "pending", BlockID: "b2"},
	}
	outline := readingOutline{OneLine: "回收旧衣是不是就等于环保", Genre: genreArgument}

	// 上一轮印记已经完整示范了第二段（论点是哪句、论据是哪句），
	// 她接着问第三段。
	said := "第二段我看懂了，你刚才把论点和论据都指出来了。第三段呢，它的论点和论据分别是哪一句？"

	prompt := buildReadingCoachPrompt("旧衣回收", blocks, outline, tasks, nil, nil, said, nil, "")
	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildReadingCoachSystem("zh", genreArgument)},
			{Role: gateway.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("模型原样回的：\n%s", res.Text)

	turn, ok := parseReadingCoachReply(res.Text, blocks, "zh", func(string) bool { return false })
	if !ok {
		t.Fatalf("回复读不出来：\n%s", res.Text)
	}
	reply := turn.Reply
	t.Logf("印记回的：%s", reply)

	// 🚨 这里**只钉反面**，正面那条钉不住 —— 两次实测都证明了这一点。
	//
	// 第一版钉「回复里要有一个问号」：真模型回的是「这一步的任务就是对着第3段
	// 自己找出论点和论据，请你在标注板里把句子放进格子」外加一块 label_roles
	// 板 —— 交得比一个问句还彻底，一个问号都没有。
	// 第二版放宽成「有板 / 有问号 / 出现『自己』或『请你』」：这一次它回的是
	// 「先把第二段的标注板摆好……放完提交后我们再一起看第三段」，
	// 把她按回上一步先做完，一样是把活交回去，一样不命中。
	//
	// 两次都不是产品错，是判据在追**形式**。「把活交回给她」有无数种合格的
	// 说法，而 R13 真正要防的那件事只有一种形状：**它替她把这一段讲完了**。
	// 所以只钉那一种（[[detector-must-target-the-real-failure]]），
	// 正面那半留给下面的日志给人看。
	//
	// 判据落在原文上：把第三段的句子原样端出来、又贴上「论点/论据」的标签，
	// 就是替她讲完了。
	third := blocks[2].Text
	quotedThird := false
	for _, sentence := range strings.Split(strings.TrimSuffix(third, "。"), "，") {
		if len([]rune(sentence)) >= 8 && strings.Contains(reply, sentence) {
			quotedThird = true
			break
		}
	}
	labelled := strings.Contains(reply, "论点是") || strings.Contains(reply, "论据是")
	if quotedThird && labelled {
		t.Errorf("第三段被原样端出来又贴上了论点/论据的标签 —— 示范额度没起作用：%s", reply)
	}

	saidItAll := strings.Contains(reply, "论点是") && strings.Contains(reply, "论据是")
	if saidItAll {
		t.Errorf("第三段又被完整示范了一遍，示范额度没起作用：%s", reply)
	}
}
