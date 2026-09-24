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
	langs := []string{"zh", "en", ""}
	// 🚨 这七个年级现在全部断言同一份 want[combo] —— 因为今天没有一行按
	// Grades 登记，取到的内容和年级无关。二期一旦有人加一行按年级登记的
	// 内容（比如 {write, zh, Grades:["junior2"]}，分数 26），它会**输给**
	// 现有的 {write, zh, Genres:["narrative"]}（分数 28，见 guidance.go
	// specificity）：那个年级专属的内容永远选不中，而这条测试照样绿，因为
	// want 里等着的仍是不分年级的那份泛化内容。**第一次有行开始登记 Grades
	// 时，want 必须按 lang/genre/grade 三轴键，不能再按 lang/genre 两轴**，
	// 否则这条测试只是给了个假的安全感。
	grades := []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}

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
		// 🚨 语言认不出来（空串、老数据）时的兜底 —— 重构前这条路走的是
		// 中文那一支，记叙文拿记叙文的块名，其余（含未知文体）拿议论文的。
		"/argument": {
			SlotKinds:    prompts.WritingPlanArgumentKinds,
			SlotMaterial: prompts.WritingPlanMaterialZH,
			SlotSkeleton: prompts.WritingPlanSkeletonZH,
		},
		"/narrative": {
			SlotKinds:    prompts.WritingPlanNarrativeKinds,
			SlotMaterial: prompts.WritingPlanMaterialZH,
			SlotSkeleton: prompts.WritingPlanSkeletonZH,
		},
	}

	// 写作面：四个槽里的三个，两种文体。
	for _, lang := range langs {
		for _, genre := range []string{"argument", "narrative"} {
			for _, grade := range grades {
				k := Key{Surface: SurfaceWrite, Lang: lang, Genre: genre, Grade: grade}
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

// 🚨 定了年级的那一行，绝不能落到一个没说年级的 Key 上。
//
// Task 1 的评审发现这条分支一个测试都没有。它今天还不要紧（本期 Grade 恒为
// ""），但二期 classes.grade 一上线它就是真的：她的年级还没读出来的时候，
// 给她初二专用的教学内容就是给错人。
func TestGradeScopedRowNeverMatchesUnknownGrade(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Grades: []string{"junior2"}}, Value: "初二专用"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Grades: []string{"junior"}}, Value: "整个初中"},
	}
	// 年级未知 —— 两行都不该接住她。
	if got, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh"}, rows); ok {
		t.Errorf("年级未知时取到了 %q，定了年级的行不该匹配", got)
	}
	// 年级知道了，就该接住。
	if got, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh", Grade: "junior2"}, rows); !ok || got != "初二专用" {
		t.Errorf("年级是 junior2 时该拿到「初二专用」，拿到 %q ok=%v", got, ok)
	}
}

// 🚨 Grades 或 Genres 里写了一个空串，是一行**写坏了的**登记。
//
// 它必须谁都不匹配 —— 而不是反过来变成「不限」，把一行本该窄的内容
// 撒给所有人。guidance.go 里 contains 对空串直接返回 false 就是为这件事，
// 而在这条测试之前没有任何东西碰过它（2026-09-22 fix round 1，mutation
// testing 发现：删掉那个 `if s == "" { return false }` 守卫，原来的 11
// 条测试一条都不红）。
func TestMalformedEmptyStringInScopeMatchesNothing(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Grades: []string{""}}, Value: "写坏了的年级行"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Genres: []string{""}}, Value: "写坏了的文体行"},
	}
	for _, k := range []Key{
		{Surface: SurfaceWrite, Lang: "zh"},
		{Surface: SurfaceWrite, Lang: "zh", Grade: "junior2"},
		{Surface: SurfaceWrite, Lang: "zh", Genre: "argument"},
	} {
		if got, ok := Pick(k, rows); ok {
			t.Errorf("%+v 取到了 %q —— 写坏了的登记行不该匹配任何 Key", k, got)
		}
	}
}

// 🚨 写作面的 SlotCoach 只有一行，所以它的判据是**在哪几个组合上取得到**，
// 而不是「取到的非空」。取得到的地方多一个，就是一批学生悄悄换了教学内容。
func TestWriteCoachIsOnlyOnEnglishArgument(t *testing.T) {
	grades := []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}
	for _, lang := range []string{"zh", "en", ""} {
		for _, genre := range []string{"argument", "narrative", "report", "explain", ""} {
			for _, grade := range grades {
				k := Key{Surface: SurfaceWrite, Lang: lang, Genre: genre, Grade: grade}
				got, err := Default().Resolve(k, SlotCoach)
				wantIt := lang == "en" && genre == "argument"
				if !wantIt {
					if err == nil {
						t.Errorf("%+v 取到了写作面的带读说明，但只有英文议论文该有", k)
					}
					continue
				}
				if err != nil {
					t.Errorf("%+v 该取到题目拆解，却取不到：%v", k, err)
					continue
				}
				if got[SlotCoach] != prompts.WritingPlanEnglishTaskSplit {
					t.Errorf("%+v 取到的不是题目拆解那一段", k)
				}
			}
		}
	}
}

// 🚨 两面不许串台。Scope.Matches 第一条比的就是 Surface，但在写作面加了
// SlotCoach 之前，没有任何东西测过「阅读面那三行还在不在」。
func TestReadCoachStillResolvesAfterWriteCoachWasAdded(t *testing.T) {
	want := map[string]string{
		"report":    prompts.ReadingCoachGenreReport,
		"explain":   prompts.ReadingCoachGenreExplain,
		"narrative": prompts.ReadingCoachGenreNarrative,
	}
	for genre, text := range want {
		// 阅读面的 Key 不带 Lang —— buildGenreCoachSection 就是这么调的。
		for _, lang := range []string{"", "zh", "en"} {
			k := Key{Surface: SurfaceRead, Lang: lang, Genre: genre}
			got, err := Default().Resolve(k, SlotCoach)
			if err != nil {
				t.Errorf("%+v 取不到带读说明：%v", k, err)
				continue
			}
			if got[SlotCoach] != text {
				t.Errorf("%+v 取到的不是 %s 该用的那一段", k, genre)
			}
		}
	}
	// 🚨 2026-09-25：阅读面的议论文**现在有**带读说明了（R13，示范额度最多
	// 一段）。这一行原来断言它没有，理由是「议论文一个字都不动」那条禁令 ——
	// 产品负责人当天点头解了那条禁令。
	//
	// 断言换方向而不是删掉：它守的那件事仍然要守 —— 两面不许串台。
	// 阅读面取到的必须是阅读那一段，不能是写作面那一段。
	got, err := Default().Resolve(Key{Surface: SurfaceRead, Genre: "argument"}, SlotCoach)
	if err != nil {
		t.Fatalf("阅读面的议论文取不到带读说明：%v", err)
	}
	if got[SlotCoach] != prompts.ReadingCoachGenreArgument {
		t.Error("阅读面的议论文取到的不是阅读那一段")
	}
	if got[SlotCoach] == prompts.WritingPlanEnglishTaskSplit {
		t.Error("阅读面串到写作面那一段去了")
	}
}
