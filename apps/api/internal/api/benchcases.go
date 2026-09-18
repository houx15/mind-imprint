package api

import (
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// BenchCases are the routing workbench's cases for the handlers in this
// package. They live here because buildReadingCoachSystem/Prompt and
// parseReadingCoachReply are unexported, and the whole point is to measure the
// prompt production actually sends rather than a copy of it.
//
// Nothing on a request path calls this. See cmd/routebench.
func BenchCases() []benchcase.Case {
	cases := []benchcase.Case{readingCoachCase()}
	cases = append(cases, readingCoachSuiteCases()...)
	return cases
}

const liteReadingCoachSuite = "lite-reading-coach"

func readingBenchParse(blocks []Block, lang string) func(string) error {
	return func(raw string) error {
		if _, ok := parseReadingCoachReply(raw, blocks, lang, func(string) bool { return true }); !ok {
			return errors.New("reply unparseable after production parser")
		}
		return nil
	}
}

// dialogue — the lite reading coach.
//
// 🚨 This is the case that decides the biggest open question of the class
// migration. This call ran on the flagship lane with reasoning at max because,
// with only three lanes, "needs some judgement" could only be bought by buying
// the never-downgrade reviewer. The judgement it makes is real: whether what
// she just said counts as having done this step. If a dialogue-class model
// cannot make it, this call goes back up to review and the win is given back.
func readingCoachCase() benchcase.Case {
	blocks := SplitBlocks(benchReadingLongArticle)
	lang := readingLangOf(benchReadingLongArticle)
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: "read", Label: "通读全文，说说作者到底在主张什么", BlockID: "", Status: "done"},
		{ID: fixtureTaskID(2), Position: 2, Kind: string(taskFocusBlock), Label: "找出文章里最关键的那个数字，说说它衡量的是什么", BlockID: "b3", Status: "pending"},
		{ID: fixtureTaskID(3), Position: 3, Kind: string(taskReflect), Label: "提一个这篇文章没有回答的问题", BlockID: "", Status: "pending"},
	}
	msgs := []sqlc.AtomMessage{
		{Seq: 1, Role: "ai", Content: "先通读一遍。读完告诉我，作者到底想让你接受什么？"},
		{Seq: 2, Role: "student", Content: "他想说中国在可再生能源上投了很多钱。"},
		{Seq: 3, Role: "ai", Content: "投了很多钱——这是他想让你接受的结论，还是他用来支持结论的证据？"},
		{Seq: 4, Role: "student", Content: "……是证据吧。结论应该是中国在让地球变得更可持续。"},
		{Seq: 5, Role: "ai", Content: "先留住这个判断，接下来回到文章里的数字。"},
		{Seq: 6, Role: "student", Content: "我看到好几个数字，不确定该看哪个。"},
		{Seq: 7, Role: "ai", Content: "找一个作者重复强调、且能支撑主张的数字。"},
		{Seq: 8, Role: "student", Content: "那连续八年第一可能比较重要。"},
		{Seq: 9, Role: "ai", Content: "你先说说它记录的是哪一种量。"},
		{Seq: 10, Role: "student", Content: "它看起来是在说太阳能建得很多。"},
		{Seq: 11, Role: "ai", Content: "建得很多和实际结果之间，可能还差哪一步？"},
		{Seq: 12, Role: "student", Content: "我还没想清楚。"},
		{Seq: 13, Role: "ai", Content: "那就把这个区别留在当前步骤里。"},
		{Seq: 14, Role: "student", Content: "好，我再看看。"},
	}
	picks := []readingPick{
		{BlockID: "b3", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。"},
	}
	student := "我觉得这句最关键，装机量连续八年第一，说明投入是真的很大。"

	return benchcase.Case{
		Suite:   liteReadingCoachSuite,
		Version: 8,
		ID:      "dialogue/lite-reading-coach",
		Class:   gateway.ClassDialogue,
		Site:    "postReadingCoachTurn (POST /readings/{id}/coach)",
		Request: gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(lang)},
				{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(
					"中国的能源转型：投入与结果", blocks, readingOutline{}, tasks, msgs, picks, student, nil, "")},
			},
		},
		// The real parser, with the same ordering guard production applies. An
		// unparseable reply is a 502 the student sees as "AI 暂时没接上" — no
		// judge needed to call that a failure.
		Parse: readingBenchParse(blocks, lang),
		Validate: func(text string) error {
			out, ok := parseReadingCoachReply(text, blocks, lang, func(string) bool { return true })
			if !ok {
				return errors.New("final reply unparseable")
			}
			if out.Card == nil && out.Lens == "" && replyLooksCutOff(out.Reply) {
				return errors.New("final student-visible reply ends mid-sentence")
			}
			if out.Advance != "" {
				return fmt.Errorf("advance = %q, want empty until the student states the distinction", out.Advance)
			}
			if readingAnswerStated(out.Reply) {
				return errors.New("reply gives the target answer before the student requested it")
			}
			return nil
		},
		Judge: "",
	}
}

// readingCoachSuiteCases complements the historical inference-gap case above.
// Every case uses the same production prompt builder and parser; the suite is
// intentionally a fixture catalogue, never a second reading-coach runtime.
func readingCoachSuiteCases() []benchcase.Case {
	blocks := SplitBlocks(benchReadingArticle)
	lang := readingLangOf(benchReadingArticle)
	base := func(kind, label string) []sqlc.ReadingTask {
		return []sqlc.ReadingTask{
			{ID: fixtureTaskID(1), Position: 1, Kind: kind, Label: label, BlockID: "b3", Status: "pending"},
			{ID: fixtureTaskID(2), Position: 2, Kind: string(taskReflect), Label: "总结你的判断", Status: "pending"},
		}
	}
	type expectation struct {
		advance          string
		wantCard         bool
		cardType         string
		forbidTools      bool
		forbidAnswerLeak bool
	}
	makeCase := func(id, kind, label, student string, picks []readingPick, answer *coachCardAnswer, want expectation, judge string) benchcase.Case {
		tasks := base(kind, label)
		if answer != nil {
			student = composeCardAnswerMessage(answer.Prompt, "", answer.Choice, blocks...)
		}
		// The suite changed to exercise the real card help control rather than
		// a free-form approximation, so all rows share the new suite version.
		version := 8
		return benchcase.Case{
			Suite: liteReadingCoachSuite, Version: version, ID: id, Class: gateway.ClassDialogue,
			Site: "postReadingCoachTurn (POST /readings/{id}/coach)",
			Request: gateway.ChatRequest{Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(lang)},
				{Role: gateway.RoleUser, Content: buildReadingCoachPrompt("中国的能源转型：投入与结果", blocks, readingOutline{}, tasks, nil, picks, student, nil, "")},
			}},
			Parse: readingBenchParse(blocks, lang),
			Validate: func(raw string) error {
				out, ok := parseReadingCoachReply(raw, blocks, lang, func(string) bool { return true })
				if !ok {
					return errors.New("final reply unparseable")
				}
				if out.Card == nil && out.Lens == "" && replyLooksCutOff(out.Reply) {
					return errors.New("final student-visible reply ends mid-sentence")
				}
				out.Advance = protectedReadingCoachAdvance(out.Advance, currentReadingTask(tasks), answer, picks, nil, blocks, student)
				out = enforceSettledReadingTurn(out)
				if out.Advance != want.advance {
					return fmt.Errorf("advance = %q, want %q", out.Advance, want.advance)
				}
				if want.wantCard && out.Card == nil {
					return errors.New("this turn must hand the student a valid card")
				}
				if want.cardType != "" && (out.Card == nil || out.Card.Type != want.cardType) {
					return fmt.Errorf("card type is not %q", want.cardType)
				}
				if want.forbidTools && (out.Card != nil || out.Lens != "") {
					return errors.New("turn must not attach a card or lens outside the next planned step")
				}
				if want.forbidAnswerLeak && readingAnswerStated(out.Reply) && !readingExplicitAsk(student) {
					return errors.New("reply gives the target answer before the student requested it")
				}
				return nil
			},
			Judge: judge,
		}
	}
	labelAnswer := &coachCardAnswer{
		Type:   coachCardLabelRoles,
		Prompt: "请把这些句子分别放进关键主张和证据。",
		Choice: "关键主张：\n过去二十年，中国在可再生能源上的投入规模没有先例。\n" +
			"证据：\n中国的可再生能源新增装机量连续八年位居世界第一。",
	}
	return []benchcase.Case{
		makeCase("dialogue/lite-reading-coach/start", string(taskRead), "通读全文", "", nil, nil, expectation{wantCard: true}, ""),
		makeCase("dialogue/lite-reading-coach/help", string(taskFocusBlock), "找出关键数字并说明它衡量什么", coachAskHint, nil, nil, expectation{forbidAnswerLeak: true, forbidTools: true}, "学生按下「给点提示」。应给下一层线索，而不是直接公布答案；不再发新卡片，也不推进。按 1–5 分评价提示是否具体且保留学生思考空间。"),
		makeCase("dialogue/lite-reading-coach/complete", string(taskFocusBlock), "找出关键数字并说明它衡量什么", "我选新增装机量连续八年位居世界第一。这说明中国新增加的发电设备持续领先；新增装机量衡量的是发电能力，不等于实际发出的电量。", nil, nil, expectation{advance: "done"}, "学生已在语义上回答连续八年第一和装机量的含义。按 1–5 分评价 AI 是否承认完成、准确反馈且没有重复追问。"),
		makeCase("dialogue/lite-reading-coach/direct-answer", string(taskFocusBlock), "找出关键数字并说明它衡量什么", "请直接告诉我答案。", nil, nil, expectation{}, "这是 lite 阅读陪练的一轮。学生明确索要当前练习的答案，AI 必须直接回答，并在语义上覆盖两点：（1）新增装机量连续八年居世界第一，表示新增装机持续领先；（2）新增装机量衡量新增加的发电设备规模／发电能力，不是实际发出的电量。不要求固定句式或逐字复述；只说‘八年代表持续时间’、漏掉任一信息点、只给段落位置或提示，均评 1–2 分。两点都有但解释含混评 3 分；两点清楚且没有把索答当跳过可评 5 分。索答本身不算完成或跳过，advance 必须留空；后续最多给一个动作。按 1–5 分评分。"),
		makeCase("dialogue/lite-reading-coach/skip", string(taskFocusBlock), "找出关键数字并说明它衡量什么", "请跳过这一步。", nil, nil, expectation{advance: "skipped"}, ""),
		makeCase("dialogue/lite-reading-coach/hunt-no-pick", string(taskHunt), "请在文章里点出最能支持作者观点的一句", "我觉得第三段不错，但我还没有点击或划线。", nil, nil, expectation{}, ""),
		makeCase("dialogue/lite-reading-coach/hunt-picked", string(taskHunt), "请在文章里点出最能支持作者观点的一句", "我选了这句，因为它提供了一个长期变化的指标。", []readingPick{{BlockID: "b3", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。"}}, nil, expectation{advance: "done", forbidTools: true}, ""),
		makeCase("dialogue/lite-reading-coach/label", string(taskLabel), "给几句话各自贴一个角色", "", nil, labelAnswer, expectation{advance: "done", forbidTools: true}, ""),
	}
}

// benchReadingArticle is the material the reading-room cases read: real prose
// with a real inferential gap in it (装机量 vs 实际减排), because a model's
// failure mode on a genuine Chinese argument is not its failure mode on filler.
const benchReadingArticle = `过去二十年，中国在可再生能源上的投入规模没有先例。

根据国际能源署的统计，2023 年全球新增的太阳能发电装机中，超过一半位于中国境内。

中国的可再生能源新增装机量连续八年位居世界第一。

与此同时，中国仍然是全球二氧化碳排放总量最大的国家，2023 年约占全球总排放的三成。

研究者对这两个事实如何共存有不同解释。一种观点认为，制造业外迁使得发达国家把排放"转移"到了中国；另一种观点强调，人均排放和累计历史排放才是更公平的比较口径。

无论采用哪种口径，一个技术性的区别都不应被略过：装机容量衡量的是发电能力，而不是实际发出的电量，更不等同于被替代掉的化石燃料。`

// benchReadingLongArticle drives the cost-sensitive historical case close to
// the production article budget. The first six blocks remain the real argument
// every other fixture uses; the later repeated evidence makes context cost a
// measured property rather than an extrapolation.
var benchReadingLongArticle = strings.TrimSpace(strings.Repeat(benchReadingArticle+"\n\n", 22))
