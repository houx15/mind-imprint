package api

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"mindimprint/api/internal/vocab"
)

// 讲义（五）的五句型，一句不少。
//
// 🚨 **阐释句是这条测试真正在守的那一句。** R4 之前主体段是四步
//（分论点句 / 论据 / 分析 / 回扣），少的正是它 —— 观点句写完直接跳到例子，
// 抽象的主张和具体的事之间没有桥。有人把它删回四步的时候，这里要响。
func TestBodyParagraphIsTheFiveSentenceShape(t *testing.T) {
	want := []string{"观点句", "阐释句", "材料句", "分析句", "结论句"}
	got := writingSentenceLabels(writingKindPoint)
	if len(got) != len(want) {
		t.Fatalf("主体段有 %d 句，讲义的五句型是 %d 句：%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 句是 %q，讲义里是 %q", i+1, got[i], want[i])
		}
	}
}

// 每一格点名的 vocab id 都要在库里。
//
// 一个拼错的 id 不会让任何东西崩 —— 它只是让那一格悄悄没有方法可给，
// 而屏幕上看不出区别。这正是「读代码看不出对错」的那一类。
func TestSentenceShapeMethodsAllExist(t *testing.T) {
	kinds := []string{
		writingKindOpening, writingKindThesis, writingKindPoint,
		writingKindCounter, writingKindRebuttal, writingKindClosing,
		writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling,
	}
	for _, k := range kinds {
		shape := writingSentenceShape(k)
		if len(shape) == 0 {
			t.Errorf("%s 没有段内结构", k)
			continue
		}
		for _, s := range shape {
			if s.Label == "" || s.Hint == "" {
				t.Errorf("%s 有一格缺标题或说明：%+v", k, s)
			}
			// 🚨 这些字直接渲染成纯文本。写 markdown 记号会原样印出来。
			if strings.Contains(s.Hint, "**") {
				t.Errorf("%s 的 %q 里有 markdown 记号，会原样印在她屏幕上", k, s.Label)
			}
			for _, id := range s.Methods {
				if _, ok := vocab.ByID(id); !ok {
					t.Errorf("%s 的 %q 点名了库里没有的方法 %q", k, s.Label, id)
				}
			}
		}
	}
}

// 分析句那一格必须给出三法，而且三法都带句式 —— 那一句可套的话正是
// 讲义值钱的地方，也是 R2 那天模型说「还差一句把它和主张连起来的话」时
// 说不出口的东西。
func TestAnalysisSlotOffersTheThreeNamedMethods(t *testing.T) {
	var slot writingSentenceRole
	for _, s := range writingSentenceShape(writingKindPoint) {
		if s.Label == "分析句" {
			slot = s
		}
	}
	want := map[string]bool{"analysis_cause": true, "analysis_suppose": true, "analysis_induce": true}
	if len(slot.Methods) != len(want) {
		t.Fatalf("分析句那一格给了 %v，要的是三法", slot.Methods)
	}
	for _, id := range slot.Methods {
		if !want[id] {
			t.Errorf("分析句那一格里混进了 %q", id)
		}
		m, _ := vocab.ByID(id)
		if len(m.Patterns) == 0 {
			t.Errorf("%s 没有句式", id)
		}
	}
}

// 🚨 Go 这一份和她屏幕上那一份必须逐字一致。
//
// 两份各自成立是故意的（一份 go:embed 不进前端构建），但分岔那天的样子是：
// 她在引导框里读到五步，陪练却按四步去判她这一段缺什么，而两边都不报错。
// 这条测试直接读 TS 那个文件，比的是 label 的**顺序和字面**。
func TestParagraphShapeMatchesFrontend(t *testing.T) {
	const tsPath = "../../../lite-web/src/writings/paragraphShape.ts"
	raw, err := os.ReadFile(tsPath)
	if err != nil {
		t.Fatalf("读不到 TS 孪生 %s：%v —— 文件挪走了就把这条测试一起改掉，别删", tsPath, err)
	}
	ts := string(raw)

	// TS 那边是一个 Record<string, ParagraphStep[]>。按 `  kind: [` 切块，
	// 再把块里的 label 顺序取出来。
	blockRe := regexp.MustCompile(`(?m)^  ([a-z]+): \[$`)
	labelRe := regexp.MustCompile(`\{ label: "([^"]+)"`)

	locs := blockRe.FindAllStringSubmatchIndex(ts, -1)
	if len(locs) == 0 {
		t.Fatal("在 TS 里一个 kind 块都没认出来 —— 那边的写法变了，这条测试的解析要跟着改")
	}
	fromTS := map[string][]string{}
	for i, loc := range locs {
		kind := ts[loc[2]:loc[3]]
		end := len(ts)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		for _, m := range labelRe.FindAllStringSubmatch(ts[loc[1]:end], -1) {
			fromTS[kind] = append(fromTS[kind], m[1])
		}
	}

	// TS 那边 opening 和 thesis 共用同一个常量（`opening: OPENING,`），
	// 所以块正则认不到它们 —— 单独比一次。
	for _, kind := range []string{writingKindOpening, writingKindThesis} {
		if len(writingSentenceLabels(kind)) == 0 {
			t.Errorf("%s 在 Go 这边没有段内结构", kind)
		}
	}
	if !strings.Contains(ts, "const OPENING: ParagraphStep[]") {
		t.Error("TS 那边的 OPENING 常量不见了 —— opening/thesis 两种的比对要跟着改")
	}
	for _, label := range writingSentenceLabels(writingKindOpening) {
		if !strings.Contains(ts, `label: "`+label+`"`) {
			t.Errorf("开篇的 %q 在 TS 那边找不到", label)
		}
	}

	for _, kind := range []string{
		writingKindPoint, writingKindCounter, writingKindRebuttal, writingKindClosing,
		writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling,
	} {
		want := writingSentenceLabels(kind)
		got := fromTS[kind]
		if len(got) == 0 {
			t.Errorf("TS 那边没有 %s 这一种块", kind)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("%s：Go 有 %d 句 %v，TS 有 %d 句 %v", kind, len(want), want, len(got), got)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s 第 %d 句：Go 是 %q，TS 是 %q", kind, i+1, want[i], got[i])
			}
		}
	}
}
