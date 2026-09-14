package api_test

import (
	"context"
	"net/http"
	"testing"

	. "mindimprint/api/internal/api"
)

// TestLiteTeacherTreeShowsStudentsKeywords is the RED test for the teacher's
// read-only view of a student's interest tree: same JSON shape as her own
// GET /api/v1/interest/tree, scoped to the student in the path.
func TestLiteTeacherTreeShowsStudentsKeywords(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO interest_keyword (user_id, text_zh, text_en, norm, field, strength, note) VALUES ($1, '金融', 'finance', '金融', 'society', 3, '')`, studentID); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Keywords []struct {
			TextZh string `json:"textZh"`
		} `json:"keywords"`
		Fields []struct {
			ID string `json:"id"`
		} `json:"fields"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/tree", &resp); code != http.StatusOK {
		t.Fatalf("tree = %d", code)
	}
	if len(resp.Keywords) != 1 || resp.Keywords[0].TextZh != "金融" || len(resp.Fields) == 0 {
		t.Fatalf("tree = %+v", resp)
	}
}

// TestLiteTeacherTreeStudentOfOtherClass404 is the authz guard: a teacher who
// does not own the class gets 404, same posture as the roster/item routes.
func TestLiteTeacherTreeAuthz(t *testing.T) {
	h, pool, _, classID, studentID := liteTeacherFixture(t)
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-tree-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/tree", nil); code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404", code)
	}
}
