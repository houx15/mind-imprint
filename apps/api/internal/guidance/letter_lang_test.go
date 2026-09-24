package guidance

import (
	"strings"
	"testing"

	"mindimprint/api/internal/prompts"
)

// 英文书信和中文书信从 2026-09-24 起拿的**不是同一份篇章结构**。
//
// 产品负责人 2026-09-24 点名的四处弱项里有一处是「english letters writing」。
//
// 这条测试钉的是**为什么要分开**，不是分开这件事本身 —— 分开本身一行代码就能
// 做到，而把它撤回去也一样容易。撤回去之后真正会坏掉的是下面这三条。
func TestEnglishLetterGetsItsOwnSkeleton(t *testing.T) {
	en := mustResolve(t, Key{Surface: SurfaceWrite, Lang: "en", Genre: "letter"})
	zh := mustResolve(t, Key{Surface: SurfaceWrite, Lang: "zh", Genre: "letter"})

	if en == zh {
		t.Fatal("英文书信还在用中文书信那份篇章结构")
	}
	if en != prompts.WritingPlanSkeletonLetterEN {
		t.Error("英文书信取到的不是英文那一份")
	}
	if zh != prompts.WritingPlanSkeletonLetter {
		t.Error("中文书信那一份被动了")
	}
}

// 🚨 第一条，也是分开的主要理由：**篇幅是硬上限，所以「写厚一点」是反的建议。**
//
// 英文应用文通常要求 80 词左右，超出要扣分。同一句「再补一个理由把这一段写厚」
// 在中文书信上没有问题，在英文这一档会把她推下一档。
// 一条会让陪练给出相反建议的差别，不是语气和措辞的差别。
func TestOnlyTheEnglishLetterCarriesTheWordCap(t *testing.T) {
	en := prompts.WritingPlanSkeletonLetterEN
	if !strings.Contains(en, "80 词") {
		t.Error("英文书信那份里没有篇幅上限")
	}
	if !strings.Contains(en, "不建议") || !strings.Contains(en, "写厚") {
		t.Error("英文书信那份没有拦住「把这一段写厚」那条建议")
	}
	// 反方向：这条上限不许漏到中文书信上 —— 中文书信没有 80 词这回事。
	if strings.Contains(prompts.WritingPlanSkeletonLetter, "80 词") {
		t.Error("英文的篇幅上限漏到中文书信那一份里了")
	}
}

// 第二条：英文这一档的要点**来自题干**，漏一个直接压一档。
// 所以第一件事是数题目要求做的动作，不是问她要说清什么。
func TestEnglishLetterPointsComeFromTheTask(t *testing.T) {
	en := prompts.WritingPlanSkeletonLetterEN
	for _, want := range []string{"题目", "动作", "降一档"} {
		if !strings.Contains(en, want) {
			t.Errorf("英文书信那份里没有 %q —— 要点来自题干这条就落不下来", want)
		}
	}
	// 「题目没说但必须写的那一句」那张表要在，它是这一档最常见的失分处。
	for _, want := range []string{"真正发出邀请", "补救的办法", "递回给对方"} {
		if !strings.Contains(en, want) {
			t.Errorf("隐性要点那张表里少了 %q", want)
		}
	}
}

// 第三条：称呼和结束语的对仗关系。中文书信没有这条。
func TestEnglishLetterCarriesTheSalutationPairing(t *testing.T) {
	en := prompts.WritingPlanSkeletonLetterEN
	for _, want := range []string{"Dear Sir or Madam", "Yours sincerely", "Best wishes"} {
		if !strings.Contains(en, want) {
			t.Errorf("英文书信那份里没有 %q", want)
		}
	}
}

// 🚨 反方向：语言认不出来（空串、老数据）时仍然走中文那一份。
//
// 按语言挑有一种对称的翻车方式 —— 把中文那几条挡住之后英文那边空了，
// 或者反过来（memory: lang-axis-only-reached-the-shallow-layer-2026-09-22）。
// 老数据上的书信不该因为这次改动换一份篇章结构。
func TestUnknownLangLetterStillGetsTheChineseSkeleton(t *testing.T) {
	got := mustResolve(t, Key{Surface: SurfaceWrite, Lang: "", Genre: "letter"})
	if got != prompts.WritingPlanSkeletonLetter {
		t.Error("语言认不出来时书信没有走中文那一份")
	}
}

// 两份都要以那一句收尾 —— method 字段只能填方法表里的 id。
// 少了它，模型会把「邀请」「二选一表态」当成 method 写进去。
func TestBothLetterSkeletonsGuardTheMethodField(t *testing.T) {
	for name, text := range map[string]string{
		"zh": prompts.WritingPlanSkeletonLetter,
		"en": prompts.WritingPlanSkeletonLetterEN,
	} {
		if !strings.Contains(text, "method 字段只能填写方法表提供的 id") {
			t.Errorf("%s 那一份没有拦住 method 字段", name)
		}
	}
}

func mustResolve(t *testing.T, k Key) string {
	t.Helper()
	got, err := Default().Resolve(k, SlotSkeleton)
	if err != nil {
		t.Fatalf("%+v 取不到篇章结构：%v", k, err)
	}
	return got[SlotSkeleton]
}
