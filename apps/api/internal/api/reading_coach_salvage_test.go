package api

// reading_coach_salvage_test.go —— 断在半路的回复，能不能救回学生要的那句话。
//
// 里面的样本**不是编的**：是 2026-09-08 用生产 prompt 打真模型
// （dashscope/deepseek-v4-pro，TestLiveEnglishCoachFirstTurnParses）录下来的原文，
// 只把正文缩短了一点。三次全都 finish_reason="stop"、completion 一两百 token，
// 断点全在 reply 之后。

import (
	"strings"
	"testing"
)

func salvageBlocks() []Block {
	return SplitBlocks(strings.Join([]string{
		"Is skipping breakfast a moral failure?",
		"The strongest modern claim is that breakfast eaters weigh less.",
		"When researchers ran the randomised version, the effect largely disappeared.",
	}, "\n\n"))
}

func TestCoachReplySurvivesTruncationAfterReply(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{
			// 断在 "card": 之后，一个字符的值都没有。
			name: "cut right at card",
			raw: `{"reply":"这篇文章开头就问了一个让人一愣的问题。这一步通读全文，读完告诉我你读到的第一个判断。",` +
				`"advance":"","focusBlock":"","tool":"","lens":"","card":`,
		},
		{
			// 断在卡片的 prompt 中间。
			name: "cut inside card prompt",
			raw: `{"reply":"我们一起来读这篇文章。现在先做第一步：通读全文。",` +
				`"advance":"","focusBlock":"","tool":"","lens":"",` +
				`"card":{"type":"pick_in_article","prompt":"读完全文后，划出最能让你看出作者「站在哪一边」的那`,
		},
		{
			// 断在 options 数组里 —— 英文原句最长，最常断在这儿。
			name: "cut inside card options",
			raw: `{"reply":"这篇文章在问一个听起来有点奇怪的问题。现在先做第一步——通读全文。",` +
				`"advance":"","focusBlock":"","tool":"","lens":"",` +
				`"card":{"type":"choose_span","prompt":"作者到底站哪边？","options":[` +
				`{"blockId":"b2","quote":"The strongest modern claim is that breakfast eaters weigh less."},` +
				`{"blockId":"b3","quote":"When researchers ran the randomised`,
		},
	}
	blocks := salvageBlocks()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseReadingCoachReply(tc.raw, blocks, "en", func(string) bool { return true })
			if !ok {
				t.Fatalf("turn thrown away — she sees 「AI 响应错误」 for a reply that arrived whole")
			}
			if !strings.HasPrefix(got.Reply, "这篇文章") && !strings.HasPrefix(got.Reply, "我们一起来读") {
				t.Errorf("reply is not the one the model sent: %q", got.Reply)
			}
			// 卡片没写完，就当它没给过：不能把半张卡片端上去。
			if got.Card != nil {
				t.Errorf("kept a card that never finished arriving: %+v", got.Card)
			}
		})
	}
}

// reply 自己断了，救不回来 —— 半句话不能端给她，调用点该再问一次。
func TestCoachReplyRejectedWhenReplyItselfIsCut(t *testing.T) {
	raw := `{"reply":"这篇文章在问一个听起来有点奇`
	if _, ok := parseReadingCoachReply(raw, salvageBlocks(), "en", func(string) bool { return true }); ok {
		t.Fatalf("accepted a half sentence as a coach turn")
	}
}

// 完整的回复不受影响：救援只在严格解析失败之后才跑。
func TestCoachReplyWholeStillParses(t *testing.T) {
	raw := `{"reply":"通读全文，先找出作者站哪一边。","advance":"done","focusBlock":"b2","tool":"","lens":"","card":null}`
	got, ok := parseReadingCoachReply(raw, salvageBlocks(), "en", func(string) bool { return true })
	if !ok {
		t.Fatalf("a whole reply stopped parsing")
	}
	if got.Advance != "done" || got.FocusBlock != "b2" {
		t.Errorf("fields lost: advance=%q focusBlock=%q", got.Advance, got.FocusBlock)
	}
}

// 🚨 模型压根没在写 JSON，直接说了人话。
//
// 2026-09-10 模拟学生走查抓到的，日志里逐字记着：256 个字符、stop_reason
// "stop"、一句**完全能用**的话。我们把它整份丢掉，给她看
// 「后台错误：AI 响应错误（model_unavailable）」—— 她当场卡死走不下去。
//
// 那句话就是 印记 说的话，只是没穿 JSON 那件外套。当 reply 用，别的字段全空：
// 她拿到真话，没有卡片、不推进，下一轮照常。这不是编造。
func TestCoachReplyThatIsJustProse(t *testing.T) {
	const prose = "你的眼睛很准——第4段是整个报道里信息最密的一段。" +
		"现在我要给你一张卡片，请你从整篇文章里挑一句。"
	got, ok := parseReadingCoachReply(prose, salvageBlocks(), "zh", func(string) bool { return true })
	if !ok {
		t.Fatal("一句能用的话不该被整份丢掉")
	}
	if got.Reply != prose {
		t.Fatalf("reply = %q，想要原话", got.Reply)
	}
	if got.Advance != "" || got.FocusBlock != "" || got.Tool != "" || got.Lens != "" {
		t.Fatalf("散文里没有这些字段，不该凭空长出来：%+v", got)
	}
}

// 🚨 它在写 JSON 只是写坏了 —— 这一种绝不能原样端给她。半截 JSON 摆在对话里
// 比一句报错更糟：报错至少是一句她看得懂的中文。
func TestBrokenJsonIsNotServedAsProse(t *testing.T) {
	for _, broken := range []string{
		`{"reply":"我们看第三段","advance":`,
		`好的，这就给你：{"reply":"看第三段"`,
	} {
		got, ok := parseReadingCoachReply(broken, salvageBlocks(), "zh", func(string) bool { return true })
		if ok && strings.Contains(got.Reply, "{") {
			t.Fatalf("把半截 JSON 当话端给了她：%q", got.Reply)
		}
	}
}
