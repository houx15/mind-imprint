package guidance

import (
	"strings"
	"testing"

	"mindimprint/api/internal/prompts"
)

// 🚨 这条测试是做这整件事的回报。
//
// AGENTS.md「提示词怎么写」第 6 条记着那次事故：parity 基线 28 个样例，
// 写作立题只覆盖了中文议论文一种，于是 @@KINDS@@ 在没被覆盖的分支上
// 原样发到了线上，而整套测试照样绿。
//
// 抽样查不出这类毛病，只有**枚举**能。
//
// 🚨 2026-09-22 fix round 1（mutation testing 发现）：只断言「取到了、不是
// 空的、没有 @@」不够 —— 少登记一行 genre（比如漏掉 en/narrative 的
// SlotKinds），Pick 会退到同语言的兄弟行（en/argument），那一行照样非空、
// 照样没有 @@，三条断言全过，而她拿到的是英文议论文的教法，不是英文记叙文
// 的。这正是这条测试要防的那类事故的形状，只是换成了「文体」这根轴。
// 所以这里钉的是**取到的是哪一段**，逐字比对，不只是「有没有拿到东西」。
func TestEveryCombinationResolves(t *testing.T) {
	langs := []string{"zh", "en"}
	stages := []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}

	// 每种 lang/genre 组合该取到哪一段正文 —— 逐字对应 registry.go 里的登记。
	want := map[string]map[Slot]string{
		"zh/argument": {
			SlotKinds:    prompts.WritingPlanArgumentKinds,
			SlotMaterial: prompts.WritingPlanMaterialZH,
			SlotSkeleton: prompts.WritingPlanSkeletonZH,
		},
		"zh/narrative": {
			SlotKinds:    prompts.WritingPlanNarrativeKinds,
			SlotMaterial: prompts.WritingPlanMaterialZH,
			SlotSkeleton: prompts.WritingPlanSkeletonZH,
		},
		"en/argument": {
			SlotKinds:    prompts.WritingPlanEnglishArgumentKinds,
			SlotMaterial: prompts.WritingPlanMaterialEN,
			SlotSkeleton: prompts.WritingPlanSkeletonEN,
		},
		"en/narrative": {
			SlotKinds:    prompts.WritingPlanEnglishNarrativeKinds,
			SlotMaterial: prompts.WritingPlanMaterialEN,
			SlotSkeleton: prompts.WritingPlanSkeletonEN,
		},
	}

	// 写作面：四个槽里的三个，两种文体。
	for _, lang := range langs {
		for _, genre := range []string{"argument", "narrative"} {
			for _, stage := range stages {
				k := Key{Surface: SurfaceWrite, Lang: lang, Genre: genre, Stage: stage}
				got, err := Default().Resolve(k, SlotKinds, SlotMaterial, SlotSkeleton)
				if err != nil {
					t.Errorf("%+v 取不齐：%v", k, err)
					continue
				}
				combo := lang + "/" + genre
				for slot, text := range got {
					if strings.TrimSpace(text) == "" {
						t.Errorf("%+v 的 %s 是空的", k, slot)
					}
					if strings.Contains(text, "@@") {
						t.Errorf("%+v 的 %s 里残留着占位符", k, slot)
					}
					if text != want[combo][slot] {
						// 内容有几千字，不把它整段打进失败信息 —— 只说
						// 是哪个组合、哪个槽拿错了。
						t.Errorf("%s 的 %s 取到的不是 %s 该用的那一段", k, slot, combo)
					}
				}
			}
		}
	}
}

// 🚨 定了学段的那一行，绝不能落到一个没说学段的 Key 上。
//
// Task 1 的评审发现这条分支一个测试都没有。它今天还不要紧（本期 Stage 恒为
// ""），但二期 classes.stage 一上线它就是真的：她的年级还没读出来的时候，
// 给她初二专用的教学内容就是给错人。
func TestStageScopedRowNeverMatchesUnknownStage(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Stages: []string{"junior2"}}, Value: "初二专用"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Stages: []string{"junior"}}, Value: "整个初中"},
	}
	// 学段未知 —— 两行都不该接住她。
	if got, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh"}, rows); ok {
		t.Errorf("学段未知时取到了 %q，定了学段的行不该匹配", got)
	}
	// 学段知道了，就该接住。
	if got, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh", Stage: "junior2"}, rows); !ok || got != "初二专用" {
		t.Errorf("学段是 junior2 时该拿到「初二专用」，拿到 %q ok=%v", got, ok)
	}
}

// 🚨 Stages 或 Genres 里写了一个空串，是一行**写坏了的**登记。
//
// 它必须谁都不匹配 —— 而不是反过来变成「不限」，把一行本该窄的内容
// 撒给所有人。guidance.go 里 contains 对空串直接返回 false 就是为这件事，
// 而在这条测试之前没有任何东西碰过它（2026-09-22 fix round 1，mutation
// testing 发现：删掉那个 `if s == "" { return false }` 守卫，原来的 11
// 条测试一条都不红）。
func TestMalformedEmptyStringInScopeMatchesNothing(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Stages: []string{""}}, Value: "写坏了的学段行"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Genres: []string{""}}, Value: "写坏了的文体行"},
	}
	for _, k := range []Key{
		{Surface: SurfaceWrite, Lang: "zh"},
		{Surface: SurfaceWrite, Lang: "zh", Stage: "junior2"},
		{Surface: SurfaceWrite, Lang: "zh", Genre: "argument"},
	} {
		if got, ok := Pick(k, rows); ok {
			t.Errorf("%+v 取到了 %q —— 写坏了的登记行不该匹配任何 Key", k, got)
		}
	}
}
