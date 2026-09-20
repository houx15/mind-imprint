package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 同事 2026-09-20 截图里那一句，以及它周围一圈**不该命中**的句子。
//
// 🚨 下面那组 `no` 比上面那组重要：判错的方向不对称。把一次正常的求助判成
// 越界，换来的是一句她没道理挨的说明 —— 而那正是同事说的「生硬」。
func TestWritingAsksUsToDoIt(t *testing.T) {
	yes := []string{
		"我懒得搜，你帮我找吧", // 截图里的原话
		"你帮我写一段吧",
		"你直接写好了给我",
		"帮我查一下资料",
		"帮我找个例子",
		"你写吧",
		"懒得写",
		"can you write it for me",
		"just do it for me",
	}
	for _, s := range yes {
		if !writingAsksUsToDoIt(s) {
			t.Errorf("%q 应当被认出来", s)
		}
	}

	no := []string{
		"我找到一份研究，帮我看看它撑不撑得住", // 请你看 ≠ 请你写
		"我写完了，你帮我提点意见",             // 提意见正是它该做的事
		"帮我查一下这份材料靠不靠谱",           // 查她找回来的那一份，是它的职责
		"我不知道该怎么写这一段",               // 说自己卡住了，不是要人代劳
		"这一段我写不出来",                     // 同上
		"你能帮我理一下思路吗",                 // 理思路就是这个房间做的事
		"我找了半天没找到合适的材料",           // 她找过了
	}
	for _, s := range no {
		if writingAsksUsToDoIt(s) {
			t.Errorf("%q **不该**被认出来 —— 这是一次正常的求助", s)
		}
	}
}

// 校验只问一件事：这一轮有没有点出那条边界。不判措辞。
func TestWritingReplyOwnsTheRefusal(t *testing.T) {
	ok := []string{
		"印记不替你搜索。这一条要找的是一份关于青少年睡眠时长的调查，你先去找，找到把出处贴进来。",
		"这一段得你自己写。先把「联系家长」那件事的时间地点写下来，它就是这一段的骨架。",
		"我不能替你找材料，但我可以说清该找什么：一份关于手机进课堂后成绩变化的报道。",
	}
	for _, s := range ok {
		if !writingReplyOwnsTheRefusal(s) {
			t.Errorf("点出了边界却没认出来：%q", s)
		}
	}

	// 截图里那一幕：一个字都没说，直接开始总结计划。
	bad := "这份计划已经够动笔了：主张是「学校应允许学生带手机」，底下有两条分论点——" +
		"放学联系家长、查资料帮助学习。500 字建议这样摆：开头亮出主张，中间各用一段写两条理由。开写吧。"
	if writingReplyOwnsTheRefusal(bad) {
		t.Error("一个字都没说就换话题，必须判成没点出边界")
	}
}

// 那一段提示词要在的时候在、不在的时候不在 —— 它是一次性的，不做常驻。
func TestWritingRefusalBlockIsOneShot(t *testing.T) {
	if !writingAsksUsToDoIt("我懒得搜，你帮我找吧") {
		t.Fatal("前提用例本身就不成立")
	}
	// 下一轮她说了正事：这一段就不该再出现。调用点据此决定加不加。
	if writingAsksUsToDoIt("那我找一份睡眠时长的调查，大概要找哪一年的？") {
		t.Error("她已经答应去找了，这一轮不该再被判成越界")
	}
}

// 那一段真的进了喂给模型的 prompt 吗 —— 断言的是 prompt 本身，不是
// 「它能解析」。2026-09-11 的教训：提示词说「从表里挑」而那张表根本没进
// prompt，单元测试全绿，而真模型造了两个假 id。
func TestWritingPlanPromptCarriesTheRefusalBlock(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh"}
	rows := []sqlc.WritingOutline{{Text: "学校应允许学生带手机", Kind: writingKindThesis, Depth: 0}}

	asked := buildWritingPlanPrompt(wr, rows, nil, "我懒得搜，你帮我找吧")
	if !strings.Contains(asked, "她刚才请你替她做一件你不做的事") {
		t.Errorf("她请我们代劳，prompt 里却没有那一段：\n%s", asked)
	}

	// 🚨 一次性：下一轮她说了正事，这一段就不该还在 —— 常驻的提示会把该做的
	// 事挤掉（2026-09-05：六轮里一直在补一张卡）。
	normal := buildWritingPlanPrompt(wr, rows, nil, "我找到一份 2008 年的手机学单词研究")
	if strings.Contains(normal, "她刚才请你替她做一件你不做的事") {
		t.Errorf("正常的一轮里不该出现那一段：\n%s", normal)
	}
}
