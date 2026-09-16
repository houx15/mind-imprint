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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/library"
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

// TestWorkspaceRecommendArticles — the model reaches for recommend_articles
// when the teacher has not named a text; the tool's result (computed from
// every enrolled student's library.Profile, no model call of its own) feeds
// back into the loop and lands on the canvas as an articles card.
func TestWorkspaceRecommendArticles(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("recommend_articles", `{}`),
		wsText("给你推荐了几篇，选一篇作为这次的材料。"),
	)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	d := library.All()[len(library.All())-1].Disciplines[0]
	seedInterestFor(t, pool, studentID, d)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBodyOfKind(classID, "reading", "帮我推荐一篇文章"))
	if rec.Code != http.StatusOK {
		t.Fatalf("recommend_articles turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if len(out.Cards) != 1 || out.Cards[0].Kind != "articles" {
		t.Fatalf("cards = %+v, want one articles card", out.Cards)
	}
	rows := out.Cards[0].Rows
	if len(rows) == 0 {
		t.Fatalf("articles card is empty, want at least one recommendation")
	}
	want, _ := disciplines.ByID(d)
	first := rows[0]
	if first["slug"] == "" || first["zhTitle"] == "" {
		t.Fatalf("first row = %+v, want a slug and a Chinese title", first)
	}
	why, _ := first["why"].([]any)
	if len(why) == 0 || why[0] != want.Zh {
		t.Fatalf("why = %v, want it to name %s", first["why"], want.Zh)
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
	for _, k := range []string{"picks", "savedPicks"} {
		if _, present := out.Patch[k]; present {
			t.Fatalf("patch computed %q here; that belongs to the preview endpoint: %+v", k, out.Patch)
		}
	}
	// slug/tier ARE in the patch — cleared to their zero value, not computed.
	// A prior library pick must not linger next to 「材料来源：个性化」 in the
	// card state (M-2); that is different from "picks belongs to the preview
	// endpoint", which is about who reads what, not about clearing what came
	// before.
	if out.Patch["slug"] != "" {
		t.Fatalf("patch[slug] = %v, want cleared to \"\"", out.Patch["slug"])
	}
	if tier, wrote := out.Patch["tier"]; !wrote || tier != nil {
		t.Fatalf("patch[tier] = %v (wrote=%v), want cleared to nil", tier, wrote)
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

// TestWorkspaceTurnDoesNotGroundACountInADate — 🚨 the evidence side used to
// read every integer the teacher typed, which made this sentence ground 1 (一篇)
// and 5 (周五). A reply inventing 「发给全班 5 人」 then walked straight through
// the check that exists to stop exactly that. Her side reads head counts now,
// the same shape the reply is checked with.
func TestWorkspaceTurnDoesNotGroundACountInADate(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，发给全班 5 人。"))
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	enrollMoreLiteStudents(t, pool, classID, 2)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周读一篇气候变化的报道，周五交"))
	if rec.Code < 400 {
		t.Fatalf("a count read out of 周五 = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "5") {
		t.Fatalf("the error does not name the offending count: %s", rec.Body)
	}
}

// TestWorkspaceTurnAcceptsTheClassSize — the system prompt's first line hands
// the model 「共 N 名学生」. A reply repeating that number is repeating data we
// supplied, so failing the turn would punish the model for being right.
func TestWorkspaceTurnAcceptsTheClassSize(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，这份作业发给全班 3 人。"))
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	enrollMoreLiteStudents(t, pool, classID, 2)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周布置什么好"))
	if rec.Code != http.StatusOK {
		t.Fatalf("the class size the prompt states = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceTurnDoesNotAssembleACountAcrossFields — 🚨 the count check runs
// field by field, never on the joined blob.
//
// It strips whitespace before matching, so any separator a join could put
// between two fields dissolves. Here the question ends 「难度 3」 and the option
// id the model minted starts 「人工智能」; joined they read as 「3 人」, a claim
// about three people that neither field makes. Measured: StatedCounts of the
// joined string is [3], and of either field on its own is empty.
//
// A Chinese option id is the realistic shape, not a contrivance — the id is the
// model's own string, which is why TestWorkspaceTurnRejectsUngroundedNameInAnOptionID
// exists one screen up.
func TestWorkspaceTurnDoesNotAssembleACountAcrossFields(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsToolCall("ask_choice", `{"question":"好的，材料定在难度 3","options":[
		{"id":"人工智能方向","label":"人工智能"},
		{"id":"climate","label":"气候方向"}
	]}`))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "挑个方向"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a count assembled out of two fields = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// enrollMoreLiteStudents grows the fixture class past one student, so a test
// about a head count has a number to look for that is not 1.
func enrollMoreLiteStudents(t *testing.T, pool *pgxpool.Pool, classID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := createStudent(t, pool, SeedSchoolID, fmt.Sprintf("lt-extra-%d@demo.local", i))
		enrollStudent(t, pool, id, classID)
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

// TestWorkspaceAskChoiceEnrichesArticleCard — an option carrying a slug comes
// back with the catalogue fields a card needs (Task 4). The option that is
// not about an article must carry no card at all, and the label — built from
// the model's own words — still has to pass §6, same as before this field
// existed.
func TestWorkspaceAskChoiceEnrichesArticleCard(t *testing.T) {
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
	if len(out.Choices) != 2 {
		t.Fatalf("choices = %+v, want 2", out.Choices)
	}
	art, found := library.BySlug("biden-creates-climate-corps")
	if !found {
		t.Fatal("fixture article missing from the embedded catalogue")
	}
	got := out.Choices[0].Article
	if got == nil {
		t.Fatalf("choice with a slug carries no article card: %+v", out.Choices[0])
	}
	if got.Slug != art.Slug || got.ZhTitle != art.ZhTitle || got.Reason != art.Reason {
		t.Fatalf("article card = %+v, want slug/zhTitle/reason from the catalogue entry %+v", got, art)
	}
	if out.Choices[1].Article != nil {
		t.Fatalf("an option that is not about an article must carry no article card: %+v", out.Choices[1])
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

// workspaceTurnBodyOfKind is workspaceTurnBody with the card's type set, which
// is what decides whether the card has a material row at all.
func workspaceTurnBodyOfKind(classID, kind, text string) string {
	b, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": text,
		"artifact": map[string]any{"kind": kind, "title": "", "dueInput": ""},
	})
	return string(b)
}

// TestWorkspaceSetMaterialRefusesAWritingCard — the defect the browser pass
// found and no live assertion could see. The model set kind=writing, searched
// the library and told her 「材料：已选「美国气候队」这篇报道」, while the card —
// which renders the material row only for a reading homework — showed nothing.
// She would have published a writing task with no article after being told one
// was chosen.
//
// The material must not be written, and the tool error must name the remedy:
// the model recovers from an error that says what to do, and ignores one that
// only says no.
func TestWorkspaceSetMaterialRefusesAWritingCard(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_material", `{"source":"library","slug":"biden-creates-climate-corps"}`),
		wsText("这份作业是写作，没有阅读材料。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBodyOfKind(classID, "writing", "写一篇议论文"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a bad argument must come back as a tool result, not a failed turn: %d %s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if _, wrote := out.Patch["slug"]; wrote {
		t.Fatalf("an article was attached to a writing card, where it cannot be shown: %v", out.Patch)
	}
	if _, wrote := out.Patch["readingSource"]; wrote {
		t.Fatalf("a reading source was written onto a writing card: %v", out.Patch)
	}
}

// TestWorkspaceSetMaterialAllowsAReadingCard — the guard reads the card, it
// does not ban the tool.
func TestWorkspaceSetMaterialAllowsAReadingCard(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_material", `{"source":"library","slug":"biden-creates-climate-corps"}`),
		wsText("材料已定。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBodyOfKind(classID, "reading", "读一篇气候的报道"))
	if rec.Code != http.StatusOK {
		t.Fatalf("set_material on a reading card = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["slug"] != "biden-creates-climate-corps" {
		t.Fatalf("patch = %v, want the article set", out.Patch)
	}
}

// TestWorkspaceReplyDeslugsAnArticle — spec §12.1's backstop. The card state
// already keeps slugs out of what the model reads, but the model still holds
// one after set_material's own tool result hands it back (that result is a
// wire value on purpose — it is what the next tool call needs). If the model
// echoes the slug into its prose anyway, the teacher must see the article's
// title, never the slug.
func TestWorkspaceReplyDeslugsAnArticle(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_material", `{"source":"library","slug":"biden-creates-climate-corps"}`),
		wsText("材料已经选好了 biden-creates-climate-corps 这篇。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBodyOfKind(classID, "reading", "读一篇气候的报道"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if strings.Contains(out.Reply, "biden-creates-climate-corps") {
		t.Fatalf("reply still carries the raw slug: %q", out.Reply)
	}
	if want := "材料已经选好了 《美国气候队》 这篇。"; out.Reply != want {
		t.Fatalf("reply = %q, want %q", out.Reply, want)
	}
}

// TestWorkspaceSetMaterialFollowsAKindSetThisTurn — set_fields switching the
// card to reading must open the material row immediately, in the same turn.
// This is the recovery path the tool error points at; if it did not work, the
// error would be advice the model cannot take.
func TestWorkspaceSetMaterialFollowsAKindSetThisTurn(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"kind":"reading"}`),
		wsToolCall("set_material", `{"source":"library","slug":"biden-creates-climate-corps"}`),
		wsText("类型改成阅读，材料已定。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBodyOfKind(classID, "writing", "改成读一篇报道"))
	if rec.Code != http.StatusOK {
		t.Fatalf("kind then material = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["kind"] != "reading" || out.Patch["slug"] != "biden-creates-climate-corps" {
		t.Fatalf("patch = %v, want both the kind and the article", out.Patch)
	}
}

// TestWorkspaceSetFieldsClearsAMaterialItIsHiding — the other direction of the
// same fault, and the one a live run found my first guard missing: the material
// was set in one turn and the kind switched in the NEXT, so a check scoped to
// one turn never saw them together.
//
// The card must never hold what it cannot render. Switching away from reading
// clears the material, and the tool result says so, because the alternative —
// refusing — leaves the model an instruction it has no tool to carry out.
func TestWorkspaceSetFieldsClearsAMaterialItIsHiding(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"kind":"writing"}`),
		wsText("改成写作了，原来那篇文章已经清掉。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	// The card already carries an article, from an earlier turn.
	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": "改成写作作业吧",
		"artifact": map[string]any{
			"kind": "reading", "readingSource": "library", "slug": "biden-creates-climate-corps",
		},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["kind"] != "writing" {
		t.Fatalf("patch = %v, want the kind she asked for", out.Patch)
	}
	if out.Patch["slug"] != "" {
		t.Fatalf("patch = %v, want the hidden article cleared, not left where nothing renders it", out.Patch)
	}
}

// TestWorkspaceSetFieldsKeepsAMaterialOnAReadingCard — the clearing is narrow.
// Rewriting other fields on a reading card must leave its article alone.
func TestWorkspaceSetFieldsKeepsAMaterialOnAReadingCard(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"title":"气候变化阅读","kind":"reading"}`),
		wsText("标题写好了。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": "标题起一个",
		"artifact": map[string]any{
			"kind": "reading", "readingSource": "library", "slug": "biden-creates-climate-corps",
		},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if _, cleared := out.Patch["slug"]; cleared {
		t.Fatalf("patch = %v, want the article left alone on a reading card", out.Patch)
	}
}

// TestWorkspaceChoiceSlugRefusesAWritingCard — the tapped-option path is not a
// back door into the same broken state.
func TestWorkspaceChoiceSlugRefusesAWritingCard(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("这份作业是写作，先把类型改成阅读才能用这篇。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID,
		"artifact": map[string]any{"kind": "writing"},
		"choiceId": "use-this", "choiceSlug": "biden-creates-climate-corps",
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("tapping an article on a writing card = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if _, wrote := out.Patch["slug"]; wrote {
		t.Fatalf("the tapped article was attached to a writing card: %v", out.Patch)
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

// TestWorkspaceTurnSetsPastedTextPastSection6Checks — F7: a teacher-pasted
// paragraph is full of real names and numbers with nothing to do with the
// roster. It must reach the card without tripping §6's name/count checks —
// set_material's text case already proved it verbatim (a substring of what
// she typed this turn), so liteWorkspaceCheckedParts skips the "text" patch
// field on purpose.
func TestWorkspaceTurnSetsPastedTextPastSection6Checks(t *testing.T) {
	pasted := "2024年，中国的可再生能源投资达到了8900亿美元，" +
		"国际能源署负责人法提赫·比罗尔说，这一数字超过了此前七个国家的总和。"
	argsJSON, err := json.Marshal(map[string]string{"source": "text", "text": pasted})
	if err != nil {
		t.Fatalf("marshal set_material args: %v", err)
	}
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_material", string(argsJSON)),
		wsText("材料已经设成她贴的这段正文了。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": "这周读这段：" + pasted,
		"artifact": map[string]any{"kind": "reading", "title": "", "dueInput": ""},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("pasted text full of names and numbers = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["readingSource"] != "text" {
		t.Fatalf("patch[readingSource] = %v, want text", out.Patch["readingSource"])
	}
	if out.Patch["text"] != pasted {
		t.Fatalf("patch[text] = %v, want the pasted paragraph verbatim", out.Patch["text"])
	}
}

// TestWorkspaceTurnRejectsInventedPastedText — the "text" argument is not a
// substring of anything she typed this turn. The model wrote (or
// summarised) it, which 铁律① forbids for material the student ends up
// reading. The stub's SECOND script is a plain-text reply (not the same
// tool call again) so the turn ends on the model recovering from the tool
// error — with only one script, SequenceStubProvider replays the same
// tool call forever and the turn would fail on "工具调用次数超出上限"
// instead, which proves nothing about set_material's own rejection.
func TestWorkspaceTurnRejectsInventedPastedText(t *testing.T) {
	argsJSON, _ := json.Marshal(map[string]string{
		"source": "text", "text": "中国是全球最大的碳排放国，但也是可再生能源投资的领先者。",
	})
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_material", string(argsJSON)),
		wsText("这段不是您这一轮贴的原文，我没法把它设成材料。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "这周读一篇关于气候的报道"))
	if rec.Code != http.StatusOK {
		t.Fatalf("model recovering after a rejected tool call = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if _, wrote := out.Patch["text"]; wrote {
		t.Fatalf("invented pasted text reached the patch anyway: %v", out.Patch)
	}
	if len(prov.Requests) < 2 {
		t.Fatalf("model was called %d times, want at least 2 — the tool error has to reach it for a second call", len(prov.Requests))
	}
	var sawError bool
	for _, m := range prov.Requests[1].Messages {
		if m.Role == gateway.RoleTool && strings.Contains(m.Content, "正文必须来自老师贴进来的内容") {
			sawError = true
		}
	}
	if !sawError {
		t.Fatal("the substring-grounding tool error never reached the model's second call")
	}
}

// TestWorkspaceTurnAcceptsAPasteCutMidNumber — M-4: the §6 exemption for the
// patch's "text" field is not about names and numbers in general — typed
// (what she pasted this turn) already grounds those. It matters for a
// substring that starts mid-number: typed states 「1200人」, and the model's
// (real, contiguous) substring starts at 「200人」. Without the exemption,
// StatedCounts would read 200 as a head count nobody stated and nothing
// this turn grounds (typed's own count list has 1200, not 200) — a false
// rejection of content already proven honest by the substring check.
func TestWorkspaceTurnAcceptsAPasteCutMidNumber(t *testing.T) {
	typed := "这是今天的报道：根据最新统计，全校已有1200人参加了这项环保活动，反响非常热烈，大家都很兴奋。"
	cutMidNumber := "200人参加了这项环保活动，反响非常热烈，大家都很兴奋。"
	if !strings.Contains(typed, cutMidNumber) {
		t.Fatal("test setup: cutMidNumber must be a real substring of typed")
	}
	if n := len([]rune(cutMidNumber)); n < 20 { // liteWorkspaceMinTextRunes, unexported — kept as a literal here
		t.Fatalf("test setup: cutMidNumber is %d runes, want at least 20 so the length floor does not mask this test", n)
	}
	argsJSON, _ := json.Marshal(map[string]string{"source": "text", "text": cutMidNumber})
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_material", string(argsJSON)),
		wsText("材料已经设成她贴的这段正文了。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": typed,
		"artifact": map[string]any{"kind": "reading", "title": "", "dueInput": ""},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("a paste cut mid-number = %d, want 200 (the exemption covers it); body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["text"] != cutMidNumber {
		t.Fatalf("patch[text] = %v, want the substring verbatim", out.Patch["text"])
	}
}

// TestWorkspaceTurnSendsCurrentTextExactlyOnce — I-1: the client's `turns`
// is history only (the current turn travels as `text`), and the server must
// not append it a second time on top of `said`. A pasted article would
// otherwise double its own token count on every loop round.
func TestWorkspaceTurnSendsCurrentTextExactlyOnce(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	current := "这周读一读这篇报道，讲的是可再生能源投资增长的情况。"
	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": current,
		"artifact": map[string]any{"kind": "reading", "title": "", "dueInput": ""},
		"turns": []map[string]string{
			{"role": "teacher", "text": "上一轮说的话"},
			{"role": "ai", "text": "上一轮的回复"},
		},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if len(prov.Requests) == 0 {
		t.Fatal("the model was never called")
	}
	count := 0
	for _, m := range prov.Requests[0].Messages {
		if strings.Contains(m.Content, current) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the current turn's text appeared %d times in the model's messages, want exactly 1", count)
	}
}

// TestWorkspaceTurnTruncatesHistoryServerSide — threadLogic.ts already
// truncates a history turn before sending, but the server does not trust
// that it did: an over-long earlier turn sent as-is must still reach the
// model capped at liteworkspace.HistoryTextCapRunes runes, ending in 「…」.
func TestWorkspaceTurnTruncatesHistoryServerSide(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	long := strings.Repeat("气", liteworkspace.HistoryTextCapRunes+500)
	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": "继续",
		"artifact": map[string]any{"kind": "reading", "title": "", "dueInput": ""},
		"turns": []map[string]string{
			{"role": "teacher", "text": long},
			{"role": "ai", "text": "收到。"},
		},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if len(prov.Requests) == 0 {
		t.Fatal("the model was never called")
	}
	var historyMsg string
	for _, m := range prov.Requests[0].Messages {
		if m.Role == gateway.RoleUser && strings.HasPrefix(m.Content, "气") {
			historyMsg = m.Content
		}
	}
	if historyMsg == "" {
		t.Fatal("the long history turn never reached the model")
	}
	if got := len([]rune(historyMsg)); got > liteworkspace.HistoryTextCapRunes+1 { // +1 for 「…」
		t.Fatalf("history turn reached the model at %d runes, want capped at %d", got, liteworkspace.HistoryTextCapRunes)
	}
	if !strings.HasSuffix(historyMsg, "…") {
		t.Fatalf("a truncated history turn must end in 「…」, got %q", historyMsg)
	}
}
