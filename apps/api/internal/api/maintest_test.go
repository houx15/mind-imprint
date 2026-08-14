package api_test

// maintest_test.go — shared test helpers for Tasks 6-9.
// Duplicates the testcontainers bootstrap from internal/store/sqlc_test.go
// because that helper lives in package store_test and is not importable here.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

// assessStubProvider is a gateway.Provider whose Stream emits a scripted
// reply then closes — same scripted-provider shape as writing_test.go's
// reviewStubProvider, just carrying whatever wire shape the caller scripts
// (assessment/report JSON, etc). Relocated here (2026-08-14, retire-old-
// evaluation-pipeline Task 1) from the now-deleted assessment_test.go: it is
// a shared fixture consumed by several surviving test files (project_finish,
// project_writing_finish, cards_growth, workspace_journey, workspace_review),
// not just the old assessment tests.
func assessStubProvider(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// assessReply is a scripted assessor JSON reply, used by project_finish_test.go.
// Relocated from the deleted assessment_test.go (see assessStubProvider doc).
const assessReply = `{"dimensions":[{"code":"D2","level":"L4","evidence":"交叉验证两个一手源"},{"code":"D5","level":"L3","evidence":"论证拆解清楚"}],"narrative":"你这次最大的跃迁在信源辨识。"}`

// dualAxisReply is a valid canonical Report reply (agent.AssessReport's
// reportWire shape) — the shared success fixture for the project, course, and
// chat surfaces. Relocated from the deleted assessment_test.go (see
// assessStubProvider doc) — still consumed by several surviving test files.
const dualAxisReply = `{"depthAxis":[
  {"code":"D1","level":"L3","evidence":"限定判断了论点范围","promptEvidence":"R2"},
  {"code":"D2","level":"L4","evidence":"交叉验证了两个一手源","promptEvidence":"R4"},
  {"code":"D3","level":"L3","evidence":"NASA 数据与结论对齐","promptEvidence":"R5"},
  {"code":"D4","level":"L3","evidence":"warrant 说明清楚","promptEvidence":""},
  {"code":"D5","level":"L3","evidence":"理由链完整","promptEvidence":""},
  {"code":"D6","level":"L3","evidence":"复盘时能说出下一步","promptEvidence":""}],
  "autonomyAxis":[
  {"code":"A1","level":2,"opportunity":"given_taken","evidence":"自己定了检索范围","promptEvidence":"R1"},
  {"code":"A2","level":1,"opportunity":"given_taken","evidence":"","promptEvidence":""},
  {"code":"A3","level":3,"opportunity":"given_taken","evidence":"设了字数与来源边界","promptEvidence":"R3"},
  {"code":"A4","level":0,"opportunity":"not_supplied","evidence":"","promptEvidence":""},
  {"code":"A5","level":2,"opportunity":"given_not_taken","evidence":"","promptEvidence":""},
  {"code":"A6","level":1,"opportunity":"given_taken","evidence":"","promptEvidence":""}],
  "promptLens":{
    "stats":[{"label":"提问轮次","value":"10"},{"label":"边界设定","value":"3"},{"label":"对抗性邀请","value":"0"}],
    "lenses":[
      {"code":"L_decisions","level":3,"evidence":"决策清楚"},
      {"code":"L_maturity","level":3,"evidence":"成熟度稳定"},
      {"code":"L_boundary","level":3,"evidence":"边界设定明确"},
      {"code":"L_adversary","level":1,"evidence":"较少邀请对抗视角"},
      {"code":"L_directive","level":2,"evidence":"指令型提问适中"},
      {"code":"L_acceptance","level":3,"evidence":"能筛选采纳"}]},
  "interactionEvidence":[
    {"round":4,"student":"我想把 thesis 限定到国内新能源投资","aiSummary":"帮你梳理了限定范围的两种做法","signal":"边界设定"}],
  "narrative":"这次协作里，你在信源辨识上完成了一次明显的跃迁。",
  "guidance":{"nextSteps":[{"title":"强化反例检验","task":"针对碳排放数据补一版反例段"}]},
  "officialProjection":{
    "standard":{"id":"ap-research","name":"AP Research"},
    "components":[
      {"name":"Academic Paper","judgement":"接近达标","reason":"论证结构完整，证据链清楚"},
      {"name":"训练用折算","judgement":"仅供参考","reason":"内部折算不等于官方评分"}],
    "alignment":[
      {"item":"论证结构","standard":"清晰的主张-证据-推理链","performance":"三段论证均有 warrant","impact":"支撑 Academic Paper 的组织维度"}],
    "readiness":{"score":72,"note":"仅作作品就绪度参考，不与 D/A 双轴合成"}},
  "workAndProcess":{
    "workSamples":[{"title":"最新稿件","text":"中国的可再生能源投资规模已经连续五年位居全球第一。"}],
    "processMaterials":[{"name":"SIFT 溯源记录","status":"completed","diagnosis":"两个信源均可交叉验证"}]}}`

// cardsByID wires Deps.SpecByID to the embedded card catalog (cards.ByID) —
// a tiny named wrapper so test setup reads `SpecByID: cardsByID()` instead
// of repeating the func literal at every call site.
func cardsByID() func(id string) (cards.Spec, bool) {
	return cards.ByID
}

// newAPITestPool spins up a throwaway Postgres, runs all migrations (incl. seed),
// and returns the pool. Container/pool torn down via t.Cleanup.
func newAPITestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("mindimprint"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := store.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// signInSeed creates a live session for the seeded student and returns the
// cookie to attach to authed requests (replaces the implicit ActAsSeed inject).
func signInSeed(t *testing.T, pool *pgxpool.Pool) *http.Cookie {
	return signInAs(t, pool, SeedUserID)
}

// signInAs creates a live session for an arbitrary user and returns its cookie.
func signInAs(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) *http.Cookie {
	t.Helper()
	q := sqlc.New(pool)
	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := q.CreateSession(context.Background(), sqlc.CreateSessionParams{
		UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &http.Cookie{Name: "mk_session", Value: raw}
}

func signInAdmin(t *testing.T, pool *pgxpool.Pool) *http.Cookie {
	return signInAs(t, pool, SeedAdminID)
}

// createTeacher inserts a verified teacher in schoolID and returns its id.
func createTeacher(t *testing.T, pool *pgxpool.Pool, schoolID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	q := sqlc.New(pool)
	u, err := q.CreateUser(context.Background(), sqlc.CreateUserParams{
		Email: email, PasswordHash: "x", Role: "teacher", SchoolID: schoolID,
		DisplayName: "T " + email, AvatarColor: "#888888",
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create teacher: %v", err)
	}
	return u.ID
}

// createStudent inserts a verified student in schoolID and returns its id.
func createStudent(t *testing.T, pool *pgxpool.Pool, schoolID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	q := sqlc.New(pool)
	u, err := q.CreateUser(context.Background(), sqlc.CreateUserParams{
		Email: email, PasswordHash: "x", Role: "student", SchoolID: schoolID,
		DisplayName: "S " + email, AvatarColor: "#888888",
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}
	return u.ID
}

// countAllLLMCalls counts every llm_call row — unscoped, for endpoints with no
// project id in scope. Relocated here (2026-08-14, retire-old-evaluation-
// pipeline Task 1) from the now-deleted ability_test.go: still consumed by
// cards_growth_test.go.
func countAllLLMCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM llm_call").Scan(&n); err != nil {
		t.Fatalf("count all llm_call rows: %v", err)
	}
	return n
}

// decodeReadyEnvelope decodes a GET/POST evaluation-report response body,
// asserting it is the "ready" envelope ({"status":"ready","report":{…}}) and
// returning the unwrapped evalreport.Report — the shared shape most
// evaluation-report tests care about (Task 4, retire-old-evaluation-pipeline:
// the read endpoints moved from returning the report directly to this
// three-state envelope).
func decodeReadyEnvelope(t *testing.T, body []byte) evalreport.Report {
	t.Helper()
	var envelope struct {
		Status string            `json:"status"`
		Report evalreport.Report `json:"report"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode evaluation-report envelope: %v — body=%s", err, body)
	}
	if envelope.Status != "ready" {
		t.Fatalf("evaluation-report envelope status = %q, want ready; body=%s", envelope.Status, body)
	}
	return envelope.Report
}

// withCookie attaches c to req and returns it (for inline request building).
func withCookie(req *http.Request, c *http.Cookie) *http.Request {
	req.AddCookie(c)
	return req
}

// newTestAPI builds a minimal *API wired to the given pool (no gateway/catalog).
func newTestAPI(pool *pgxpool.Pool) *API {
	return New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false})
}

// mustUUID parses a UUID string and panics on error (test-only convenience).
func mustUUID(s string) uuid.UUID {
	return uuid.MustParse(s)
}

// mustNewQueries returns a *sqlc.Queries wired to pool (test helper).
func mustNewQueries(pool *pgxpool.Pool) *sqlc.Queries {
	return sqlc.New(pool)
}

// createClassViaAPI POSTs /api/v1/classes with the given name and returns the new class id.
func createClassViaAPI(t *testing.T, h http.Handler, cookie *http.Cookie, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"name": name})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("createClassViaAPI got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Class struct {
			ID string `json:"id"`
		} `json:"class"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("createClassViaAPI decode: %v", err)
	}
	return resp.Class.ID
}

// enrollStudent directly inserts a student enrollment into the DB.
func enrollStudent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, classID string) {
	t.Helper()
	q := sqlc.New(pool)
	if _, err := q.CreateEnrollment(context.Background(), sqlc.CreateEnrollmentParams{
		UserID:      userID,
		ClassID:     uuid.MustParse(classID),
		RoleInClass: "student",
	}); err != nil {
		t.Fatalf("enrollStudent: %v", err)
	}
}

// seedSecondSchool inserts a second school and returns its id.
func seedSecondSchool(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO schools (id, name) VALUES ($1, $2)`, id, "Other School"); err != nil {
		t.Fatalf("seedSecondSchool: %v", err)
	}
	return id
}

// signInViaAPI signs in via the real POST /api/v1/auth/signin endpoint and
// returns the mk_session cookie from the Set-Cookie response header.
func signInViaAPI(t *testing.T, h http.Handler, email, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"email": email, "password": password})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signin", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("signInViaAPI: want 200, got %d — body: %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mk_session" {
			return c
		}
	}
	t.Fatalf("signInViaAPI: mk_session cookie not found in response")
	return nil
}
