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
	"net/http"
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
		2: "mentions other student: 王小明",
		3: "digit not in facts: 437",
		4: "quote not in corpus: 雨水是宝贵的资源",
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
		2: "这份报告没有这个段落：writing",
		3: "这份报告没有这个段落：summary",
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
	if got := toolResultOf(t, prov, 2); !strings.Contains(got, "quote not in corpus: 雨水不是废水") {
		t.Fatalf("tool result = %s, want the hidden quote refused", got)
	}
	if out := decodeWorkspaceTurn(t, rec); len(out.Patch) != 0 {
		t.Fatalf("patch = %v, want empty", out.Patch)
	}
}
