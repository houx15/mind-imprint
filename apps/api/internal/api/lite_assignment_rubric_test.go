package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"mindimprint/api/internal/liteassign"
)

type assignmentPayloadResp struct {
	Assignment struct {
		ID      string          `json:"id"`
		Payload json.RawMessage `json:"payload"`
	} `json:"assignment"`
}

// The rubric stays editable after a student started, through its own field;
// kind and payload stay locked.
func TestAssignmentRubricEditableAfterStart(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	aid, _, _ := startWritingHomework(t, h, pool, teacher, classID, studentID)
	path := "/api/v1/lite/teacher/assignments/" + aid
	points := map[string]any{"scale": "points", "max": 20, "dimensions": []map[string]any{{"name": "论证", "note": ""}}, "focus": "重点看论证"}

	var resp assignmentPayloadResp
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{"rubric": points}, &resp); code != http.StatusOK {
		t.Fatalf("patch rubric after start = %d", code)
	}
	if r := liteassign.EffectiveRubric(resp.Assignment.Payload); r.Scale != "points" || r.Max != 20 || r.Focus != "重点看论证" {
		t.Fatalf("stored rubric = %+v", r)
	}

	// Resending unchanged settings is not a change, and a rubric inside the
	// payload does not overwrite the stored one.
	same := map[string]any{"kind": "writing", "payload": map[string]any{
		"prompt": "写一篇关于雨的记叙文", "targetWords": 800, "lang": "zh",
		"rubric": map[string]any{"scale": "letter", "dimensions": []map[string]any{{"name": "x", "note": ""}}, "focus": ""},
	}}
	if code := assignJSON(t, h, teacher, "PATCH", path, same, &resp); code != http.StatusOK {
		t.Fatalf("resend settings = %d", code)
	}
	if r := liteassign.EffectiveRubric(resp.Assignment.Payload); r.Scale != "points" {
		t.Fatalf("payload rubric overwrote the stored one: %+v", r)
	}

	changed := map[string]any{"payload": map[string]any{"prompt": "换一个题目", "targetWords": 800, "lang": "zh"}}
	if code, ec := writeErrorCode(t, h, teacher, "PATCH", path, changed); code != http.StatusConflict || ec != "assignment_started" {
		t.Fatalf("prompt change after start = %d %s", code, ec)
	}
	badRubric := map[string]any{"rubric": map[string]any{"scale": "points", "max": 0, "dimensions": []map[string]any{{"name": "论证"}}}}
	if code, ec := writeErrorCode(t, h, teacher, "PATCH", path, badRubric); code != http.StatusBadRequest || ec != "invalid_rubric_max" {
		t.Fatalf("invalid rubric = %d %s", code, ec)
	}
	if code := assignJSON(t, h, teacher, "PATCH", path, map[string]any{"rubric": nil}, &resp); code != http.StatusOK {
		t.Fatalf("reset rubric = %d", code)
	}
	if r := liteassign.EffectiveRubric(resp.Assignment.Payload); r.Scale != "letter" || r.Dimensions[0].Name != "内容" {
		t.Fatalf("after reset = %+v, want the zh default", r)
	}

	reading := createAssignment(t, h, teacher, classID, readingAssignmentBody("读", map[string]any{"source": "text", "text": "一段正文。"}, []string{studentID.String()}))
	if code, ec := writeErrorCode(t, h, teacher, "PATCH", "/api/v1/lite/teacher/assignments/"+reading, map[string]any{"rubric": points}); code != http.StatusBadRequest || ec != "rubric_not_writing" {
		t.Fatalf("rubric on reading = %d %s", code, ec)
	}
}

// A writing homework created without a rubric reads back with the zh default
// baked into its payload, everywhere the teacher end shapes that payload
// (create, list, detail): the frontend never keeps its own copy of the Go
// default names and notes.
func TestAssignmentPayloadCarriesDefaultRubric(t *testing.T) {
	h, _, teacher, classID, studentID := liteTeacherFixture(t)

	var created assignmentPayloadResp
	if code := assignJSON(t, h, teacher, "POST", "/api/v1/lite/teacher/classes/"+classID+"/assignments",
		writingAssignmentBody([]string{studentID.String()}), &created); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	aid := created.Assignment.ID
	if r := liteassign.EffectiveRubric(created.Assignment.Payload); r.Scale != "letter" || r.Dimensions[0].Name != "内容" {
		t.Fatalf("create response = %+v", r)
	}

	var list struct {
		Assignments []struct {
			Payload json.RawMessage `json:"payload"`
		} `json:"assignments"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/assignments", &list); code != http.StatusOK {
		t.Fatalf("list = %d", code)
	}
	if len(list.Assignments) != 1 {
		t.Fatalf("list count = %d", len(list.Assignments))
	}
	if r := liteassign.EffectiveRubric(list.Assignments[0].Payload); r.Scale != "letter" || r.Dimensions[0].Name != "内容" {
		t.Fatalf("list payload = %+v", r)
	}

	var detail assignmentPayloadResp
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/assignments/"+aid, &detail); code != http.StatusOK {
		t.Fatalf("detail = %d", code)
	}
	if r := liteassign.EffectiveRubric(detail.Assignment.Payload); r.Scale != "letter" || r.Dimensions[0].Name != "内容" {
		t.Fatalf("detail payload = %+v", r)
	}
}
