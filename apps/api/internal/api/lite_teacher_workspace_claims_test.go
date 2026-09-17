package api_test

// lite_teacher_workspace_claims_test.go — three production findings from
// 2026-09-17, each held down where a prompt line alone did not hold it:
//
//   - the class chat said a page was opened when open_page only offered a
//     button, and the report chat offered to add a section no tool can add
//     (falseClaim: one rewrite, then the turn fails);
//   - the AI wrote 他/她 with no gender on record (the teacher now sets it;
//     every prompt that names a student carries it);
//   - a rejected reply was logged with the students' names in it.

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

func genderPath(classID, userID string) string {
	return "/api/v1/lite/teacher/classes/" + classID + "/students/" + userID + "/gender"
}

// lastUserMessage is the last user-role message of the i-th model request.
func lastUserMessage(t *testing.T, prov *gateway.SequenceStubProvider, i int) string {
	t.Helper()
	if i >= len(prov.Requests) {
		t.Fatalf("only %d model requests, want request %d", len(prov.Requests), i)
	}
	msgs := prov.Requests[i].Messages
	for j := len(msgs) - 1; j >= 0; j-- {
		if msgs[j].Role == gateway.RoleUser {
			return msgs[j].Content
		}
	}
	return ""
}

func TestWorkspaceHomeOpenedPageClaimIsRewritten(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("open_page", `{"target":"classWeekly"}`),
		wsText("已经打开了本周报告页面。"),
		wsText("请点击下方按钮前往本周报告。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看本周报告"))
	nav := requireNavigate(t, rec)
	if nav.View != "classWeekly" {
		t.Fatalf("navigate = %+v", nav)
	}
	if got := decodeHomeTurn(t, rec.Body.Bytes()).Reply; got != "请点击下方按钮前往本周报告。" {
		t.Fatalf("reply = %q, want the rewrite", got)
	}
	if len(prov.Requests) != 3 {
		t.Fatalf("model requests = %d, want 3", len(prov.Requests))
	}
	if got := lastUserMessage(t, prov, 2); !strings.Contains(got, "没有通过检查") || !strings.Contains(got, "页面已经打开") {
		t.Fatalf("rewrite request = %q", got)
	}
}

func TestWorkspaceHomeOpenedPageClaimTwiceFails(t *testing.T) {
	askArgs, _ := json.Marshal(map[string]any{
		"question": "接下来看什么？",
		"options": []map[string]string{
			{"id": "a", "label": "已为您跳转到本周报告"},
			{"id": "b", "label": "看家长报告"},
		},
	})
	prov := gateway.NewSequenceStubProvider(
		wsText("已经打开了本周报告页面。"),
		wsToolCall("ask_choice", string(askArgs)),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看本周报告"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("turn = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "页面已经打开") {
		t.Fatalf("502 body does not give the reason: %s", rec.Body)
	}
}

// A reply that points at a button with none under it is rewritten; the
// rewrite here calls open_page, so the button is there.
func TestWorkspaceHomeButtonWithoutOpenPageIsRewritten(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsText("请点击下方按钮前往本周报告。"),
		wsToolCall("open_page", `{"target":"classWeekly"}`),
		wsText("请点击下方按钮前往本周报告。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	nav := requireNavigate(t, postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看本周报告")))
	if nav.View != "classWeekly" {
		t.Fatalf("navigate = %+v", nav)
	}
	if got := lastUserMessage(t, prov, 1); !strings.Contains(got, "没有调用 open_page") {
		t.Fatalf("rewrite request = %q", got)
	}
}

// An offer is not a claim: 「要打开本周报告吗」 goes out as it is.
func TestWorkspaceHomeOfferToOpenIsNotAClaim(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("要打开本周报告吗？"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这周怎么样"))
	if rec.Code != http.StatusOK || len(prov.Requests) != 1 {
		t.Fatalf("turn = %d after %d requests; body=%s", rec.Code, len(prov.Requests), rec.Body)
	}
}

// An option offering to remind a student (live, 2026-09-17: 「提醒该生开始学习」)
// is rewritten: no class-chat tool reaches a student.
// Live, 2026-09-17: asked who had not started, the chat listed the students
// on the canvas and replied only 「接下来想看什么？」.
func TestWorkspaceHomeBareQuestionAfterDataIsRewritten(t *testing.T) {
	ask := func(question string) string {
		b, _ := json.Marshal(map[string]any{
			"question": question,
			"options":  []map[string]string{{"id": "a", "label": "看看全班本周概况"}, {"id": "b", "label": "不用了"}},
		})
		return string(b)
	}
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_students", `{"filter":"inactive_this_week"}`),
		wsToolCall("ask_choice", ask("接下来想看什么？")),
		wsToolCall("ask_choice", ask("名单上的学生本周还没有开始学习。接下来想看什么？")),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这周谁还没开始学习？"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if got := decodeHomeTurn(t, rec.Body.Bytes()).Reply; !strings.HasPrefix(got, "名单上的学生本周还没有开始学习。") {
		t.Fatalf("reply = %q, want the rewrite with an answer first", got)
	}
	if got := lastUserMessage(t, prov, 2); !strings.Contains(got, "回复只有一个追问") {
		t.Fatalf("rewrite request = %q", got)
	}
}

// ask_choice's answer field is the reply's first sentence.
func TestWorkspaceHomeAskChoiceAnswerComesFirst(t *testing.T) {
	b, _ := json.Marshal(map[string]any{
		"answer":   "名单上的学生本周还没有开始学习。",
		"question": "接下来想看什么？",
		"options":  []map[string]string{{"id": "a", "label": "看看全班本周概况"}, {"id": "b", "label": "不用了"}},
	})
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_students", `{"filter":"inactive_this_week"}`),
		wsToolCall("ask_choice", string(b)),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这周谁还没开始学习？"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if got := decodeHomeTurn(t, rec.Body.Bytes()).Reply; got != "名单上的学生本周还没有开始学习。\n\n接下来想看什么？" {
		t.Fatalf("reply = %q", got)
	}
	if len(prov.Requests) != 2 {
		t.Fatalf("model requests = %d, want no rewrite", len(prov.Requests))
	}
}

func TestWorkspaceHomeReminderOfferIsRewritten(t *testing.T) {
	ask := func(label string) string {
		b, _ := json.Marshal(map[string]any{
			"question": "接下来您想怎么做？",
			"options": []map[string]string{
				{"id": "a", "label": "看看全班本周概况"},
				{"id": "b", "label": label},
			},
		})
		return string(b)
	}
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("ask_choice", ask("提醒该生开始学习")),
		wsToolCall("ask_choice", ask("给这些学生布置作业")),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这周谁还没开始学习？"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	out := decodeHomeTurn(t, rec.Body.Bytes())
	for _, c := range out.Choices {
		if strings.Contains(c.Label, "提醒") {
			t.Fatalf("choices = %+v, the reminder offer went out", out.Choices)
		}
	}
	if got := lastUserMessage(t, prov, 1); !strings.Contains(got, "没有工具能给学生发消息") {
		t.Fatalf("rewrite request = %q", got)
	}
}

// The fixture report has overview, reading and next. 写作 and 兴趣 are not
// sections of it, and no tool adds one.
func TestWorkspaceReportSectionClaimsAreRewritten(t *testing.T) {
	bad, _ := json.Marshal(map[string]any{
		"question": "要怎么改？",
		"options": []map[string]string{
			{"id": "a", "label": "新增一个『写作』段落"},
			{"id": "b", "label": "改写「下一步建议」"},
		},
	})
	good, _ := json.Marshal(map[string]any{
		"question": "要改哪一段？",
		"options": []map[string]string{
			{"id": "a", "label": "改写「阅读」"},
			{"id": "b", "label": "改写「下一步建议」"},
		},
	})
	h, _, teacher, _, _, reportID, prov := reportWorkspaceFixture(t,
		wsToolCall("ask_choice", string(bad)),
		wsToolCall("ask_choice", string(good)),
	)
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "帮我改一改", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if len(out.Choices) != 2 || out.Choices[0].Label != "改写「阅读」" {
		t.Fatalf("choices = %+v, want the rewrite's", out.Choices)
	}
	got := lastUserMessage(t, prov, 2)
	if !strings.Contains(got, "不能新增或删除段落") || !strings.Contains(got, "「总体概述」、「阅读」、「下一步建议」") {
		t.Fatalf("rewrite request = %q", got)
	}
}

func TestWorkspaceReportMissingSectionTwiceFails(t *testing.T) {
	h, _, teacher, _, _, reportID, _ := reportWorkspaceFixture(t,
		wsText("我可以把兴趣部分写得更具体。"),
		wsText("我可以改写「兴趣」。"),
	)
	rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "还能改什么", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("turn = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "这份报告没有「兴趣」段落") {
		t.Fatalf("502 body does not give the reason: %s", rec.Body)
	}
}

// The report prompt names only this report's sections and her pronoun as the
// teacher set it, read at turn time.
func TestWorkspaceReportPromptCarriesSectionsAndPronoun(t *testing.T) {
	h, _, teacher, classID, studentID, reportID, prov := reportWorkspaceFixture(t,
		wsText("好的。"),
		wsText("好的。"),
	)
	if rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看看", nil)); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	system := prov.Requests[1].Messages[0].Content
	for _, want := range []string{"只有这几段：「总体概述」、「阅读」、「下一步建议」", "林知遥（称谓：她）"} {
		if !strings.Contains(system, want) {
			t.Fatalf("system prompt lacks %q:\n%s", want, system)
		}
	}
	if strings.Contains(system, "如「阅读」") {
		t.Fatalf("system prompt still names a section as an example:\n%s", system)
	}

	if code, body := parentDo(t, h, teacher, "PUT", genderPath(classID, studentID.String()), `{"gender":"male"}`, nil); code != http.StatusOK {
		t.Fatalf("PUT gender = %d %s", code, body)
	}
	if rec := postWorkspaceTurn(t, h, teacher, reportTurnBody(reportID, "看看", nil)); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if system := prov.Requests[2].Messages[0].Content; !strings.Contains(system, "林知遥（称谓：他）") {
		t.Fatalf("system prompt lacks the set pronoun:\n%s", system)
	}
}

func TestLiteStudentGenderSetting(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_students", `{"filter":"all"}`),
		wsText("名单在卡片上。"),
	)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	path := genderPath(classID, studentID.String())

	rosterGender := func() string {
		t.Helper()
		var out struct {
			Roster []struct {
				ID     string `json:"id"`
				Gender string `json:"gender"`
			} `json:"roster"`
		}
		if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &out); code != http.StatusOK {
			t.Fatalf("roster = %d", code)
		}
		if len(out.Roster) != 1 {
			t.Fatalf("roster = %+v", out.Roster)
		}
		return out.Roster[0].Gender
	}

	if got := rosterGender(); got != "" {
		t.Fatalf("gender before any setting = %q, want empty", got)
	}
	if code, body := parentDo(t, h, teacher, "PUT", path, `{"gender":"male"}`, nil); code != http.StatusOK {
		t.Fatalf("PUT male = %d %s", code, body)
	}
	if got := rosterGender(); got != "male" {
		t.Fatalf("gender = %q, want male", got)
	}

	// list_students hands the model the pronoun next to the name.
	if rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "列出全班")); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if got := toolResultOf(t, prov, 1); !strings.Contains(got, `"称谓":"他"`) {
		t.Fatalf("list_students result = %s, want the pronoun", got)
	}
	if system := prov.Requests[0].Messages[0].Content; !strings.Contains(system, "代词按「称谓」写") {
		t.Fatalf("home prompt lacks the pronoun rule:\n%s", system)
	}

	for _, bad := range []string{`{"gender":"other"}`, `{}`, `not json`} {
		if code, body := parentDo(t, h, teacher, "PUT", path, bad, nil); code != http.StatusBadRequest {
			t.Fatalf("PUT %s = %d %s, want 400", bad, code, body)
		}
	}
	if code, body := parentDo(t, h, teacher, "PUT", path, `{"gender":""}`, nil); code != http.StatusOK {
		t.Fatalf("PUT clear = %d %s", code, body)
	}
	if got := rosterGender(); got != "" {
		t.Fatalf("gender after clearing = %q, want empty", got)
	}

	// Another teacher, and a student who is not in the class, are 404.
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-gender-other@demo.local"))
	if code, _ := parentDo(t, h, other, "PUT", path, `{"gender":"female"}`, nil); code != http.StatusNotFound {
		t.Fatalf("other teacher PUT = %d, want 404", code)
	}
	stranger := createStudent(t, pool, SeedSchoolID, "lt-gender-stranger@demo.local")
	if code, _ := parentDo(t, h, teacher, "PUT", genderPath(classID, stranger.String()), `{"gender":"female"}`, nil); code != http.StatusNotFound {
		t.Fatalf("PUT for a student outside the class = %d, want 404", code)
	}
}

// A reply that fails the name check is logged without the names in it.
func TestWorkspaceGroundingFailureLogHasNoStudentNames(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	prov := gateway.NewSequenceStubProvider(wsText("欧阳明月这周还没有开始学习。"))
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "欧阳明月")
	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这周怎么样"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("turn = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	logged := buf.String()
	if !strings.Contains(logged, "reply failed the grounding check") {
		t.Fatalf("no grounding log line:\n%s", logged)
	}
	if strings.Contains(logged, "欧阳明月") {
		t.Fatalf("log line names the student:\n%s", logged)
	}
	if !strings.Contains(logged, "[学生]这周还没有开始学习") {
		t.Fatalf("log line lost the text:\n%s", logged)
	}
}
