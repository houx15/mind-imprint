package api

// Prompt assembly for writing_plan.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"log/slog"
	"strconv"
	"strings"

	"mindimprint/api/internal/guidance"
	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

// 🔑 每一个写进这段提示词散文里的方法名（『并列论证』『对比论证』『留个悬念』
// 『先抛一个问题』『开门见山』『先承认，再反驳』……）都必须**逐字**存在于
// packages/contracts/vocab/methods.json 的某个 name 或 formal_name 里。下面
// buildWritingPlanPrompt 会附上【可用的方法】并要求「只能用这里的名字，别造新
// 词」——示范里出现一个库里没有的名字（曾经的「并列论证」「正反对比」「钩子式
// 开头」，以及 2026-08-28 改名后已经不再是任何 name 的『正反』），就是在同一口
// 气里教模型造词，正是这个词表存在要防的漂移。改这里的例子前先对一遍
// methods.json。散文里优先用学生读得懂的 name，正式名称留给她点开的那张卡片。
const writingPlanSystem = prompts.WritingPlanSystem

const writingPlanArgumentKinds = prompts.WritingPlanArgumentKinds

// 记叙文那一份。来源是两份记叙文讲义：细节描写和抑扬转情法（抑→渡→转→扬）。
const writingPlanNarrativeKinds = prompts.WritingPlanNarrativeKinds

// writingPlanSystemFor 按**文体和语言**组装立题那份系统提示词。
//
// 四块按这两条轴挑（见 writing_plan_lang.go 开头那段）：
//
//	@@KINDS@@     这一篇有哪几种块，以及它们在这门课上叫什么
//	@@MATERIAL@@  一条理由底下该有什么
//	@@SKELETON@@  一整篇常见的几种摆法
//	@@COACH@@     这一篇特有的带法。**可选** —— 今天只有英文议论文有一节
//	              （题目拆解），取不到就整行删掉，不是故障。
//
// 🚨 lang 这条轴是 2026-09-22 补的。在这之前一个写英文议论文的学生拿到的是
// 语文高考那一套：材料按「社会/历史例子最硬、个人经历最弱」排次序、骨架是
// 总—分—总和起承转合、块名是中心论点/分论点/论据。同事的原话：
// 「english writing is quite different from chinese. but now we use the same
// guidance. strange」。
//
// 🚨 印记仍然用中文跟她说话 —— 换掉的是教的内容，不是说话的语言。
//
// 2026-09-22：选哪一段由 internal/guidance 决定，这里只负责把取回来的几段
// 填进模板。正文一个字没动。
//
// 🚨 grade 这条轴是二期 a 补的接线：今天登记表里没有任何一份按年级分的内容，
// 所以填不填、填哪个年级，Resolve 落到的都是同一份（见 guidance 的通配回退）。
// 这不是没做完——是把轴先通到位，等二期 b 真的登记年级专属内容时，这里不用
// 再改一行。传进来的必须是内部取值（classGrades 里的枚举），不是给她看的文案。
func writingPlanSystemFor(genre string, lang string, grade string) string {
	k := guidance.Key{Surface: guidance.SurfaceWrite, Lang: lang, Genre: genre, Grade: grade}
	parts, err := guidance.Default().Resolve(k,
		guidance.SlotKinds, guidance.SlotMaterial, guidance.SlotSkeleton)
	if err != nil {
		// 🚨 退到中文议论文那一套，而不是发一份带着 @@KINDS@@ 的提示词出去。
		// 登记漏了是我们的 bug，但她那一轮仍然要有一个能用的老师。
		slog.Error("writing plan guidance missing, falling back", "err", err,
			"lang", lang, "genre", genre)
		parts = map[guidance.Slot]string{
			guidance.SlotKinds:    writingPlanArgumentKinds,
			guidance.SlotMaterial: writingPlanMaterialZH,
			guidance.SlotSkeleton: writingPlanSkeletonZH,
		}
	}

	// 这一篇的带读说明是**可选**的：今天只有英文议论文有一节（题目拆解）。
	// 取不到不是故障 —— 中文议论文和两种记叙文本来就没有这一节，所以这里
	// 不记日志、不兜底，直接把那一行占位符整行删掉。阅读面的
	// buildGenreCoachSection 是同一个形状。
	//
	// 🚨 上面那三块走了兜底（退到中文议论文）的时候，这一节也要一起丢掉。
	// 否则会拼出一份「中文高考的材料次序 + 英文题目拆解」的混合体 —— 那比
	// 两者中的任何一个都糟。今天走不到（覆盖测试保证那三块取得齐），
	// 但兜底分支存在的意义就是为了走不到的那天。
	coach := ""
	if err == nil {
		if p, cerr := guidance.Default().Resolve(k, guidance.SlotCoach); cerr == nil {
			coach = p[guidance.SlotCoach]
		}
	}

	english := lang == langEnglish
	s := strings.Replace(writingPlanSystem, "@@KINDS@@", parts[guidance.SlotKinds], 1)
	s = strings.Replace(s, "@@MATERIAL@@", parts[guidance.SlotMaterial], 1)
	s = strings.Replace(s, "@@SKELETON@@", parts[guidance.SlotSkeleton], 1)
	// @@COACH@@ 是 prompts.WritingPlanSystem 里的占位符，和上面三个同一套。
	if coach == "" {
		// 整行删掉 —— 留下一个空行会让中文那三条分支和今天差一个字节。
		s = strings.Replace(s, "@@COACH@@\n", "", 1)
	} else {
		// 多补一个 \n：常量结尾没有换行，不补的话小标题前面没有空行。
		s = strings.Replace(s, "@@COACH@@", coach+"\n", 1)
	}
	s = strings.Replace(s, "%d", strconv.Itoa(writingPlanMaxNewNodes), 1)
	if english {
		s += "\n这篇是英文写作。讨论图中已有内容时，请使用与该节点对应的英文术语并解释其作用，例如 topic sentence 或 commentary。节点 text 必须使用英文，对话 reply 用中文。计划检查中的中文标签只是计数名称，不覆盖这些教学术语。\n"
	}
	return s
}

// buildWritingPlanPrompt renders the current map (with ids, so the model can
// point at a parent) plus the windowed conversation.
func buildWritingPlanPrompt(wr sqlc.Writing, rows []sqlc.WritingOutline, msgs []sqlc.AtomMessage, studentText string) string {
	return renderWritingPlanPrompt(selectWritingPlanContext(wr, rows, msgs, studentText)).Text
}

func renderWritingPlanPrompt(c writingPlanContext) promptassembly.Document {
	wr, rows, studentText := c.Writing, c.Rows, c.StudentText
	var b promptassembly.Builder
	b.Mark("assignment", "mixed", "internal/api/writing_plan_prompt.go")
	// An assigned writing's topic is the teacher's prompt; 她一开始说想写的是 would
	// put it in her mouth. See writingTopicLine.
	b.WriteString(writingTopicLine(wr, "她一开始说想写的是："))
	// 🚨 This used to be 「这篇用英文写（但你和她用中文讨论）」 — which had the
	// coaching/content split right but never said that the OUTLINE NODES are
	// content. An English piece therefore grew a Chinese mind map, because the
	// nodes read as part of the discussion. writingLangLine names the nodes
	// explicitly; see writing_lang.go.
	b.WriteString(writingLangLine(wr))
	// 🚨 This line, with the unit hard-coded as 「字」, is where 「500字很短，两个
	// 都展开容易平」 came from on a 500-WORD English essay: the prompt tells the
	// model to size her sub-arguments off this number, so a 5x unit error lands
	// straight in the advice. writingLengthLine derives the unit from wr.Lang.
	b.WriteString(writingLengthLine(wr, "目标篇幅"))
	if wr.TargetWords != nil {
		b.WriteString("（篇幅只用来判断要几条分论点，别追着她凑字数。）\n")
	}

	// 计划现在有什么、还缺什么，由服务端数出来当事实给它——不让它每轮从十六轮
	// 对话里重新推一遍「她定下中心论点了吗」。见 writing_plan_state.go。
	b.Mark("readiness", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString(c.Shape.promptBlock(c.Need))

	// 讲义（四）的分论点三原则里，「扣得住」是唯一机械可判的一条 ——
	// 数出来当事实给它，别让它每轮自己比一遍。全扣得住就一个字都不加。
	// 见 writing_points_check.go。
	b.Mark("point-relevance", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString(writingPointsCheckBlock(wr, rows))

	// 她的分论点还不够的时候，给出讲义（四）的那四个角度，
	// 让她知道下一条该往哪个方向想。够了就不摆。
	b.Mark("point-angles", "instruction", "internal/api/writing_plan_prompt.go")
	b.WriteString(writingPointAnglesBlock(wr, rows, c.Need.Points))

	// 她连着两轮等于没答 → 这一轮别再问了。**只在真的停滞时出现，不做常驻**
	// （2026-09-05：常驻提示会把该做的事挤掉）。
	b.Mark("stalled", "instruction", "internal/api/writing_plan_prompt.go")
	if c.Stalled {
		b.WriteString(writingPlanStalledBlock)
	}

	// 她请我们替她搜索或替她写 → 这一轮先说明再往下走。同样是一次性的。
	// 见 writing_refusal.go（同事 2026-09-20 的意见 8）。
	b.Mark("delegation-request", "instruction", "internal/api/writing_plan_prompt.go")
	if c.AskedToDoIt {
		b.WriteString(writingRefusalBlock)
	}

	b.Mark("outline", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n【当前的图】\n")
	if len(rows) == 0 {
		b.WriteString("（图是空的。先检查她本轮是否已经表达观点或理由，已表达就直接整理；缺失才询问。）\n")
	} else {
		// 🚨 不再给节点编号。模型不需要指着某一个节点说「挂在它下面」——
		// 位置由 kind 算出来（writing_kind.go）。给它一份带 id 的清单，只会
		// 请它做一件它做不好的事：2026-09-18 实测，它抄 36 位 UUID 会抄丢
		// 一整段，节点于是被静默丢掉，重试一次照样抄错。
		for _, r := range rows {
			indent := strings.Repeat("  ", int(r.Depth))
			line := indent + "- " + r.Text
			if lbl := writingKindLabel(writingKindOf(r), r.Source); lbl != "" {
				line += "（" + lbl + "）"
			}
			// 出处（0158）。带着出处的那一块是**她找回来的材料** —— 见上面
			// 「她拿回来一份材料的时候，你要查它」。不给出处，印记 连它是她
			// 自己的经历还是一份研究都分不出来，更别说查它。
			if src := strings.TrimSpace(r.Source); src != "" {
				line += "【出处：" + src + "】"
			}
			b.WriteString(line + "\n")
		}
	}

	// Filtered by the piece's LANGUAGE, not just listed: an English sentence
	// frame offered inside a Chinese essay is a bug (vocab.For's doc comment).
	// Both names go in — 印记 says the plain one to her, and knows the formal
	// one for when she asks what it is really called.
	b.Mark("methods", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n【可用的方法】（只能用这里的名字，别造新词）\n")
	for _, m := range c.Methods {
		b.WriteString("- " + m.Label() + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
	}

	b.Mark("history", "context", "internal/api/writing_plan_prompt.go")
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

	b.Mark("latest-input", "context", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n【她刚刚说的】\n" + studentText + "\n")
	b.Mark("extraction-boundary", "instruction", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n只能从「她刚刚说的」这段话里提取节点。她这段话里没有新的点子，add 就给空数组。向她说明时用「观点」或直接说具体想法。已经说清的理由直接整理，只问仍缺少的内容。\n")
	return b.Document(c.Selection)
}
