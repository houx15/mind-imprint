package api_test

import (
	"context"
	"net/http"
	"testing"
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
