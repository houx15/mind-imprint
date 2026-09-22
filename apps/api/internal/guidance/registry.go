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
