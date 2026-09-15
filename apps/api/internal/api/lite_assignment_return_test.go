package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	. "mindimprint/api/internal/api"
)

type recipientView struct {
	Status       string  `json:"status"`
	StatusLabel  string  `json:"statusLabel"`
	ReturnedAt   *string `json:"returnedAt"`
	ReturnDueAt  *string `json:"returnDueAt"`
	ReturnNote   *string `json:"returnNote"`
	VersionCount int     `json:"versionCount"`
}

func recipientStatus(t *testing.T, h http.Handler, teacher *http.Cookie, aid string) recipientView {
	t.Helper()
	var detail struct {
		Recipients []recipientView `json:"recipients"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+aid, &detail); code != http.StatusOK || len(detail.Recipients) != 1 {
		t.Fatalf("detail = %d %+v", code, detail.Recipients)
	}
	return detail.Recipients[0]
}

// The status path only; the return endpoint arrives in Task 6, so the return
// is written with SQL here.
func TestReturnedAndResubmittedStatuses(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	if got := recipientStatus(t, h, teacher, aid); got.Status != "done" || got.VersionCount != 1 || got.ReturnedAt != nil {
		t.Fatalf("after finish = %+v", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE lite_assignment_recipient
		SET returned_at = now(), return_due_at = now() + interval '2 days', return_note = '请补充第二段的论据'
		WHERE assignment_id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	got := recipientStatus(t, h, teacher, aid)
	if got.Status != "returned" || got.StatusLabel != "已退回" || got.ReturnDueAt == nil || got.ReturnNote == nil || *got.ReturnNote != "请补充第二段的论据" {
		t.Fatalf("returned = %+v", got)
	}

	var inbox struct {
		Items []struct {
			Status      string  `json:"status"`
			StatusLabel string  `json:"statusLabel"`
			ReturnDueAt *string `json:"returnDueAt"`
			ReturnNote  *string `json:"returnNote"`
		} `json:"items"`
	}
	getJSON(t, h, student, "/api/v1/lite/inbox", &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0].Status != "returned" || inbox.Items[0].ReturnDueAt == nil || inbox.Items[0].ReturnNote == nil {
		t.Fatalf("inbox = %+v", inbox.Items)
	}

	var list struct {
		Assignments []struct {
			Counts map[string]int `json:"counts"`
		} `json:"assignments"`
	}
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/assignments", &list)
	if len(list.Assignments) != 1 || list.Assignments[0].Counts["returned"] != 1 || len(list.Assignments[0].Counts) != 7 {
		t.Fatalf("counts = %+v", list.Assignments)
	}

	// Past the return deadline with no new version → overdue.
	if _, err := pool.Exec(ctx, `UPDATE lite_assignment_recipient SET return_due_at = now() - interval '1 minute' WHERE assignment_id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if got := recipientStatus(t, h, teacher, aid); got.Status != "overdue" {
		t.Fatalf("past return due = %s, want overdue", got.Status)
	}

	// A version submitted after the return → resubmitted, whatever the deadline.
	if _, err := pool.Exec(ctx, `INSERT INTO writing_version (atom_id, number, title, body, word_count, submitted_at)
		VALUES ($1, 2, '雨', '第二版。', 4, now())`, atomID); err != nil {
		t.Fatal(err)
	}
	if got := recipientStatus(t, h, teacher, aid); got.Status != "resubmitted" || got.StatusLabel != "已重新提交" || got.VersionCount != 2 {
		t.Fatalf("resubmitted = %+v", got)
	}
}

func returnPath(aid, userID string) string {
	return "/api/v1/lite/teacher/assignments/" + aid + "/recipients/" + userID + "/return"
}

func TestReturnWritingHomework(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	path := returnPath(aid, studentID.String())
	future := time.Now().Add(72 * time.Hour).Format(time.RFC3339)

	if code, ec := writeErrorCode(t, h, teacher, "POST", path, map[string]any{"dueAt": future}); code != http.StatusConflict || ec != "no_submission" {
		t.Fatalf("return before a version = %d %s", code, ec)
	}
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	// Past the original deadline the homework is locked.
	if _, err := pool.Exec(ctx, `UPDATE lite_assignment SET due_at = now() - interval '1 hour' WHERE id = $1`, aid); err != nil {
		t.Fatal(err)
	}
	if code, ec := writeErrorCode(t, h, student, "POST", "/api/v1/writings/"+atomID+"/revise", nil); code != http.StatusForbidden || ec != "writing_locked" {
		t.Fatalf("revise when locked = %d %s", code, ec)
	}

	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	if code, ec := writeErrorCode(t, h, teacher, "POST", path, map[string]any{"dueAt": past}); code != http.StatusBadRequest || ec != "invalid_due_at" {
		t.Fatalf("past dueAt = %d %s", code, ec)
	}
	if code, ec := writeErrorCode(t, h, teacher, "POST", path, map[string]any{"dueAt": future, "note": strings.Repeat("字", 501)}); code != http.StatusBadRequest || ec != "invalid_note" {
		t.Fatalf("long note = %d %s", code, ec)
	}

	var resp struct {
		Recipient recipientView `json:"recipient"`
	}
	if code := assignJSON(t, h, teacher, "POST", path, map[string]any{"dueAt": future, "note": "  请补充第二段的论据  "}, &resp); code != http.StatusOK {
		t.Fatalf("return = %d", code)
	}
	if resp.Recipient.Status != "returned" || resp.Recipient.ReturnNote == nil || *resp.Recipient.ReturnNote != "请补充第二段的论据" {
		t.Fatalf("return response = %+v", resp.Recipient)
	}

	// Returned: the lock uses the new deadline, so she can edit again.
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/revise", nil, nil); code != http.StatusOK {
		t.Fatalf("revise after return = %d", code)
	}
	if code := assignJSON(t, h, student, "PUT", "/api/v1/writings/"+atomID+"/draft", map[string]any{"body": "补了论据。"}, nil); code != http.StatusOK {
		t.Fatalf("draft after return = %d", code)
	}

	// Returning again overwrites; an empty note is stored as NULL.
	later := time.Now().Add(96 * time.Hour).Format(time.RFC3339)
	if code := assignJSON(t, h, teacher, "POST", path, map[string]any{"dueAt": later}, &resp); code != http.StatusOK {
		t.Fatalf("second return = %d", code)
	}
	if resp.Recipient.ReturnNote != nil || resp.Recipient.ReturnDueAt == nil {
		t.Fatalf("second return = %+v", resp.Recipient)
	}
}

func TestReturnRefusals(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	future := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	aid, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}

	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ret-other@demo.local"))
	if code, _ := writeErrorCode(t, h, other, "POST", returnPath(aid, studentID.String()), map[string]any{"dueAt": future}); code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404", code)
	}
	stranger := createStudent(t, pool, SeedSchoolID, "ret-stranger@demo.local")
	if code, _ := writeErrorCode(t, h, teacher, "POST", returnPath(aid, stranger.String()), map[string]any{"dueAt": future}); code != http.StatusNotFound {
		t.Fatalf("not a recipient = %d, want 404", code)
	}
	if code, _ := writeErrorCode(t, h, teacher, "POST", returnPath(aid, "not-a-uuid"), map[string]any{"dueAt": future}); code != http.StatusNotFound {
		t.Fatalf("bad user id = %d, want 404", code)
	}
	if code, _ := writeErrorCode(t, h, student, "POST", returnPath(aid, studentID.String()), map[string]any{"dueAt": future}); code != http.StatusForbidden && code != http.StatusNotFound {
		t.Fatalf("student calling return = %d, want 403 or 404", code)
	}

	reading := createAssignment(t, h, teacher, classID, readingAssignmentBody("读", map[string]any{"source": "text", "text": "一段正文。"}, []string{studentID.String()}))
	if code, ec := writeErrorCode(t, h, teacher, "POST", returnPath(reading, studentID.String()), map[string]any{"dueAt": future}); code != http.StatusBadRequest || ec != "not_writing_assignment" {
		t.Fatalf("reading homework = %d %s", code, ec)
	}
}
