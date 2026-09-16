package api_test

// lite_teacher_workspace_home_test.go — the home surface of the teacher
// workspace (§12.5, D2): class_snapshot, list_students, list_assignments,
// open_page and the navigate field open_page sets. The §6 checks, the loop
// bound and metering are shared with the assignment surface and are already
// held down in lite_teacher_workspace_test.go; these tests are about what is
// new here — the navigate contract and the two closed-set lookups open_page
// makes.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func homeTurnBody(classID, text string) string {
	b, _ := json.Marshal(map[string]any{"surface": "home", "classId": classID, "text": text})
	return string(b)
}

type homeNavigateJSON struct {
	View         string  `json:"view"`
	ClassID      string  `json:"classId"`
	UserID       *string `json:"userId"`
	AssignmentID *string `json:"assignmentId"`
	Label        string  `json:"label"`
}

type homeTurnJSON struct {
	Reply   string                `json:"reply"`
	Choices []liteworkspaceChoice `json:"choices"`
	Cards   []struct {
		Kind string          `json:"kind"`
		Rows json.RawMessage `json:"rows"`
	} `json:"cards"`
	Navigate *homeNavigateJSON `json:"navigate"`
}

// liteworkspaceChoice mirrors liteworkspace.Choice's wire shape without
// importing the package again here — decodeHomeTurn only reads id/label.
type liteworkspaceChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func decodeHomeTurn(t *testing.T, body []byte) homeTurnJSON {
	t.Helper()
	var out homeTurnJSON
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode home turn: %v — body=%s", err, body)
	}
	return out
}

// homeAssignmentBody is a minimal valid writing-assignment body: kind
// "writing" needs no reading-material fields, so it is the cheapest real
// assignment to seed for a list_assignments/open_page test.
func homeAssignmentBody(title string, userIDs []string) map[string]any {
	return map[string]any{
		"kind": "writing", "title": title, "instructions": "",
		"payload": map[string]any{"prompt": "写一篇随笔", "targetWords": 500, "lang": "zh"},
		"dueAt":   time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		"userIds": userIDs,
	}
}

// createHomeAssignment POSTs a writing assignment and returns its id.
func createHomeAssignment(t *testing.T, h http.Handler, teacher *http.Cookie, classID, title string, userIDs []string) string {
	t.Helper()
	var created struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		homeAssignmentBody(title, userIDs), &created); code != http.StatusCreated {
		t.Fatalf("create assignment %q: %d", title, code)
	}
	return created.Assignment.ID
}

// liteHomeFixtureIDs is liteTeacherFixtureWithProvider with the provider
// left for the caller to supply AFTER the ids exist: open_page's
// {target:"student"} and {target:"assignment"} tests need a REAL id inside
// the scripted tool call, and liteTeacherFixtureWithProvider creates the
// student only after the provider is already built. newHandler may be
// called more than once (with different providers) against the same pool —
// class creation and enrollment do not depend on which provider is bound.
// className is the class's name — a test asserting the class-name half of
// verbatimSpans needs to control it.
func liteHomeFixtureIDs(t *testing.T, className string) (pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID, newHandler func(gateway.Provider) http.Handler) {
	t.Helper()
	pool = newAPITestPool(t)
	newHandler = func(prov gateway.Provider) http.Handler {
		return New(Deps{
			Queries: sqlc.New(pool), Pool: pool, Provider: prov,
			ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
		}).Handler()
	}
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	bootstrap := newHandler(nil)
	teacher = signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-home-teacher@demo.local"))
	classID = createClassViaAPI(t, bootstrap, teacher, className)
	studentID = createStudent(t, pool, SeedSchoolID, "lt-home-student@demo.local")
	enrollStudent(t, pool, studentID, classID)
	return
}

// TestWorkspaceHomeRejectsOutsiders — the home surface uses the same
// ownership check as the assignment surface (liteWorkspaceSubjectFromClass).
func TestWorkspaceHomeRejectsOutsiders(t *testing.T) {
	h, pool, _, classID, studentID := liteTeacherFixtureWithProvider(t, writingTextStubProvider("这个班这周还不错。"))

	if rec := postWorkspaceTurn(t, h, nil, homeTurnBody(classID, "这个班怎么样")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out = %d, want 401; body=%s", rec.Code, rec.Body)
	}
	student := signInAs(t, pool, studentID)
	if rec := postWorkspaceTurn(t, h, student, homeTurnBody(classID, "这个班怎么样")); rec.Code != http.StatusForbidden {
		t.Fatalf("student = %d, want 403; body=%s", rec.Code, rec.Body)
	}
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ws-home-other@demo.local"))
	if rec := postWorkspaceTurn(t, h, other, homeTurnBody(classID, "这个班怎么样")); rec.Code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceHomeMetersOnDialogue — every workspace surface runs on the
// same dialogue tier and the same purpose; the home surface must not be an
// exception.
func TestWorkspaceHomeMetersOnDialogue(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("这个班这周还不错。"))
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这个班怎么样"))
	if rec.Code != http.StatusOK {
		t.Fatalf("home turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var purpose, tier, surface string
	err := pool.QueryRow(context.Background(),
		`SELECT purpose, tier, surface FROM llm_call ORDER BY created_at DESC LIMIT 1`,
	).Scan(&purpose, &tier, &surface)
	if err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if purpose != "lite_teacher_workspace" || tier != "chaperone" || surface != "lite" {
		t.Fatalf("llm_call purpose=%q tier=%q surface=%q", purpose, tier, surface)
	}
}

// TestWorkspaceHomeOpenPageInvalidTarget — a target outside the closed set is
// a tool error, not a failed turn: the model still has budget to recover.
func TestWorkspaceHomeOpenPageInvalidTarget(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("open_page", `{"target":"gradebook"}`),
		wsText("这个班还没有成绩册这个页面。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去成绩册"))
	if rec.Code != http.StatusOK {
		t.Fatalf("invalid target = %d, want 200 (a tool error, not a failed turn); body=%s", rec.Code, rec.Body)
	}
	out := decodeHomeTurn(t, rec.Body.Bytes())
	if out.Navigate != nil {
		t.Fatalf("navigate = %+v, want nil after a rejected target", out.Navigate)
	}
	if !toolResultContains(prov, "没有这个页面") {
		t.Fatalf("tool result did not name the closed-set error; messages=%+v", prov.LastRequest.Messages)
	}
}

// TestWorkspaceHomeOpenPageForeignUserID — a userId that is not on THIS
// roster (a random id: not one of ours anywhere) is a tool error.
func TestWorkspaceHomeOpenPageForeignUserID(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("open_page", `{"target":"student","userId":"`+uuid.New().String()+`"}`),
		wsText("这名学生不在这个班里。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看她的学习页"))
	if rec.Code != http.StatusOK {
		t.Fatalf("foreign userId = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeHomeTurn(t, rec.Body.Bytes())
	if out.Navigate != nil {
		t.Fatalf("navigate = %+v, want nil after a userId not on the roster", out.Navigate)
	}
	if !toolResultContains(prov, "这个学生不在班里") {
		t.Fatalf("tool result did not name the roster error; messages=%+v", prov.LastRequest.Messages)
	}
}

// TestWorkspaceHomeOpenPageForeignAssignmentID — an assignmentId that does
// not belong to this class (a random id: not one of ours anywhere) is a tool
// error.
func TestWorkspaceHomeOpenPageForeignAssignmentID(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("open_page", `{"target":"assignment","assignmentId":"`+uuid.New().String()+`"}`),
		wsText("这份作业不属于这个班。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看这份作业"))
	if rec.Code != http.StatusOK {
		t.Fatalf("foreign assignmentId = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeHomeTurn(t, rec.Body.Bytes())
	if out.Navigate != nil {
		t.Fatalf("navigate = %+v, want nil after an assignmentId not in this class", out.Navigate)
	}
	if !toolResultContains(prov, "这份作业不属于这个班") {
		t.Fatalf("tool result did not name the ownership error; messages=%+v", prov.LastRequest.Messages)
	}
}

// TestWorkspaceHomeOpenPageSetsNavigate — a valid open_page call for each of
// the five closed-set targets sets navigate with the fields and the Chinese
// label §12.5 item 6 specifies. The server performs no navigation: this is
// only ever the offer.
func TestWorkspaceHomeOpenPageSetsNavigate(t *testing.T) {
	t.Run("classWeekly", func(t *testing.T) {
		prov := gateway.NewSequenceStubProvider(
			wsToolCall("open_page", `{"target":"classWeekly"}`),
			wsText("这是本周报告的入口。"),
		)
		h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
		nav := requireNavigate(t, postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看本周报告")))
		if nav.View != "classWeekly" || nav.ClassID != classID || nav.Label != "本周报告" {
			t.Fatalf("navigate = %+v", nav)
		}
		if nav.UserID != nil || nav.AssignmentID != nil {
			t.Fatalf("navigate = %+v, want no userId/assignmentId", nav)
		}
	})

	t.Run("assignmentNew", func(t *testing.T) {
		prov := gateway.NewSequenceStubProvider(
			wsToolCall("open_page", `{"target":"assignmentNew"}`),
			wsText("这是布置作业的入口。"),
		)
		h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
		nav := requireNavigate(t, postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去布置作业")))
		if nav.View != "assignmentNew" || nav.ClassID != classID || nav.Label != "布置作业" {
			t.Fatalf("navigate = %+v", nav)
		}
	})

	t.Run("parentReports", func(t *testing.T) {
		prov := gateway.NewSequenceStubProvider(
			wsToolCall("open_page", `{"target":"parentReports"}`),
			wsText("这是家长报告的入口。"),
		)
		h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
		nav := requireNavigate(t, postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看家长报告")))
		if nav.View != "parentReports" || nav.ClassID != classID || nav.Label != "家长报告" {
			t.Fatalf("navigate = %+v", nav)
		}
	})

	t.Run("student", func(t *testing.T) {
		pool, teacher, classID, studentID, newHandler := liteHomeFixtureIDs(t, "Lite Home Class")
		renameLiteStudent(t, pool, studentID, "林知遥")
		prov := gateway.NewSequenceStubProvider(
			wsToolCall("open_page", `{"target":"student","userId":"`+studentID.String()+`"}`),
			wsText("这是林知遥的学习页。"),
		)
		h := newHandler(prov)
		nav := requireNavigate(t, postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看林知遥的学习页")))
		if nav.View != "student" || nav.ClassID != classID || nav.Label != "林知遥的学习页" {
			t.Fatalf("navigate = %+v", nav)
		}
		if nav.UserID == nil || *nav.UserID != studentID.String() {
			t.Fatalf("navigate.userId = %v, want %s", nav.UserID, studentID)
		}
	})

	t.Run("assignment", func(t *testing.T) {
		_, teacher, classID, studentID, newHandler := liteHomeFixtureIDs(t, "Lite Home Class")
		bootstrap := newHandler(nil)
		aid := createHomeAssignment(t, bootstrap, teacher, classID, "阅读理解练习", []string{studentID.String()})
		prov := gateway.NewSequenceStubProvider(
			wsToolCall("open_page", `{"target":"assignment","assignmentId":"`+aid+`"}`),
			wsText("这是那份作业的入口。"),
		)
		h := newHandler(prov)
		nav := requireNavigate(t, postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "带我去看那份作业")))
		// Label is 《标题》, not the bare title (round 2 fix #4): quoted so it
		// passes the count check's quote-anchored title blanking on its own.
		if nav.View != "assignment" || nav.ClassID != classID || nav.Label != "《阅读理解练习》" {
			t.Fatalf("navigate = %+v", nav)
		}
		if nav.AssignmentID == nil || *nav.AssignmentID != aid {
			t.Fatalf("navigate.assignmentId = %v, want %s", nav.AssignmentID, aid)
		}
	})
}

// TestWorkspaceAssignmentResponseHasNoNavigateKey — omitempty must keep the
// assignment surface's response byte-identical to before D2: no surface
// without an open_page tool should grow a navigate key.
func TestWorkspaceAssignmentResponseHasNoNavigateKey(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的，先定题目。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "帮我布置作业"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, has := raw["navigate"]; has {
		t.Fatalf("assignment response carries a navigate key: %s", rec.Body)
	}
}

// TestWorkspaceHomeListAssignmentsGroundedFromRealData — the count the reply
// repeats has to be backed by the real query classAssignmentSummaries runs
// (the same one listLiteAssignments' HTTP route uses), in Chinese, never the
// wire status or kind.
func TestWorkspaceHomeListAssignmentsGroundedFromRealData(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("这份作业还有 1 人没开始。"),
	)
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	createHomeAssignment(t, h, teacher, classID, "阅读理解练习", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这个班有什么作业"))
	if rec.Code != http.StatusOK {
		t.Fatalf("list_assignments turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeHomeTurn(t, rec.Body.Bytes())
	if len(out.Cards) != 1 || out.Cards[0].Kind != "assignments" {
		t.Fatalf("cards = %+v, want one assignments card", out.Cards)
	}
	toolMsg := toolResultMessage(prov)
	if !strings.Contains(toolMsg, `"未开始":1`) {
		t.Fatalf("tool result = %s, want a real 未开始:1 count", toolMsg)
	}
	if strings.Contains(toolMsg, "not_started") || strings.Contains(toolMsg, `"reading"`) || strings.Contains(toolMsg, `"writing"`) {
		t.Fatalf("tool result carried a wire value: %s", toolMsg)
	}
}

// TestWorkspaceHomeAssignmentTitleHeadCountShapeDoesNotFailTurn — an
// assignment title that happens to contain a 数字+人 shape ("3 人小组汇报")
// is catalogue/teacher data, not a model claim, but its digits still sit
// inside checked text once a tool hands the title back (list_assignments,
// or the navigate label from open_page{target:assignment}). Without
// grounding the title's own numbers, a legitimate turn that echoes the real
// title would fail §6's count check.
func TestWorkspaceHomeAssignmentTitleHeadCountShapeDoesNotFailTurn(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("「3 人小组汇报」这份作业还没有人开始。"),
	)
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	createHomeAssignment(t, h, teacher, classID, "3 人小组汇报", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "有什么作业还没交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a title with a head-count shape = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceHomeClassSnapshotNoWireValues — class_snapshot's tool result
// carries Chinese-labelled facts only; nothing a model could echo verbatim
// that a teacher has never seen on the assignment form.
func TestWorkspaceHomeClassSnapshotNoWireValues(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("class_snapshot", `{}`),
		wsText("这周挺活跃的。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这个班这周怎么样"))
	if rec.Code != http.StatusOK {
		t.Fatalf("class_snapshot turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	for _, m := range prov.LastRequest.Messages {
		if m.Role != gateway.RoleSystem && m.Role != gateway.RoleTool {
			continue
		}
		for _, wire := range []string{"reading", "writing", "not_started", "in_progress", "inactive_this_week"} {
			if strings.Contains(m.Content, wire) {
				t.Fatalf("message carried wire value %q: %s", wire, m.Content)
			}
		}
	}
}

// TestWorkspaceHomeUnrelatedHeadCountNearATitleFails — the reviewer's exact
// repro of the round-1 hole: grounding a title's OWN digits used to free
// that digit to justify ANY claim in the turn, not only a claim about the
// title itself. A 1-student class, an assignment titled 「3 人小组汇报」, the
// model calls list_assignments and then states an UNRELATED head count that
// shares the same digit — 「有 3 位学生逾期」, never grounded by any tool and
// never a substring the reply echoes verbatim — must still fail §6.
func TestWorkspaceHomeUnrelatedHeadCountNearATitleFails(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("有 3 位学生逾期。"),
	)
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	createHomeAssignment(t, h, teacher, classID, "3 人小组汇报", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "有什么作业还没交"))
	if rec.Code < 400 {
		t.Fatalf("an unrelated head count sharing a title's digit = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "3") {
		t.Fatalf("the error does not name the offending count: %s", rec.Body)
	}
}

// TestWorkspaceHomeTitlePrefixOfAnotherTitleBothBlanked — one assignment's
// title is a literal prefix of another's. Longest-span-first blanking must
// still remove both when the reply echoes both verbatim, regardless of which
// one physically contains the other's text.
func TestWorkspaceHomeTitlePrefixOfAnotherTitleBothBlanked(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("「5 人合作报告展示」和「5 人合作报告」都还没有学生开始。"),
	)
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	createHomeAssignment(t, h, teacher, classID, "5 人合作报告", []string{studentID.String()})
	createHomeAssignment(t, h, teacher, classID, "5 人合作报告展示", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "有什么作业还没交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("echoing a title and a title that contains it = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceHomeClassNameHeadCountShapeDoesNotFailTurn — the other half of
// verbatimSpans: a class name that itself reads as a head count (「高一（3）
// 班」-shaped — here 「三年级3人实验班」, which contains a literal 「3人」) must
// not fail a turn that names her own class.
func TestWorkspaceHomeClassNameHeadCountShapeDoesNotFailTurn(t *testing.T) {
	_, teacher, classID, _, newHandler := liteHomeFixtureIDs(t, "三年级3人实验班")
	prov := gateway.NewSequenceStubProvider(wsText("「三年级3人实验班」这周挺活跃的。"))
	h := newHandler(prov)

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这个班这周怎么样"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a class name with a head-count shape = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceHomeOpenPageLastCallWins — two open_page calls in one turn:
// documents and pins that the second call's navigate replaces the first's.
// Nothing in the surface deduplicates or refuses a second call; run.nav is a
// plain field the tool overwrites.
func TestWorkspaceHomeOpenPageLastCallWins(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("open_page", `{"target":"classWeekly"}`),
		wsToolCall("open_page", `{"target":"parentReports"}`),
		wsText("已经给你准备好家长报告的入口。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)

	nav := requireNavigate(t, postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "先看本周报告，等等改主意去看家长报告")))
	if nav.View != "parentReports" || nav.Label != "家长报告" {
		t.Fatalf("navigate = %+v, want the SECOND open_page call to win", nav)
	}
}

// TestWorkspaceHomeBlankingDoesNotWeakenNameCheck — the name check must keep
// reading the ORIGINAL, unblanked text. An assignment titled 「林知遥小组的
// 报告」 is a verbatim span (blanked before the COUNT check runs); 林知遥 is a
// real roster student's name that no tool returned this turn (list_assignments
// returns titles, never names), so naming her is still an ungrounded name —
// if blanking ever leaked into the name check, her name would vanish along
// with the span that contains it and this would wrongly pass.
func TestWorkspaceHomeBlankingDoesNotWeakenNameCheck(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("「林知遥小组的报告」这份作业还没有人开始。"),
	)
	h, pool, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	renameLiteStudent(t, pool, studentID, "林知遥")
	createHomeAssignment(t, h, teacher, classID, "林知遥小组的报告", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "有什么作业还没交"))
	if rec.Code < 400 {
		t.Fatalf("a real roster name inside a blanked title span = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "林知遥") {
		t.Fatalf("the error does not name the offending student: %s", rec.Body)
	}
}

// TestWorkspaceHomeUnquotedTitleDigitsStillFail — round 2's measured bypass:
// blanking ANY occurrence of a title (round 1's implementation) let a title
// exactly 「3人」 make that digit invisible EVERYWHERE in the turn, not only
// where the title itself was named. An UNQUOTED claim sharing the title's
// digits — 「3人没有交作业，请督促。」, which does not name the assignment at
// all — must still fail: only a QUOTED occurrence of a title is blanked.
func TestWorkspaceHomeUnquotedTitleDigitsStillFail(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("3人没有交作业，请督促。"),
	)
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	createHomeAssignment(t, h, teacher, classID, "3人", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "有什么作业还没交"))
	if rec.Code < 400 {
		t.Fatalf("an UNQUOTED claim sharing a title's exact digits = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "3") {
		t.Fatalf("the error does not name the offending count: %s", rec.Body)
	}
}

// TestWorkspaceHomeQuotedTitleDigitsPass — the other side of the same fix: a
// title exactly 「3人」, QUOTED, still passes — quoting is how the model names
// the title rather than stating an unrelated count, and the check has to
// tell the two apart.
func TestWorkspaceHomeQuotedTitleDigitsPass(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("「3人」这份作业还没有人交，请督促。"),
	)
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	createHomeAssignment(t, h, teacher, classID, "3人", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "有什么作业还没交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("a QUOTED title that is itself a count shape = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceHomeUnquotedTitleDoesNotFuseFragments — round 2's measured
// fusion case: an assignment titled 「汇报」 sits, UNQUOTED, between a digit
// and a counter word — 「3汇报人没交。」. Quote-anchored title blanking never
// touches an unquoted occurrence, so "汇报" is never removed and "3" and "人"
// never become adjacent; this must pass with no phantom count.
func TestWorkspaceHomeUnquotedTitleDoesNotFuseFragments(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("list_assignments", `{}`),
		wsText("3汇报人没交。"),
	)
	h, _, teacher, classID, studentID := liteTeacherFixtureWithProvider(t, prov)
	createHomeAssignment(t, h, teacher, classID, "汇报", []string{studentID.String()})

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "有什么作业还没交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("an unquoted title sitting between a digit and a counter word = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestWorkspaceHomeClassNameThatIsPurelyACountShapeStaysChecked — the guard
// on the class-name half of blanking: a class named EXACTLY 「3人」 (nothing
// else in the name) is not blanked at all, so a reply naming her own class
// still states an unrelated-looking head count and is still checked — the
// one case where blanking would have removed real information rather than a
// false alarm.
func TestWorkspaceHomeClassNameThatIsPurelyACountShapeStaysChecked(t *testing.T) {
	_, teacher, classID, _, newHandler := liteHomeFixtureIDs(t, "3人")
	prov := gateway.NewSequenceStubProvider(wsText("「3人」这周挺活跃的。"))
	h := newHandler(prov)

	rec := postWorkspaceTurn(t, h, teacher, homeTurnBody(classID, "这个班这周怎么样"))
	if rec.Code < 400 {
		t.Fatalf("a class name that IS a pure count shape = %d, want a failure; body=%s", rec.Code, rec.Body)
	}
}

// toolResultMessage returns the content of the last tool-role message the
// stub provider's final request carried.
func toolResultMessage(prov *gateway.SequenceStubProvider) string {
	var out string
	for _, m := range prov.LastRequest.Messages {
		if m.Role == gateway.RoleTool {
			out = m.Content
		}
	}
	return out
}

func toolResultContains(prov *gateway.SequenceStubProvider, substr string) bool {
	return strings.Contains(toolResultMessage(prov), substr)
}

// requireNavigate decodes rec as a home turn, fails the test if the status is
// not 200 or navigate is nil, and returns the navigate DTO.
func requireNavigate(t *testing.T, rec *httptest.ResponseRecorder) *homeNavigateJSON {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeHomeTurn(t, rec.Body.Bytes())
	if out.Navigate == nil {
		t.Fatalf("navigate = nil, want it set; body=%s", rec.Body)
	}
	return out.Navigate
}
