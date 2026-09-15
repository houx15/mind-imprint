package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
)

func studentGradings(t *testing.T, f *gradingFixture, student *http.Cookie, atomID string) []map[string]any {
	t.Helper()
	var resp struct {
		Gradings []map[string]any `json:"gradings"`
	}
	if code := getJSON(t, f.h, student, "/api/v1/writings/"+atomID+"/gradings", &resp); code != http.StatusOK {
		t.Fatalf("student gradings = %d", code)
	}
	return resp.Gradings
}

type inboxView struct {
	Items []struct {
		Type         string `json:"type"`
		ID           string `json:"id"`
		AtomID       string `json:"atomId"`
		WritingTitle string `json:"writingTitle"`
		SentAt       string `json:"sentAt"`
		Unread       bool   `json:"unread"`
	} `json:"items"`
	Unread int `json:"unread"`
}

// Students never see a grading that is not sent: not queued, not a draft,
// not failed, and never the ai copy or the error.
func TestStudentSeesOnlySentGradings(t *testing.T) {
	f := newGradingFixture(t)
	aid, atomID, student := f.submit(t)
	f.queueAll(t, aid, false)
	gid := f.rows(t, aid)[0].Grading.ID
	if got := studentGradings(t, f, student, atomID); len(got) != 0 {
		t.Fatalf("queued visible: %v", got)
	}
	f.runJobs(t)
	if got := studentGradings(t, f, student, atomID); len(got) != 0 {
		t.Fatalf("draft visible: %v", got)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'failed', error = 'x' WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	if got := studentGradings(t, f, student, atomID); len(got) != 0 {
		t.Fatalf("failed visible: %v", got)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE lite_grading SET status = 'draft', error = NULL WHERE id = $1`, gid); err != nil {
		t.Fatal(err)
	}
	var inbox inboxView
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	for _, it := range inbox.Items {
		if it.Type == "grading" {
			t.Fatalf("unsent grading in the inbox: %+v", it)
		}
	}

	if code := assignJSON(t, f.h, f.teacher, "POST", "/api/v1/lite/teacher/gradings/"+gid+"/send", nil, nil); code != http.StatusOK {
		t.Fatalf("send = %d", code)
	}
	got := studentGradings(t, f, student, atomID)
	if len(got) != 1 || got[0]["id"] != gid || got[0]["versionNumber"] != float64(1) || got[0]["seen"] != false {
		t.Fatalf("sent = %v", got)
	}
	for _, hidden := range []string{"ai", "status", "error", "requestedBy", "reviewedAt"} {
		if _, ok := got[0][hidden]; ok {
			t.Fatalf("student response leaks %q: %v", hidden, got[0])
		}
	}
	var content struct {
		Overall struct{ Grade string } `json:"overall"`
	}
	raw, _ := json.Marshal(got[0]["content"])
	if json.Unmarshal(raw, &content) != nil || content.Overall.Grade != "B+" || got[0]["rubric"] == nil {
		t.Fatalf("content/rubric = %v", got[0])
	}

	// Inbox: one unread grading item; the assignment was seen when she started it.
	inbox = inboxView{}
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	var grading *struct {
		Type         string `json:"type"`
		ID           string `json:"id"`
		AtomID       string `json:"atomId"`
		WritingTitle string `json:"writingTitle"`
		SentAt       string `json:"sentAt"`
		Unread       bool   `json:"unread"`
	}
	for i := range inbox.Items {
		if inbox.Items[i].Type == "grading" {
			grading = &inbox.Items[i]
		}
	}
	if grading == nil || grading.ID != gid || grading.AtomID != atomID || !grading.Unread || grading.SentAt == "" || grading.WritingTitle == "" || inbox.Unread != 1 {
		t.Fatalf("inbox = %+v", inbox)
	}
	if code := assignJSON(t, f.h, student, "POST", "/api/v1/lite/inbox/gradings/"+gid+"/seen", nil, nil); code != http.StatusNoContent {
		t.Fatalf("seen = %d", code)
	}
	inbox = inboxView{}
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 0 {
		t.Fatalf("after seen unread = %d", inbox.Unread)
	}

	// An edit re-sends and makes it unread again.
	if code := assignJSON(t, f.h, f.teacher, "PATCH", "/api/v1/lite/teacher/gradings/"+gid, map[string]any{}, nil); code != http.StatusOK {
		t.Fatalf("re-save = %d", code)
	}
	inbox = inboxView{}
	getJSON(t, f.h, student, "/api/v1/lite/inbox", &inbox)
	if inbox.Unread != 1 {
		t.Fatalf("after re-send unread = %d, want 1", inbox.Unread)
	}

	// Another student cannot read or mark it.
	stranger := signInAs(t, f.pool, createStudent(t, f.pool, SeedSchoolID, "gr-stranger@demo.local"))
	if code := getJSON(t, f.h, stranger, "/api/v1/writings/"+atomID+"/gradings", nil); code != http.StatusNotFound {
		t.Fatalf("stranger read = %d", code)
	}
	if code, _ := writeErrorCode(t, f.h, stranger, "POST", "/api/v1/lite/inbox/gradings/"+gid+"/seen", nil); code != http.StatusNotFound {
		t.Fatalf("stranger seen = %d", code)
	}

	// The teacher's item page shows the latest version's grading.
	var item struct {
		Writing struct {
			Grading *struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"grading"`
		} `json:"writing"`
	}
	if code := getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/classes/"+f.classID+"/students/"+f.studentID.String()+"/items/"+atomID, &item); code != http.StatusOK {
		t.Fatalf("item = %d", code)
	}
	if item.Writing.Grading == nil || item.Writing.Grading.ID != gid || item.Writing.Grading.Status != "sent" {
		t.Fatalf("item grading = %+v", item.Writing.Grading)
	}
}

// TestStudentGradingsOnlySentAcrossEveryStatus (Ruling 1): one writing atom
// with a grading row sitting in every non-sent status — including "running"
// and a "draft" row that still carries a leftover error from a failed
// regrade — plus one sent row. Only the sent row is ever returned, and its
// raw JSON keys (not just the Go struct) never carry ai/status/error/
// requested_by/reviewed_at.
func TestStudentGradingsOnlySentAcrossEveryStatus(t *testing.T) {
	f := newGradingFixture(t)
	_, atomID, student := f.submit(t)
	aID := uuid.MustParse(atomID)
	classID := uuid.MustParse(f.classID)
	ctx := context.Background()

	rubric := []byte(`{"scale":"letter","dimensions":[{"name":"内容","note":"x"}]}`)
	ai := []byte(`{"overall":{"grade":"B","comment":"x"},"dimensions":[],"points":[]}`)
	content := []byte(`{"overall":{"grade":"B+","comment":"x"},"dimensions":[],"points":[]}`)
	errText := "批改失败：模型无响应"

	mkVersion := func(n int) uuid.UUID {
		var vid uuid.UUID
		if err := f.pool.QueryRow(ctx,
			`INSERT INTO writing_version (atom_id, number, title, body, word_count) VALUES ($1,$2,'t','b',1) RETURNING id`,
			aID, n).Scan(&vid); err != nil {
			t.Fatal(err)
		}
		return vid
	}
	mkGrading := func(n int, status string, aiVal, contentVal []byte, errVal *string) uuid.UUID {
		vid := mkVersion(n)
		var gid uuid.UUID
		if err := f.pool.QueryRow(ctx, `
			INSERT INTO lite_grading (atom_id, version_id, user_id, class_id, rubric, status, ai, content, error, requested_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$3) RETURNING id`,
			aID, vid, f.studentID, classID, rubric, status, aiVal, contentVal, errVal).Scan(&gid); err != nil {
			t.Fatal(err)
		}
		return gid
	}

	mkGrading(2, "queued", nil, nil, nil)
	mkGrading(3, "running", nil, nil, nil)
	mkGrading(4, "draft", ai, content, nil)
	mkGrading(5, "draft", ai, content, &errText) // a regrade that failed but kept its previous draft
	mkGrading(6, "failed", nil, nil, &errText)
	sentGID := mkGrading(7, "sent", ai, content, nil)
	if _, err := f.pool.Exec(ctx, `UPDATE lite_grading SET sent_at = now() WHERE id = $1`, sentGID); err != nil {
		t.Fatal(err)
	}

	got := studentGradings(t, f, student, atomID)
	if len(got) != 1 || got[0]["id"] != sentGID.String() {
		t.Fatalf("only the sent row should be visible, got %v", got)
	}
	for _, forbidden := range []string{"ai", "status", "error", "requestedBy", "requested_by", "reviewedAt", "reviewed_at"} {
		if _, ok := got[0][forbidden]; ok {
			t.Fatalf("student response leaks %q: %v", forbidden, got[0])
		}
	}
}
