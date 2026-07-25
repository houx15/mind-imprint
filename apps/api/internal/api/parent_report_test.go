package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// parentReportForTest mirrors ParentReportDTO's JSON shape for decoding.
type parentReportForTest struct {
	Cover struct {
		Name      string `json:"name"`
		Subject   string `json:"subject"`
		Klass     string `json:"klass"`
		TypeLabel string `json:"typeLabel"`
		DateStr   string `json:"dateStr"`
		WarmLine  string `json:"warmLine"`
	} `json:"cover"`
	DRows []struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Badge   string `json:"badge"`
		Reading string `json:"reading"`
	} `json:"dRows"`
	ARows []struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		State   string `json:"state"`
		Reading string `json:"reading"`
	} `json:"aRows"`
	ProseReady bool    `json:"proseReady"`
	Prose      *string `json:"prose"`
}

func TestGetParentReport_DeterministicNoProse(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	q := mustNewQueries(pool)

	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pr-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Parent Report Class")

	studentID := createStudent(t, pool, SeedSchoolID, "pr-student@demo.local")
	enrollStudent(t, pool, studentID, classID)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "中国是否让地球更可持续？",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	report := agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "论证深度", Level: "L1", Evidence: "e1"},
			{Code: "D2", Name: "证据运用", Level: "L2", Evidence: "e2"},
			{Code: "D3", Name: "视角覆盖", Level: "L3", Evidence: "e3"},
			{Code: "D4", Name: "结构组织", Level: "L4", Evidence: "e4"},
			{Code: "D5", Name: "反思迭代", Level: "NA", Evidence: ""},
			{Code: "D6", Name: "真实性", Level: "L2", Evidence: "e6"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A1", Name: "主动提问", Level: 0, Opportunity: "not_supplied", Evidence: "a1"},
			{Code: "A2", Name: "主动质疑", Level: 1, Opportunity: "given_not_taken", Evidence: "a2"},
			{Code: "A3", Name: "主动求证", Level: 2, Opportunity: "given_taken", Evidence: "a3"},
			{Code: "A4", Name: "主动修订", Level: 3, Opportunity: "given_taken", Evidence: "a4"},
			{Code: "A5", Name: "主动拓展", Level: 4, Opportunity: "given_taken", Evidence: "a5"},
			{Code: "A6", Name: "主动核查", Level: 5, Opportunity: "given_taken", Evidence: "a6"},
		},
	}
	scores, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert project evaluation: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(
		"GET", fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s", classID, studentID, proj.ID), nil,
	), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var dto parentReportForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if dto.ProseReady {
		t.Error("prose should be absent before composition")
	}
	if dto.Prose != nil {
		t.Errorf("prose sentinel should be nil before composition, got %v", *dto.Prose)
	}
	if len(dto.DRows) != 6 || len(dto.ARows) != 6 {
		t.Fatalf("want 6 D + 6 A rows, got %d/%d", len(dto.DRows), len(dto.ARows))
	}
	// Badges are the parent 四台阶 labels, never L-codes or numbers.
	for _, d := range dto.DRows {
		switch d.Badge {
		case "起步", "发展", "熟练", "优秀", "暂无":
		default:
			t.Errorf("D badge %q not a 四台阶 label", d.Badge)
		}
	}
	for _, a := range dto.ARows {
		switch a.State {
		case "观察到主动信号", "偶有·多在引导后", "暂未观察到":
		default:
			t.Errorf("A state %q not a 三态 label", a.State)
		}
	}
	if dto.Cover.Subject != "研究项目 · 中国是否让地球更可持续？" {
		t.Errorf("cover.subject = %q", dto.Cover.Subject)
	}
	if dto.Cover.Klass != "Parent Report Class" {
		t.Errorf("cover.klass = %q", dto.Cover.Klass)
	}
	if dto.Cover.WarmLine != "" {
		t.Errorf("cover.warmLine should be empty before composition, got %q", dto.Cover.WarmLine)
	}
}

// parentProseStubReply is a valid agent.ParentProse JSON matching the report
// built by parentReportSeed below (D5 is NA, so wantedDepth = D1,D2,D3,D4,D6;
// all six autonomy signals get a reading). No bare internal codes/terms appear
// in any text value.
const parentProseStubReply = `{
  "glance":"本次项目里，孩子在信源甄别和论证展开上有明显进步。",
  "dOverview":"认知深度总体在稳步提升，各维度都留下了可看的证据。",
  "aOverview":"主动性信号本次以给出机会后主动响应为主。",
  "opportunity":"本次任务里，平台在多个环节主动创造了让孩子自己思考的机会，孩子接住了大部分，也有个别环节还在观察中；这些没被抓住的机会算平台待改进，不是孩子的短板。",
  "warmLine":"这周看到了她愿意多问一句的样子。",
  "dReadings":{
    "D1":"能把论点范围收得更精确，明确说明了适用条件。",
    "D2":"用了两个独立信源相互印证，而不是只信一个说法。",
    "D3":"引用的数据和最终结论对得上，没有断层。",
    "D4":"整体论证结构说明清楚，前后呼应。",
    "D6":"这部分内容能看出是她自己组织的语言，不是照搬。"
  },
  "aReadings":{
    "A1":"这次她自己主动提出了好几个疑问，不用引导。",
    "A2":"面对给出的机会，她还没有真正去质疑给出的说法。",
    "A3":"给到查证机会后，她主动去核实了信息来源。",
    "A4":"收到反馈后，她主动修改了论证里的漏洞。",
    "A5":"她把学到的方法用到了新的材料上。",
    "A6":"提交前，她自己又检查了一遍数据来源。"
  },
  "advice":[
    {"title":"多问一句","text":"晚饭时问问她这次论证里哪个证据她自己觉得最有说服力。"},
    {"title":"陪读原始资料","text":"周末一起翻一下她引用的原始报告，看看她怎么理解数据。"}
  ]
}`

// seedParentReportProject creates a teacher-owned class, an enrolled student,
// and a project with a stored evaluation matching parentProseStubReply's
// coverage (D5 NA, the rest real levels; all six autonomy signals). Returns
// the class id, student id, project id, and the teacher's cookie.
func seedParentReportProject(t *testing.T, pool *pgxpool.Pool, h http.Handler, teacherEmail, studentEmail string) (classID, studentID, projectID string, teacherCookie *http.Cookie) {
	t.Helper()
	q := mustNewQueries(pool)
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, teacherEmail))
	cls := createClassViaAPI(t, h, teacher, "Parent Prose Class")
	student := createStudent(t, pool, SeedSchoolID, studentEmail)
	enrollStudent(t, pool, student, cls)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: student, Qualification: "0457", Title: "中国是否让地球更可持续？",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	report := agent.Report{
		DepthAxis: []agent.DepthDim{
			{Code: "D1", Name: "论证深度", Level: "L1", Evidence: "e1"},
			{Code: "D2", Name: "证据运用", Level: "L2", Evidence: "e2"},
			{Code: "D3", Name: "视角覆盖", Level: "L3", Evidence: "e3"},
			{Code: "D4", Name: "结构组织", Level: "L4", Evidence: "e4"},
			{Code: "D5", Name: "反思迭代", Level: "NA", Evidence: ""},
			{Code: "D6", Name: "真实性", Level: "L2", Evidence: "e6"},
		},
		AutonomyAxis: []agent.AutonomySignal{
			{Code: "A1", Name: "主动提问", Level: 0, Opportunity: "not_supplied", Evidence: "a1"},
			{Code: "A2", Name: "主动质疑", Level: 1, Opportunity: "given_not_taken", Evidence: "a2"},
			{Code: "A3", Name: "主动求证", Level: 2, Opportunity: "given_taken", Evidence: "a3"},
			{Code: "A4", Name: "主动修订", Level: 3, Opportunity: "given_taken", Evidence: "a4"},
			{Code: "A5", Name: "主动拓展", Level: 4, Opportunity: "given_taken", Evidence: "a5"},
			{Code: "A6", Name: "主动核查", Level: 5, Opportunity: "given_taken", Evidence: "a6"},
		},
	}
	scores, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert project evaluation: %v", err)
	}
	return cls, student.String(), proj.ID.String(), teacher
}

func countParentReportLLMCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'parent_report'`).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	return n
}

func TestPostParentProse_ComposesOnceThenCostFree(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(parentProseStubReply)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()

	classID, studentID, projectID, teacher := seedParentReportProject(t, pool, h, "pp-teacher@demo.local", "pp-student@demo.local")

	url := fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s/prose", classID, studentID, projectID)

	post := func() parentReportForTest {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost, url, nil), teacher))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var dto parentReportForTest
		if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
			t.Fatalf("decode: %v — body=%s", err, rec.Body)
		}
		return dto
	}

	first := post()
	if !first.ProseReady {
		t.Fatalf("first POST should compose and mark proseReady: %+v", first)
	}
	if n1 := countParentReportLLMCalls(t, pool); n1 != 1 {
		t.Fatalf("want 1 parent_report llm_call, got %d", n1)
	}

	// Second POST: first-open-wins, no new spend.
	second := post()
	if !second.ProseReady {
		t.Fatal("second POST should still report proseReady from the stored row")
	}
	if n2 := countParentReportLLMCalls(t, pool); n2 != 1 {
		t.Fatalf("second POST must not spend; llm_calls=%d", n2)
	}
}

func TestPostParentProse_RejectionNeverWalls(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(pool)
	deps.Provider = assessStubProvider(`not json at all`)
	deps.EvalResolver = fakeEvalResolver()
	h := New(deps).Handler()

	classID, studentID, projectID, teacher := seedParentReportProject(t, pool, h, "pp-fail-teacher@demo.local", "pp-fail-student@demo.local")

	url := fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s/prose", classID, studentID, projectID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost, url, nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d; a rejected composition must never wall the screen: body=%s", rec.Code, rec.Body.String())
	}
	var dto parentReportForTest
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if dto.ProseReady {
		t.Fatalf("prose must be absent after a rejected composition: %+v", dto)
	}
	if dto.Prose != nil {
		t.Errorf("prose sentinel should be nil after a rejected composition, got %v", *dto.Prose)
	}
	if len(dto.DRows) != 6 || len(dto.ARows) != 6 {
		t.Fatalf("deterministic rows must still render: %d D / %d A rows", len(dto.DRows), len(dto.ARows))
	}
	if n := countParentReportLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows = %d; want exactly 1 — cost is recorded even on rejection", n)
	}
}

// TestGetParentReport_ForeignTeacher404 — a teacher who does not own the
// student's class gets 404 (existence-hidden), mirroring
// TestStudentReportProjectHappyPath's sibling guard in teacher_read_test.go.
func TestGetParentReport_ForeignTeacher404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	q := mustNewQueries(pool)

	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pr-owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "Parent Report Owned Class")

	studentID := createStudent(t, pool, SeedSchoolID, "pr-student2@demo.local")
	enrollStudent(t, pool, studentID, classID)

	proj, err := q.CreateProject(context.Background(), sqlc.CreateProjectParams{
		UserID: studentID, Qualification: "0457", Title: "另一个项目",
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	report := agent.Report{
		DepthAxis:    []agent.DepthDim{{Code: "D1", Name: "论证深度", Level: "L1", Evidence: "e1"}},
		AutonomyAxis: []agent.AutonomySignal{{Code: "A1", Name: "主动提问", Level: 1, Opportunity: "given_taken", Evidence: "a1"}},
	}
	scores, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if _, err := q.InsertProjectEvaluation(context.Background(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgtype.UUID{Bytes: proj.ID, Valid: true},
		Scores:    scores, Narrative: "n", Model: "test-model", Tier: "flagship",
	}); err != nil {
		t.Fatalf("insert project evaluation: %v", err)
	}

	outsider := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pr-outsider@demo.local"))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(
		"GET", fmt.Sprintf("/api/v1/classes/%s/students/%s/parent-report/project/%s", classID, studentID, proj.ID), nil,
	), outsider))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign teacher got %d, want 404: body=%s", rec.Code, rec.Body.String())
	}
}
