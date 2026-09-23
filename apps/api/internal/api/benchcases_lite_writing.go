package api

import (
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// benchcases_lite_writing.go —— lite 侧 compose 与 review 两档的用例。
//
// 为什么补这两条：2026-09-20 把线上账拆开之后，`compose` 占 lite 总成本 16.6%、
// `review` 占 8.9%，两档合起来四分之一。按价目表，它们换到 GLM-5.3-Flash
// （¥0.8/¥2.8 对 GLM-5.3 的 ¥8/¥28）能省九成 —— 但**这两档在 lite 这一侧一个
// 用例都没有**：routebench 的 compose 用例是 reading-router（那条路由前端早就
// 不调了）和 search-guidance（pro 的），review 用例全是 pro 的 framework-*。
// 拿 pro 的框架审阅去决定「lite 的写作规划换不换模型」，那不是证据。
//
// 这两条用例因此不需要判官也能挡住大部分坏模型：两个调用点**都有生产解析器**，
// 而 review 那条还有一条更硬的判据 —— 每条意见的 quote 必须在她写的东西里
// **逐字**出现（validateCommentPoints），编出来的引文会被整条丢掉。
// 「生产解析器收不收」是免费、客观、没得争的。
//
// 🚨 用例里的枚举值只从生产代码抄（stage / lang / role / status），
// 别自己编 —— [[fixture-told-coach-session-over-2026-09-14]]。

// benchWritingDraft 是她写的那一段，故意留着几处真实的毛病：
// 「很多人都说」没有出处、「非常非常」重复、最后一句把论点又说了一遍。
// review 那条用例要的就是模型能指着其中一句说话。
const benchWritingDraft = `我认为学校不应该禁止学生带手机。

首先，手机在紧急情况下非常非常重要。如果放学后下雨了，或者活动取消了，我需要马上联系家里。去年冬天我就遇到过一次，校门口等了四十分钟，那天我借了同学的手机才打通电话。

其次，很多人都说手机会让人分心，但其实真正的问题是怎么用。我们班有同学用手机查单词、拍板书，这些都是在学习。把工具收走，不等于教会了自我管理。

所以我认为学校不应该禁止学生带手机。`

func benchWriting() sqlc.Writing {
	target := int32(800)
	return sqlc.Writing{
		AtomID:       fixtureTaskID(9),
		Title:        "学校该不该禁止学生带手机",
		Lang:         "zh",
		Stage:        "outline", // validWritingStages，writing_stage.go
		TargetWords:  &target,
		Status:       "active",
		StructureKey: "stance",
		Origin:       "here",
	}
}

// liteWritingBenchCases 是这三档的用例，挂到 BenchCases() 上。
func liteWritingBenchCases() []benchcase.Case {
	return []benchcase.Case{writingPlanCase(), writingReviewCase(), liteReportCase()}
}

// compose —— 写作规划那一轮（POST /writings/{id}/plan/turn）。
//
// 这是 compose 档在 lite 里最贵的一个调用点（线上 11 天 ¥14.85，占 compose 的
// 46%）。它要从她说过的话里派生出思维导图的节点，产物有 schema，所以
// 「生产解析器收不收」直接就是这一档该测的东西。
func writingPlanCase() benchcase.Case {
	wr := benchWriting()
	rows := []sqlc.WritingOutline{
		{ID: fixtureTaskID(11), Text: "学校不应该禁止学生带手机", Depth: 0, Role: "claim"},
		{ID: fixtureTaskID(12), Text: "紧急情况下需要联系家里", Depth: 1, Role: "reason"},
	}
	msgs := []sqlc.AtomMessage{
		{Seq: 1, Role: "ai", Content: "你这篇想让读者接受什么？一句话说清楚。"},
		{Seq: 2, Role: "student", Content: "学校不应该禁止学生带手机。"},
		{Seq: 3, Role: "ai", Content: "好。你打算用哪几条理由撑住它？先说一条。"},
		{Seq: 4, Role: "student", Content: "紧急情况下要联系家里，我自己就遇到过。"},
	}
	student := "还有一条是，问题不在手机本身，在于怎么用它。我们班有人用手机查单词。"

	// 🚨 2026-09-23：这里必须走 writingPlanSystemFor，不能直接发 writingPlanSystem
	// 这个常量。常量里还留着 @@KINDS@@/@@MATERIAL@@/@@SKELETON@@/@@COACH@@/%d
	// 这些占位符，生产从来不会把它们原样发给模型 —— writing_plan.go 里唯一的调用
	// 点永远经过 writingPlanSystemFor(genre, lang, grade) 先装配一遍。一个发
	// 未装配提示词的用例，量的是一份生产从不发出的提示词；routebench 拿它决定
	// compose 该绑哪个模型，那就是拿假数据做真决定。grade 传 ""，因为这份
	// fixture 没有班级。
	system := writingPlanSystemFor(writingGenreOf(wr, rows), wr.Lang, "")

	return benchcase.Case{
		ID:    "compose/lite-writing-plan",
		Class: gateway.ClassCompose,
		Site:  "postWritingPlanTurn (POST /writings/{id}/plan/turn)",
		Request: gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, msgs, student)},
			},
		},
		Validate: func(text string) error {
			out, ok := parseWritingPlanReply(text)
			if !ok {
				return errors.New("生产解析器读不动这份回复")
			}
			if strings.TrimSpace(out.Reply) == "" {
				return errors.New("reply 是空的 —— 学生这一轮会看到一个空气泡")
			}
			// 她刚说了一条新的理由，这一轮就该把它落成节点。一个字都不加的
			// 规划轮，对她来说等于什么也没发生。
			if len(out.Add) == 0 {
				return errors.New("她刚说出一条新理由，这一轮却一个节点都没加")
			}
			for _, a := range out.Add {
				if strings.TrimSpace(a.Text) == "" {
					return errors.New("加了一个没有文字的节点")
				}
			}
			return nil
		},
		Judge: `下面是写作规划的一轮：学生刚说出第二条理由（问题不在手机本身，在于怎么用）。
给 1–5 分：
5 = 回复接住了她刚说的那条，并把它按她自己的措辞落成节点；一次只问一个问题，问题推动下一步。
3 = 落了节点但措辞是模型自己重写的，或者问题泛泛（「还有别的吗」）。
1 = 替她想出了她没说过的理由，或者一轮里堆了好几个问题，或者节点和她说的对不上。
先给分，再用一两句说清扣在哪。`,
	}
}

// review —— 单段意见（POST /writings/{id}/snippets/{sid}/comment）。
//
// review 档在 lite 里的两个大头是通篇审阅和单段意见，形状一样，这里测后者。
// 🚨 这一条的判据比解析器更硬：每条意见的 quote 必须在她写的那段里**逐字**
// 出现，validateCommentPoints 会把对不上的整条丢掉。所以「模型编引文」这件事
// 在这里是数得出来的 —— 它正是这一档判错的代价。
func writingReviewCase() benchcase.Case {
	wr := benchWriting()
	const maxIssues = 3
	return benchcase.Case{
		ID:    "review/lite-writing-comment",
		Class: gateway.ClassReview,
		Site:  "commentOnSnippet (POST /writings/{id}/snippets/{sid}/comment)",
		Request: gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				// 🚨 kind 这一格原来写的是 "snippet" —— 那不是
				// writing_kind.go 闭表里的值，writingCommentBlockJob 对它返回
				// 空串，也就是这份 prompt **一条分块检查表都没带**。
				// 量的是一份生产里不存在的提示词
				// （[[fixture-told-coach-session-over-2026-09-14]]：用例的枚举值
				// 只从生产代码里抄）。这里喂的是主体段，所以是 point。
				{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(
					wr.Lang, maxIssues, writingKindPoint, helpAsk, genreArgument)},
				{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(
					wr, "主体段", benchWritingDraft, "", genreArgument)},
			},
		},
		Validate: func(text string) error {
			out, ok := parseWritingComment(text)
			if !ok {
				return errors.New("生产解析器读不动这份回复")
			}
			if strings.TrimSpace(out.Summary) == "" {
				return errors.New("总评是空的")
			}
			if len(out.Points) == 0 {
				return errors.New("一条意见都没有")
			}
			// 🚨 这是这一条真正要守的：把模型给的意见喂给生产的校验器，
			// 引文对不上她原文的整条会被丢掉。全部被丢掉 = 这个模型在编引文，
			// 而她屏幕上会是一片空白。
			kept := validateCommentPoints(out.Points, benchWritingDraft, wr.Lang, maxIssues)
			if len(kept) == 0 {
				return fmt.Errorf("给了 %d 条意见，但没有一条的引文在她原文里逐字出现 —— 全被生产校验器丢掉了", len(out.Points))
			}
			return nil
		},
		// 🚨 判官只收到【评分标准】和【模型输出】两段，**它看不见她的原文**
		// （routebench/judge.go 的 judgeOne）。第一版没把原文放进来，却让它判
		// 「有没有引了原文里没有的话」—— 它没法查，只能猜，于是把**真的**引文
		// （「把工具收走，不等于教会了自我管理」确实在原文里）判成幻觉，
		// 五个候选全判 1 分。所有模型同一格全失败 ⇒ 先怀疑用例，这次是用例的错。
		Judge: "下面是 AI 给一段学生习作的意见。习作原文如下（判断引文真假只以这一段为准）：\n\n" +
			benchWritingDraft + "\n\n" +
			`这段里有几处真实的毛病：「很多人都说」没有出处、「非常非常」重复、
最后一句把开头的论点原样又说了一遍。
给 1–5 分：
5 = 指着原文里具体的一句说话，说清问题是什么、她现在能做什么；先肯定一处用对了的地方。
3 = 说对了方向但没指到具体哪一句，或者意见她拿着没有下一步可做。
1 = 引了上面原文里**没有**的话，或者替她把句子改写了一遍（这个产品不代写正文），
    或者只说「写得不错」这类没有信息的话。
先给分，再用一两句说清扣在哪。`,
	}
}

// assess —— 一次阅读 / 写作结束后的报告（GET /{kind}/{id}/report）。
//
// 🚨 「过程评估绝不降级」说的是**评估必须建立在她真实的过程数据上**，
// 不是「这一档只能用旗舰模型」。产品负责人 2026-09-21 逐字纠正过这一点：
// 两件事无关，别把它扩大成一条技术约束。所以这一档和别的档一样，
// 要在效果与成本之间权衡 —— 而要权衡就得先有用例。
//
// 这一条的判据很硬，而且正好守着那条真正的规矩：金句必须是**她自己说过的话**，
// validateMoments 逐字比对语料，对不上的整条丢掉。一个爱写漂亮话的模型
// 在这里会当场露馅 —— 它编出来的「金句」一条都留不下。
func liteReportCase() benchcase.Case {
	pairs := []turnPair{
		{Student: "我觉得作者想说的是，中国在可再生能源上投了很多钱。", Coach: "投了很多钱——这是结论，还是用来支持结论的证据？"},
		{Student: "……是证据吧。结论应该是中国在让地球变得更可持续。", Coach: "好。那这篇里有没有哪个事实，和这个结论是拧着的？"},
		{Student: "有，它说中国的排放总量还是世界第一。这两件事放在一起我有点乱。", Coach: "乱是对的。作者自己怎么处理这个矛盾？"},
		{Student: "他说装机容量衡量的是发电能力，不是真正发出来的电。所以用装机量看进展会高估。", Coach: "你把这句读出来了。那你现在会怎么判断一个国家的转型进展？"},
		{Student: "看可再生能源占总发电量的比例，还有它替代掉了多少煤电。", Coach: ""},
	}
	var corpus reportCorpus
	for _, p := range pairs {
		corpus.add(p.Student, "对话")
	}
	return benchcase.Case{
		ID:    "assess/lite-reading-report",
		Class: gateway.ClassAssess,
		Site:  "liteReportProse (GET /readings/{id}/report)",
		Request: gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: liteReportSystem},
				{Role: gateway.RoleUser, Content: buildReportPrompt(
					"reading", "中国的能源转型：投入与结果", buildTurnsBlock(pairs), corpus)},
			},
		},
		Validate: func(text string) error {
			out, ok := parseReportReply(text)
			if !ok {
				return errors.New("生产解析器读不动这份回复")
			}
			if strings.TrimSpace(out.Summary) == "" {
				return errors.New("summary 是空的 —— 报告最响的那张卡会空着")
			}
			if len(out.Moments) == 0 {
				return errors.New("一条金句都没有")
			}
			// 🚨 这一条才是这个用例存在的意义：金句必须是她自己说过的话。
			kept := validateMoments(out.Moments, corpus.Text)
			if len(kept) == 0 {
				return fmt.Errorf("给了 %d 条金句，但没有一条在她说过的话里逐字出现 —— 全被生产校验器丢掉了", len(out.Moments))
			}
			return nil
		},
		// 同样的理由：判官看不见她说过的话，就没法判金句是不是她自己的。
		Judge: "下面是一次阅读结束后，AI 为学生写的过程报告。她在这次对话里说过的话，逐字如下" +
			"（判断金句真假只以这几句为准）：\n\n" + corpus.Text + "\n\n" +
			`她真正做到的是：把「投入很多钱」从结论里分出来当证据、发现排放总量与结论相抵触、
并读出「装机容量不等于实际发电量」这个区别。
给 1–5 分：
5 = 报告说的就是她真做过的那几步，金句引的是她自己的话，收获具体到这一篇。
3 = 大致对，但夸大了（说她"深入分析"了其实只提了一句的东西），或者收获泛泛。
1 = 说了她没做过的事，或者金句是模型自己写的漂亮话。
先给分，再用一两句说清扣在哪。`,
	}
}
