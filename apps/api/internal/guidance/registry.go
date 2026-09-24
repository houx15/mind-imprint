package guidance

import (
	"sync"

	"mindimprint/api/internal/prompts"
)

// Default 是生产在用的那一份注册表。
//
// 🚨 这里只登记「哪段正文在什么情况下用」，**不改任何一个字的正文**。
// 正文仍然住在 internal/prompts，这里引的是它的常量。
var Default = sync.OnceValue(func() *Registry {
	r := NewRegistry()

	// ── 写作立题 ────────────────────────────────────────────────────────
	// 这四行是 writingPlanSystemFor 原来那个 switch 的逐条搬家。
	// 中文两种文体各一份节点类型表，英文两种各一份。
	w := func(lang, genre string) Scope {
		s := Scope{Surface: SurfaceWrite, Lang: lang}
		if genre != "" {
			s.Genres = []string{genre}
		}
		return s
	}
	r.Add(SlotKinds, w("zh", ""), prompts.WritingPlanArgumentKinds)
	r.Add(SlotKinds, w("zh", "narrative"), prompts.WritingPlanNarrativeKinds)
	r.Add(SlotKinds, w("en", ""), prompts.WritingPlanEnglishArgumentKinds)
	r.Add(SlotKinds, w("en", "narrative"), prompts.WritingPlanEnglishNarrativeKinds)
	// 书信（2026-09-23）。产品负责人：「书信 is a very important format in
	// junior english. but currently we would guide students to write a letter
	// under the structure of 议论文.」
	r.Add(SlotKinds, w("zh", "letter"), prompts.WritingPlanLetterKinds)
	r.Add(SlotKinds, w("en", "letter"), prompts.WritingPlanLetterKindsEN)
	// 散文（2026-09-23）。块的种类和记叙文共用，换的是怎么问 ——
	// 差别在整篇怎么合起来，不在某一段是什么。两种语言同一份。
	r.Add(SlotKinds, w("zh", "prose"), prompts.WritingPlanProseKinds)
	r.Add(SlotKinds, w("en", "prose"), prompts.WritingPlanProseKinds)
	// 读后续写（2026-09-24）。高考英语写作里它占 25 分、43% 的题量，在这之前
	// 一件都没做 —— 一道续写题会落到记叙文那一支，于是没有人告诉学生
	// 「第一段末句要把人物送到第二段首句的场面里」。
	//
	// 两种语言同一份：它是英语高考的题型，中文那边没有对应的题；挂 zh 只是为了
	// 语言判错时不至于空着（那时它会退到下面那条没有 Lang 的兜底）。
	r.Add(SlotKinds, w("zh", "continuation"), prompts.WritingPlanContinuationKinds)
	r.Add(SlotKinds, w("en", "continuation"), prompts.WritingPlanContinuationKinds)
	// 概要写作（2026-09-24）。高考英语写作 16% 的题量、10 分。
	// 🚨 教学内容是补出来的，不是老师那几份讲义里的 —— 见
	// prompts.WritingPlanSkeletonSummary 上面那段注释和
	// docs/2026-09-24-teaching-rulings.md。
	r.Add(SlotKinds, w("zh", "summary"), prompts.WritingPlanSummaryKinds)
	r.Add(SlotKinds, w("en", "summary"), prompts.WritingPlanSummaryKinds)

	// 年级门槛（2026-09-25）。只按学段登记，不按语言也不按文体 ——
	// 「她学到哪儿了」和她这一篇写什么、用哪种语言无关。
	// 取不到不是故障（她的班没填年级就是空串），调用方按可选处理。
	r.Add(SlotCeiling, Scope{Surface: SurfaceWrite, Grades: []string{"junior"}},
		prompts.WritingCeilingJunior)
	r.Add(SlotCeiling, Scope{Surface: SurfaceWrite, Grades: []string{"senior"}},
		prompts.WritingCeilingSenior)

	r.Add(SlotMaterial, w("zh", ""), prompts.WritingPlanMaterialZH)
	r.Add(SlotMaterial, w("en", ""), prompts.WritingPlanMaterialEN)
	// 🚨 SlotMaterial 原来**只按语言登记，不按文体** —— 于是每一种文体拿到的
	// 都是议论文那份「根据观点选择材料」。议论文和记叙文大体用得上，
	// 这两档是错的：续写的材料就是前文，概要根本没有自己的材料。
	// 一个被告知「去找一条研究来支撑」的概要写作学生，照做就会写出一篇一定
	// 扣分的概要。
	//
	// 书信、散文那两档也在拿议论文这一份，那是 2026-09-24 之前就有的事，
	// 没有在这一次一起改：它们至少「找材料」这件事本身是成立的，而改它们要
	// 各写一份新文本，那是另一件事（记在 docs/2026-09-24-teaching-rulings.md）。
	for _, lang := range []string{"zh", "en"} {
		r.Add(SlotMaterial, w(lang, "continuation"), prompts.WritingPlanMaterialContinuation)
		r.Add(SlotMaterial, w(lang, "summary"), prompts.WritingPlanMaterialSummary)
		// 2026-09-25 补上书信和散文。上一轮留着没动，理由是「它们至少『找材料』
		// 这件事本身成立」—— 只说对了一半：应用文的证据是**细节与画面**不是
		// 出处（源里逐字「CRAAP 式溯源在这一面不适用」），散文的材料是她自己
		// 看见的和想起来的。要一封 80 词的邀请信「说明来源是否可辨识」，
		// 是在把她往扣分的方向推。
		r.Add(SlotMaterial, w(lang, "letter"), prompts.WritingPlanMaterialLetter)
		r.Add(SlotMaterial, w(lang, "prose"), prompts.WritingPlanMaterialProse)
	}
	r.Add(SlotMaterial, Scope{Surface: SurfaceWrite, Genres: []string{"continuation"}},
		prompts.WritingPlanMaterialContinuation)
	r.Add(SlotMaterial, Scope{Surface: SurfaceWrite, Genres: []string{"summary"}},
		prompts.WritingPlanMaterialSummary)
	r.Add(SlotMaterial, Scope{Surface: SurfaceWrite, Genres: []string{"letter"}},
		prompts.WritingPlanMaterialLetter)
	r.Add(SlotMaterial, Scope{Surface: SurfaceWrite, Genres: []string{"prose"}},
		prompts.WritingPlanMaterialProse)

	r.Add(SlotSkeleton, w("zh", ""), prompts.WritingPlanSkeletonZH)
	r.Add(SlotSkeleton, w("en", ""), prompts.WritingPlanSkeletonEN)
	// 🚨 书信的篇章结构 2026-09-24 起**按语言分开**。
	//
	// 原来两份是同一个常量，理由是「那几种安排（邀请 / 建议 / 道歉 / 感谢 /
	// 申请）是按信的用途分的，不按语言分」。那句话今天仍然成立 —— 英文那份
	// 里的安排和中文那份是同一批。分开是因为**另外三样**，它们不是语气和
	// 措辞，而是会让陪练给出相反的建议：
	//
	//   1. 英文应用文通常要求 80 词左右，超出要扣分 ⇒「再补一个理由把这一段
	//      写厚」在英文这一档是错的建议，在中文书信上没有问题。
	//   2. 英文这一档的要点来自题干、漏一个直接压一档 ⇒ 第一件事是数题目里
	//      要求做的动作，不是问她要说清什么。
	//   3. 称呼和结束语有对仗关系（Dear Sir or Madam ↔ Yours sincerely），
	//      缺称呼或缺署名是独立的一处失分。中文书信没有这条对仗。
	//
	// 产品负责人 2026-09-24 点的四处弱项里有一处正是「english letters
	// writing」。详见 prompts.WritingPlanSkeletonLetterEN 上面那段注释。
	r.Add(SlotSkeleton, w("zh", "letter"), prompts.WritingPlanSkeletonLetter)
	r.Add(SlotSkeleton, w("en", "letter"), prompts.WritingPlanSkeletonLetterEN)
	r.Add(SlotSkeleton, w("zh", "prose"), prompts.WritingPlanSkeletonProse)
	r.Add(SlotSkeleton, w("en", "prose"), prompts.WritingPlanSkeletonProse)
	r.Add(SlotSkeleton, w("zh", "continuation"), prompts.WritingPlanSkeletonContinuation)
	r.Add(SlotSkeleton, w("en", "continuation"), prompts.WritingPlanSkeletonContinuation)
	r.Add(SlotSkeleton, w("zh", "summary"), prompts.WritingPlanSkeletonSummary)
	r.Add(SlotSkeleton, w("en", "summary"), prompts.WritingPlanSkeletonSummary)

	// 🚨 语言认不出来（空串、老数据）时的兜底。改之前这条路走的是中文那一支：
	// 记叙文拿记叙文的块名，其余拿议论文的。少了这四行，它会拿到议论文的
	// 块名 —— 毛病表那一侧 2026-09-22 已经补过同样的四行，这里当时漏了。
	// 没有 Lang，所以分数（16 / 20）低于上面任何一行，现有的挑选一个都不会动。
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite}, prompts.WritingPlanArgumentKinds)
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite, Genres: []string{"narrative"}},
		prompts.WritingPlanNarrativeKinds)
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite, Genres: []string{"letter"}},
		prompts.WritingPlanLetterKinds)
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Genres: []string{"letter"}},
		prompts.WritingPlanSkeletonLetter)
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite, Genres: []string{"prose"}},
		prompts.WritingPlanProseKinds)
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Genres: []string{"prose"}},
		prompts.WritingPlanSkeletonProse)
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite, Genres: []string{"continuation"}},
		prompts.WritingPlanContinuationKinds)
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Genres: []string{"continuation"}},
		prompts.WritingPlanSkeletonContinuation)
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite, Genres: []string{"summary"}},
		prompts.WritingPlanSummaryKinds)
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Genres: []string{"summary"}},
		prompts.WritingPlanSkeletonSummary)
	r.Add(SlotMaterial, Scope{Surface: SurfaceWrite}, prompts.WritingPlanMaterialZH)
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite}, prompts.WritingPlanSkeletonZH)

	// 🚨 写作面的带读说明只有英文议论文有一行：题目拆解（TOPIC + TASK）。
	// 中文议论文的题目不是这个形状，两种记叙文没有 TASK 可拆 —— 所以这里
	// **不登记兜底行**。取不到不是故障，是这一篇本来就没有这一节，由
	// writingPlanSystemFor 把「没有」翻译成「整节不出现」。
	// 和阅读面的 SlotCoach 同一条纪律（议论文那边也没有登记）。
	r.Add(SlotCoach, Scope{Surface: SurfaceWrite, Lang: "en", Genres: []string{"argument"}},
		prompts.WritingPlanEnglishTaskSplit)

	// ── 阅读带读说明 ────────────────────────────────────────────────────
	// 🚨 议论文没有这一节，所以议论文不登记 —— Resolve 取不到就是取不到，
	// 由调用方 buildGenreCoachSection 把「没有」翻译成空字符串。
	rd := func(genre string) Scope {
		return Scope{Surface: SurfaceRead, Genres: []string{genre}}
	}
	// 议论文（2026-09-25，产品负责人点头之后）。装的是 R13：示范额度最多一段，
	// 之后每一段都先请学生说。见 prompts.ReadingCoachGenreArgument 上面那段注释。
	r.Add(SlotCoach, rd("argument"), prompts.ReadingCoachGenreArgument)
	r.Add(SlotCoach, rd("report"), prompts.ReadingCoachGenreReport)
	r.Add(SlotCoach, rd("explain"), prompts.ReadingCoachGenreExplain)
	r.Add(SlotCoach, rd("narrative"), prompts.ReadingCoachGenreNarrative)
	// 2026-09-23。产品负责人：「since we will face reading poems/文言文 in
	// chinese reading…… these are very important scene in junior study.」
	r.Add(SlotCoach, rd("poem"), prompts.ReadingCoachGenrePoem)
	r.Add(SlotCoach, rd("classical"), prompts.ReadingCoachGenreClassical)
	// 2026-09-24。散文成为一种阅读体裁 —— reading-suggestion.md 里
	// 「先列一下放着」那一句是旧的，产品负责人：「this is very old...
	// and we are about to do them now.」
	r.Add(SlotCoach, rd("prose"), prompts.ReadingCoachGenreProse)

	return r
})
