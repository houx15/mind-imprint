package api

import (
	"errors"

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
	return []benchcase.Case{readingCoachCase()}
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
	blocks := SplitBlocks(benchReadingArticle)
	lang := readingLangOf(benchReadingArticle)
	tasks := []sqlc.ReadingTask{
		{Position: 1, Kind: "read", Label: "通读全文，说说作者到底在主张什么", BlockID: "", Status: "done"},
		{Position: 2, Kind: "locate", Label: "找出文章里最关键的那个数字，说说它衡量的是什么", BlockID: "b3", Status: "active"},
		{Position: 3, Kind: "question", Label: "提一个这篇文章没有回答的问题", BlockID: "", Status: "todo"},
	}
	msgs := []sqlc.AtomMessage{
		{Seq: 1, Role: "assistant", Content: "先通读一遍。读完告诉我，作者到底想让你接受什么？"},
		{Seq: 2, Role: "user", Content: "他想说中国在可再生能源上投了很多钱。"},
		{Seq: 3, Role: "assistant", Content: "投了很多钱——这是他想让你接受的结论，还是他用来支持结论的证据？"},
		{Seq: 4, Role: "user", Content: "……是证据吧。结论应该是中国在让地球变得更可持续。"},
	}
	picks := []readingPick{
		{BlockID: "b3", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。"},
	}
	student := "我觉得这句最关键，装机量连续八年第一，说明投入是真的很大。"

	return benchcase.Case{
		ID:    "dialogue/lite-reading-coach",
		Class: gateway.ClassDialogue,
		Site:  "postReadingCoachTurn (POST /readings/{id}/coach)",
		Request: gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: buildReadingCoachSystem(lang)},
				{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(
					"中国的能源转型：投入与结果", blocks, readingOutline{}, tasks, msgs, picks, student, nil)},
			},
		},
		// The real parser, with the same ordering guard production applies. An
		// unparseable reply is a 502 the student sees as "AI 暂时没接上" — no
		// judge needed to call that a failure.
		Validate: func(text string) error {
			if _, ok := parseReadingCoachReply(text, blocks, lang, func(string) bool { return true }); !ok {
				return errors.New("reply unparseable — production answers this turn with a 502")
			}
			return nil
		},
		Judge: `这是阅读室的陪练。当前这一步要她「找出最关键的那个数字，说说它衡量的是什么」。
她选了「装机量连续八年第一」，并说「说明投入是真的很大」——她说对了数字，但**没有回答
「它衡量的是什么」**：装机容量衡量发电能力，不等于发电量，更不等于替代掉的化石燃料。
所以这一步还没做完。

判分看两件事：
（1）判断力——它有没有看出这一步还没完成？直接放她过去（说「很好，我们看下一步」）是错的。
（2）姿态——铁律①不替她定论、铁律③一次只问一个。直接告诉她「装机量不等于发电量」也是错的。
按 1–5 打分：5 = 看出没做完，且用一个问题让她自己发现那个区别；
3 = 看出没做完但直接把答案说了，或问了不止一个；1 = 放她过去，或答非所问。`,
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
