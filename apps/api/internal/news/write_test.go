package news

import (
	"strings"
	"testing"
)

// write_test.go —— 「照着原文写」那一步的校验。
//
// 这个文件里的每一条都对应一个**实测见过的**产出。挑一条真实的新闻当底本：
// Quanta 2026-09-08 那条，学生看到的版本是
//
//	标题：AI白天使11天？数学难题争议更大
//	它想问你：AI 找到一个特例就叫「解决」了一个难题，那之前数学家证明的过程算什么？
//
// 原文通篇没有讨论过「特例」，原标题里也没有「11 天」。下面的用例把这两件事
// 分别钉住。

const quantaTitle = "AI Has Solved One of Math’s $1 Million Millennium Prize Problems"

// quantaBody 是那篇报道的开头，逐字照抄。
const quantaBody = "On the morning of Tuesday, September 8, mathematicians at OpenAI announced " +
	"that a group of 10,000 autonomous AI agents under their direction, running on an advanced " +
	"model not available to the public, had found a “singularity” in the Navier-Stokes equations " +
	"in three dimensions — thus resolving one of the six remaining Millennium Prize Problems " +
	"posed in 2000 by the Clay Mathematics Institute. Not everyone agrees on what the result " +
	"means for the future of mathematical research."

func writeReplyJSON(titleZh, summary, hook, evidence string) string {
	esc := func(s string) string {
		return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s)
	}
	return `{"titleZh":"` + esc(titleZh) + `","summary":"` + esc(summary) +
		`","hook":"` + esc(hook) + `","evidence":"` + esc(evidence) + `"}`
}

func quantaItem() Item {
	return Item{Title: quantaTitle, Source: "Quanta Magazine", Summary: quantaBody}
}

// 一条写对了的，应该原样通过。
func TestParseWriteReplyAcceptsGroundedCopy(t *testing.T) {
	raw := writeReplyJSON(
		"AI 解决了数学界一道百万美元的千禧年大奖难题",
		"OpenAI 的数学家宣布，一万个 AI 代理在三维纳维-斯托克斯方程里找到了一个奇点。这是六道千禧年大奖难题之一。",
		"「找到一个奇点」和「解决一道难题」是同一件事吗？",
		"Not everyone agrees on what the result means for the future of mathematical research.",
	)
	got, err := ParseWriteReply(raw, quantaItem(), quantaBody)
	if err != nil {
		t.Fatalf("一条照着原文写的被打回来了：%v", err)
	}
	if got.Evidence == "" {
		t.Error("evidence 没有被带出来 —— 它要显示在问题旁边")
	}
}

// 🚨 这是这整个改动存在的理由。
//
// 「同行质疑『解决』的定义 —— 它找到了一个特例，还是真的证明了结论？」听上去
// 像一个好问题，而正文里没有任何一句话支持它。模型交不出那句原话，这一条就
// 不该出现在星图上。
func TestParseWriteReplyRejectsAnInventedControversy(t *testing.T) {
	raw := writeReplyJSON(
		"AI 解决了数学界一道百万美元的千禧年大奖难题",
		"同行质疑「解决」的定义。",
		"AI 找到一个特例就叫「解决」了难题，那数学家的证明算什么？",
		// 读上去很像原文，实际是编的。
		"Peers questioned whether finding a special case counts as solving the problem.",
	)
	_, err := ParseWriteReply(raw, quantaItem(), quantaBody)
	if err == nil {
		t.Fatal("一个正文里没有的争议被收下了")
	}
	if !strings.Contains(err.Error(), "找不到") {
		t.Errorf("打回去的理由没说清是哪里不对：%v", err)
	}
}

// 排版上的差别不算编造：弯引号写成直的、破折号写短、换行被吞掉，都还是同一句话。
func TestParseWriteReplyAcceptsEvidenceWithDifferentPunctuation(t *testing.T) {
	raw := writeReplyJSON(
		"AI 解决了数学界一道百万美元的千禧年大奖难题",
		"摘要。",
		"「奇点」和「解决」是一回事吗？",
		`had found a "singularity" in the Navier-Stokes equations in three dimensions - thus resolving one of the six remaining Millennium Prize Problems`,
	)
	if _, err := ParseWriteReply(raw, quantaItem(), quantaBody); err != nil {
		t.Fatalf("只是标点不同的原话被当成编的了：%v", err)
	}
}

// 抄半句、或者把两句拼起来，都不算原话。
func TestParseWriteReplyRejectsTooShortEvidence(t *testing.T) {
	raw := writeReplyJSON("AI 解决了一道千禧年大奖难题", "摘要。", "站得住吗？", "OpenAI")
	if _, err := ParseWriteReply(raw, quantaItem(), quantaBody); err == nil {
		t.Error("一个词被当成原话收下了")
	}
}

func TestParseWriteReplyRejectsStitchedEvidence(t *testing.T) {
	// 两段真的原话，中间的部分被跳过了 —— 拼起来的句子说的不是原文说的事。
	raw := writeReplyJSON("AI 解决了一道千禧年大奖难题", "摘要。", "站得住吗？",
		"mathematicians at OpenAI announced that a group of 10,000 autonomous AI agents means for the future of mathematical research.")
	if _, err := ParseWriteReply(raw, quantaItem(), quantaBody); err == nil {
		t.Error("拼接出来的「原话」被收下了")
	}
}

/* ── 标题 ───────────────────────────────────────────────────────────────── */

// 🚨「AI白天使11天？数学难题争议更大」——那个 11 不在原标题里，也不在正文里。
// 它是模型从旧 prompt 自己的示例钩子（「AI 十一天做完了数学家几年的活」）里
// 抄来的。示例已经删了，这条校验是那类错误的兜底。
func TestCheckTitleRejectsANumberThatIsNotInTheHeadline(t *testing.T) {
	if err := checkTitle("AI 用 11 天解决了一道千禧年大奖难题", quantaTitle); err == nil {
		t.Fatal("凭空多出来的「11 天」被收下了")
	}
}

// 🚨 同一条标题的另一半毛病：原标题是陈述句，译文却变成了提问。
// 一个问号把标题变成了这条新闻没说过的事，而且它和「它想问你」抢同一个位置。
func TestCheckTitleRejectsAQuestionMarkTheHeadlineDoesNotHave(t *testing.T) {
	if err := checkTitle("AI 解决了千禧年难题？争议更大", quantaTitle); err == nil {
		t.Fatal("被改写成提问的标题被收下了")
	}
}

// 原标题本来就是问句时（Quanta 常这么写），译文里的问号是对的。判据跟着原标题走。
func TestCheckTitleKeepsAQuestionWhenTheHeadlineAsksOne(t *testing.T) {
	en := "What Is Math’s Mysterious Langlands Program Really About?"
	if err := checkTitle("数学神秘的朗兰兹纲领到底在讲什么？", en); err != nil {
		t.Fatalf("原标题就是问句，译文的问号被打回了：%v", err)
	}
}

// 中文写数的方式不算编造：$1 Million 的如实译法是「100 万美元」，而 100 这个数
// 在原标题里确实不存在。误判一次的代价是丢掉一颗好星。
func TestCheckTitleAllowsChineseMyriadForms(t *testing.T) {
	if err := checkTitle("AI 解决了数学界一道 100 万美元的千禧年大奖难题", quantaTitle); err != nil {
		t.Fatalf("「100 万」被当成编造的数字了：%v", err)
	}
}

// 原文里出现过的数字照抄，当然可以。
func TestCheckTitleAllowsNumbersFromTheHeadline(t *testing.T) {
	en := "10,000 AI Agents Ran for 11 Days"
	if err := checkTitle("10000 个 AI 代理跑了 11 天", en); err != nil {
		t.Fatalf("原标题里有的数字被打回了：%v", err)
	}
}

// 一个话题标签不是标题的翻译。
func TestCheckTitleRejectsATagInsteadOfATranslation(t *testing.T) {
	if err := checkTitle("AI 与数学", quantaTitle); err == nil {
		t.Error("一个四字标签被当成标题的翻译收下了")
	}
}

/* ── 钩子 ───────────────────────────────────────────────────────────────── */

// 「它想问你」下面摆一句陈述，是这一屏唯一的谎。
func TestParseWriteReplyRejectsAHookThatIsNotAQuestion(t *testing.T) {
	raw := writeReplyJSON("AI 解决了一道千禧年大奖难题", "摘要。",
		"这展示了 AI 在数学推理上的潜力。",
		"Not everyone agrees on what the result means for the future of mathematical research.")
	if _, err := ParseWriteReply(raw, quantaItem(), quantaBody); err == nil {
		t.Error("一句陈述被当成钩子收下了")
	}
}

func TestParseWriteReplyRejectsAnOverlongHook(t *testing.T) {
	long := strings.Repeat("这个结论到底撑不撑得住呢", 6) + "？"
	raw := writeReplyJSON("AI 解决了一道千禧年大奖难题", "摘要。", long,
		"Not everyone agrees on what the result means for the future of mathematical research.")
	if _, err := ParseWriteReply(raw, quantaItem(), quantaBody); err == nil {
		t.Errorf("%d 字的钩子被收下了", len([]rune(long)))
	}
}

/* ── 打回去的话 ─────────────────────────────────────────────────────────── */

// 🚨 重写那一轮只有说清楚哪里错了才有意义：只说「格式错了」，模型多半原样再写
// 一遍。所以每一条打回的理由都必须**指名那个字段**。
func TestWriteRejectionsNameTheField(t *testing.T) {
	cases := []struct {
		name, raw, want string
	}{
		{"没有 evidence",
			writeReplyJSON("AI 解决了一道千禧年大奖难题", "摘要。", "站得住吗？", ""),
			"evidence"},
		{"标题空着",
			writeReplyJSON("", "摘要。", "站得住吗？", "Not everyone agrees on what the result means."),
			"titleZh"},
		{"钩子不是问题",
			writeReplyJSON("AI 解决了一道千禧年大奖难题", "摘要。", "这很厉害。",
				"Not everyone agrees on what the result means for the future of mathematical research."),
			"hook"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseWriteReply(c.raw, quantaItem(), quantaBody)
			if err == nil {
				t.Fatal("这一条本该被打回")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("打回的话里没提 %q：%v", c.want, err)
			}
		})
	}
}

// 🚨 **中文标题会暴露英文标题藏起来的政治。** 实测漏过一条
// "Venice Biennale President Defends Russia Inclusion" —— 抓取那一层看的是英文
// 原标题，一个信号词都不含；而它如实译过来的中文里，「俄罗斯」明明白白。
//
// 拆成两步之后这道闸从 select 搬到了这里：选星那一步不写中文，最早能看到中文
// 的地方就是这儿。
func TestParseWriteReplyDropsPoliticsTheEnglishTitleHid(t *testing.T) {
	it := Item{
		Title:   "Venice Biennale President Defends Russia Inclusion in New Interview",
		Source:  "Hyperallergic",
		Summary: "The president defended the decision to include Russia, saying culture and sanctions should stay apart in this case.",
	}
	raw := writeReplyJSON(
		"威尼斯双年展主席为邀请俄罗斯参展辩护",
		"主席在采访中为邀请俄罗斯辩护。",
		"文化和制裁能分开吗？",
		"The president defended the decision to include Russia, saying culture and sanctions should stay apart in this case.",
	)
	if _, err := ParseWriteReply(raw, it, it.Summary); err == nil {
		t.Error("一条政治新闻在写这一步通过了")
	}
}

/* ── 原文取哪一份 ───────────────────────────────────────────────────────── */

// 抓回来的够长就用它；不够长的是导航残渣（Nature 那几个源实测返回 0 段落），
// 拿它当正文会让 evidence 永远对不上，而失败的原因看上去像是模型的错。
func TestGroundTextPrefersTheFetchedArticleWhenItIsLongEnough(t *testing.T) {
	it := Item{Body: "feed 自带的正文", Summary: "导语"}
	long := strings.Repeat("真正的正文。", 200)
	if got := GroundText(long, it); got != long {
		t.Error("抓回来的正文没被用上")
	}
	if got := GroundText("导航 残渣", it); got != "feed 自带的正文" {
		t.Errorf("残渣被当成正文了：%q", got)
	}
	if got := GroundText("", Item{Summary: "导语"}); got != "导语" {
		t.Errorf("两条路都空时没退回导语：%q", got)
	}
}
