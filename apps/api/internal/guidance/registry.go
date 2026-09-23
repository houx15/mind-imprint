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

	r.Add(SlotMaterial, w("zh", ""), prompts.WritingPlanMaterialZH)
	r.Add(SlotMaterial, w("en", ""), prompts.WritingPlanMaterialEN)

	r.Add(SlotSkeleton, w("zh", ""), prompts.WritingPlanSkeletonZH)
	r.Add(SlotSkeleton, w("en", ""), prompts.WritingPlanSkeletonEN)

	// 🚨 语言认不出来（空串、老数据）时的兜底。改之前这条路走的是中文那一支：
	// 记叙文拿记叙文的块名，其余拿议论文的。少了这四行，它会拿到议论文的
	// 块名 —— 毛病表那一侧 2026-09-22 已经补过同样的四行，这里当时漏了。
	// 没有 Lang，所以分数（16 / 20）低于上面任何一行，现有的挑选一个都不会动。
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite}, prompts.WritingPlanArgumentKinds)
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite, Genres: []string{"narrative"}},
		prompts.WritingPlanNarrativeKinds)
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

	return r
})
