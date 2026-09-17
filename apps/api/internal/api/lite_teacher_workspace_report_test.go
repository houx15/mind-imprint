package api_test

// lite_teacher_workspace_report_test.go — the parent report surface of the
// teacher workspace (§5.3, §12.6, D3). The loop bound, metering and the §6
// checks are shared and held down in lite_teacher_workspace_test.go; these
// tests are about what is new here: who may revise which report, that a
// revised section passes the draft's own checks, that the tool never writes
// the database, and that a correct revision is not then failed by the shared
// head-count check.

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
	"mindimprint/api/internal/gateway"
)

func reportTurnBody(reportID, text string, artifact map[string]any) string {
	req := map[string]any{"surface": "parentReport", "reportId": reportID, "text": text}
	if artifact != nil {
		req["artifact"] = artifact
	}
	b, _ := json.Marshal(req)
	return string(b)
}

func reviseCall(section, text string) []gateway.StreamEvent {
	args, _ := json.Marshal(map[string]string{"section": section, "text": text})
	return wsToolCall("revise_section", string(args))
}

// toolResultOf is the tool result the i-th model request carried last: the
// answer to the tool call the model made in request i-1.
func toolResultOf(t *testing.T, prov *gateway.SequenceStubProvider, i int) string {
	t.Helper()
	if i >= len(prov.Requests) {
		t.Fatalf("only %d model requests, want request %d", len(prov.Requests), i)
	}
	msgs := prov.Requests[i].Messages
	last := msgs[len(msgs)-1]
	if last.Role != gateway.RoleTool {
		t.Fatalf("request %d ends with role %q, want a tool result", i, last.Role)
	}
	return last.Content
}

func workspaceCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'lite_teacher_workspace'`).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	return n
}

func storedReportBody(t *testing.T, pool *pgxpool.Pool, reportID string) string {
	t.Helper()
	var body string
	if err := pool.QueryRow(context.Background(),
		`SELECT coalesce(body::text, '') FROM lite_parent_report WHERE id = $1`, reportID).Scan(&body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return body
}

// reportWorkspaceFixture is parentFixture plus a classmate, 王小明, and a
// generated report (the provider's first script is the draft). Its sections
// are overview, reading and next.
func reportWorkspaceFixture(t *testing.T, scripts ...[]gateway.StreamEvent) (h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID, reportID string, prov *gateway.SequenceStubProvider) {
	t.Helper()
	prov = gateway.NewSequenceStubProvider(append([][]gateway.StreamEvent{weeklyReply(parentValidReply)}, scripts...)...)
	h, pool, teacher, classID, studentID = parentFixture(t, prov)
	wang := createStudent(t, pool, SeedSchoolID, "ws-report-wang@demo.local")
	enrollStudent(t, pool, wang, classID)
	renameLiteStudent(t, pool, wang, "王小明")
	reportID = generateParentReport(t, h, teacher, classID, studentID).Report.ID
	return
}

// TestWorkspaceReportNamedRequestIsNotAnsweredWithAQuestion — production
// 2026-09-17: 「请把总体概述写得更具体一些」 got 「您想怎么改写？」 and four
// options. A request that names a section and a change gets one rewrite when
// the model only asks; the rewrite revises the section.
func TestWorkspaceReportNamedRequestIsNotAnsweredWithAQuestion(t *testing.T) {
	askArgs, _ := json.Marshal(map[string]any{
		"question": "您想怎么改写「总体概述」？",
		"options": []map[string]string{
			{"id": "a", "label": "列出作品篇名"},
			{"id": "b", "label": "保留现有内容"},
		},
	})
	revised := "这段时间的学习记录显示，阅读和写作都有完成的作品。"
	h, _, teacher, _, _, reportID, prov := reportWorkspaceFixture(t,
		wsToolCall("ask_choice", string(askArgs)),
		reviseCall("overview", revised),
		wsText("已改写「总体概述」。"),
	)
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "请把总体概述写得更具体一些。", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	bodyPatch, _ := out.Patch["body"].(map[string]any)
	if bodyPatch["overview"] != revised {
		t.Fatalf("patch = %v, want the rewrite to revise the overview", out.Patch)
	}
	if got := lastUserMessage(t, prov, 2); !strings.Contains(got, "不要反问") {
		t.Fatalf("rewrite request = %q", got)
	}

	// A message that names no section may still be answered with options.
	h2, _, teacher2, _, _, reportID2, _ := reportWorkspaceFixture(t, wsToolCall("ask_choice", string(askArgs)))
	rec = postWorkspaceTurn(t, h2, teacher2, reportTurnBody(reportID2, "帮我改一下", nil))
	if rec.Code != http.StatusOK || len(decodeWorkspaceTurn(t, rec).Choices) != 2 {
		t.Fatalf("unnamed request = %d %s, want the options", rec.Code, rec.Body)
	}

	// The bounce-back also arrives as a plain question with no tool call
	// (production 2026-09-17).
	h3, _, teacher3, _, _, reportID3, prov3 := reportWorkspaceFixture(t,
		wsText("需要我这样改写「总体概述」吗，还是您有其他方向？"),
		reviseCall("overview", revised),
		wsText("已改写「总体概述」。"),
	)
	rec = postWorkspaceTurn(t, h3, teacher3, reportTurnBody(reportID3, "请把总体概述写得更具体一些。", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("plain question = %d %s", rec.Code, rec.Body)
	}
	if bodyPatch, _ := decodeWorkspaceTurn(t, rec).Patch["body"].(map[string]any); bodyPatch["overview"] != revised {
		t.Fatalf("plain question was not rewritten: %s", rec.Body)
	}
	if got := lastUserMessage(t, prov3, 2); !strings.Contains(got, "不要反问") {
		t.Fatalf("rewrite request = %q", got)
	}

	// A refusal with no question is an answer, not a bounce: it goes out.
	const refusal = "事实里没有修改作文的记录，这部分没有写进去。"
	h4, _, teacher4, _, _, reportID4, prov4 := reportWorkspaceFixture(t, wsText(refusal))
	rec = postWorkspaceTurn(t, h4, teacher4, reportTurnBody(reportID4, "请把总体概述写得更具体一些。", nil))
	if rec.Code != http.StatusOK || decodeWorkspaceTurn(t, rec).Reply != refusal {
		t.Fatalf("refusal = %d %s, want it unchanged", rec.Code, rec.Body)
	}
	if len(prov4.Requests) != 2 {
		t.Fatalf("refusal spent %d model calls, want 2 (draft + turn)", len(prov4.Requests))
	}
}

// TestWorkspaceReportRejectsOutsiders — ownership follows
// loadTeacherParentReport: another teacher, a malformed id and a missing
// report are all 404; a student is 403. None of them reaches the model.
func TestWorkspaceReportRejectsOutsiders(t *testing.T) {
	h, pool, teacher, _, studentID, reportID, _ := reportWorkspaceFixture(t, wsText("好的。"))

	student := signInAs(t, pool, studentID)
	if rec := postWorkspaceTurn(t, h, student, reportTurnBody(reportID, "改一下", nil)); rec.Code != http.StatusForbidden {
		t.Fatalf("student = %d, want 403; body=%s", rec.Code, rec.Body)
	}
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ws-report-other@demo.local"))
	if rec := postWorkspaceTurn(t, h, other, reportTurnBody(reportID, "改一下", nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	for _, id := range []string{"not-a-uuid", "", uuid.NewString()} {
		if rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(id, "改一下", nil)); rec.Code != http.StatusNotFound {
			t.Fatalf("reportId %q = %d, want 404; body=%s", id, rec.Code, rec.Body)
		}
	}
	if n := workspaceCalls(t, pool); n != 0 {
		t.Fatalf("workspace llm_call rows = %d, want 0", n)
	}
}

// TestWorkspaceReportRefusesAfterStudentLeft — the same refusal redraft
// gives, before any model call.
func TestWorkspaceReportRefusesAfterStudentLeft(t *testing.T) {
	h, pool, teacher, classID, studentID, reportID, _ := reportWorkspaceFixture(t, wsText("好的。"))
	mustExec(t, pool, `DELETE FROM enrollments WHERE user_id = $1 AND class_id = $2`, studentID, classID)

	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "改一下阅读", nil))
	wantParentError(t, "turn after she left", rec.Code, rec.Body.String(), http.StatusConflict, "student_left")
	if !strings.Contains(rec.Body.String(), "该学生已不在本班") {
		t.Fatalf("message = %s", rec.Body)
	}
	code, body := parentDo(t, h, teacher, "POST", parentReportPath(reportID)+"/redraft", `{"replaceBody":true}`, nil)
	wantParentError(t, "redraft after she left", code, body, http.StatusConflict, "student_left")
	if n := workspaceCalls(t, pool); n != 0 {
		t.Fatalf("workspace llm_call rows = %d, want 0", n)
	}
}

// TestWorkspaceReportReviseSectionRefusesWhatTheDraftWouldRefuse — a
// classmate's name, a digit not in the facts and a quote not in the facts are
// each a tool error, the patch stays empty, and the next call that gets it
// right is applied. The tool never writes the report row.
func TestWorkspaceReportReviseSectionRefusesWhatTheDraftWouldRefuse(t *testing.T) {
	good := "林知遥读完《城市里的雨水花园》，写下「雨水不是废水」。"
	h, pool, teacher, _, _, reportID, prov := reportWorkspaceFixture(t,
		reviseCall("reading", "她和王小明一起读完《城市里的雨水花园》。"),
		// 437 is no date, day count or total a default range can produce.
		reviseCall("reading", "这段时间读完 437 篇文章。"),
		reviseCall("reading", "她写下「雨水是宝贵的资源」。"),
		reviseCall("reading", good),
		wsText("已修改阅读段落。"),
	)
	before := storedReportBody(t, pool, reportID)

	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "把阅读那段写具体一点", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	// Request 0 is the draft; the workspace turn is requests 1..5.
	for i, want := range map[int]string{
		2: "写了其他学生的名字：王小明",
		3: "数字不在事实里：437",
		4: "引文不是事实里学生的原话：雨水是宝贵的资源",
	} {
		if got := toolResultOf(t, prov, i); !strings.Contains(got, `"ok":false`) || !strings.Contains(got, want) {
			t.Fatalf("tool result %d = %s, want an error naming %q", i, got, want)
		}
	}
	if got := toolResultOf(t, prov, 5); !strings.Contains(got, `"ok":true`) || !strings.Contains(got, "阅读") {
		t.Fatalf("tool result 5 = %s, want ok with the heading", got)
	}

	out := decodeWorkspaceTurn(t, rec)
	bodyPatch, _ := out.Patch["body"].(map[string]any)
	if len(out.Patch) != 1 || len(bodyPatch) != 1 || bodyPatch["reading"] != good {
		t.Fatalf("patch = %v, want only body.reading = %q", out.Patch, good)
	}
	if after := storedReportBody(t, pool, reportID); after != before {
		t.Fatalf("stored body changed: %s -> %s", before, after)
	}
	if n := workspaceCalls(t, pool); n != 5 {
		t.Fatalf("workspace llm_call rows = %d, want 5", n)
	}
}

// TestWorkspaceReportReviseSectionRefusesASectionOrLength — a section this
// report does not show and a text over 2000 characters are tool errors.
func TestWorkspaceReportReviseSectionRefusesASectionOrLength(t *testing.T) {
	h, _, teacher, _, _, reportID, prov := reportWorkspaceFixture(t,
		reviseCall("writing", "读完《城市里的雨水花园》。"),
		reviseCall("summary", "读完《城市里的雨水花园》。"),
		reviseCall("reading", strings.Repeat("读", 2001)),
		wsText("没有改动。"),
	)
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "改一下写作", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	for i, want := range map[int]string{
		2: "这份报告没有「写作」段落",
		3: "没有这个段落：summary",
		4: "超过 2000 字",
	} {
		if got := toolResultOf(t, prov, i); !strings.Contains(got, `"ok":false`) || !strings.Contains(got, want) {
			t.Fatalf("tool result %d = %s, want an error naming %q", i, got, want)
		}
	}
	if out := decodeWorkspaceTurn(t, rec); len(out.Patch) != 0 {
		t.Fatalf("patch = %v, want empty", out.Patch)
	}
}

// TestWorkspaceReportGoodRevisionPassesEndToEnd — a realistic revision whose
// title and quote carry a head-count shape (3人), naming her and quoting a
// fragment of her words, passes the draft's checks AND the shared §6 checks.
// The section argument may be the Chinese heading.
func TestWorkspaceReportGoodRevisionPassesEndToEnd(t *testing.T) {
	const reply = `{"overview":"这段时间读完《3人小组的雨水花园》。","reading":"读完《3人小组的雨水花园》。","next":"请和她聊一聊雨水花园。"}`
	revised := "林知遥读完《3人小组的雨水花园》，写下「我们3人一组种下雨水花园」。\n她在文中提到「3人一组」的分工。"
	prov := gateway.NewSequenceStubProvider(
		weeklyReply(reply),
		reviseCall("阅读", revised),
		wsText("已修改阅读段落，引用了《3人小组的雨水花园》里林知遥写下的「3人一组」。"),
	)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	backdateWeeklyStart(t, pool, classID)
	renameLiteStudent(t, pool, studentID, "林知遥")
	setFemale(t, pool, studentID)
	reading := seedParentReading(t, pool, studentID, false)
	mustExec(t, pool, `UPDATE reading SET title = '3人小组的雨水花园' WHERE atom_id = $1`, reading)
	seedReport(t, pool, reading, "reading", `{"version":1,"moments":[{"quote":"我们3人一组种下雨水花园","where":""}]}`)
	wang := createStudent(t, pool, SeedSchoolID, "ws-report-good-wang@demo.local")
	enrollStudent(t, pool, wang, classID)
	renameLiteStudent(t, pool, wang, "王小明")
	gen := generateParentReport(t, h, teacher, classID, studentID)
	if gen.DraftError != nil {
		t.Fatalf("draft rejected: %s", *gen.DraftError)
	}

	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(gen.Report.ID, "阅读那段请引用她的原话", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	bodyPatch, _ := out.Patch["body"].(map[string]any)
	if bodyPatch["reading"] != revised {
		t.Fatalf("patch = %v, want body.reading = %q", out.Patch, revised)
	}
}

// TestWorkspaceReportRevisionWithAnUngroundedHeadCountFails — the shared
// check still reads a revised section. 28 is a fact (the default range is 28
// days), so the draft's own digit check lets 「28 人」 through; the head-count
// check does not, because no fact counts people.
func TestWorkspaceReportRevisionWithAnUngroundedHeadCountFails(t *testing.T) {
	h, _, teacher, _, _, reportID, _ := reportWorkspaceFixture(t,
		reviseCall("overview", "这 28 天里班上 28 人读完《城市里的雨水花园》。"),
		wsText("已修改总体概述。"),
	)
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "改一下总体概述", nil))
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "没有依据的人数：28") {
		t.Fatalf("turn = %d %s, want 502 naming the count 28", rec.Code, rec.Body)
	}
}

// TestWorkspaceReportCanvasShowsHeadingsTextAndVisibleFacts — the model reads
// each visible section under its Chinese heading with the text the editor
// shows (the artifact's, over the stored one), then the visible facts. No
// section key is in the prompt, and a hidden quote is neither shown nor
// accepted.
func TestWorkspaceReportCanvasShowsHeadingsTextAndVisibleFacts(t *testing.T) {
	h, _, teacher, _, _, reportID, prov := reportWorkspaceFixture(t,
		reviseCall("reading", "她写下「雨水不是废水」。"),
		wsText("那句原话已隐藏，不能引用。"),
	)
	if code, body := parentDo(t, h, teacher, "PATCH", parentReportPath(reportID), `{"hidden":{"moments":["雨水不是废水"],"keywords":[]}}`, nil); code != http.StatusOK {
		t.Fatalf("PATCH hidden = %d %s", code, body)
	}
	// The stored overview still quotes the hidden moment; the artifact's
	// overview (what the editor shows now) does not, so the only place the
	// quote could reach the prompt from is the facts.
	artifact := map[string]any{"body": map[string]string{
		"overview": "这段时间读完《城市里的雨水花园》。",
		"next":     "老师正在改的下一步建议。",
		"writing":  "不显示的段落",
	}}
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "阅读那段引用她的原话", artifact))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	system := prov.Requests[1].Messages[0].Content
	for _, want := range []string{
		"### 总体概述\n", "### 阅读\n读完《城市里的雨水花园》。", "### 下一步建议\n老师正在改的下一步建议。",
		"## 可用的事实", "完成阅读《城市里的雨水花园》", "林知遥",
	} {
		if !strings.Contains(system, want) {
			t.Fatalf("system prompt lacks %q:\n%s", want, system)
		}
	}
	for _, banned := range []string{"### 写作", "不显示的段落", "雨水不是废水", "overview", "reading", "next"} {
		if strings.Contains(system, banned) {
			t.Fatalf("system prompt contains %q:\n%s", banned, system)
		}
	}
	if got := toolResultOf(t, prov, 2); !strings.Contains(got, "引文不是事实里学生的原话：雨水不是废水") {
		t.Fatalf("tool result = %s, want the hidden quote refused", got)
	}
	if out := decodeWorkspaceTurn(t, rec); len(out.Patch) != 0 {
		t.Fatalf("patch = %v, want empty", out.Patch)
	}
}

// namedReportFixture: a student named herName with one finished reading
// titled title (moment 「雨水不是废水」), the given classmates, and a generated
// report. The provider's first script is the draft.
func namedReportFixture(t *testing.T, herName, title string, classmates []string, scripts ...[]gateway.StreamEvent) (h http.Handler, teacher *http.Cookie, reportID string) {
	t.Helper()
	draft, _ := json.Marshal(map[string]string{
		"overview": "读完《" + title + "》，写下「雨水不是废水」。",
		"reading":  "读完《" + title + "》。",
		"next":     "请和她聊一聊雨水花园。",
	})
	prov := gateway.NewSequenceStubProvider(append([][]gateway.StreamEvent{weeklyReply(string(draft))}, scripts...)...)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	backdateWeeklyStart(t, pool, classID)
	renameLiteStudent(t, pool, studentID, herName)
	setFemale(t, pool, studentID)
	reading := seedParentReading(t, pool, studentID, true)
	mustExec(t, pool, `UPDATE reading SET title = $2 WHERE atom_id = $1`, reading, title)
	for i, name := range classmates {
		mate := createStudent(t, pool, SeedSchoolID, fmt.Sprintf("ws-report-mate-%d@demo.local", i))
		enrollStudent(t, pool, mate, classID)
		renameLiteStudent(t, pool, mate, name)
	}
	gen := generateParentReport(t, h, teacher, classID, studentID)
	if gen.DraftError != nil {
		t.Fatalf("draft rejected: %s", *gen.DraftError)
	}
	return h, teacher, gen.Report.ID
}

func wantTurn(t *testing.T, what string, rec *httptest.ResponseRecorder, status int, contains string) {
	t.Helper()
	if rec.Code != status || !strings.Contains(rec.Body.String(), contains) {
		t.Fatalf("%s = %d %s, want %d containing %q", what, rec.Code, rec.Body, status, contains)
	}
}

// TestWorkspaceReportQuotedTitleNamingAClassmate — her title names a
// classmate. The quoted title is a reference to her work, in the reply and in
// a section; an unquoted claim about the classmate is not, and fails.
func TestWorkspaceReportQuotedTitleNamingAClassmate(t *testing.T) {
	const title = "李明推荐的雨水花园"
	h, teacher, reportID := namedReportFixture(t, "林知遥", title, []string{"李明"},
		wsText("李明这周没有交作业。"),
		wsText("已修改《"+title+"》相关的段落。"),
		reviseCall("reading", "读完《"+title+"》。"),
		wsText("已修改阅读段落。"),
	)
	wantTurn(t, "unquoted claim about 李明",
		postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看一下", nil)),
		http.StatusBadGateway, "没有依据的学生姓名：李明")
	wantTurn(t, "quoted title in the reply",
		postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "改一下", nil)), http.StatusOK, "")
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "改一下阅读", nil))
	wantTurn(t, "quoted title in a section", rec, http.StatusOK, "")
	if body, _ := decodeWorkspaceTurn(t, rec).Patch["body"].(map[string]any); body["reading"] != "读完《"+title+"》。" {
		t.Fatalf("patch = %v, want the reading section", body)
	}
}

// TestWorkspaceReportTitleThatIsAClassmatesName — a title that is exactly a
// classmate's name is not blanked for the name check, so a quoted title
// followed by a claim about her still fails.
func TestWorkspaceReportTitleThatIsAClassmatesName(t *testing.T) {
	h, teacher, reportID := namedReportFixture(t, "林知遥", "李明", []string{"李明"},
		wsText("「李明」还没有交作业。"),
	)
	wantTurn(t, "quoted title that is a classmate's name",
		postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看一下", nil)),
		http.StatusBadGateway, "没有依据的学生姓名：李明")
}

// TestWorkspaceReportHerNameContainsAClassmates —she is 王丽华, a classmate
// is 王丽. Naming her is not naming 王丽; naming 王丽 still fails; a section
// naming her applies.
func TestWorkspaceReportHerNameContainsAClassmates(t *testing.T) {
	revised := "王丽华读完《城市里的雨水花园》。"
	h, teacher, reportID := namedReportFixture(t, "王丽华", "城市里的雨水花园", []string{"王丽"},
		wsText("已看过王丽华的报告。"),
		wsText("王丽没有交作业。"),
		reviseCall("reading", revised),
		wsText("已修改王丽华的阅读段落。"),
	)
	wantTurn(t, "naming her", postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看一下", nil)), http.StatusOK, "")
	wantTurn(t, "naming 王丽", postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看一下", nil)),
		http.StatusBadGateway, "没有依据的学生姓名：王丽")
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "改一下阅读", nil))
	wantTurn(t, "section naming her", rec, http.StatusOK, "")
	if body, _ := decodeWorkspaceTurn(t, rec).Patch["body"].(map[string]any); body["reading"] != revised {
		t.Fatalf("patch = %v, want the reading section", body)
	}
}

// TestWorkspaceReportClassmateNameContainsHers — she is 王丽, a classmate is
// 王丽华. Blanking her name must not hide the classmate.
func TestWorkspaceReportClassmateNameContainsHers(t *testing.T) {
	h, teacher, reportID := namedReportFixture(t, "王丽", "城市里的雨水花园", []string{"王丽华"},
		wsText("王丽华没有交作业。"),
		wsText("王丽读完了《城市里的雨水花园》。"),
	)
	wantTurn(t, "naming 王丽华", postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看一下", nil)),
		http.StatusBadGateway, "没有依据的学生姓名：王丽华")
	wantTurn(t, "naming her", postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看一下", nil)), http.StatusOK, "")
}

// TestWorkspaceReportToolResultsCarryNoKeysOrEnglish — every error
// revise_section can return reaches the model without a section key or the
// checker's English text (§12.1).
func TestWorkspaceReportToolResultsCarryNoKeysOrEnglish(t *testing.T) {
	calls := [][]gateway.StreamEvent{
		reviseCall("summary", "读完《城市里的雨水花园》。"), // unknown section
		reviseCall("writing", "读完《城市里的雨水花园》。"), // section this report does not show
		reviseCall("reading", ""),                        // empty
		reviseCall("reading", strings.Repeat("读", 2001)), // too long
		reviseCall("reading", "读完三篇文章。"),                 // Chinese numeral count
		reviseCall("overview", "读完城市里的雨水花园》。"),           // unmatched closing mark
		reviseCall("overview", "读完《城市里的雨水花园。"),           // unclosed quote
		reviseCall("overview", "她写下「雨水是资源」。"),            // quote not in corpus
		reviseCall("next", "请读《不存在的书》。"),                 // title not in titles
		reviseCall("next", "请读 437 篇。"),                  // digit not in facts
		reviseCall("interests", "读完《城市里的雨水花园》。"),         // section this report does not show
		reviseCall("reading", "她和王小明一起读完《城市里的雨水花园》。"),    // other student
	}
	// Three turns of at most five tool calls each, each closed by a reply.
	var scripts [][]gateway.StreamEvent
	for i, c := range calls {
		scripts = append(scripts, c)
		if i%5 == 4 || i == len(calls)-1 {
			scripts = append(scripts, wsText("请说明要改哪一段。"))
		}
	}
	h, _, teacher, _, _, reportID, prov := reportWorkspaceFixture(t, scripts...)
	for range 3 {
		if rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "改一下", nil)); rec.Code != http.StatusOK {
			t.Fatalf("turn = %d %s", rec.Code, rec.Body)
		}
	}

	seen := map[string]bool{}
	for _, req := range prov.Requests {
		for _, m := range req.Messages {
			if m.Role == gateway.RoleTool {
				seen[m.Content] = true
			}
		}
	}
	if len(seen) != len(calls) {
		t.Fatalf("distinct tool results = %d, want %d: %v", len(seen), len(calls), seen)
	}
	banned := []string{
		"overview", "reading", "writing", "projects", "interests", "next",
		"chinese numeral", "closing mark", "unclosed", "not in corpus", "not in titles",
		"not in facts", "mentions other",
	}
	wants := []string{"三篇", "雨水是资源", "不存在的书", "437", "王小明", "「写作」", "「兴趣」", "summary"}
	var all strings.Builder
	for got := range seen {
		all.WriteString(got)
		if !strings.Contains(got, `"ok":false`) {
			t.Fatalf("tool result = %s, want an error", got)
		}
		for _, b := range banned {
			if strings.Contains(got, b) {
				t.Fatalf("tool result contains %q: %s", b, got)
			}
		}
	}
	for _, w := range wants {
		if !strings.Contains(all.String(), w) {
			t.Fatalf("no tool result keeps %q: %s", w, all.String())
		}
	}
}

// TestWorkspaceReportDoesNotGroundTheClassSize — the report prompt never
// states the class size, so a class size in the reply fails.
func TestWorkspaceReportDoesNotGroundTheClassSize(t *testing.T) {
	h, pool, teacher, classID, _, reportID, _ := reportWorkspaceFixture(t, wsText("全班2人里她最先读完。"))
	var students int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM enrollments WHERE class_id = $1 AND role_in_class = 'student'`, classID).Scan(&students); err != nil {
		t.Fatal(err)
	}
	if students != 2 {
		t.Fatalf("roster = %d, want 2", students)
	}
	wantTurn(t, "class size in the reply",
		postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "她读得怎么样", nil)),
		http.StatusBadGateway, "没有依据的人数：2")
}

// setFemale sets a fixture student's gender: the scripted drafts call her 她,
// and a pronoun the teacher did not set fails the draft.
func setFemale(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()
	mustExec(t, pool, `UPDATE users SET gender = 'female' WHERE id = $1`, userID)
}
