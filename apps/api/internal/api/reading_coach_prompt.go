package api

// Prompt assembly for reading_coach.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

// readingCoachSystem is sent with every reading-coach turn. Keep it to
// executable rules; incident histories and lengthy rationale do not belong in
// a repeated model context.
const readingCoachSystem = prompts.ReadingCoachSystem

// openLensLine —— 她屏幕上此刻正开着一副透镜的时候，prompt 里加的那一节。
//
// 🚨 产品负责人 2026-09-17：「找不到句子的时候，没法在聊天框打字求助 ai。」
// 输入框在透镜开着的时候是锁死的，所以她唯一的出路是跳过。现在输入框放开了，
// 于是模型必须知道这件事 —— 否则她一开口，它就照常往下领新的一步，而屏幕上
// 那副透镜还在等她。
func openLensLine(anyOpen bool, name string) string {
	if !anyOpen {
		return ""
	}
	which := "一副透镜"
	if n := strings.TrimSpace(name); n != "" {
		which = "透镜「" + n + "」"
	}
	return "\n【她屏幕上正开着" + which + "，还没做完】\n" +
		"这一轮**不要领新的一步，也不要给卡片或者另一副透镜**（advance 留空，card、lens 都留空）。\n" +
		"她这一轮开口，多半是在这副透镜上卡住了。按这三种回应：\n" +
		"- 她说找不到这样的句子 → **先信她**。回去看一眼这篇文章：真的没有，就说清楚文章没有适合这项分析的句子，" +
		"请她跳过（卡片上那个跳过就在那儿）。真的有，就指到第几段、哪个词附近，让她自己去读那一句。\n" +
		"- 她问这副透镜到底要她干什么 → 用一句白话说清这种分析在看什么，再当场拿这篇里的某一句做一遍示范。\n" +
		"- 她问别的 → 回答她，然后一句话把她送回那副透镜。\n"
}

// toolAnswerLine —— 这一轮是她在段落工具（想一想 / 仿写）底下写的那一段。
//
// 🚨 和 openLensLine 同一个位置、同一个理由：它排在 prompt **最后**，压过
// 前面那条按步骤写的推进判据。
//
// 实测（2026-09-17，6 次）：只有 system prompt 里那一节说明时，6 次里有 5 次
// 只夸一句就转进清单上的「你怎么看」，有一次对她写的那段一个字没提 —— 末尾那条
// 「本步要她给出自己的判断」的判据赢了（[[reading-room-rulings-2026-09-17]]
// 第一条：散文跨不过判据，第四次）。
func toolAnswerLine(toolAnswerTurn bool) string {
	if !toolAnswerTurn {
		return ""
	}
	return "\n【这一轮不按上面那条推进判据走】\n" +
		"她刚才是在**段落工具**（想一想 / 仿写）底下写了一段，要你给反馈。这一轮只做这一件事：\n" +
		"1. 先说她写对了什么，引她的原话，具体到那一句。\n" +
		"2. 再只说**一处**最值得改的地方和为什么。仿写就看她有没有用上那个写法；想一想就看她的想法有没有落在那一段上、有没有说明相应的原文依据。\n" +
		"3. 🚨 不替她写：不给示范段、不把她那段改写一遍。\n" +
		"4. 说完就停。advance 留空，card、lens 都留空。**不要**在这一轮里开始清单上的下一件事，" +
		"最多用一句话提一下清单现在停在哪一步，**不要说有卡片在等她** —— 这一轮没有卡片。\n"
}

func buildReadingCoachPrompt(
	title string,
	blocks []Block,
	outline readingOutline,
	tasks []sqlc.ReadingTask,
	msgs []sqlc.AtomMessage,
	picks []readingPick,
	studentText string,
	lensDone *readingLensDone,
	// openLens 非空时，她屏幕上正开着一副透镜。见 openLensLine。
	openLens string,
) string {
	return renderReadingCoachPrompt(selectReadingCoachContext(title, blocks, outline, tasks, msgs, picks, studentText, lensDone, openLens)).Text
}

func renderReadingCoachPrompt(c readingCoachContext) promptassembly.Document {
	title, blocks, outline, tasks := c.Title, c.Blocks, c.Outline, c.Tasks
	picks, studentText, lensDone, openLens := c.Picks, c.StudentText, c.LensDone, c.OverrideInstruction
	var b promptassembly.Builder
	b.Mark("title", "context", "internal/api/reading_coach_context.go")
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}

	// 导读。它是排读法那一次就算出来的（reading_outline.go），学生屏幕上也摆着
	// 同一份 —— 所以这里给它，是为了让你和她看的是同一张地图，不是为了让你把它
	// 念一遍。
	b.Mark("outline", "mixed", "internal/api/reading_coach_prompt.go")
	if strings.TrimSpace(outline.OneLine) != "" || strings.TrimSpace(outline.Shape) != "" {
		b.WriteString("\n【全文分析（阅读引导结束后才向学生展示，请勿提前复述）】\n")
		if v := strings.TrimSpace(outline.OneLine); v != "" {
			b.WriteString("这篇在问：" + v + "\n")
		}
		if v := strings.TrimSpace(outline.Gist); v != "" {
			b.WriteString("中心思想：" + v + "\n")
		}
		if v := strings.TrimSpace(outline.Shape); v != "" {
			b.WriteString("结构：" + v + "\n")
		}
		b.WriteString("每段后面标着它在全文中的作用。**核心段停下来讨论；提供论据的段落可以连续阅读，" +
			"读后说明这些材料与前面哪项观点有关；过渡段简要说明即可。** 这是你安排节奏的依据。\n")
	}

	// 体裁。议论文和认不出来的体裁这一节是空的 —— 那两种的 prompt 和
	// 2026-09-17 之前一字不差。见 reading_genre.go。
	b.Mark("genre", "instruction", "internal/api/reading_coach_prompt.go")
	b.WriteString(buildGenreCoachSection(outline.Genre))

	// 这篇分成的几个部分。通读那一步照着它一部分一部分地走 —— 见 system
	// prompt 的「一部分一部分地走」。
	//
	// 🚨 段号由**服务端**数（和 readingBlockTag 同一份）。让模型自己从 b3 数出
	// 「第三段」，它会数错，而她屏幕上那个号码是服务端给的 —— 两边对不上，
	// 她照着去找就找不到。
	b.Mark("parts", "mixed", "internal/api/reading_coach_prompt.go")
	if len(outline.Parts) > 0 {
		ord := make(map[string]int, len(blocks))
		for i, blk := range blocks {
			ord[blk.ID] = i + 1
		}
		var parts strings.Builder
		ok := true
		for i, p := range outline.Parts {
			from, okFrom := ord[p.From]
			to, okTo := ord[p.To]
			if !okFrom || !okTo {
				// 正文换过了（她重新粘了一份），段 id 对不上。整份切法不给 ——
				// 指着不存在的段落的台阶比没有台阶更糟，而通读那一步本来就有
				// 「没有分部分的时候」那条后路。
				ok = false
				break
			}
			parts.WriteString(itoaSmall(i+1) + ". " + p.Title +
				"：第" + itoaSmall(from) + "–" + itoaSmall(to) + "段")
			if p.Does != "" {
				parts.WriteString("，" + p.Does)
			}
			parts.WriteString("\n")
		}
		if ok {
			b.WriteString("\n【这篇分成几个部分（清单上的通读步骤一步一个）】\n")
			b.WriteString(parts.String())
			b.WriteString("**当前这一步只管它标明的那几段。** 别的部分有它们自己的步骤，" +
				"这一轮一个字都不用提。\n")
		}
	}

	// 这一轮只展开这一步真正要读的那几段，其余点名但不铺开。见
	// reading_disclosure.go：整篇文章占了这份 prompt 的九成，而服务端本来就
	// 知道这一步管哪几段。nil = 不收窄（没有分部分，或这一步要通观全文）。
	b.Mark("article", "context", "internal/api/reading_coach_prompt.go")
	scope := c.Scope
	b.WriteString("\n【文章，按段落】\n")
	if scope != nil {
		b.WriteString("（这一步只展开它要读的那几段；其余段落在下面点名，" +
			"它们存在，只是这一轮不看。需要回头看别的部分时就说出来。）\n")
	}
	for _, part := range c.Article {
		if part.Omitted == "scope" {
			b.WriteString(part.Tag + "：（这一段这一轮没展开，但它存在）\n")
		} else if part.Omitted == "budget" {
			b.WriteString(part.Tag + "：（这一段没放进来，但它存在）\n")
		} else {
			b.WriteString(part.Tag + "：" + part.Text + "\n")
		}
	}

	b.Mark("tasks", "mixed", "internal/api/reading_coach_prompt.go")
	b.WriteString("\n【你排的读法】\n")
	current := c.Current
	for _, t := range tasks {
		mark := "待办"
		switch t.Status {
		case "done":
			mark = "已完成"
		case "skipped":
			mark = "已跳过"
		}
		// The kind rides on every line, in the same English identifiers ("hunt",
		// "connect", …) the system prompt's own "## 两种特别的步骤" section names
		// them by — so a task line and the instructions that govern it are
		// actually joined up, instead of the model reverse-inferring a kind from
		// a Chinese label. Harmless to show her-facing paragraph tags too: like
		// the block-id tags above, this is an internal marker for the model, not
		// prose it is told to repeat to her.
		line := "- [" + mark + "] (" + t.Kind + ") " + t.Label
		if t.Detail != "" {
			line += "：" + t.Detail
		}
		if t.BlockID != "" {
			line += "（这一步看 " + t.BlockID + "）"
		}
		if current != nil && t.ID == current.ID {
			line += "   ← **她现在在这一步**"
		}
		b.WriteString(line + "\n")
	}
	if current == nil {
		b.WriteString("\n所有步骤都走完了。跟她说一句收尾的话，别再领新的一步。\n")
	}

	b.Mark("history", "context", "internal/api/reading_coach_prompt.go")
	b.WriteString("\n【你们刚才聊的】\n")
	tail := c.History
	any := false
	for _, m := range tail {
		var who string
		switch m.Role {
		case "student":
			who = "她"
		case "ai":
			who = "你"
		default:
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString(who + "：" + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（还没聊过。）\n")
	}

	// 🚨 她屏幕上现在摆着的那张卡片/板，原样给它看。
	//
	// 模型只看得见自己说过的**话**，看不见随那句话发出去的 card —— 而那张卡有时
	// 根本不是它写的（标注论证那一步由服务端兜底摆板，见
	// reading_coach_board_build.go）。于是它会对着一块自己没见过的板提要求：
	// 实测「把主张那张换成文章里某个人亲口说的话」，而板上四句全是叙述句，
	// 一句引语都没有 —— 她照着做不到，当场卡死。
	//
	// 递出去的东西要让它知道，这和「没送到要告诉它」是同一条闭环的两半。
	b.Mark("open-card", "mixed", "internal/api/reading_coach_prompt.go")
	if card := c.OpenCard; card != nil {
		b.WriteString("\n【她屏幕上现在摆着这张卡片，你看不到，所以照着它说话】\n")
		b.WriteString("类型：" + card.Type + "　问题：" + card.Prompt + "\n")
		for i, o := range card.Options {
			ord, ok := readingPickOrdinal(blocks, o.BlockID)
			where := ""
			if ok {
				where = "（第" + itoaSmall(ord) + "段）"
			}
			b.WriteString("  " + itoaSmall(i+1) + ". " + where + "「" + o.Quote + "」\n")
		}
		for i, w := range card.Words {
			b.WriteString("  " + itoaSmall(i+1) + ". " + w.Term + "\n")
		}
		if len(card.Labels) > 0 {
			b.WriteString("格子：" + strings.Join(card.Labels, " / ") + "\n")
		}
		b.WriteString("🚨 **只能要求她用板上真有的东西。** 板上没有的句子、没有的词，" +
			"不要让她去找 —— 她手上只有上面这几样。\n")
	}

	// 🚨 上一轮那张卡片没发出去的话，当面告诉它为什么。
	//
	// 它自己发现不了：写完就交出去了，下一轮的上文里只有它说过的话。线上实测
	// 连着六轮在说「点这张卡」而卡片每轮都被丢掉 —— 她屏幕上是一句句指着空气的
	// 话。这是「闭环」的失败那一侧：AI 递出去的东西没送到，也得让它知道。
	b.Mark("delivery-failure", "mixed", "internal/api/reading_coach_prompt.go")
	if why := c.DroppedCardReason; why != "" {
		fix := cardFixIt[cardReject(why)]
		if fix == "" {
			fix = "这一轮换一件事做，或者直接把这一步说清楚。"
		}
		b.WriteString("\n【你上一轮递出去的东西没有到她屏幕上】\n" +
			"原因：" + why + "\n怎么改：" + fix + "\n" +
			"她那边只有你说的话，没有卡片、也没有透镜 —— 所以**不要再提「这张卡」" +
			"「这副透镜」「上面那块板」**，她看不到。\n" +
			"这一轮要么按上面的规矩重新给一次，要么就什么都不给，用一句具体的指令" +
			"把这一步说清楚。\n" +
			"🚨 **这件事不要说给她听。** 她不需要知道我们这边有校验、有规则、" +
			"有什么「系统不收」——那是我们的事，说出来只会让她觉得这个房间在出故障。\n")
	}

	// 🚨 这一步是不是卡住了（同一步带了三轮以上还没动）。卡住了才加这一节，
	// 没卡住一个字都不加 —— 常驻的提示会抢掉这一轮真正该做的事。
	// 见 reading_coach_repeat.go。
	b.Mark("stalled", "instruction", "internal/api/reading_coach_prompt.go")
	if c.StepStuck {
		b.WriteString(coachStuckNudge)
	}

	// Structurally distinct from what she typed: a pick is a pointer at a real
	// paragraph, never spoken to the model as a block id (only 第几段, same as
	// the paragraph listing above) — the id is an internal marker, not
	// something the model should ever try to repeat back to her.
	b.Mark("picks", "context", "internal/api/reading_coach_prompt.go")
	if len(picks) > 0 {
		b.WriteString("\n【她在文章里点出来的句子】\n")
		for _, p := range picks {
			ord, ok := readingPickOrdinal(blocks, p.BlockID)
			if !ok {
				continue
			}
			b.WriteString("第" + itoaSmall(ord) + "段：「" + p.Quote + "」\n")
		}
	}

	// A completed lens is HER WORK, so it gets its own section rather than
	// being folded into 【她刚刚说的】 — the finding is 印记's own earlier
	// evaluation and must never read as a sentence she uttered.
	b.Mark("lens-completion", "mixed", "internal/api/reading_coach_prompt.go")
	if lensDone.clean() {
		b.WriteString("\n【她刚做完一副透镜】\n")
		if n := strings.TrimSpace(lensDone.CardName); n != "" {
			b.WriteString("透镜：" + n + "\n")
		}
		b.WriteString("她自己在文章里找的那一句：「" + strings.TrimSpace(lensDone.Quote) + "」\n")
		if f := strings.TrimSpace(lensDone.Finding); f != "" {
			b.WriteString("你当时对这一句的复核（这是你自己的话，不是她说的）：" + f + "\n")
		}
		// 🚨 复核的结论也要给，而且要说明她已经看过了 —— 否则这一轮会跟几秒钟
		// 前屏幕上那个结论对着干。见 readingLensDone 的 Verdict。
		if w := verdictWord[strings.TrimSpace(lensDone.Verdict)]; w != "" {
			b.WriteString("你当时给出的结论（**她屏幕上已经看到了这一句**）：" + w + "\n")
			if vr := strings.TrimSpace(lensDone.VerdictReason); vr != "" {
				b.WriteString("你当时给的理由：" + vr + "\n")
			}
			b.WriteString("🚨 **这一轮不要跟这个结论相反。** 选句不符合分析要求时，不要改口说它符合；" +
				"选句符合要求时，不要反过来否定。她已经读过上面那一句了，" +
				"前后反馈应保持一致，并说明原文依据。\n" +
				"判的是**那一句话**，不是她这个人 —— 承认她动手做了，说清楚问题在哪儿，然后往下走。\n")
		}
	}

	b.Mark("latest-input", "context", "internal/api/reading_coach_prompt.go")
	if studentText != "" {
		b.WriteString("\n【她刚刚说的】\n" + studentText + "\n")
	} else if lensDone.clean() {
		// 🚨 NOT the 「她刚点了「开始」」 line below. She did not press 开始 —
		// she just finished a lens, and telling the model otherwise makes it
		// re-introduce the reading plan from the top (the PBL repeat-itself
		// bug, in this room). See readingLensDone's own comment.
		b.WriteString("\n【她刚刚说的】\n（这一轮她没打字——她是把透镜做完了。按「她刚做完一副透镜」那一节的三件事回应她。）\n")
	} else {
		// 🚨 这里原来写的是「介绍一下你排的读法」。那是一道自相矛盾的题：
		// 要它介绍，又不许它说这篇文章的内容，它只能去说「这篇大概在讲什么」
		// ——也就是内容。真模型给出的那一份把文章讲了一遍、替她减压、还以
		// 「读完告诉我一声」收尾，三条规矩一句话里全违反了。
		//
		// 导读卡（一句话 + 结构 + 段落承重）现在由前端确定性地渲染，摆在正文
		// 顶上。所以这一轮它**没有内容可讲**，只剩下递第一张卡片这一件事。
		// 见 reading_outline.go 和 docs/2026-09-10-reading-guidance-redesign.md。
		b.WriteString("\n【她刚刚说的】\n（她刚点了「开始」，还没说话。" +
			"全文总结将在阅读引导结束后展示，学生目前还没有看到。" +
			"**不要再复述一遍**，也不要讲这篇文章的内容。" +
			"直接领她进第一步，并且用一张卡片把她领进去。）\n")
	}
	// 她按了卡片底下那颗「给点提示」。级数按当前这张卡片数，见 helpRequestSection。
	b.Mark("current-step", "instruction", "internal/api/reading_coach_prompt.go")
	b.WriteString(readingCurrentStepInstruction(tasks, studentText))
	b.Mark("help-request", "instruction", "internal/api/reading_coach_help.go")
	b.WriteString(helpRequestSection(studentText, coachHintRound(tail), lastOpenCard(tail)))
	// 透镜开着这件事排在最后：它**取消**上面那条推进判据（这一轮不推进），
	// 而最后一节才是这一轮真正的指令。
	b.Mark("override", "instruction", "internal/api/reading_coach_prompt.go:openLensLine/toolAnswerLine")
	b.WriteString(openLens)

	return b.Document(c.Selections...)
}

// Bind the generic teaching rules to the one active task. This is a prompt
// projection only; it never settles state or invents a completion signal.
func readingCurrentStepInstruction(tasks []sqlc.ReadingTask, studentTexts ...string) string {
	studentText := ""
	if len(studentTexts) > 0 {
		studentText = studentTexts[0]
	}
	current := currentReadingTask(tasks)
	if current == nil {
		return "\n【本轮状态】读法清单已结束，简短收尾，不再布置阅读任务。\n"
	}
	rule := "按当前任务文字判断。学生已完成要求的各项内容，就给 done；只完成部分就留空并只提示缺少的那一项。不要添加第二个例子、更多证据或额外点击作为完成门槛。"
	switch current.Kind {
	case string(taskRead):
		// 🚨 这一条判的是**这一步标明的那几段**，不是整篇。通读在清单上已经
		// 一步一个部分（reading_plan.go 的 readingPartSteps），所以「答了就
		// done」在这里是对的 —— 它推进的是一个部分，下一步就是下一个部分。
		// 在拆步之前，这条判据和 system prompt 里「走完最后一个部分才 done」
		// 直接打架，而末尾的判据赢：一张卡答完，整个通读当场结束。
		rule = "本步只管它标明的那几段，不是整篇。学生明确说这几段读完了，或已回答本步的通读卡片，就给 done。" +
			"认可后直接介绍清单里的下一步（多半就是下一个部分），不再加一道通读测验。" +
			"reply 里说段落范围要照本步说明里那几段说，不要说「通读全文」。"
	case string(taskFocusBlock):
		// 🚨 产品负责人 2026-09-17：「切入精读部分，并没有交代为什么 ai 选中的
		// 段落是需要精读的段落。」这一段是排读法时挑出来的，理由只在服务端 —— 她
		// 屏幕上出现的是一句「往下翻到第 4 段」，凭什么是第 4 段没有人告诉她。
		if strings.TrimSpace(studentText) == "" {
			rule = "她尚未对当前步骤作答：reply 先用一句话说清**这一段为什么值得细读**（它在全文里承担什么：" +
				"唯一给数据的地方、论证的转折处、作者明确表达主要观点的一段……），再领她做。" +
				"这句话说的是这一段在文章里的位置和作用，不是它的内容摘要。"
		} else {
			rule = "她已经对当前步骤作答或提出操作请求：不要重新介绍这一段或默认发卡。按当前任务文字判断：她做完要求的各项就给 done，只完成部分就留空并只提示缺少的那一项；任务有多个信息点时，只答其中一个仍是部分回答，不能替她补上缺少答案。判断分两项：学生有没有指出数字（数量、比例、年数或排名均可），有没有说明该数字统计或比较的对象。只抄数字还需解释；两项都已说清就必须 done，不追加找第二个数字、换一种数字或跨段比较的要求。你从原文知道而她尚未说出的信息，不能算作她已回答。解释这一步的术语可引用其他段落，不能因证据不在当前段而要求重做。"
		}
	case string(taskConnect):
		// 🚨 产品负责人 2026-09-17：「阅读的链接自身那个部分有点抽象了，还有点
		// 鸡肋。」抽象是因为问法本身是空的（「这篇讲的事你碰到过吗」）——一个
		// 没有落点的问题，她只能泛泛答一句。落点要从**这篇文章里**取。
		rule = "本步的问题必须挂在这篇文章的一个具体处上：先说出文章里的某一件事、某个数字、或者作者的某一个判断（一句），" +
			"再问她在自己这儿碰到过什么与它对得上或对不上的事。不要问「你有什么感想」「你碰到过吗」这类范围过于宽泛的问题。" +
			"学生已经表达自己的经历或联想，就给 done。尊重这段经历，不要求它符合文章的标准答案。"
	case string(taskHunt):
		rule = "只看【她在文章里点出来的句子】是否有真实选句。有选句就给 done，不要求它与你偏好的句子相同，也不要求再选一句。没有真实选句时留空并说明点击操作。"
	case string(taskLabel):
		// 🚨 「依据她的分类简短反馈并给 done」后面那半句是 2026-09-17 加的。
		// 产品负责人在一篇文章里被同一块板问了四次（「three of them are the
		// same one」）—— 摆完之后觉得有一两张放错，就再发一块板让她从头摆，
		// 而她每次摆的其实是同一件事。判断该由**话**来给，不由第二块板来收。
		rule = "已收到标注板的真实作答时，依据她的分类简短反馈并给 done；尚未提交时引导她使用标注板。" +
			"普通文字说摆好了不能替代真实作答。" +
			"🚨 觉得她有一两张放错了，就在 reply 里说清那一句为什么该换个位置，然后照常给 done —— " +
			"**绝对不要再发第二块标注板**。这一步只摆一次板。"
	case string(taskCritique):
		// 🚨 2026-09-17 新增，顶掉了 透镜 那一步。产品负责人逐字：
		//
		//	we can invite students to give some comments on this, like do they
		//	agree with author's view, do they think the evidence is enough, or
		//	do they think if there is another possiblity.
		//	give perspectives suggestion, like another possibility,
		//	credibility, another explanation, etc.
		//
		// 「你怎么看」直接问出去，收到的是「我觉得挺好的」。她需要的不是一个
		// 更大的问题，是**几个角度**——而角度要挂在这篇文章的具体处上，
		// 否则又是一个 taskConnect 那样的空问题。
		rule = "本步要她给出自己的判断，不是复述作者。reply 先在**这篇文章里**点出一处她可以下手的地方" +
			"（作者的某一个判断、某一条证据、某个只有一个来源的说法），再给她两三个角度让她挑一个，" +
			"写成一行一个的短列表。角度从这几种里挑：**同不同意**这个判断、" +
			"作者提供的**材料是否足以说明这一点**、有没有**另一种解释**、这个说法的**来源可不可信**、" +
			"是否还需了解其他相关人物的情况。然后用一张 short_text 卡请她写。" +
			"她写出了自己的判断就给 done —— **不要求她的判断和你一致**，也不要求她写长。" +
			"她说同意作者，就请她说一句凭什么同意；那也是一个判断。"
	case string(taskSequence):
		// 2026-09-17，同事的阅读模块 PRD：报道「搭建事件时间线」，记叙「事件卡
		// 排序；切换发生顺序／讲述顺序」。板摆完就算做完，和标注步同一条判据
		// （代码里也兜着，见 answeredOrderBoard 的调用点）。
		rule = "本步用一块 order_events 排序板把几件事交给她，请她按发生的先后排好；给板的那一轮不要说出正确的先后。" +
			"已收到排序板的真实作答时，先接住她的排法，再用一两句说清**讲述顺序和发生顺序哪里不一样、作者为什么这样安排**" +
			"（她排错了一处，就指出原文哪几个字能看出先后），然后给 done。**不要再发第二块排序板。**" +
			"普通文字说排好了不能替代真实作答。"
	case string(taskLens):
		rule = "已收到【她刚做完一副透镜】时，反馈她的实际分析并给 done，不再要求她操作已完成的透镜；没有完成回传时按透镜步骤继续。学生询问如何开始时，说明当前工具的用途和一个具体操作即可；不猜测她害怕，不替她完成整份分析。解释时说清证据说明了什么或需要核实什么，不使用「支撑」等抽象比喻。"
	}
	return "\n【本轮推进判据】\n当前步骤：" + current.Kind + "；任务：" + current.Label + "。\n" +
		"先检查学生是否明确要求跳过当前步骤：如是，advance 必须为 skipped。单独说「我放弃」是索答，不是跳过；明确索答要直接回答，advance 留空。否则：" + rule + "\n" +
		"概念或词义提问可以直接解释；解释不算学生已经完成分析任务。学生已经完成时，advance 必须为 done；不能因介绍下一步而把 advance 留空。明确跳过时不要附加 card 或 lens。一次只推进当前一步。\n" +
		readingNextStepHandoff(tasks, current) +
		"先输出 advance，再写 reply。只根据学生实际作答判定推进，不依据你将要给出的回复。找数字并解释的任务中，分别从学生原话查找「数字」和「统计对象」，不能从文章或你自己的解释补齐。仅说「增长15%」「连续多年第一」只完成选数，advance 留空；说「某校学生人数比去年增长15%」已同时说明数字与对象，应 done。排名也可以，但学生必须说清谁按什么指标排名。两项已齐不追加任务。若她仅报出数字，reply 只请她查看数字附近的名称、单位或比较词来说明对象，不写出那个对象的名称，不先解释再请她重复。在她尚未说明对象时，不引用或复述原文整句，即使为了肯定她找到了数字也不能这样做，因为整句包含了答案；输出完整 JSON，包含 advance。给学生的文字用完整、具体的教学语言，写明所讨论的内容和关系。她以「是不是」「所以……吧」尝试推断时，暂不宣布两者的区别或因果结论；只联系当前要判断的问题，指出可比较的对象或原文位置，请她观察它们的关系。不要先讲解完整区别，再问她能不能据此判断。她已解释清楚时才具体反馈并推进。段落定位写第几段，不写 b 加数字。学生仅求提示时，本轮只说明从哪里观察，不报出待找的数字、不解释题目要求她自己解释的对象；不能先替她完成再让她复述。明确索答或独立词义提问则直接解释。最后检查最高优先级：她明确要求跳过时，只确认跳过，advance 必须是 skipped，其他工具字段留空，不交接下一步、不追问；这个情况不适用上面的教学引导与完成交接。\n"
}

// readingNextStepHandoff gives a completed step a concrete next action in
// the same reply. Skipped steps are explicitly excluded by the preceding
// current-step instruction.
func readingNextStepHandoff(tasks []sqlc.ReadingTask, current *sqlc.ReadingTask) string {
	var next *sqlc.ReadingTask
	for i := range tasks {
		if tasks[i].Status == "pending" && tasks[i].Position > current.Position {
			if next == nil || tasks[i].Position < next.Position {
				next = &tasks[i]
			}
		}
	}
	if next == nil {
		return "仅当 advance 为 done 且这是最后一步时，用一两句收尾：说清她这一篇完成了什么，不再布置任务。\n"
	}
	return "仅当 advance 为 done 时，reply 的最后一段直接交接下一步：「" + next.Label + "」（" + next.Detail + "）。" +
		"说明这一步为什么值得做、她现在要做什么；能用一张 card 承担就给下一步的 card。不要等她回一句「好」才开始。\n"
}

// readingCoachToolMenu renders the paragraph tools for the article's language
// into the coach's own prompt, from the same table the explain endpoint
// validates against — so an id the coach names is always an id the endpoint
// will accept.
func readingCoachToolMenu(lang string) string {
	var b strings.Builder
	for _, t := range readingBlockToolsFor(lang) {
		b.WriteString("- tool=" + t.ID + " · " + t.Label + "\n")
	}
	return b.String()
}

// readingCoachLensMenu renders the reading deck — id + name + when to reach
// for it — from the registry-backed agent.ReadingDeck(), the same
// single-source-of-truth shape readingCoachToolMenu uses for the paragraph
// tools. If the deck can't be resolved, an empty string is returned: the
// coach then simply never names a lens, the safe direction to fail in.
func readingCoachLensMenu() string {
	deck, err := agent.ReadingDeck()
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, c := range deck {
		b.WriteString("- lens=" + c.CardID + " · " + c.Name + " · " + c.Trigger + "\n")
	}
	return b.String()
}

// buildReadingCoachSystem assembles the coach's system prompt for the
// article's language. Both placeholders are filled with strings.Replace at
// count 1 each — not fmt.Sprintf, and each call targets its own distinct
// token (%s for the paragraph-tool menu, %LENS% for the lens menu) so the
// second substitution never collides with or re-consumes the first.
func buildReadingCoachSystem(lang string) string {
	system := strings.Replace(readingCoachSystem, "%s", readingCoachToolMenu(lang), 1)
	system = strings.Replace(system, "%LENS%", readingCoachLensMenu(), 1)
	return system
}
