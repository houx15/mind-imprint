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

	r.Add(SlotMaterial, w("zh", ""), prompts.WritingPlanMaterialZH)
	r.Add(SlotMaterial, w("en", ""), prompts.WritingPlanMaterialEN)

	r.Add(SlotSkeleton, w("zh", ""), prompts.WritingPlanSkeletonZH)
	r.Add(SlotSkeleton, w("en", ""), prompts.WritingPlanSkeletonEN)
	// 🚨 书信的篇章结构**两种语言同一份**：那几种安排（邀请 / 建议 / 道歉 /
	// 感谢 / 申请）是按信的用途分的，不按语言分。语气和措辞的差别写在
	// SlotKinds 那两份里（英文那份专门讲 register），不重复一遍。
	r.Add(SlotSkeleton, w("zh", "letter"), prompts.WritingPlanSkeletonLetter)
	r.Add(SlotSkeleton, w("en", "letter"), prompts.WritingPlanSkeletonLetter)
	r.Add(SlotSkeleton, w("zh", "prose"), prompts.WritingPlanSkeletonProse)
	r.Add(SlotSkeleton, w("en", "prose"), prompts.WritingPlanSkeletonProse)

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
