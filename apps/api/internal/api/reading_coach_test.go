package api_test

// reading_coach_test.go — 带读: the AI leads, she doesn't manage stages.
//
// The guarantee that carries this design: the STUDENT never sets a step's
// status. She reads and answers; the coach decides whether that counted. So
// these tests drive the coach turn and assert the plan moved (or didn't) as a
// consequence of what she said — never as a consequence of a button.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type coachTurnJSON struct {
	Reply         string            `json:"reply"`
	Tasks         []readingTaskJSON `json:"tasks"`
	CurrentTaskID string            `json:"currentTaskId"`
	FocusBlock    string            `json:"focusBlock"`
	Finished      bool              `json:"finished"`
}

func coachTurn(t *testing.T, h http.Handler, cookie *http.Cookie, id, text string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	body := `{"text":` + strconv.Quote(text) + `}`
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("POST", "/api/v1/readings/"+id+"/coach", strings.NewReader(body)), cookie))
	return rec
}

func decodeCoachTurn(t *testing.T, rec *httptest.ResponseRecorder) coachTurnJSON {
	t.Helper()
	var out coachTurnJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode coach turn: %v — body=%s", err, rec.Body)
	}
	return out
}

// TestReadingCoach_StartPlansAndLeadsHerIn — 开始 is the ONLY button. Pressing
// it with no plan yet must produce one and walk her into step 1, not refuse.
func TestReadingCoach_StartPlansAndLeadsHerIn(t *testing.T) {
	// One stub serves both calls: the planning call reads routineKey/steps and
	// ignores the rest, the coach call reads reply/advance and ignores the
	// rest, so a single merged object satisfies both.
	const both = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
	  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
	  "reply":"我们先整体读一遍，别停下来查词。读完跟我说一声。","advance":"","focusBlock":"b3"}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(both))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	rec := coachTurn(t, h, cookie, id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("start = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeCoachTurn(t, rec)
	if strings.TrimSpace(out.Reply) == "" {
		t.Fatalf("the coach said nothing on 开始")
	}
	if len(out.Tasks) == 0 {
		t.Fatalf("开始 did not produce a plan")
	}
	if out.CurrentTaskID == "" {
		t.Fatalf("no current step after 开始 — she has nothing to be led into")
	}
	if out.Finished {
		t.Fatalf("finished on the very first turn")
	}
	// Nothing is complete yet: being led INTO step one is not doing it.
	for _, task := range out.Tasks {
		if task.Status != "pending" {
			t.Fatalf("step %q was already %q on the opening turn", task.Label, task.Status)
		}
	}
}

// TestReadingCoach_AdvancesWhenTheModelSaysSo — the whole point. She talks;
// the coach decides the step counted. No button anywhere in this flow.
func TestReadingCoach_AdvancesWhenTheModelSaysSo(t *testing.T) {
	const advancing = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[],
	  "reply":"读到了，那我们看第三段。","advance":"done","focusBlock":"b3"}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(advancing))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	first := decodeCoachTurn(t, coachTurn(t, h, cookie, id, ""))
	firstStep := first.CurrentTaskID
	if firstStep == "" {
		t.Fatalf("setup: no first step")
	}

	second := decodeCoachTurn(t, coachTurn(t, h, cookie, id, "读完了，大概讲城市比郊区热。"))
	if second.CurrentTaskID == firstStep {
		t.Fatalf("the coach said done but the plan did not move")
	}
	var moved bool
	for _, task := range second.Tasks {
		if task.ID == firstStep && task.Status == "done" {
			moved = true
		}
	}
	if !moved {
		t.Fatalf("step 1 is not marked done; tasks=%+v", second.Tasks)
	}
}

// TestReadingCoach_StaysPutWhenSheHasNotDoneIt — the other half of the same
// judgement. An empty advance leaves her exactly where she was.
func TestReadingCoach_StaysPutWhenSheHasNotDoneIt(t *testing.T) {
	const staying = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[],
	  "reply":"先别急着问结论——先从头到尾读一遍，读完跟我说。","advance":"","focusBlock":""}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(staying))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	first := decodeCoachTurn(t, coachTurn(t, h, cookie, id, ""))
	second := decodeCoachTurn(t, coachTurn(t, h, cookie, id, "这篇结论是什么？"))
	if second.CurrentTaskID != first.CurrentTaskID {
		t.Fatalf("the plan moved on a turn the coach did not advance")
	}
	for _, task := range second.Tasks {
		if task.Status != "pending" {
			t.Fatalf("step %q became %q without the coach advancing", task.Label, task.Status)
		}
	}
}

// TestReadingCoach_SkipIsRecordedNotRefused — 铁律②/④. She asks to skip; the
// coach records it and moves on rather than arguing.
func TestReadingCoach_SkipIsRecordedNotRefused(t *testing.T) {
	const skipping = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[],
	  "reply":"行，那跳过这步。","advance":"skipped","focusBlock":""}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(skipping))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	first := decodeCoachTurn(t, coachTurn(t, h, cookie, id, ""))
	second := decodeCoachTurn(t, coachTurn(t, h, cookie, id, "这步我想跳过。"))
	var skipped bool
	for _, task := range second.Tasks {
		if task.ID == first.CurrentTaskID && task.Status == "skipped" {
			skipped = true
		}
	}
	if !skipped {
		t.Fatalf("the skip was not recorded; tasks=%+v", second.Tasks)
	}
}

// TestReadingCoach_CannotJumpSeveralSteps — a model inventing an advance value
// must not move her at all. Failing toward "stay put" is the safe direction:
// jumping three steps is the coach reading the article FOR her.
func TestReadingCoach_CannotJumpSeveralSteps(t *testing.T) {
	const rogue = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[],
	  "reply":"我们直接跳到最后。","advance":"all_done","focusBlock":""}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(rogue))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	first := decodeCoachTurn(t, coachTurn(t, h, cookie, id, ""))
	second := decodeCoachTurn(t, coachTurn(t, h, cookie, id, "都读完了。"))
	if second.CurrentTaskID != first.CurrentTaskID {
		t.Fatalf("an invented advance value moved the plan")
	}
	if second.Finished {
		t.Fatalf("an invented advance value finished the whole reading")
	}
}

// TestReadingCoach_FocusBlockPrefersThePlansOwnParagraph — the plan already
// decided which paragraph a step is about; a per-turn guess must not scroll
// her somewhere the step never meant.
func TestReadingCoach_FocusBlockPrefersThePlansOwnParagraph(t *testing.T) {
	const wrongFocus = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
	  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
	  "reply":"看第一段吧。","advance":"done","focusBlock":"b1"}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(wrongFocus))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	// This stub advances on every turn, so exactly ONE turn is what lands her
	// on the focus_block step — step 1 completes and step 2 becomes current.
	// That step carries b3 from the plan, and it must win over the turn's own
	// "b1". (Driving a second turn here would advance PAST the focus step and
	// assert nothing, which is how this test first failed.)
	out := decodeCoachTurn(t, coachTurn(t, h, cookie, id, ""))
	if out.FocusBlock != "b3" {
		t.Fatalf("focusBlock = %q, want the plan's own b3", out.FocusBlock)
	}
}

// TestReadingCoach_ReachesForAParagraphTool — 想一想 and 仿写 are the coach's
// instruments, not a menu she is left to browse. It names one, and the room
// opens it on the paragraph the step is about.
func TestReadingCoach_ReachesForAParagraphTool(t *testing.T) {
	const withTool = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],
	  "steps":[{"kind":"read","detail":"先整体读一遍。"}],
	  "reply":"这一段的写法值得你自己练一遍——我给你开了仿写。","advance":"done","focusBlock":"b3","tool":"imitate"}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(withTool))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	var out struct {
		Tool       string `json:"tool"`
		FocusBlock string `json:"focusBlock"`
	}
	rec := coachTurn(t, h, cookie, id, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Tool != "imitate" {
		t.Fatalf("tool = %q, want the one the coach reached for", out.Tool)
	}
	if out.FocusBlock == "" {
		t.Fatalf("a tool arrived with no paragraph to open on")
	}
}

// TestReadingCoach_DropsAToolItCannotOpen — an invented id, a tool from the
// other language, or a tool with no paragraph behind it. In every case the
// REPLY still stands: losing an instrument must not cost her the turn.
func TestReadingCoach_DropsAToolItCannotOpen(t *testing.T) {
	cases := []struct{ name, reply string }{
		{"invented id", `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[],
		  "reply":"看这段。","advance":"","focusBlock":"b3","tool":"rewrite_it_for_her"}`},
		{"wrong language", `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[],
		  "reply":"看这段。","advance":"","focusBlock":"b3","tool":"grammar"}`},
		{"no paragraph", `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[],
		  "reply":"想一想。","advance":"","focusBlock":"","tool":"questions"}`},
	}
	for _, tc := range cases {
		h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(tc.reply))
		id := createReadingAtom(t, h, cookie)
		putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

		var out struct {
			Tool  string `json:"tool"`
			Reply string `json:"reply"`
		}
		rec := coachTurn(t, h, cookie, id, "")
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: decode: %v — body=%s", tc.name, err, rec.Body)
		}
		if out.Tool != "" {
			t.Fatalf("%s: tool = %q, want it dropped", tc.name, out.Tool)
		}
		if strings.TrimSpace(out.Reply) == "" {
			t.Fatalf("%s: the reply was lost along with the tool", tc.name)
		}
	}
}

// TestReadingCoach_ModelFailureSurfaces — USER RULE: a real 502.
func TestReadingCoach_ModelFailureSurfaces(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, streamErrorProvider{})
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	if rec := coachTurn(t, h, cookie, id, ""); rec.Code != http.StatusBadGateway {
		t.Fatalf("coach on model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
}

// ---------------------------------------------------------------------------
// 想一想 / 仿写 — the two shaped tools
// ---------------------------------------------------------------------------

// TestReadingBlockTools_IncludeTheWritingPair — they are language-independent,
// because "what does this paragraph DO" is not a language-specific question.
func TestReadingBlockTools_IncludeTheWritingPair(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	for _, article := range []struct{ title, body string }{
		{"城市为什么比郊区热？", zhArticle},
		{"The Urban Heat Island", enArticle},
	} {
		id := createReadingAtom(t, h, cookie)
		putReadingSourceHTTP(t, h, cookie, id, article.title, article.body)
		_, ids := listBlockTools(t, h, cookie, id)
		for _, want := range []string{"questions", "imitate"} {
			if !containsString(ids, want) {
				t.Fatalf("%s tools = %v, missing %q", article.title, ids, want)
			}
		}
	}
}

// TestReadingBlockQuestions_AreQuestionsOnly — the same filter the writing
// room's guiding box uses. A declarative sentence in the list is a sentence
// she could paste, which is what the shape exists to prevent.
func TestReadingBlockQuestions_AreQuestionsOnly(t *testing.T) {
	const mixed = `{"questions":[
	  "这一段里哪个数字最让你意外？",
	  "你可以写：城市热岛是水泥造成的。",
	  "如果你住在市中心，晚上会怎么感觉到这件事？"]}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(mixed))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	rec := explainBlock(t, h, cookie, id, "b3", "questions")
	if rec.Code != http.StatusOK {
		t.Fatalf("questions = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(out.Body, "你可以写") {
		t.Fatalf("a ready-to-paste sentence survived the questions filter:\n%s", out.Body)
	}
	if !strings.Contains(out.Body, "最让你意外") {
		t.Fatalf("the real questions were dropped:\n%s", out.Body)
	}
}

// TestReadingBlockImitate_HasNowhereToPutASampleParagraph is the 铁律
// assertion for 仿写. The shape carries a MOVE and TOPICS; there is no field a
// sample paragraph could live in, so even a model that wants to write one for
// her cannot deliver it.
func TestReadingBlockImitate_HasNowhereToPutASampleParagraph(t *testing.T) {
	// A model trying its best to smuggle prose through: an extra field, and a
	// sample sentence tucked into the topic list.
	const sneaky = `{"move":"先给一个日常场景，再解释背后的原理。",
	  "sample":"夏天的傍晚，如果你从市中心骑车回郊区，会明显感到凉快下来。",
	  "tryThis":["地铁早高峰为什么特别挤","为什么下雨天外卖会变慢"]}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(sneaky))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	rec := explainBlock(t, h, cookie, id, "b3", "imitate")
	if rec.Code != http.StatusOK {
		t.Fatalf("imitate = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(out.Body, "夏天的傍晚") {
		t.Fatalf("a sample paragraph reached the student through 仿写:\n%s", out.Body)
	}
	if !strings.Contains(out.Body, "先给一个日常场景") {
		t.Fatalf("the move was lost:\n%s", out.Body)
	}
	if !strings.Contains(out.Body, "地铁早高峰") {
		t.Fatalf("the topics to try were lost:\n%s", out.Body)
	}
}

// TestReadingBlockShaped_UnparseableIsAnError — a shaped reply that will not
// parse must NEVER be rendered raw: rendering it raw is exactly how a sample
// paragraph would reach her through the one tool built to prevent that.
func TestReadingBlockShaped_UnparseableIsAnError(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("这一段可以这样仿写：夏天的傍晚……"))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	for _, tool := range []string{"questions", "imitate"} {
		if rec := explainBlock(t, h, cookie, id, "b3", tool); rec.Code != http.StatusBadGateway {
			t.Fatalf("%s with an unshaped reply = %d, want 502; body=%s", tool, rec.Code, rec.Body)
		}
	}
}
