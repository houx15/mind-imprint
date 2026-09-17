package api

import (
	"strings"
	"testing"
)

// 产品负责人 2026-09-17 第三轮走查逐字报的两条。见 reading_coach_leak.go。

func TestProtocolWordNeverReachesHerScreen(t *testing.T) {
	// 截图里她读到的那一句。
	leaky := "然后看第4张：你把\"hesperornithiforms…died out\"放对比，完全正确。\n\n这一步做完。advance给done。"
	if firstProtocolLeak(leaky) == "" {
		t.Fatal("「advance给done」没被抓到 —— 她会在屏幕上读到它")
	}
	got := stripProtocolLeak(leaky)
	if strings.Contains(got, "advance") {
		t.Errorf("协议词还在：%q", got)
	}
	// 拿掉的只能是那一句。前面那些字一个都不许动。
	if !strings.Contains(got, "你把\"hesperornithiforms…died out\"放对比，完全正确。") {
		t.Errorf("把别的话也拿掉了：%q", got)
	}
	if !strings.Contains(got, "这一步做完。") {
		t.Errorf("同一行里没漏的那半句也被拿掉了：%q", got)
	}
}

func TestProtocolLeakVariants(t *testing.T) {
	for _, leaky := range []string{
		"这一步做完。advance给done。",
		"advance: done",
		"我把 advance 设为 done。",
		"下一步 focusBlock 指向第5段。",
		`我会给 "advance":"skipped"。`,
	} {
		if firstProtocolLeak(leaky) == "" {
			t.Errorf("没抓到：%q", leaky)
		}
	}
}

// 🚨 判据收得紧：单独一个 advance 放过。英文文章里真的有这个词，而回复引原文
// 是正常的 —— 误伤一次是一轮重试（花钱、让她多等）。
func TestProtocolLeakDoesNotFireOnTheArticlesOwnEnglish(t *testing.T) {
	for _, fine := range []string{
		"第3段那句「the researchers advance a different explanation」，注意这个 advance。",
		"作者在这里推进了一步，说法比上一段强。",
		"你说的「done」这个词，在这里是「做完了」的意思。",
	} {
		if w := firstProtocolLeak(fine); w != "" {
			t.Errorf("一句正常的话被判成漏协议词（%q）：%q", w, fine)
		}
	}
}

// 2026-09-17 入口走查（星图那篇果蝇脑图）：取值单独漏出来，前面没有 advance。
// 只拿掉那个词，前面那半句是真话，要留着。
func TestBareStatusWordIsStrippedAlone(t *testing.T) {
	leaky := "你点的是第7段开头那句，它确实最干脆。第6段交代了雌果蝇图谱做到了什么规模，第7段接着讲这次怎么做，done。"
	if firstProtocolLeak(leaky) == "" {
		t.Fatal("「，done。」没被抓到")
	}
	want := "你点的是第7段开头那句，它确实最干脆。第6段交代了雌果蝇图谱做到了什么规模，第7段接着讲这次怎么做。"
	if got := stripProtocolLeak(leaky); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	for _, leaky := range []string{"这一步完成了。done", "好，我们跳过这一步 skipped。", "这一段读完了。Done。"} {
		if firstProtocolLeak(leaky) == "" {
			t.Errorf("没抓到：%q", leaky)
		}
		if got := stripProtocolLeak(leaky); strings.Contains(strings.ToLower(got), "done") || strings.Contains(got, "skipped") {
			t.Errorf("没拿干净：%q → %q", leaky, got)
		}
	}
	// 引英文原文时那个词是原文的一部分。
	for _, fine := range []string{
		"第2段说「the work is done.」，你觉得是谁做完的？",
		"注意 well done 这个搭配。",
	} {
		if w := firstProtocolLeak(fine); w != "" {
			t.Errorf("误判（%q）：%q", w, fine)
		}
	}
}

// 整条回复只有协议词的时候，还给原话 —— 空回复比一句怪话更糟，而且
// 「这一轮什么都没请她做」那道闸在守着。
func TestStripProtocolLeakNeverReturnsNothing(t *testing.T) {
	if got := stripProtocolLeak("advance给done。"); strings.TrimSpace(got) == "" {
		t.Error("整条被吃掉了 —— 她会看到一条空白的回复")
	}
}

func TestReplyMustNotCallHerShe(t *testing.T) {
	en := SplitBlocks("Aid groups are scrambling to help people caught in the war.\n\nIsrael fought back and sent planes.")
	if !replyCallsHerShe("她刚才那句说得准，往下我们看第3段。", en) {
		t.Error("英文文章上出现的「她」只可能指学生，应该抓到")
	}
	if replyCallsHerShe("你刚才那句说得准，往下我们看第3段。", en) {
		t.Error("一句正常的「你」被判成说漏嘴")
	}

	// 🚨 文章本身讲的是一个女性时，讲解里当然会有「她」—— 一律放过。
	// 宁可放过，不可误伤。
	zh := SplitBlocks("她把那本书放回书架，转身走了。\n\n多年以后她才明白那天意味着什么。")
	if replyCallsHerShe("第1段里的「她」是谁？作者到第2段才说。", zh) {
		t.Error("文章里真有「她」，讲解引它不该被判成说漏嘴")
	}
}
