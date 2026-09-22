package guidance

import (
	"strings"
	"testing"
)

// 🚨 这条测试是做这整件事的回报。
//
// AGENTS.md「提示词怎么写」第 6 条记着那次事故：parity 基线 28 个样例，
// 写作立题只覆盖了中文议论文一种，于是 @@KINDS@@ 在没被覆盖的分支上
// 原样发到了线上，而整套测试照样绿。
//
// 抽样查不出这类毛病，只有**枚举**能。
func TestEveryCombinationResolves(t *testing.T) {
	langs := []string{"zh", "en"}
	stages := []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}

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
				for slot, text := range got {
					if strings.TrimSpace(text) == "" {
						t.Errorf("%+v 的 %s 是空的", k, slot)
					}
					if strings.Contains(text, "@@") {
						t.Errorf("%+v 的 %s 里残留着占位符", k, slot)
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
