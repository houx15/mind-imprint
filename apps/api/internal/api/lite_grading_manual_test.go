package api_test

// 人工批改: a grading the teacher writes herself. The owner found on
// 2026-09-17 that every grading had to start from an AI run.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestLiteGradingManual(t *testing.T) {
	f := newGradingFixture(t)
	_, atomID, student := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	manual := map[string]any{"mode": "manual"}

	var resp struct {
		Grading struct {
			teacherGradingView
			Source string `json:"source"`
		} `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, manual, &resp); code != http.StatusOK {
		t.Fatalf("manual = %d", code)
	}
	g := resp.Grading
	if g.Status != "draft" || g.Source != "teacher" || g.ReviewedAt != nil {
		t.Fatalf("manual row = %+v", g)
	}
	// A blank sheet: the rubric's four dimensions, nothing filled in, no points.
	var c struct {
		Overall    struct{ Grade, Comment string }
		Dimensions []struct{ Name, Grade, Comment string }
		Points     []any
	}
	if err := json.Unmarshal(g.Content, &c); err != nil {
		t.Fatalf("content %s: %v", g.Content, err)
	}
	if c.Overall.Grade != "" || len(c.Dimensions) != 4 || c.Dimensions[0].Name != "内容" || c.Dimensions[0].Grade != "" || c.Points == nil || len(c.Points) != 0 {
		t.Fatalf("blank content = %s", g.Content)
	}
	// No model call, no job.
	if n := countGradingCalls(t, f.pool); n != 0 {
		t.Fatalf("model calls = %d", n)
	}
	if jobs := f.enq.drain(); len(jobs) != 0 {
		t.Fatalf("jobs = %v", jobs)
	}

	// A second request, either kind, opens the same row instead of replacing it.
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", single, manual); code != http.StatusConflict || ec != "grading_exists" {
		t.Fatalf("manual again = %d %s", code, ec)
	}
	if code, ec := writeErrorCode(t, f.h, f.teacher, "POST", single, nil); code != http.StatusConflict || ec != "grading_exists" {
		t.Fatalf("AI over manual = %d %s", code, ec)
	}

	// Saving without grades is refused; with grades it saves and sends.
	path := "/api/v1/lite/teacher/gradings/" + g.ID
	empty := map[string]any{"overall": map[string]any{"grade": "", "comment": "写得不错"}, "dimensions": c.Dimensions, "points": []any{}}
	rec := doJSON(t, f.h, f.teacher, "PATCH", path, mustJSON(t, map[string]any{"content": empty}))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "总评的等级不在评分标准内") {
		t.Fatalf("save without grade = %d %s", rec.Code, rec.Body)
	}
	point := []map[string]any{{"kind": "issue", "quote": nil, "text": "第二段请补充数据来源。", "action": "补一句数据出处。", "source": "teacher"}}
	if code := assignJSON(t, f.h, f.teacher, "PATCH", path, map[string]any{"content": gradingContent("B", point)}, nil); code != http.StatusOK {
		t.Fatalf("save = %d", code)
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", path+"/send", nil, nil); code != http.StatusOK {
		t.Fatalf("send = %d", code)
	}

	// The student reads it as the teacher's, not as AI-drafted.
	var mine struct {
		Gradings []struct {
			Source string `json:"source"`
		} `json:"gradings"`
	}
	if code := getJSON(t, f.h, student, "/api/v1/writings/"+atomID+"/gradings", &mine); code != http.StatusOK || len(mine.Gradings) != 1 || mine.Gradings[0].Source != "teacher" {
		t.Fatalf("student view = %d %+v", code, mine)
	}
}

// 重新批改 is allowed over a manual draft: she may start by hand and then ask
// the AI after all.
func TestLiteGradingManualThenAI(t *testing.T) {
	f := newGradingFixture(t, gradingValidReply)
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Source string `json:"source"`
		} `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, map[string]any{"mode": "manual"}, &resp); code != http.StatusOK {
		t.Fatalf("manual = %d", code)
	}
	gid := resp.Grading.ID
	if code := assignJSON(t, f.h, f.teacher, "POST", "/api/v1/lite/teacher/gradings/"+gid+"/regrade", nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" {
		t.Fatalf("regrade manual = %d %+v", code, resp.Grading)
	}
	f.runJobs(t)
	if code := getJSON(t, f.h, f.teacher, "/api/v1/lite/teacher/gradings/"+gid, &resp); code != http.StatusOK || resp.Grading.Status != "draft" || resp.Grading.Source != "ai" {
		t.Fatalf("after AI = %d %+v", code, resp.Grading)
	}
}

// 人工批改 does not need the job queue.
func TestLiteGradingManualWithoutQueue(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "gm-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Manual Class")
	studentID := createStudent(t, pool, SeedSchoolID, "gm-student@demo.local")
	enrollStudent(t, pool, studentID, classID)
	_, atomID, student := startWritingHomework(t, h, pool, teacher, classID, studentID)
	if code := assignJSON(t, h, student, "POST", "/api/v1/writings/"+atomID+"/finish", nil, nil); code != http.StatusOK {
		t.Fatalf("finish = %d", code)
	}
	single := "/api/v1/lite/teacher/classes/" + classID + "/students/" + studentID.String() + "/items/" + atomID + "/gradings"
	if code, ec := writeErrorCode(t, h, teacher, "POST", single, nil); code != http.StatusServiceUnavailable || ec != "grading_queue_unavailable" {
		t.Fatalf("AI without queue = %d %s", code, ec)
	}
	if code := assignJSON(t, h, teacher, "POST", single, map[string]any{"mode": "manual"}, nil); code != http.StatusOK {
		t.Fatalf("manual without queue = %d", code)
	}
}

// An AI grading that has not produced anything yet (queued, or failed on its
// first run) is still an AI grading. Real-user walk, 2026-09-17: a failed one
// was labelled 「人工批改」.
func TestLiteGradingQueuedAIIsNotManual(t *testing.T) {
	f := newGradingFixture(t)
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading struct {
			Status string `json:"status"`
			Source string `json:"source"`
		} `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK || resp.Grading.Status != "queued" || resp.Grading.Source != "ai" {
		t.Fatalf("queued AI grading = %d %+v", code, resp.Grading)
	}
}

// When the AI fails and leaves nothing, 人工批改 takes over that version.
// Real-user walk, 2026-09-17: the model failed three runs in a row and the
// teacher had no way to grade the essay herself.
func TestLiteGradingManualAfterAFailedAI(t *testing.T) {
	f := newGradingFixture(t, "抱歉，我无法批改。")
	_, atomID, _ := f.submit(t)
	single := "/api/v1/lite/teacher/classes/" + f.classID + "/students/" + f.studentID.String() + "/items/" + atomID + "/gradings"
	var resp struct {
		Grading struct {
			ID      string          `json:"id"`
			Status  string          `json:"status"`
			Source  string          `json:"source"`
			Content json.RawMessage `json:"content"`
		} `json:"grading"`
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, nil, &resp); code != http.StatusOK {
		t.Fatalf("AI = %d", code)
	}
	gid := resp.Grading.ID
	f.runJobs(t)
	if g := f.grading(t, gid); g.Status != "failed" {
		t.Fatalf("after two bad replies = %+v", g)
	}
	if code := assignJSON(t, f.h, f.teacher, "POST", single, map[string]any{"mode": "manual"}, &resp); code != http.StatusOK {
		t.Fatalf("manual after failure = %d", code)
	}
	if resp.Grading.ID != gid || resp.Grading.Status != "draft" || resp.Grading.Source != "teacher" || !strings.Contains(string(resp.Grading.Content), `"name":"内容"`) {
		t.Fatalf("manual after failure = %+v %s", resp.Grading, resp.Grading.Content)
	}
	if g := f.grading(t, gid); g.Error != nil {
		t.Fatalf("the old failure is still shown: %v", *g.Error)
	}
}
