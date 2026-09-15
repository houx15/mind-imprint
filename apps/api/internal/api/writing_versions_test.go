package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type versionRow struct {
	Number      int
	Title       string
	Body        string
	WordCount   int
	SubmittedAt time.Time
}

// newBroughtWriting creates a writing she brought in with a finished body:
// it lands on the draft stage with writing_draft already filled.
func newBroughtWriting(t *testing.T, h http.Handler, c *http.Cookie, body string) string {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	if code := assignJSON(t, h, c, "POST", "/api/v1/writings", map[string]any{"idea": "雨", "lang": "zh", "body": body}, &created); code != http.StatusCreated {
		t.Fatalf("create writing = %d", code)
	}
	return created.ID
}

func versionRows(t *testing.T, pool *pgxpool.Pool, atomID string) []versionRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT number, title, body, word_count, submitted_at FROM writing_version WHERE atom_id = $1 ORDER BY number`, atomID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []versionRow
	for rows.Next() {
		var v versionRow
		if err := rows.Scan(&v.Number, &v.Title, &v.Body, &v.WordCount, &v.SubmittedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// startWritingHomework: the teacher assigns a writing to the fixture student,
// she starts it and writes a draft. Nothing is finished yet.
func startWritingHomework(t *testing.T, h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) (aid, atomID string, student *http.Cookie) {
	t.Helper()
	student = signInAs(t, pool, studentID)
	aid = createAssignment(t, h, teacher, classID, writingAssignmentBody([]string{studentID.String()}))
	atomID = startAssignment(t, h, student, aid).AtomID
	if code := assignJSON(t, h, student, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": "雨下了一整天。"}, nil); code != http.StatusOK {
		t.Fatalf("put draft = %d", code)
	}
	return aid, atomID, student
}

func TestWritingVersionFirstFinishCreatesVersionOne(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	id := newBroughtWriting(t, h, student, "我读了 NASA 的报告，数据是 2024 年的。")

	var first struct {
		Status     string  `json:"status"`
		FinishedAt *string `json:"finishedAt"`
		RevisingAt *string `json:"revisingAt"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+id+"/finish", nil, &first); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if first.Status != "finished" || first.FinishedAt == nil || first.RevisingAt != nil {
		t.Fatalf("finish response = %+v", first)
	}
	got := versionRows(t, pool, id)
	if len(got) != 1 || got[0].Number != 1 || got[0].Title != "雨" ||
		got[0].Body != "我读了 NASA 的报告，数据是 2024 年的。" || got[0].WordCount != 15 {
		t.Fatalf("versions = %+v", got)
	}

	// A second finish while not revising changes nothing: no version 2,
	// finished_at stays where it was.
	var again struct {
		FinishedAt *string `json:"finishedAt"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+id+"/finish", nil, &again); code != http.StatusOK {
		t.Fatalf("second finish = %d", code)
	}
	if n := len(versionRows(t, pool, id)); n != 1 {
		t.Fatalf("second finish added a version: %d rows", n)
	}
	if again.FinishedAt == nil || *again.FinishedAt != *first.FinishedAt {
		t.Fatalf("finishedAt moved: %v → %v", first.FinishedAt, again.FinishedAt)
	}
}

func TestWritingVersionFinishWithoutDraftAddsNothing(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	var created struct {
		ID string `json:"id"`
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings", map[string]any{"idea": "雨", "lang": "zh"}, &created); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+created.ID+"/finish", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("finish without draft = %d, want 400", code)
	}
	if n := len(versionRows(t, pool, created.ID)); n != 0 {
		t.Fatalf("versions = %d, want 0", n)
	}
}

// A homework she never submitted stays submittable after the deadline.
func TestWritingVersionLateFirstSubmissionIsNotLocked(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if _, err := pool.Exec(context.Background(), `UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("late first finish = %d, want 200", code)
	}
	if n := len(versionRows(t, pool, atomID)); n != 1 {
		t.Fatalf("versions = %d, want 1", n)
	}
}
