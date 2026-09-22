package api

// reading_routines_internal_test.go — unit test over the routine library
// itself (unexported `readingRoutines`/`taskConnect`/`taskHunt`), following
// the internal-test convention used by cards_test.go/teacher_read_internal_test.go
// for unexported symbols. reading_plan_test.go is `package api_test` (black
// box, HTTP-level) and cannot see these identifiers, so this TYPE-level
// guarantee lives here instead.

import (
	"strings"
	"testing"
)

// Every routine in the library carries one step that is hers and ends by
// sending her back into the article. This is the TYPE-level guarantee the
// spec rests on: a routine without a connect step is a routine that does not
// exist, which is a stronger promise than any line of prompt.
func TestEveryRoutineHasConnectAndEndsInHunt(t *testing.T) {
	for _, r := range readingRoutines {
		connects := 0
		hunts := 0
		for _, s := range r.Steps {
			switch s.Kind {
			case taskConnect:
				connects++
			case taskHunt:
				hunts++
			}
			if string(s.Kind) == "quiz" {
				t.Fatalf("routine %s still has a quiz step", r.Key)
			}
		}
		if connects != 1 {
			t.Errorf("routine %s: want exactly 1 connect step, got %d", r.Key, connects)
		}
		if hunts != 1 {
			t.Errorf("routine %s: want exactly 1 hunt step, got %d", r.Key, hunts)
		}
		if last := r.Steps[len(r.Steps)-1]; last.Kind != taskHunt {
			t.Errorf("routine %s: want the last step to be a hunt, got %q", r.Key, last.Kind)
		}
		if len(strings.TrimSpace(r.Steps[len(r.Steps)-1].Detail)) == 0 {
			t.Errorf("routine %s: the hunt step must say what to go find", r.Key)
		}
	}
}

// TestFocusBlockLabel — 「精读重点段落第 X 段」 must carry the REAL paragraph
// number. The failure modes worth naming are the two that would send her
// looking for a paragraph that does not exist: 「第 0 段」 from an unset
// ordinal, and a literal X left in the template.
func TestFocusBlockLabel(t *testing.T) {
	if got := focusBlockLabel(3); got != "精读重点段落第3段" {
		t.Errorf("focusBlockLabel(3) = %q", got)
	}
	for _, ord := range []int{0, -1} {
		got := focusBlockLabel(ord)
		if got != "精读重点段落" {
			t.Errorf("focusBlockLabel(%d) = %q, want the un-numbered fallback", ord, got)
		}
		if strings.Contains(got, "第0段") || strings.Contains(got, "第 0 段") ||
			strings.Contains(got, "X") {
			t.Errorf("focusBlockLabel(%d) rendered a fake paragraph: %q", ord, got)
		}
	}
}

// TestBuildReadingTasks_FocusLabelCarriesTheParagraphNumber — the label is
// built where the block is chosen, so the number always matches the blockId
// on the same row (and the 第N段 the coach prompt prints for it).
func TestBuildReadingTasks_FocusLabelCarriesTheParagraphNumber(t *testing.T) {
	blocks := []Block{{ID: "b1"}, {ID: "b2"}, {ID: "b3"}}
	routine, ok := findReadingRoutine("zh-scan-focus-lens")
	if !ok {
		t.Fatal("the default zh routine is gone")
	}
	_, kinds, labels, _, blockIDs := buildReadingTasks(
		routine, readingPlanReply{FocusBlocks: []string{"b3"}}, blocks, nil)

	found := false
	for i, kind := range kinds {
		if kind != string(taskFocusBlock) {
			continue
		}
		found = true
		if blockIDs[i] != "b3" {
			t.Fatalf("focus step points at %q", blockIDs[i])
		}
		if labels[i] != "精读重点段落第3段" {
			t.Errorf("focus label = %q, want the real paragraph number", labels[i])
		}
	}
	if !found {
		t.Fatal("no focus step was built")
	}
	for _, l := range labels {
		if strings.Contains(l, "第0段") || strings.Contains(l, "X 段") {
			t.Errorf("a label rendered a fake paragraph number: %q", l)
		}
	}
}

// 模型挑了一套不服务这个体裁的读法，服务端要换成服务它的那一套，
// 而且必须留在同一种语言里 —— 英文文章换成中文读法，她会看到一份
// 读不懂的清单。
func TestPickRoutineForGenreStaysInTheSameLanguage(t *testing.T) {
	var english readingRoutine
	for _, r := range readingRoutines {
		if r.Lang == "en" && r.serves(genreArgument) {
			english = r
			break
		}
	}
	if english.Key == "" {
		t.Fatal("库里没有服务英文议论文的读法，这条测试的前提不成立")
	}
	got := pickRoutineForGenre(english, genreNarrative)
	if got.Lang != "en" {
		t.Errorf("换成了 %q 语言的读法（key=%s），该留在 en", got.Lang, got.Key)
	}
	if !got.serves(genreNarrative) {
		t.Errorf("换来的读法 %s 不服务记叙文", got.Key)
	}
}

// 模型挑对了就不要动它。
func TestPickRoutineForGenreKeepsAGoodChoice(t *testing.T) {
	for _, r := range readingRoutines {
		if !r.serves(genreReport) {
			continue
		}
		if got := pickRoutineForGenre(r, genreReport); got.Key != r.Key {
			t.Errorf("挑对了还被换掉：%s → %s", r.Key, got.Key)
		}
	}
}

// 体裁为空（老数据、模型漏填）时原样保留 —— 那就是今天的样子。
func TestPickRoutineForGenreKeepsChoiceWhenGenreUnknown(t *testing.T) {
	r := readingRoutines[0]
	if got := pickRoutineForGenre(r, ""); got.Key != r.Key {
		t.Errorf("体裁未知时不该换：%s → %s", r.Key, got.Key)
	}
}
