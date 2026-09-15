package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

type liteRosterRow struct {
	ID              string `json:"id"`
	MinutesTotal    int32  `json:"minutesTotal"`
	MinutesThisWeek int32  `json:"minutesThisWeek"`
	Turns           int32  `json:"turns"`
	ReadingsDone    int32  `json:"readingsDone"`
	ReadingsTotal   int32  `json:"readingsTotal"`
	WritingsTotal   int32  `json:"writingsTotal"`
}

// liteTeacherFixture: a lite school, a teacher who owns a class, one enrolled student.
func liteTeacherFixture(t *testing.T) (h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) {
	t.Helper()
	return liteTeacherFixtureWithProvider(t, nil)
}

// liteTeacherFixtureWithProvider is liteTeacherFixture with a scripted model
// behind it — Task 5's item-detail report slice is the first lite teacher
// endpoint that can reach the provider (via ensureAtomReport's phase 2), so
// every other lite teacher test keeps passing nil.
func liteTeacherFixtureWithProvider(t *testing.T, prov gateway.Provider) (h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) {
	t.Helper()
	pool = newAPITestPool(t)
	h = New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Provider: prov,
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	teacher = signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-teacher@demo.local"))
	classID = createClassViaAPI(t, h, teacher, "Lite Class")
	studentID = createStudent(t, pool, SeedSchoolID, "lt-student@demo.local")
	enrollStudent(t, pool, studentID, classID)
	return
}

func getJSON(t *testing.T, h http.Handler, c *http.Cookie, path string, out any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", path, nil), c))
	if out != nil && rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s: %v body=%s", path, err, rec.Body)
		}
	}
	return rec.Code
}

// seedLiteReadingForUser creates a reading atom for userID and returns its atom id.
// When status is "finished" it also sets reading.finished_at via SQL (CreateReading
// itself only knows 'active','finished' as a CHECK-constrained default).
func seedLiteReadingForUser(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, status string, activeSeconds int32) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(pool)
	atom, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: userID})
	if err != nil {
		t.Fatalf("seedLiteReadingForUser: create atom: %v", err)
	}
	if _, err := q.CreateReading(ctx, sqlc.CreateReadingParams{AtomID: atom.ID, Title: "Test reading", Lang: "zh"}); err != nil {
		t.Fatalf("seedLiteReadingForUser: create reading: %v", err)
	}
	if status == "finished" {
		if _, err := pool.Exec(ctx, `UPDATE reading SET status = 'finished', finished_at = now() WHERE atom_id = $1`, atom.ID); err != nil {
			t.Fatalf("seedLiteReadingForUser: finish: %v", err)
		}
	}
	if activeSeconds != 0 {
		if _, err := pool.Exec(ctx, `UPDATE atom SET active_seconds = $2 WHERE id = $1`, atom.ID, activeSeconds); err != nil {
			t.Fatalf("seedLiteReadingForUser: active_seconds: %v", err)
		}
	}
	return atom.ID
}

// seedLiteWritingForUser creates a writing atom for userID and returns its atom id.
func seedLiteWritingForUser(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(pool)
	atom, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: userID})
	if err != nil {
		t.Fatalf("seedLiteWritingForUser: create atom: %v", err)
	}
	if _, err := q.CreateWriting(ctx, sqlc.CreateWritingParams{AtomID: atom.ID, Title: "Test writing", Lang: "zh"}); err != nil {
		t.Fatalf("seedLiteWritingForUser: create writing: %v", err)
	}
	if status == "finished" {
		if _, err := pool.Exec(ctx, `UPDATE writing SET status = 'finished', finished_at = now() WHERE atom_id = $1`, atom.ID); err != nil {
			t.Fatalf("seedLiteWritingForUser: finish: %v", err)
		}
	}
	return atom.ID
}

// seedAtomMessage inserts one atom_message row directly (seq auto-assigned).
func seedAtomMessage(t *testing.T, pool *pgxpool.Pool, atomID uuid.UUID, role, content string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO atom_message (atom_id, seq, role, content) VALUES ($1, (SELECT COALESCE(max(seq),0)+1 FROM atom_message WHERE atom_id=$1), $2, $3)`,
		atomID, role, content); err != nil {
		t.Fatalf("seedAtomMessage: %v", err)
	}
}

func TestLiteRosterCountsLiteActivity(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	// One finished reading with 600s total, 120s bucket today, two student turns + one ai turn.
	readingAtom := seedLiteReadingForUser(t, pool, studentID, "finished", 600)
	if _, err := pool.Exec(ctx, `INSERT INTO atom_active_day (atom_id, day, seconds) VALUES ($1, (now() AT TIME ZONE 'Asia/Shanghai')::date, 120)`, readingAtom); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"student", "ai", "student"} {
		seedAtomMessage(t, pool, readingAtom, role, "hello")
	}
	seedLiteWritingForUser(t, pool, studentID, "active")

	var resp struct {
		Roster []liteRosterRow `json:"roster"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &resp); code != http.StatusOK {
		t.Fatalf("roster = %d", code)
	}
	if len(resp.Roster) != 1 {
		t.Fatalf("rows = %d", len(resp.Roster))
	}
	r := resp.Roster[0]
	if r.MinutesTotal != 10 || r.MinutesThisWeek != 2 || r.Turns != 2 || r.ReadingsDone != 1 || r.ReadingsTotal != 1 || r.WritingsTotal != 1 {
		t.Fatalf("row = %+v", r)
	}
}

// seedAtomMessageAt inserts one student atom_message with an explicit created_at.
func seedAtomMessageAt(t *testing.T, pool *pgxpool.Pool, atomID uuid.UUID, at time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO atom_message (atom_id, seq, role, content, created_at) VALUES ($1, (SELECT COALESCE(max(seq),0)+1 FROM atom_message WHERE atom_id=$1), 'student', 'x', $2)`,
		atomID, at); err != nil {
		t.Fatalf("seedAtomMessageAt: %v", err)
	}
}

func seedBucket(t *testing.T, pool *pgxpool.Pool, atomID uuid.UUID, day time.Time, seconds int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO atom_active_day (atom_id, day, seconds) VALUES ($1, $2::date, $3)`,
		atomID, day.Format("2006-01-02"), seconds); err != nil {
		t.Fatalf("seedBucket: %v", err)
	}
}

// TestLiteRosterActiveDaysAndCounts is final review M3: activeDaysThisWeek
// (bucket days UNION student-message days, Beijing week), lastActiveAt, and
// the done/total counts.
func TestLiteRosterActiveDaysAndCounts(t *testing.T) {
	h, pool, teacher, classID, active := liteTeacherFixture(t)
	ctx := context.Background()
	inactive := createStudent(t, pool, SeedSchoolID, "lt-inactive@demo.local")
	enrollStudent(t, pool, inactive, classID)
	boundary := createStudent(t, pool, SeedSchoolID, "lt-boundary@demo.local")
	enrollStudent(t, pool, boundary, classID)

	now := time.Now()
	today := liteweek.Day(now)
	weekStart := liteweek.WeekStart(now)
	isMonday := today.Equal(weekStart)

	reading := seedLiteReadingForUser(t, pool, active, "finished", 600)
	seedBucket(t, pool, reading, today, 120)
	wantDays := int32(1)
	if !isMonday {
		// An earlier day of this week, recorded twice (bucket + message): the
		// UNION must count it once.
		seedBucket(t, pool, reading, weekStart, 60)
		seedAtomMessageAt(t, pool, reading, weekStart.Add(12*time.Hour))
		wantDays = 2
	} else {
		// No earlier day this week: the same pair on last Sunday is not counted.
		lastSunday := weekStart.AddDate(0, 0, -1)
		seedBucket(t, pool, reading, lastSunday, 60)
		seedAtomMessageAt(t, pool, reading, lastSunday.Add(12*time.Hour))
	}
	seedLiteWritingForUser(t, pool, active, "finished")
	seedLiteWritingForUser(t, pool, active, "active")
	project := seedLiteWebsiteProject(t, pool, active)
	if _, err := pool.Exec(ctx, `UPDATE pbl_project SET status = 'keeping' WHERE atom_id = $1`, project); err != nil {
		t.Fatal(err)
	}

	// Boundary student: a message at exactly Monday 00:00 Beijing counts,
	// one at the previous Sunday 23:59:59 does not.
	edge := seedLiteReadingForUser(t, pool, boundary, "active", 0)
	seedAtomMessageAt(t, pool, edge, weekStart)
	seedAtomMessageAt(t, pool, edge, weekStart.Add(-time.Second))

	var resp struct {
		Roster []LiteRosterRowDTO `json:"roster"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &resp); code != http.StatusOK {
		t.Fatalf("roster = %d", code)
	}
	byID := map[string]LiteRosterRowDTO{}
	for _, r := range resp.Roster {
		byID[r.ID] = r
	}

	a := byID[active.String()]
	if a.ActiveDaysThisWeek != wantDays {
		t.Fatalf("activeDaysThisWeek = %d, want %d (monday=%v)", a.ActiveDaysThisWeek, wantDays, isMonday)
	}
	if a.LastActiveAt == nil {
		t.Fatalf("lastActiveAt = nil for an active student")
	}
	if a.ReadingsDone != 1 || a.WritingsDone != 1 || a.WritingsTotal != 2 || a.ProjectsDone != 1 {
		t.Fatalf("counts = %+v, want readingsDone 1, writingsDone 1, writingsTotal 2, projectsDone 1", a)
	}

	in := byID[inactive.String()]
	if in.LastActiveAt != nil || in.ActiveDaysThisWeek != 0 {
		t.Fatalf("inactive student = %+v, want lastActiveAt nil and 0 active days", in)
	}

	if b := byID[boundary.String()]; b.ActiveDaysThisWeek != 1 {
		t.Fatalf("boundary activeDaysThisWeek = %d, want 1 (Monday 00:00 in, Sunday 23:59:59 out)", b.ActiveDaysThisWeek)
	}
}

func TestLiteRosterNoBucketsIsMinusOne(t *testing.T) {
	h, _, teacher, classID, _ := liteTeacherFixture(t)
	var resp struct {
		Roster []liteRosterRow `json:"roster"`
	}
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &resp)
	if resp.Roster[0].MinutesThisWeek != -1 {
		t.Fatalf("minutesThisWeek = %d, want -1", resp.Roster[0].MinutesThisWeek)
	}
	// No atoms at all: total time is unknown too, not zero.
	if resp.Roster[0].MinutesTotal != -1 {
		t.Fatalf("minutesTotal = %d, want -1", resp.Roster[0].MinutesTotal)
	}
}

func TestLiteRosterAuthz(t *testing.T) {
	h, pool, _, classID, studentID := liteTeacherFixture(t)
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/classes/"+classID+"/roster", nil); code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404", code)
	}
	student := signInAs(t, pool, studentID)
	if code := getJSON(t, h, student, "/api/v1/lite/teacher/classes/"+classID+"/roster", nil); code != http.StatusForbidden {
		t.Fatalf("student = %d, want 403", code)
	}
}

func TestLiteRosterProSchool404(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixture(t)
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'pro'`); err != nil {
		t.Fatal(err)
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", nil); code != http.StatusNotFound {
		t.Fatalf("pro school = %d, want 404", code)
	}
}

type liteItemRow struct {
	AtomID  string `json:"atomId"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Minutes int32  `json:"minutes"`
	Turns   int32  `json:"turns"`
}

func TestLiteStudentPageListsItems(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	r := seedLiteReadingForUser(t, pool, studentID, "finished", 300)
	seedAtomMessage(t, pool, r, "student", "x")
	seedLiteWritingForUser(t, pool, studentID, "active")

	var resp struct {
		Student liteRosterRow `json:"student"`
		Items   []liteItemRow `json:"items"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String(), &resp); code != http.StatusOK {
		t.Fatalf("student page = %d", code)
	}
	if resp.Student.ID != studentID.String() || len(resp.Items) != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	var reading *liteItemRow
	for i := range resp.Items {
		if resp.Items[i].AtomID == r.String() {
			reading = &resp.Items[i]
		}
	}
	if reading == nil || reading.Kind != "reading" || reading.Status != "finished" || reading.Minutes != 5 || reading.Turns != 1 {
		t.Fatalf("reading row = %+v", reading)
	}
}

// TestLiteRosterOverdueAssignments is Task 6: an unstarted assignment whose
// deadline has passed counts as overdue on the roster row, and shows
// status "overdue" on the student page's assignment list.
func TestLiteRosterOverdueAssignments(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	if _, err := pool.Exec(context.Background(),
		`UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}

	var roster struct {
		Roster []LiteRosterRowDTO `json:"roster"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &roster); code != http.StatusOK {
		t.Fatalf("roster = %d", code)
	}
	if len(roster.Roster) != 1 || roster.Roster[0].OverdueAssignments != 1 {
		t.Fatalf("roster = %+v", roster.Roster)
	}

	var page struct {
		Assignments []StudentAssignmentDTO `json:"assignments"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String(), &page); code != http.StatusOK {
		t.Fatalf("student page = %d", code)
	}
	if len(page.Assignments) != 1 || page.Assignments[0].ID != aid ||
		page.Assignments[0].Status != "overdue" || page.Assignments[0].StatusLabel == "" {
		t.Fatalf("assignments = %+v", page.Assignments)
	}
}

// TestLiteRosterOverdueExcludesFinishedLate: an assignment finished after its
// deadline is "done_late", not "overdue" — it must not count.
func TestLiteRosterOverdueExcludesFinishedLate(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid := createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	student := signInAs(t, pool, studentID)
	started := startAssignment(t, h, student, aid)
	if _, err := pool.Exec(context.Background(),
		`UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE writing SET status = 'finished', finished_at = now() WHERE atom_id = $1`, started.AtomID); err != nil {
		t.Fatal(err)
	}

	var roster struct {
		Roster []LiteRosterRowDTO `json:"roster"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &roster); code != http.StatusOK {
		t.Fatalf("roster = %d", code)
	}
	if roster.Roster[0].OverdueAssignments != 0 {
		t.Fatalf("overdueAssignments = %d, want 0 (finished late)", roster.Roster[0].OverdueAssignments)
	}

	var page struct {
		Assignments []StudentAssignmentDTO `json:"assignments"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String(), &page); code != http.StatusOK {
		t.Fatalf("student page = %d", code)
	}
	if len(page.Assignments) != 1 || page.Assignments[0].Status != "done_late" {
		t.Fatalf("assignments = %+v", page.Assignments)
	}
}

func TestLiteStudentPageStudentOfOtherClass404(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixture(t)
	stranger := createStudent(t, pool, SeedSchoolID, "lt-stranger@demo.local")
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+stranger.String(), nil); code != http.StatusNotFound {
		t.Fatalf("stranger = %d, want 404", code)
	}
}
