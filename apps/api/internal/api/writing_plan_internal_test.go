package api

// writing_plan_internal_test.go — white-box tests for writing_plan.go that
// need package-internal access: the raw system-prompt string (unexported
// constant) and rootInsertPosition (unexported helper).
//
// TestWritingPlanSystem_TeachesWholePieceJudgment is the RED-PHASE fix for
// B0: TestPlanTurn_AcceptsATopLevelOpeningBesideTheThesis (writing_plan_test.go,
// package api_test) exercises insertPlanNode/insertPlanNode's storage layer
// through a scripted stub — it passed even before the prompt changed, because
// nothing in storage ever forbade a second depth-0 node. This test instead
// pins the actual prompt text, so it genuinely fails without the change B0
// makes.

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

func TestWritingPlanSystem_TeachesWholePieceJudgment(t *testing.T) {
	// 🚨 查的是**装配好的那一份**，不是那个 const。
	// R4 把「这一块是什么」的清单按文体拆成了两份，const 里只剩一个占位符 ——
	// 继续查 const 等于查一份谁都收不到的东西。
	writingPlanSystem := writingPlanSystemFor(genreArgument, "zh", "")
	for _, want := range []string{
		// 🚨 2026-09-20：原来这里钉的是「最上层不止中心论点」—— 那句话在教模型
		// 怎么摆节点。模型不再摆节点了，所以钉住的换成新的那条契约本身：
		// 它只说这一块是什么，位置不归它管。
		"系统根据 kind 安排节点位置与标题",
		"「closing」 结尾",
		"「reasoning」 道理",
		"不超过 200 字",
	} {
		if !strings.Contains(writingPlanSystem, want) {
			t.Fatalf("writingPlanSystem is missing %q", want)
		}
	}
	if strings.Contains(writingPlanSystem, "不超过 120 字") {
		t.Fatalf("writingPlanSystem still carries the old 120-字 cap")
	}
	// 🚨 判据要盯住**真失败本身**，不是它的影子（[[detector-must-target-the-real-failure]]）。
	// 真失败是「提示词还在向模型要一个 parentId」，而不是「提示词里出现了
	// parentId 这个词」—— 那句「不要给 parentId」正是我们要的话，第一版判据
	// 把它也判成了失败。所以只查**输出格式里的那个字段**（带引号的那一个）。
	if strings.Contains(writingPlanSystem, `"parentId"`) {
		t.Fatalf("writingPlanSystem's output format still carries a parentId field")
	}
}

// TestWritingPlanSystem_NamesOnlyRealMethods enforces the 🔑 rule stated in
// writing_plan.go and writing_guide.go: a method name written in PROSE inside a
// prompt must exist verbatim in the vocabulary library. The prompt turns around
// and tells the model 「只能用这里的名字，别造新词」, so a demonstration sentence
// naming something the library does not have teaches it to invent terms in the
// same breath as forbidding it. This test is the reason the 2026-08-28 rename
// could not quietly leave 『正反』 behind.
func TestWritingPlanSystem_NamesOnlyRealMethods(t *testing.T) {
	known := map[string]bool{}
	for _, m := range vocab.All() {
		known[m.Name] = true
		if m.FormalName != "" {
			known[m.FormalName] = true
		}
	}
	// Every 『…』 inside these prompts is a method name by convention.
	for _, prompt := range []struct {
		what string
		text string
	}{
		{"writingPlanSystem", writingPlanSystem},
		{"writingGuideTeachingRules", writingGuideTeachingRules},
		{"writingOpeningSystem", writingOpeningSystem},
	} {
		for _, quoted := range bracketed(prompt.text) {
			if !known[quoted] {
				t.Errorf("%s names 『%s』, which is not a name or formal_name in packages/contracts/vocab/methods.json", prompt.what, quoted)
			}
		}
	}
	// Guard against this test passing vacuously if the quoting convention
	// changes and bracketed stops finding anything.
	if n := len(bracketed(writingPlanSystem)); n < 2 {
		t.Fatalf("found only %d 『』-quoted method names in writingPlanSystem — the prompt or the quoting convention changed, and this test is no longer checking anything", n)
	}
	// Examples need not list a fixed menu on every turn. The selected
	// professional term must still resolve through the shared vocabulary.
	if !known["并列论证"] || !strings.Contains(writingGuideTeachingRules, "并列论证") {
		t.Fatal("guide example must use a registered method name")
	}
	// The retired name must be gone everywhere, including the setup prompt.
	for _, prompt := range []string{writingPlanSystem, writingGuideTeachingRules, writingOpeningSystem, writingGuideSystem, writingGuideBatchSystem} {
		if strings.Contains(prompt, "正反") {
			t.Errorf("a prompt still names 正反, which is no longer any method's name (point_contrast is 比一比 / 对比论证)")
		}
	}
}

// bracketed pulls out every 『…』 span, the convention these prompts use to
// quote a method name.
func bracketed(s string) []string {
	var out []string
	rest := s
	for {
		i := strings.Index(rest, "『")
		if i < 0 {
			return out
		}
		rest = rest[i+len("『"):]
		j := strings.Index(rest, "』")
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j+len("』"):]
	}
}

func TestRootInsertPosition(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{Text: "不该一刀切禁手机", Role: "中心论点", Depth: 0, Position: 0},
		{Text: "理由一", Role: "一条理由", Depth: 1, Position: 1},
	}

	cases := []struct {
		name string
		role string
		want int32
	}{
		{"开篇排在最前", writingKindOpening, 0},
		{"结尾排在最后", writingKindClosing, int32(len(rows))},
		// 图上还没有开篇，所以中心论点就排在 0。有开篇时排在它后面，
		// 由下面那条子用例守着。
		{"中心论点排在开篇之后", writingKindThesis, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := topLevelInsertPosition(c.role, rows); got != c.want {
				t.Fatalf("topLevelInsertPosition(%q, rows) = %d, want %d", c.role, got, c.want)
			}
		})
	}
	withOpening := append([]sqlc.WritingOutline{
		{ID: uuid.New(), Kind: writingKindOpening, Depth: 0, Position: 0},
	}, rows...)
	if got := topLevelInsertPosition(writingKindThesis, withOpening); got != 1 {
		t.Fatalf("有开篇时中心论点应排在它后面，得到 %d", got)
	}
}

// 两份清单和 writing_kind.go 的闭表必须对得上。
//
// 🚨 这条守的是 R4 差点漏掉的那件事：闭表里加了记叙文那四种 kind，
// 而立题的提示词还写着「只能是下面这十个之一」—— 也就是模型**永远不会**
// 用到它们，一篇记叙文照旧被摆成中心论点／分论点／论据。
// 闭表和提示词是两处各自成立的东西，中间没有编译器。
func TestWritingPlanKindListsMatchTheClosedSet(t *testing.T) {
	argument := writingPlanSystemFor(genreArgument, "zh", "")
	narrative := writingPlanSystemFor(genreNarrative, "zh", "")

	for _, k := range []string{
		writingKindThesis, writingKindPoint, writingKindEvidence, writingKindReference,
		writingKindReasoning, writingKindCounter, writingKindRebuttal, writingKindGap,
		writingKindOpening, writingKindClosing,
	} {
		if !strings.Contains(argument, "「"+k+"」") {
			t.Errorf("议论文那一份提示词里没有 %q —— 模型开不出这一种块", k)
		}
	}
	for _, k := range []string{
		writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling,
		writingKindOpening, writingKindClosing,
	} {
		if !strings.Contains(narrative, "「"+k+"」") {
			t.Errorf("记叙文那一份提示词里没有 %q —— 记叙文那半边是死代码", k)
		}
	}

	// 🚨 两边不许串台。
	for _, k := range []string{writingKindThesis, writingKindPoint, writingKindCounter} {
		if strings.Contains(narrative, "「"+k+"」") {
			t.Errorf("记叙文那一份里混进了议论文的 %q —— 记叙文没有分论点", k)
		}
	}
	for _, k := range []string{
		writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling,
	} {
		if strings.Contains(argument, "「"+k+"」") {
			t.Errorf("议论文那一份里混进了记叙文的 %q", k)
		}
	}

	// 占位符必须被换掉 —— 换漏了，模型会收到一份没有 kind 清单的提示词，
	// 于是每一条 add 都被丢掉，而屏幕上只是「图没长出来」。
	for name, s := range map[string]string{"议论文": argument, "记叙文": narrative} {
		if strings.Contains(s, "@@KINDS@@") {
			t.Errorf("%s那一份里的占位符没被换掉", name)
		}
		if strings.Contains(s, "%d") {
			t.Errorf("%s那一份里的 %%d 没被换掉", name)
		}
	}
}

// 🚨 四种「文体 × 语言」的组合，每一种都要装配得出来一份完整的提示词。
//
// 为什么单写一条：2026-09-22 的提示词重构（b151e7ed）把 127 个文件里的文本
// 抽进了 `internal/prompts`，他们自己带了一条按 wire request 求 sha256 的
// parity 测试，基线是重构前的 main —— 那是对的做法。但那份基线只有 28 个
// 样例，写作立题这一族**只覆盖了一个**（`bench/compose/lite-writing-plan`，
// 中文议论文）。R5 加的另外三条分支（英文议论文、中文记叙文、英文记叙文）
// 不在基线里，也就是说重构如果把其中一条的占位符漏掉，整套测试是绿的，
// 而线上那一篇会收到一份带着 `@@KINDS@@` 字样的提示词 —— 模型看到它会
// 照着那个字面意思回答，而屏幕上只是「图没长出来」。
//
// 这一条把四个组合都钉住，判的是**装配的完整性**，不是某一句话的措辞。
func TestWritingPlanSystemFor_EveryGenreAndLangAssembles(t *testing.T) {
	// 🚨 2026-09-23 加上书信。AGENTS.md「提示词怎么写」第 6 条：新开一条分支
	// 就补一条装配测试 —— 占位符在没被覆盖的那条分支上漏掉，整套测试照样绿，
	// 而线上那一篇收到的是字面写着 @@KINDS@@ 的提示词。
	for _, genre := range []string{genreArgument, genreNarrative, genreLetter, genreProse} {
		for _, lang := range []string{"zh", langEnglish} {
			name := genre + "/" + lang
			got := writingPlanSystemFor(genre, lang, "")

			// 占位符一个都不许活着出去。
			for _, ph := range []string{"@@KINDS@@", "@@MATERIAL@@", "@@SKELETON@@", "@@COACH@@", "%d"} {
				if strings.Contains(got, ph) {
					t.Errorf("%s：占位符 %q 没被换掉 —— 模型会收到它的字面意思", name, ph)
				}
			}
			// 三块都得真的装进去了：块名清单、材料那一节、骨架那一节。
			if !strings.Contains(got, "「opening」") || !strings.Contains(got, "「closing」") {
				t.Errorf("%s：块名清单没装进去", name)
			}
			// 🚨 钉**术语**，不钉小标题。这一节的标题 2026-09-22 从
			// 「一整篇的骨架」改成了「常见文章结构」—— 文案会改，而
			// 结构的名字是这个仓库自己的词表（vocab 那条规矩：术语是我们的）。
			// 第一版我钉的正是那个标题，于是四种组合全红，而产品没事。
			skeletonMark := "总—分"
			if lang == langEnglish {
				skeletonMark = "Thesis"
			}
			// 🚨 书信 2026-09-24 起**按语言分开**。分开的理由不是措辞：
			// 英文应用文 80 词的硬上限会让「把这一段写厚」成为反的建议，
			// 要点来自题干而不由她自拟，称呼与结束语还有对仗关系。
			// 见 guidance/letter_lang_test.go。
			if genre == genreLetter {
				skeletonMark = "常见的信件结构"
				if lang == langEnglish {
					skeletonMark = "英文信的格式要素"
				}
			}
			// 散文同理：它的几种结构（一线串珠、以人为线索…）按线索分，不按语言分。
			if genre == genreProse {
				skeletonMark = "常见的散文结构"
			}
			if !strings.Contains(got, skeletonMark) {
				t.Errorf("%s：骨架那一节没装进去（找不到 %q）", name, skeletonMark)
			}
			// 输出格式那一节是解析器的契约，丢了整族 add 都会被丢掉。
			if !strings.Contains(got, `"ready"`) || !strings.Contains(got, `"kind"`) {
				t.Errorf("%s：输出格式那一节没装进去", name)
			}

			// 🚨 题目拆解只挂在英文议论文上。中文议论文的题目不是这个形状，
			// 英文记叙文没有 TASK 可拆 —— 串到那三条分支上就是给错教学内容，
			// 而她看不出来（印记仍然在用中文跟她说话）。
			const taskSplitMark = "TOPIC 与 TASK"
			wantTaskSplit := lang == langEnglish && genre == genreArgument
			if has := strings.Contains(got, taskSplitMark); has != wantTaskSplit {
				t.Errorf("%s：题目拆解那一节 在=%v，应该 在=%v", name, has, wantTaskSplit)
			}

			// 文体不许串台。
			if genre == genreNarrative && strings.Contains(got, "「thesis」") {
				t.Errorf("%s：记叙文那一份里混进了中心论点", name)
			}
			if genre == genreArgument && strings.Contains(got, "「scene」") {
				t.Errorf("%s：议论文那一份里混进了场景", name)
			}
		}
	}
}

// 🚨 拼完的成品里不许残留 @@。模板上多开一个洞而忘了登记，
// 只有这一条查得出来 —— guidance 那边查的是槽的内容，不是成品。
func TestWritingPlanSystemForLeavesNoPlaceholder(t *testing.T) {
	for _, lang := range []string{"zh", langEnglish} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			s := writingPlanSystemFor(genre, lang, "")
			if strings.Contains(s, "@@") {
				t.Errorf("%s/%s 的提示词里残留着占位符", lang, genre)
			}
			if strings.Contains(s, "%d") {
				t.Errorf("%s/%s 的提示词里残留着 %%d", lang, genre)
			}
			if len([]rune(s)) < 500 {
				t.Errorf("%s/%s 的提示词只有 %d 字，像是少拼了几段",
					lang, genre, len([]rune(s)))
			}
		}
	}
}
