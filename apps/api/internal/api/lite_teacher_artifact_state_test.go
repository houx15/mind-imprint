package api_test

import (
	"net/http"
	"testing"
)

func TestTeacherArtifactStateUsesSettledStudentVerdict(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atomID := seedLiteWebsiteProject(t, pool, studentID)
	for _, verdict := range []string{"pending", "kept", "revise", "dropped"} {
		var err error
		if verdict == "pending" {
			_, err = pool.Exec(t.Context(), `INSERT INTO pbl_artifact(atom_id,kind,title,payload) VALUES($1,'draft',$2,'{}')`, atomID, verdict)
		} else {
			_, err = pool.Exec(t.Context(), `INSERT INTO pbl_artifact(atom_id,kind,title,payload,verdict,why,settled_at) VALUES($1,'draft',$2,'{}',$2,'学生实际判断',now())`, atomID, verdict)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var response struct {
		Project struct {
			Artifacts []struct {
				ID        string
				Title     string
				Verdict   string
				Why       string
				CreatedAt string
			}
		}
	}
	path := "/api/v1/lite/teacher/classes/" + classID + "/students/" + studentID.String() + "/items/" + atomID.String()
	if code := getJSON(t, h, teacher, path, &response); code != http.StatusOK {
		t.Fatal(code)
	}
	if len(response.Project.Artifacts) != 4 {
		t.Fatal(response)
	}
	for _, artifact := range response.Project.Artifacts {
		if artifact.Verdict != artifact.Title || artifact.ID == "" || artifact.CreatedAt == "" {
			t.Fatal(artifact)
		}
		if artifact.Verdict == "pending" && artifact.Why != "" {
			t.Fatal("pending artifact has a student verdict", artifact)
		}
		if artifact.Verdict != "pending" && artifact.Why != "学生实际判断" {
			t.Fatal(artifact)
		}
	}
}
