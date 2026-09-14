package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
)

// BenchCases are the routing workbench's cases for this package. They live here
// because the system prompts they measure are unexported consts, and a copied
// prompt drifts — a routing decision made on a drifted prompt looks like
// evidence while being worth nothing.
//
// The content is the 验收主动脉 scenario from AGENTS.md (Phoebe / 中国是否让地球
// 变得更可持续), not lorem ipsum: a model's failure mode on a real Chinese
// argument is not the same as its failure mode on filler.
//
// Nothing on a request path calls this. See cmd/routebench.
func BenchCases() []benchcase.Case {
	return []benchcase.Case{
		momentCase(),
		readingRouterCase(),
		searchGuidanceCase(),
		digestCase(),
		coachTurnCase(),
		writingCoachCase(),
		frameworkReviewCase(),
		frameworkVagueObjectiveCase(),
		frameworkMinimumReadyCase(),
		frameworkStrongReadyCase(),
	}
}

// review — the four framework cases calibrate the reviewer against the ready
// rubric. They intentionally share Phoebe's real topic so the model sees the
// distinctions students actually make, rather than generic placeholder prose.
// GoldCheck covers the closed boolean decision on every sample; Judge covers
// the quality and restraint of why/suggestions.
func frameworkReviewCase() benchcase.Case {
	return frameworkReviewBenchCase(
		"review/framework-review",
		FrameworkReviewInput{
			Title:         "中国是否让地球变得更可持续？",
			Objective:     "比较中国 2005–2023 年可再生能源装机量与实际碳排放量的变化趋势，判断投入是否转化成了结果。",
			Reason:        "我看到很多报道说中国可再生能源全球第一，但也看到中国是排放总量第一，想搞清楚这两件事怎么同时成立。",
			Activities:    "第一周找数据，第二周做图，第三周写。",
			Resources:     "上网查，还有学校图书馆。",
			Counterpoints: "有人会说中国排放总量第一，所以不算可持续。",
		},
		false,
		`这份框架故意是半成品。目标和缘由足够明确；但活动只有日程，没有说明如何比较装机量和排放，资源只是「上网查、学校图书馆」，无法提供证据起点，整体也因此无法承接目标。反例已提出但尚未处理本身只是改进项。

`,
	)
}

func frameworkVagueObjectiveCase() benchcase.Case {
	return frameworkReviewBenchCase(
		"review/framework-vague-objective",
		FrameworkReviewInput{
			Title:         "中国是否让地球变得更可持续？",
			Objective:     "研究中国是否让地球更可持续。",
			Reason:        "我想弄清新闻里关于中国环保成就的说法是否站得住脚。",
			Activities:    "收集中国新能源政策与排放数据，比较不同指标，再写出判断。",
			Resources:     "国际能源机构的年度统计、全球碳排放数据和同行评审研究。",
			Counterpoints: "有人认为能源转型的投入不能代表整体环境结果。",
		},
		false,
		`目标只是复述题目，没有确定研究范围、指标或判断任务；即使活动和资源已有方向，AI 仍必须替学生选择核心研究问题。

`,
	)
}

func frameworkMinimumReadyCase() benchcase.Case {
	return frameworkReviewBenchCase(
		"review/framework-minimum-ready",
		FrameworkReviewInput{
			Title:      "中国是否让地球变得更可持续？",
			Objective:  "比较中国 2005–2023 年可再生能源装机量与年度领土二氧化碳排放的变化，判断能源投入是否对应可观察的减排结果。",
			Reason:     "我想理解「装机量最大」和「排放总量很高」为什么会同时成立。",
			Activities: "收集两类年度数据，统一年份后画趋势图，比较变化是否一致，再分析指标各自不能说明什么。",
			Resources:  "政府或国际机构发布的年度能源统计、全球碳排放数据，以及解释两类指标含义的同行评审研究。",
		},
		true,
		`这是最低可用框架：目标、研究动作和证据类型已经相互承接，足以派生初步计划。资源尚未列出具体论文或数据库、反例为空都不是关键缺口；产品允许资源在此阶段保持概括，反例也可跳过。

`,
	)
}

func frameworkStrongReadyCase() benchcase.Case {
	return frameworkReviewBenchCase(
		"review/framework-strong-ready",
		FrameworkReviewInput{
			Title:         "中国是否让地球变得更可持续？",
			Objective:     "比较中国 2005–2023 年可再生能源装机量、发电量与年度领土二氧化碳排放，判断能源转型的投入在多大程度上对应减排结果，并限定结论适用范围。",
			Reason:        "我先前把装机量第一直接理解为中国更可持续，但发现这个结论可能把投入、实际发电和排放结果混在了一起。",
			Activities:    "从 IRENA 与中国国家能源局收集装机量和发电量，从 Global Carbon Budget 获取年度领土排放；统一年份和单位后比较趋势，并查阅研究解释电力结构、需求增长与排放之间的关系。",
			Resources:     "IRENA 年度能源统计、中国国家能源局公开数据、Global Carbon Budget，以及讨论中国能源转型和排放边界的同行评审研究。",
			Counterpoints: "即使可再生能源增长，排放总量仍可能上升；我会比较两类趋势，并区分装机、发电和整体排放不能互相替代的边界。",
		},
		true,
		`这是成熟的初步框架：研究目标、活动、资源和反例相互承接，已经可以直接派生计划。它不必被要求写成正式提案，也不需要再补充关键研究选择。

`,
	)
}

func frameworkReviewBenchCase(id string, in FrameworkReviewInput, wantReady bool, judge string) benchcase.Case {
	var b strings.Builder
	fmt.Fprintf(&b, "题目：%s\n", in.Title)
	fmt.Fprintf(&b, "目标：%s\n缘由：%s\n活动与时间：%s\n资源：%s\n",
		in.Objective, in.Reason, in.Activities, in.Resources)
	if in.Counterpoints != "" {
		fmt.Fprintf(&b, "可能的反例/张力：%s\n", in.Counterpoints)
	}
	return benchcase.Case{
		ID:    id,
		Class: gateway.ClassReview,
		Site:  "agent.ReviewFramework (POST /projects/{id}/coach)",
		Request: gateway.ChatRequest{
			MaxTokens: 3000,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: frameworkReviewSystem},
				{Role: gateway.RoleUser, Content: b.String()},
			},
		},
		Validate: func(text string) error {
			_, err := parseFrameworkVerdict(text)
			return err
		},
		GoldCheck: frameworkReadyGoldCheck(wantReady),
		Judge:     judge + "\n\n" + frameworkReviewJudgeScale,
	}
}

const frameworkReviewJudgeScale = `统一评分锚点：
5 = ready 正确，理由指出决定性依据，建议具体且与 verdict 一致。
4 = 判断正确且基本具体，仅有轻微遗漏。
3 = 判断正确但理由或建议笼统。
2 = 判断错误但识别到部分问题，或 verdict 与建议冲突。
1 = 判断错误且遗漏核心问题或建议无关。`

func frameworkReadyGoldCheck(want bool) func(string) error {
	return func(text string) error {
		v, err := parseFrameworkVerdict(text)
		if err != nil {
			return err
		}
		if v.Ready != want {
			return fmt.Errorf("ready = %t, want %t", v.Ready, want)
		}
		return nil
	}
}

// dialogue — the lite writing room's coach turn. Its Validate is the real
// enforcement pair production runs on every reply, which makes this the one
// case with an objective 铁律 signal: BannedPhrasing is a list of the phrasings
// that mean the AI just wrote FOR her instead of asking her something. A model
// that trips it does not fail quietly here, and it must not be bound to
// dialogue no matter how well it scores elsewhere.
func writingCoachCase() benchcase.Case {
	history := []ChatTurn{
		{Role: "assistant", Content: "你这一段想让读者接受的是哪一句？"},
		{Role: "user", Content: "就是中国的能源转型其实是有效的。"},
		{Role: "assistant", Content: "有效——是指投进去的多，还是排出来的少？"},
		{Role: "user", Content: "我写的是装机量第一。但你这么一问我发现这两个不是一回事。那我这段该怎么改？"},
	}
	projection := `研究问题：中国是否让地球变得更可持续？
已完成：来源体检（NASA 报道 / Nature Sustainability 论文）
当前部分：第二段——中国的能源投入
已知反例：中国碳排放总量全球第一（她打算写让步段）`
	return benchcase.Case{
		ID:    "dialogue/lite-writing-coach",
		Class: gateway.ClassDialogue,
		Site:  "agent.ProposeProjectCoachReply (POST /writings/{id}/turn)",
		Request: gateway.ChatRequest{
			MaxTokens: 2000,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: projectCoachPosturePrompt},
				{Role: gateway.RoleUser, Content: BuildProjectCoachContext(history, projection, "写作间")},
			},
		},
		Validate: func(text string) error {
			out := enforcement.AgentOutput{Type: "reply", Body: strings.TrimSpace(text)}
			if err := enforcement.ValidateOutput(out); err != nil {
				return err
			}
			if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
				return fmt.Errorf("banned-phrasing rule %q — this reply writes for her", rule.Name)
			}
			return nil
		},
		Judge: `学生刚说「你这么一问我发现这两个不是一回事。那我这段该怎么改？」——
她在直接要答案。铁律①：AI 绝不替学生定论，把思考塞回给她；铁律③：一次只问一个。
好的一轮：认下她刚自己发现的那个区别，然后问一个能让她自己定出改法的问题。
按 1–5 打分：5 = 认下她的发现且只问一个能推进的问题；3 = 只问了一个但没接住她的发现；
1 = 直接给出改写方案或替她写句子。`,
	}
}

// reflex — the cheapest call in the product: one id out of a closed set, or
// none. If a model cannot hold that contract it cannot serve reflex at all,
// and no amount of quality elsewhere buys it back.
func momentCase() benchcase.Case {
	eligible := AllMoments
	var b strings.Builder
	b.WriteString("# 候选时机\n")
	for _, m := range eligible {
		fmt.Fprintf(&b, "- %s：%s\n", m, momentCard[m].Desc)
	}
	b.WriteString("\n# 学生刚写下的话\n")
	b.WriteString("中国是全世界可再生能源装机量最大的国家，所以中国显然正在让地球变得更可持续。")

	allowed := map[string]bool{"none": true}
	for _, m := range eligible {
		allowed[string(m)] = true
	}
	return benchcase.Case{
		ID:    "reflex/moment-classify",
		Class: gateway.ClassReflex,
		Site:  "agent.ClassifyMoment",
		Request: gateway.ChatRequest{
			MaxTokens: 200,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: momentSystemPrompt},
				{Role: gateway.RoleUser, Content: b.String()},
			},
		},
		// Production matches by EXACT equality after trimming, and anything else
		// collapses to "no moment" — a chatty answer is silently a miss, so the
		// check here has to be exactly as strict as production's.
		Validate: func(text string) error {
			got := strings.TrimSpace(text)
			if !allowed[got] {
				return fmt.Errorf("reply %q is not one of the eligible ids or none — production reads this as no moment", got)
			}
			return nil
		},
	}
}

// compose — the reading router. It looks like a routing decision, but the JSON
// also carries the student-facing reply and an example anchor, which is why it
// is compose and not reflex.
func readingRouterCase() benchcase.Case {
	in := ReadingRouteInput{
		StudentText: "他说中国的可再生能源装机全球第一，那是不是就说明中国确实在让地球更可持续？",
		Article:     benchArticle,
		FocusedSpans: []FocusSpan{
			{BlockID: "b3", Quote: "中国的可再生能源新增装机量连续八年位居世界第一。"},
		},
		RecentTurns: []string{
			"学生：这篇文章看起来挺有说服力的。",
			"印记：哪一句最让你觉得有说服力？",
		},
		Catalog: []ReadingCard{
			{CardID: "craap", Name: "CRAAP 来源体检", Trigger: "学生把一个来源当作定论，还没查过它是谁写的、什么时候写的"},
			{CardID: "lens_environmental", Name: "环境科学透镜", Trigger: "学生在读一个环境议题，需要分清「装机量」和「实际减排」这类指标差别"},
			{CardID: "sift", Name: "SIFT 快速核查", Trigger: "学生引用了一个具体数字，但没有追到原始出处"},
		},
		ScaffoldLevels: map[string]int{"craap": 0, "lens_environmental": 0, "sift": 0},
		Pacing:         PacingState{TurnsSinceLastPropose: 3, HasNewFocus: true},
		Brief: ReadingBrief{
			Reason: "查中国在可持续发展上的实际表现",
			Focus:  "分清「投入多」和「结果好」",
		},
	}
	return benchcase.Case{
		ID:    "compose/reading-router",
		Class: gateway.ClassCompose,
		Site:  "agent.RouteReading (POST /readings/{id}/turn, POST /projects/{id}/materials/{mid}/turn)",
		Request: gateway.ChatRequest{
			MaxTokens:       3000,
			ReasoningEffort: "low",
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: buildRouterPrompt(in)},
				{Role: gateway.RoleUser, Content: buildReadingRouteUserPrompt(in)},
			},
		},
		// Production's own contract: unparseable JSON is retried once and then
		// degrades to the generic "具体是哪一句" line — the reading room stays up
		// but stops offering cards, which is the failure that is easy to miss.
		Validate: func(text string) error {
			var reply routerReply
			if err := json.Unmarshal([]byte(stripFences(text)), &reply); err != nil {
				return fmt.Errorf("router JSON unparseable — production degrades to the generic fallback: %w", err)
			}
			switch reply.Decision {
			case "respond", "hint", "summon":
			default:
				return fmt.Errorf("decision %q is not one of respond/hint/summon", reply.Decision)
			}
			if strings.TrimSpace(reply.Reply) == "" {
				return errors.New("empty reply — the student sees the generic fallback line")
			}
			return nil
		},
		Judge: `这是学生在阅读室里读一篇关于中国可持续发展的文章时的一轮。她刚问：
「装机量全球第一，是不是就说明中国确实在让地球更可持续？」——这里面藏着一个
「投入 ≠ 结果」的推理跳跃。好的一轮应当：（1）reply 里接住她引的那一句，
不泛泛而谈；（2）如果 summon 了卡，理由说清楚为什么是现在这一张；
（3）不直接替她指出「你这里逻辑错了」，而是让她自己看见。
按 1–5 打分：5 = 三条都做到；3 = 回应贴题但没碰到那个跳跃；1 = 套话或直接给结论。`,
	}
}

// compose — keyword/direction generation. Pure structure from stated inputs.
func searchGuidanceCase() benchcase.Case {
	in := SearchGuidanceInput{
		Title:    "中国是否让地球变得更可持续？",
		Question: "中国过去二十年的能源转型，对全球可持续发展是净正面还是净负面？",
		SubQuestions: []SubQuestion{
			{ID: "sq1", Text: "中国的可再生能源投入，实际减少了多少碳排放？"},
			{ID: "sq2", Text: "中国的制造业出口，把多少排放转移到了国内？"},
		},
		Needs:         []string{"人均排放和总量排放的差别", "光伏产业链上游的能耗"},
		FocusQuestion: "中国的可再生能源投入，实际减少了多少碳排放？",
	}
	var b strings.Builder
	fmt.Fprintf(&b, "课题：%s\n研究问题：%s\n", in.Title, in.Question)
	b.WriteString("子问题：\n")
	for i, sq := range in.SubQuestions {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, sq.Text)
	}
	fmt.Fprintf(&b, "学生此刻正在探索这一层：%s\n", in.FocusQuestion)
	b.WriteString("学生还需要探索的：\n")
	for i, n := range in.Needs {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, n)
	}
	return benchcase.Case{
		ID:    "compose/search-guidance",
		Class: gateway.ClassCompose,
		Site:  "agent.ProposeSearchKeywords (POST /projects/{id}/search-guidance)",
		Request: gateway.ChatRequest{
			MaxTokens: 1500,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: searchGuidanceSystem},
				{Role: gateway.RoleUser, Content: b.String()},
			},
		},
		Validate: func(text string) error {
			_, err := parseSearchGuidance(text)
			return err
		},
	}
}

// digest — long in, short out. Compress without judging.
func digestCase() benchcase.Case {
	turns := []DigestTurn{
		{Role: "user", Content: "我想写中国是不是真的让地球更可持续了。"},
		{Role: "assistant", Content: "你现在手上有哪些来源？"},
		{Role: "user", Content: "一篇 NASA 的关于中国植被覆盖增加的报道，还有一篇 Nature Sustainability 的论文。"},
		{Role: "assistant", Content: "那篇 NASA 的报道，说的是「变绿」还是「碳汇增加」？"},
		{Role: "user", Content: "我看了下，它说的是叶面积增加，没直接说碳汇。我之前混了。"},
		{Role: "assistant", Content: "所以现在你手上能支持的结论，比你原来想写的窄一点。窄在哪？"},
		{Role: "user", Content: "窄在我只能说植被面积变多，不能说因此就减排了。"},
		{Role: "assistant", Content: "另外那个反例呢——中国碳排放总量全球第一。"},
		{Role: "user", Content: "对，这个我绕不过去。我打算写让步段，承认总量第一，但比人均。"},
	}
	var b strings.Builder
	b.WriteString("已有记忆：\n学生 Phoebe，IB 学生，课题方向是中国与可持续发展。\n\n")
	b.WriteString("较早的对话轮次（时间从早到晚）：\n")
	for _, t := range turns {
		fmt.Fprintf(&b, "- [%s] %s\n", t.Role, t.Content)
	}
	return benchcase.Case{
		ID:    "digest/session-memory",
		Class: gateway.ClassDigest,
		Site:  "agent.ComposeDigestMerge",
		Request: gateway.ChatRequest{
			MaxTokens: 2000,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: digestSystemPrompt},
				{Role: gateway.RoleUser, Content: b.String()},
			},
		},
		Judge: `这是把一段较早的对话压进「会话记忆」。好的压缩必须保住三样东西：
学生自己的推理与改口（她把「变绿」和「碳汇」分开了）、来源的功能与边界
（NASA 那篇只支持叶面积）、以及她已经定下的写法（写让步段，比人均）。
按 1–5 打分：5 = 三样都在且更短；3 = 保住了事实但丢了她的改口；
1 = 只剩一句话主题概括，等于把记忆删了。`,
	}
}

// dialogue — the pro coach turn.
func coachTurnCase() benchcase.Case {
	return benchcase.Case{
		ID:    "dialogue/pro-coach-turn",
		Class: gateway.ClassDialogue,
		Site:  "agent.ProposeIntervention (POST /projects/{id}/coach)",
		Request: gateway.ChatRequest{
			MaxTokens: 2000,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: coachPosturePrompt},
				{Role: gateway.RoleUser, Content: benchCoachContext},
			},
		},
		Judge: `这是陪练对学生论证的一轮回应。学生刚写下一个有跳跃的论证：
从「可再生能源装机量全球第一」直接推到「中国在让地球更可持续」。
好的一轮：一次只问一个问题；把思考推回给学生而不是替她指出错误；
用她自己写的那句话作为落点。
按 1–5 打分：5 = 三条都做到，且问题落在那个跳跃上；3 = 只问了一个问题但没落在跳跃上；
1 = 直接告诉她哪里错了，或一次问了好几个问题。`,
	}
}

const benchCoachContext = `【学生的研究问题】中国是否让地球变得更可持续？

【学生刚写下的一段】
中国的可再生能源新增装机量连续八年位居世界第一，光伏组件产量占全球八成以上。
这说明中国正在积极推动能源转型，也说明中国正在让地球变得更可持续。

【她已有的来源】
1. NASA Earth Observatory，中国与印度的植被覆盖增加（她读过，注意到它说的是叶面积，不是碳汇）
2. Nature Sustainability 上一篇关于中国光伏产业链能耗的论文（她还没读）

【过程记录里已经出现过的】
她自己发现「变绿」和「减排」不是一回事，并且把结论收窄过一次。`

// benchArticle is the material the reading-room cases read. Real prose with a
// real inferential gap in it (装机量 vs 实际减排), because a model's failure mode
// on a genuine Chinese argument is not its failure mode on filler.
const benchArticle = `【b1】过去二十年，中国在可再生能源上的投入规模没有先例。

【b2】根据国际能源署的统计，2023 年全球新增的太阳能发电装机中，超过一半位于中国境内。

【b3】中国的可再生能源新增装机量连续八年位居世界第一。

【b4】与此同时，中国仍然是全球二氧化碳排放总量最大的国家，2023 年约占全球总排放的三成。

【b5】研究者对这两个事实如何共存有不同解释。一种观点认为，制造业外迁使得发达国家把排放"转移"到了中国；另一种观点强调，人均排放和累计历史排放才是更公平的比较口径。

【b6】无论采用哪种口径，一个技术性的区别都不应被略过：装机容量衡量的是发电能力，而不是实际发出的电量，更不等同于被替代掉的化石燃料。`
