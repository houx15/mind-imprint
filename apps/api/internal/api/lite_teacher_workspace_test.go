package api_test

// lite_teacher_workspace_test.go — 教师工作台的一轮。
//
// What these tests hold down is the turn's failure behaviour, not its prose:
// a turn that cannot finish must fail visibly, a reply that names a student
// nobody looked up must not reach the teacher, and every model call inside
// the loop must leave a row we can bill.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteworkspace"
)

type workspaceTurnJSON struct {
	Reply   string                 `json:"reply"`
	Choices []liteworkspace.Choice `json:"choices"`
	Patch   map[string]any         `json:"patch"`
	Cards   []struct {
		Kind string           `json:"kind"`
		Rows []map[string]any `json:"rows"`
	} `json:"cards"`
}

// wsToolCall scripts one model turn that reaches for a tool. The stub provider
// only implements Stream, so the arguments travel as ArgsJSON — the same shape
// a real OpenAI-compatible channel streams.
func wsToolCall(name, argsJSON string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventToolUse, ToolUse: &gateway.StreamToolUse{ID: "call_" + name, Name: name, ArgsJSON: argsJSON}},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopToolCall},
	}
}

// wsText scripts one model turn that answers in prose and stops.
func wsText(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

func postWorkspaceTurn(t *testing.T, h http.Handler, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/lite/teacher/workspace/turn", strings.NewReader(body))
	if cookie != nil {
		req = withCookie(req, cookie)
	}
	h.ServeHTTP(rec, req)
	return rec
}

func decodeWorkspaceTurn(t *testing.T, rec *httptest.ResponseRecorder) workspaceTurnJSON {
	t.Helper()
	var out workspaceTurnJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode workspace turn: %v — body=%s", err, rec.Body)
	}
	return out
}

// renameLiteStudent gives a seeded student a real name. The grounding check
// compares roster names against the reply, so the fixture's "S <email>" would
// make every assertion about a name vacuous.
func renameLiteStudent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE users SET display_name = $2 WHERE id = $1`, userID, name); err != nil {
		t.Fatalf("rename student: %v", err)
	}
}

func workspaceTurnBody(classID, text string) string {
	b, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": text,
		"artifact": map[string]any{"kind": "reading", "title": "", "dueInput": ""},
	})
	return string(b)
}

// TestWorkspaceTurnRejectsOutsiders — the route spends tokens and reads a
// class roster, so it must be closed to everyone but the teacher of that class.
func TestWorkspaceTurnRejectsOutsiders(t *testing.T) {
	h, pool, _, classID, studentID := liteTeacherFixtureWithProvider(t, writingTextStubProvider("好的。"))

	if rec := postWorkspaceTurn(t, h, nil, workspaceTurnBody(classID, "布置阅读作业")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out = %d, want 401; body=%s", rec.Code, rec.Body)
	}

	student := signInAs(t, pool, studentID)
	if rec := postWorkspaceTurn(t, h, student, workspaceTurnBody(classID, "布置阅读作业")); rec.Code != http.StatusForbidden {
		t.Fatalf("student = %d, want 403; body=%s", rec.Code, rec.Body)
	}

	// A teacher who does not teach this class gets not-found, the same answer
	// every other lite teacher route gives, so the class id cannot be probed.
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ws-other@demo.local"))
	if rec := postWorkspaceTurn(t, h, other, workspaceTurnBody(classID, "布置阅读作业")); rec.Code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceTurnRejectsUnknownSurface — home and parentReport carry other
// tools and another artifact. Answering them with the assignment tool set
// would be worse than refusing.
func TestWorkspaceTurnRejectsUnknownSurface(t *testing.T) {
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, writingTextStubProvider("好的。"))
	body, _ := json.Marshal(map[string]any{"surface": "home", "classId": classID, "text": "这个班怎么样"})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("surface=home = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceTurnFailsWhenToolLoopExhausted — the model keeps calling tools
// and never answers. The turn fails with the real cause; it must not truncate
// to whatever the last round happened to produce.
func TestWorkspaceTurnFailsWhenToolLoopExhausted(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsToolCall("set_fields", `{"title":"气候变化议论文"}`))
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	before := countAllLLMCalls(t, pool)
	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "帮我布置这周的作业"))
	if rec.Code < 400 {
		t.Fatalf("exhausted tool loop = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "工具调用次数超出上限") {
		t.Fatalf("body does not say why the turn failed: %s", rec.Body)
	}

	if prov.Calls != liteworkspace.ToolLoopMax {
		t.Fatalf("made %d model calls, want exactly %d — the cap is the cap", prov.Calls, liteworkspace.ToolLoopMax)
	}
	// Every one of those calls was paid for, including the last one that
	// produced nothing usable.
	if n := countAllLLMCalls(t, pool) - before; n != liteworkspace.ToolLoopMax {
		t.Fatalf("recorded %d llm_call rows for %d model calls", n, liteworkspace.ToolLoopMax)
	}
}

// TestWorkspaceTurnRejectsUngroundedName — §6's one structural check. The
// model names a student without anything having looked her up, so the turn
// fails and the reply is never rendered.
func TestWorkspaceTurnRejectsUngroundedName(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("林知遥这周没有写作，先给她单独布置。"))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周布置什么好"))
	if rec.Code < 400 {
		t.Fatalf("ungrounded name = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "林知遥") {
		t.Fatalf("the error does not name the offending student: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "先给她单独布置") {
		t.Fatalf("the rejected reply was rendered anyway: %s", rec.Body)
	}
}

// TestWorkspaceTurnRejectsNameFromItsOwnEarlierTurn — the grounding evidence
// is what the tools returned plus what the TEACHER typed. A name that only
// ever appeared in the model's own earlier turn is not evidence: that is where
// a fabrication comes from, so accepting it would launder the fabrication one
// turn later.
func TestWorkspaceTurnRejectsNameFromItsOwnEarlierTurn(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("那就按林知遥的情况来定。"))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": "那就这么办",
		"turns": []map[string]string{
			{"role": "teacher", "text": "这周布置什么好"},
			{"role": "ai", "text": "林知遥这周没有写作。"},
		},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code < 400 {
		t.Fatalf("name carried over from an AI turn = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "林知遥") {
		t.Fatalf("the error does not name the offending student: %s", rec.Body)
	}
}

// TestWorkspaceTurnRejectsUngroundedNameInThePatch — the reply is not the only
// thing the teacher reads. A name written into the card's instructions reaches
// her, and then reaches her whole class on publish, so it has to clear the
// same check the prose does.
func TestWorkspaceTurnRejectsUngroundedNameInThePatch(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"instructions":"这周的作业参考林知遥上次那篇。"}`),
		wsText("说明已经写好了。"),
	)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周布置什么好"))
	if rec.Code < 400 {
		t.Fatalf("fabricated name in the card = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "林知遥") {
		t.Fatalf("the error does not name the offending student: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "参考林知遥上次那篇") {
		t.Fatalf("the rejected card text was returned anyway: %s", rec.Body)
	}
}

// TestWorkspaceTurnRejectsUngroundedNameInAnOptionID — an option id is a string
// the model writes, and the client sends it back as her next turn. Left
// unchecked it is a second door into the card: mint {id:"林知遥-alone"}, she
// clicks, and the name arrives looking like something she typed.
func TestWorkspaceTurnRejectsUngroundedNameInAnOptionID(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsToolCall("ask_choice", `{"question":"发给谁？","options":[
		{"id":"林知遥-alone","label":"单独布置"},
		{"id":"whole-class","label":"全班"}
	]}`))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周布置什么好"))
	if rec.Code < 400 {
		t.Fatalf("fabricated name in an option id = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "林知遥") {
		t.Fatalf("the error does not name the offending student: %s", rec.Body)
	}
}

// TestWorkspaceTurnDoesNotGroundAChoiceID — the other half of the same door.
// Even if an option id carrying a name somehow reached the client, clicking it
// must not make that name evidence: the id is the model's own string, and only
// what she typed counts as hers.
func TestWorkspaceTurnDoesNotGroundAChoiceID(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，那就只发给林知遥。"))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "choiceId": "林知遥-alone",
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code < 400 {
		t.Fatalf("name laundered through a choice id = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "林知遥") {
		t.Fatalf("the error does not name the offending student: %s", rec.Body)
	}
}

// TestWorkspaceTurnClampsTitleToWhatPublishAccepts — the title and the
// instructions have different caps (200 and 2000). One shared cap would let a
// tool write a title the card displays and the publish endpoint then rejects,
// which is a failure at the last step over a value we handed her ourselves.
func TestWorkspaceTurnClampsTitleToWhatPublishAccepts(t *testing.T) {
	long := strings.Repeat("气", 900)
	args, _ := json.Marshal(map[string]any{"title": long, "instructions": long})
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", string(args)),
		wsText("标题和说明已经填好。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "帮我起个标题"))
	if rec.Code != http.StatusOK {
		t.Fatalf("set_fields = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)

	title, _ := out.Patch["title"].(string)
	if n := len([]rune(title)); n != 200 {
		t.Fatalf("title is %d runes; parseAssignmentTitle rejects anything over 200", n)
	}
	// That the clamped value is one publish actually accepts is asserted
	// against parseAssignmentTitle itself in
	// lite_teacher_workspace_internal_test.go, which can reach it.

	// The instructions keep their own, larger cap — clamping them to the title
	// limit would silently throw away most of what the model wrote.
	ins, _ := out.Patch["instructions"].(string)
	if n := len([]rune(ins)); n != 900 {
		t.Fatalf("instructions are %d runes, want the 900 written (cap is 2000)", n)
	}
}

// TestWorkspaceTurnAcceptsNameTheTeacherTyped — the other side of the same
// rule. She may talk about a student by name, and the reply may answer her in
// those words without any tool call.
func TestWorkspaceTurnAcceptsNameTheTeacherTyped(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，这份作业只发给林知遥。"))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": "只发给林知遥",
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("name the teacher typed = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceTurnAcceptsNameATooLReturned — a name list_students handed back
// this turn is evidence, and the card carries the rows the panel renders.
func TestWorkspaceTurnAcceptsNameAToolReturned(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_students", `{"filter":"no_writing_yet"}`),
		wsText("名单上的学生这周还没有写作，先给林知遥布置。"),
	)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "谁这周还没写作"))
	if rec.Code != http.StatusOK {
		t.Fatalf("grounded name = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if len(out.Cards) != 1 || out.Cards[0].Kind != "students" {
		t.Fatalf("cards = %+v, want one students card", out.Cards)
	}
	if len(out.Cards[0].Rows) != 1 || out.Cards[0].Rows[0]["name"] != "林知遥" {
		t.Fatalf("students card rows = %+v", out.Cards[0].Rows)
	}
}

// TestWorkspaceTurnClampsChoices — the panel renders one row of buttons. More
// than four is a row that wraps, and a blank label is a button that says
// nothing, so both are dropped before the reply leaves the server.
func TestWorkspaceTurnClampsChoices(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsToolCall("ask_choice", `{"question":"这次偏重哪一块？","options":[
		{"id":"structure","label":"论证结构"},
		{"id":"blank","label":"   "},
		{"id":"evidence","label":"证据使用"},
		{"id":"language","label":"语言表达"},
		{"id":"length","label":"篇幅"},
		{"id":"genre","label":"体裁"}
	]}`))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "布置一份写作作业"))
	if rec.Code != http.StatusOK {
		t.Fatalf("ask_choice = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Reply != "这次偏重哪一块？" {
		t.Fatalf("reply = %q, want the question ask_choice asked", out.Reply)
	}
	if len(out.Choices) != liteworkspace.MaxChoices {
		t.Fatalf("got %d choices, want %d", len(out.Choices), liteworkspace.MaxChoices)
	}
	for _, c := range out.Choices {
		if strings.TrimSpace(c.Label) == "" {
			t.Fatalf("a blank label reached the panel: %+v", out.Choices)
		}
	}
	// ask_choice ends the turn: the model is not called again after it.
	if prov.Calls != 1 {
		t.Fatalf("made %d model calls after ask_choice, want 1", prov.Calls)
	}
}

// TestWorkspaceTurnMetersOnDialogue — the workspace runs on the dialogue tier.
// Nothing here judges a student's work, so the flagship tier would be money
// spent for no reason.
func TestWorkspaceTurnMetersOnDialogue(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，先定题目。"))
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	if rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "帮我布置作业")); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var purpose, model, tier, surface string
	var prompt, completion int
	err := pool.QueryRow(context.Background(),
		`SELECT purpose, model, tier, surface, prompt_tokens, completion_tokens FROM llm_call ORDER BY created_at DESC LIMIT 1`,
	).Scan(&purpose, &model, &tier, &surface, &prompt, &completion)
	if err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if purpose != "lite_teacher_workspace" || surface != "lite" {
		t.Fatalf("llm_call purpose=%q surface=%q", purpose, surface)
	}
	// fakeResolver serves the chat lane (dialogue) and fakeEvalResolver the
	// flagship one (assess); the model name is what tells them apart.
	if model != "deepseek-chat" || tier != "chaperone" {
		t.Fatalf("llm_call model=%q tier=%q — the turn resolved the wrong capability class", model, tier)
	}
	if prompt != 100 || completion != 20 {
		t.Fatalf("llm_call tokens = %d/%d, want the usage the call reported", prompt, completion)
	}
}

// TestWorkspaceTurnPatchesOnlyWhatToolsWrote — the client applies the patch
// field by field and keeps whatever the teacher edited meanwhile, which only
// works if a field no tool touched is absent rather than blank.
func TestWorkspaceTurnPatchesOnlyWhatToolsWrote(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"kind":"writing","title":"中国是否让地球变得更可持续？","dueAt":"2026-09-18T18:00"}`),
		wsText("题目和截止时间已经填好，说明还需要你补一句。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周写一篇议论文，周五交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("set_fields = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["kind"] != "writing" || out.Patch["title"] != "中国是否让地球变得更可持续？" {
		t.Fatalf("patch = %+v", out.Patch)
	}
	// The card holds Beijing wall-clock text, the same shape the datetime
	// input reads back — converted once, at publish.
	if out.Patch["dueInput"] != "2026-09-18T18:00" {
		t.Fatalf("patch dueInput = %v, want the wall-clock string", out.Patch["dueInput"])
	}
	if _, present := out.Patch["instructions"]; present {
		t.Fatalf("patch carries a field no tool wrote: %+v", out.Patch)
	}
}

// TestWorkspaceTurnRefusesARelativeDeadline — 周五 is not an instant. The
// model is told today's Beijing date and resolves it itself; a relative phrase
// comes back as a tool error it can fix, and the turn still finishes.
func TestWorkspaceTurnRefusesARelativeDeadline(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"dueAt":"周五下午"}`),
		wsText("截止时间我再确认一下。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "周五交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if _, present := out.Patch["dueInput"]; present {
		t.Fatalf("a relative phrase was written to the card: %+v", out.Patch)
	}
}

// TestWorkspaceTurnSetsMaterialWithoutPickingPerStudent — personalized reading
// writes the patch and stops. Who reads what is the personalized-reading
// preview endpoint's answer, and a second one here would be a second source of
// truth for the same question.
func TestWorkspaceTurnSetsMaterialWithoutPickingPerStudent(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_material", `{"source":"personalized"}`),
		wsText("材料已设为个性化阅读。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "每个人读不一样的"))
	if rec.Code != http.StatusOK {
		t.Fatalf("set_material = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["readingSource"] != "personalized" {
		t.Fatalf("patch = %+v, want readingSource personalized", out.Patch)
	}
	if out.Cards == nil {
		t.Fatalf("cards decoded as null; the client walks it every turn and it must be []")
	}
	for _, k := range []string{"picks", "savedPicks", "slug"} {
		if _, present := out.Patch[k]; present {
			t.Fatalf("patch computed %q here; that belongs to the preview endpoint: %+v", k, out.Patch)
		}
	}
}

// TestWorkspaceTurnFailsWhenTheModelSaysNothing — an empty reply must not
// render as an empty bubble she would answer into. That is the dead-terminal
// bug: she keeps typing at a turn that already failed.
func TestWorkspaceTurnFailsWhenTheModelSaysNothing(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText(""))
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	before := countAllLLMCalls(t, pool)
	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "帮我布置作业"))
	if rec.Code < 400 {
		t.Fatalf("empty reply = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	// The call still cost tokens, so it is still on the bill.
	if n := countAllLLMCalls(t, pool) - before; n != 1 {
		t.Fatalf("recorded %d llm_call rows for a call that produced nothing, want 1", n)
	}
}

// TestWorkspaceTurnRejectsUngroundedHeadCount — §6's other half. A name is
// checked against the roster; a head count had nothing but the prompt behind
// it until now, and a live run produced 「发给全班 3 人」 with no tool having
// counted anything. A wrong number about her own class must not reach her.
func TestWorkspaceTurnRejectsUngroundedHeadCount(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，这份作业会发给全班 12 人。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周布置什么好"))
	if rec.Code < 400 {
		t.Fatalf("ungrounded head count = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "12") {
		t.Fatalf("the error does not name the offending count: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "发给全班") {
		t.Fatalf("the rejected reply was rendered anyway: %s", rec.Body)
	}
}

// TestWorkspaceTurnAcceptsACountAToolReturned — the check grounds, it does not
// ban. list_students counted the class this turn, so the reply may say so.
func TestWorkspaceTurnAcceptsACountAToolReturned(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_students", `{"filter":"all"}`),
		wsText("名单拉出来了，一共 1 人。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "班里有谁"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a count list_students returned = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceTurnKeepsDigitsThatAreNotHeadCounts — 🚨 the narrowness IS the
// feature. A due time, a tier and a word count all carry digits, and a checker
// that fired on them would fail turns for saying nothing wrong.
func TestWorkspaceTurnKeepsDigitsThatAreNotHeadCounts(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"dueAt":"2026-09-18T18:00","instructions":"不少于 800 字。"}`),
		wsText("截止定在 2026-09-18 18:00，难度 3 档，给你 2 个方向可以选。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "周五交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a reply full of innocent digits = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceSetRecipientsTakesAFilter — 「发给全班」 must cost one tool call.
// The model used to answer that intent with userIds:["all"], a filter name in
// the id field, and spend two more model calls recovering; one live turn hit
// 6 of 6 doing it.
func TestWorkspaceSetRecipientsTakesAFilter(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_recipients", `{"filter":"all"}`),
		wsText("已经设定好收件人。"),
	)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "发给全班"))
	if rec.Code != http.StatusOK {
		t.Fatalf("set_recipients with a filter = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	ids, _ := out.Patch["userIds"].([]any)
	if len(ids) != 1 || ids[0] != studentID.String() {
		t.Fatalf("userIds = %v, want the one enrolled student", out.Patch["userIds"])
	}
	// She is about to send homework to a group she named by condition, so the
	// canvas has to show who that turned out to be.
	if len(out.Cards) != 1 || out.Cards[0].Kind != "students" {
		t.Fatalf("cards = %+v, want one students card", out.Cards)
	}
}

// TestWorkspaceSetRecipientsRejectsAFilterNameAsAnID — the failure that cost
// the round trips. The message has to say what to do instead, or the model
// guesses again.
func TestWorkspaceSetRecipientsRejectsAFilterNameAsAnID(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_recipients", `{"userIds":["all"]}`),
		wsText("收件人还没定。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "发给全班"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a bad argument must come back as a tool result, not a failed turn: %d %s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if _, wrote := out.Patch["userIds"]; wrote {
		t.Fatalf("a filter name written as a recipient: %v", out.Patch)
	}
}

// TestWorkspaceAskChoiceCarriesASlug — the field that stops the model from
// smuggling an article into the option id. A bad slug is a tool error on the
// SAME turn, while there is still budget to fix it.
func TestWorkspaceAskChoiceCarriesASlug(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsToolCall("ask_choice", `{"question":"读哪篇？","options":[
		{"id":"a","label":"美国气候队","slug":"biden-creates-climate-corps"},
		{"id":"b","label":"个性化阅读"}
	]}`))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "找篇气候的文章"))
	if rec.Code != http.StatusOK {
		t.Fatalf("ask_choice with a slug = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if len(out.Choices) != 2 || out.Choices[0].Slug != "biden-creates-climate-corps" {
		t.Fatalf("choices = %+v, want the first one carrying the slug", out.Choices)
	}
	if out.Choices[1].Slug != "" {
		t.Fatalf("an option that is not about an article must carry no slug: %+v", out.Choices[1])
	}
}

func TestWorkspaceAskChoiceRejectsAnInventedSlug(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("ask_choice", `{"question":"读哪篇？","options":[
			{"id":"a","label":"美国气候队","slug":"american-climate-corps"},
			{"id":"b","label":"个性化阅读"}
		]}`),
		wsText("我再查一下这篇文章。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "找篇气候的文章"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a bad slug must come back as a tool result, not a failed turn: %d %s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if len(out.Choices) != 0 {
		t.Fatalf("an option list with an invented slug must not reach her: %+v", out.Choices)
	}
}

// TestWorkspaceChoiceSlugSetsTheMaterial — the point of the whole field: she
// taps 「用这篇」 and the article is set before the model runs, so the model
// never searches for its own choice id.
func TestWorkspaceChoiceSlugSetsTheMaterial(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，材料就定这篇。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID,
		"choiceId": "use-this", "choiceSlug": "biden-creates-climate-corps",
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("clicking an article option = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["readingSource"] != "library" || out.Patch["slug"] != "biden-creates-climate-corps" {
		t.Fatalf("patch = %v, want the material set from the tapped option", out.Patch)
	}
	// The whole point: no search round trip. One call is the turn's own reply.
	if prov.Calls != 1 {
		t.Fatalf("made %d model calls; a tapped article costs none of them", prov.Calls)
	}
}

// TestWorkspaceChoiceSlugIgnoresAnUnknownArticle — the field is client-supplied
// and therefore trusted for nothing. An id that is not in the catalogue writes
// nothing; the turn still runs.
func TestWorkspaceChoiceSlugIgnoresAnUnknownArticle(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("我先查一下库里有什么。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID,
		"choiceId": "use-this", "choiceSlug": "not-a-real-article",
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("unknown choiceSlug = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if _, wrote := out.Patch["slug"]; wrote {
		t.Fatalf("a slug the catalogue does not carry was written anyway: %v", out.Patch)
	}
}
