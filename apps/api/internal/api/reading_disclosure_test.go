package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 渐进披露默认关（见 readingDisclosureEnabled）。这些测试量的是它开着时的
// 行为，所以每一条自己把它打开 —— 而不是依赖跑测试的人记得设环境变量。
func enableDisclosure(t *testing.T) {
	t.Helper()
	t.Setenv("READING_DISCLOSURE", "1")
}

// 默认必须是关的：这条守的是「谁 deploy main 都不会把它带上线」。
func TestDisclosureIsOffByDefault(t *testing.T) {
	t.Setenv("READING_DISCLOSURE", "")
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskRead), BlockID: "b4", Status: "pending"},
	}
	if got := readingDisclosureScope(discBlocks(9), discOutline(), tasks, nil, nil); got != nil {
		t.Fatal("渐进披露默认必须是关的 —— 它还没过长文走查那一关")
	}
}

// 每段 400 字 × 9 段 = 3,600 字，越过 readingDisclosureMinRunes 的门槛。
func discBlocks(n int) []Block {
	out := make([]Block, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, Block{ID: "b" + itoaSmall(i), Text: strings.Repeat("字", 400)})
	}
	return out
}

func discOutline() readingOutline {
	return readingOutline{Parts: []readingPart{
		{From: "b1", To: "b3", Title: "第一部分"},
		{From: "b4", To: "b6", Title: "第二部分"},
		{From: "b7", To: "b9", Title: "第三部分"},
	}}
}

// 通读这一步只管它那一个部分 —— 展开的正好是那三段。
func TestDisclosureReadStepKeepsOnlyItsPart(t *testing.T) {
	enableDisclosure(t)
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskRead), BlockID: "b1", Status: "done"},
		{ID: fixtureTaskID(2), Position: 2, Kind: string(taskRead), BlockID: "b4", Status: "pending"},
	}
	got := readingDisclosureScope(discBlocks(9), discOutline(), tasks, nil, nil)
	if got == nil {
		t.Fatal("expected a narrowed scope for a 通读 step that names a part")
	}
	for _, id := range []string{"b4", "b5", "b6"} {
		if !got[id] {
			t.Errorf("%s is in the current part and must be expanded", id)
		}
	}
	for _, id := range []string{"b1", "b2", "b3", "b7", "b8", "b9"} {
		if got[id] {
			t.Errorf("%s is in another part and must not be expanded", id)
		}
	}
}

// 精读：那一段加左右各一段。一段话很少能脱离邻居读懂。
func TestDisclosureFocusStepKeepsNeighbours(t *testing.T) {
	enableDisclosure(t)
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskFocusBlock), BlockID: "b5", Status: "pending"},
	}
	got := readingDisclosureScope(discBlocks(9), discOutline(), tasks, nil, nil)
	for _, id := range []string{"b4", "b5", "b6"} {
		if !got[id] {
			t.Errorf("%s must be expanded (the focus paragraph and its neighbours)", id)
		}
	}
	if got["b7"] || got["b3"] {
		t.Error("only one neighbour each side should be expanded")
	}
}

// 🚨 不划定段落范围的步骤要通观全文。收窄它们等于拿掉它们的依据。
func TestDisclosureLeavesOpenEndedStepsAlone(t *testing.T) {
	enableDisclosure(t)
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskReflect), BlockID: "b2", Status: "pending"},
	}
	if got := readingDisclosureScope(discBlocks(9), discOutline(), tasks, nil, nil); got != nil {
		t.Fatalf("a reflect step must see the whole article, got a narrowed scope: %v", got)
	}
}

// 没有分部分就没有依据收窄 —— 照旧全给。
func TestDisclosureWithoutPartsKeepsEverything(t *testing.T) {
	enableDisclosure(t)
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskRead), BlockID: "b1", Status: "pending"},
	}
	if got := readingDisclosureScope(discBlocks(9), readingOutline{}, tasks, nil, nil); got != nil {
		t.Fatal("no parts means no basis to narrow; the whole article must be sent")
	}
}

// 🚨 短文章一律不收窄：省不到什么，而解释性的那一段可能正是她这一步的依据。
func TestDisclosureSkipsShortArticles(t *testing.T) {
	enableDisclosure(t)
	short := make([]Block, 0, 6)
	for i := 1; i <= 6; i++ {
		short = append(short, Block{ID: "b" + itoaSmall(i), Text: strings.Repeat("字", 40)})
	}
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskFocusBlock), BlockID: "b3", Status: "pending"},
	}
	if got := readingDisclosureScope(short, discOutline(), tasks, nil, nil); got != nil {
		t.Fatal("a short article must not be narrowed — nothing to save, real risk")
	}
}

// 🚨 承重段永远展开。走查那篇文章的定义句在最后一段，而她这一步要用的正是那个
// 定义 —— 按「这一段加左右各一段」收窄会把它收起来，印记 就失去了依据。
func TestDisclosureAlwaysKeepsCoreParagraphs(t *testing.T) {
	enableDisclosure(t)
	outline := discOutline()
	outline.Load = map[string]string{"b9": loadCore}
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskFocusBlock), BlockID: "b3", Status: "pending"},
	}
	got := readingDisclosureScope(discBlocks(9), outline, tasks, nil, nil)
	if got == nil {
		t.Fatal("expected a narrowed scope")
	}
	if !got["b9"] {
		t.Error("a core paragraph must stay expanded even when far from the current step")
	}
	if got["b7"] {
		t.Error("a non-core paragraph in another part should still be collapsed")
	}
}

// 她自己点出来的句子，无论属于哪一部分都要展开：印记 正要跟她谈的就是它。
func TestDisclosureAlwaysKeepsHerPicks(t *testing.T) {
	enableDisclosure(t)
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskRead), BlockID: "b1", Status: "pending"},
	}
	picks := []readingPick{{BlockID: "b8", Quote: "她点的那一句"}}
	got := readingDisclosureScope(discBlocks(9), discOutline(), tasks, picks, nil)
	if !got["b8"] {
		t.Error("a paragraph she picked must be expanded even though it is in another part")
	}
}
