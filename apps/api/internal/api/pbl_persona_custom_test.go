package api_test

import (
	"encoding/json"
	. "mindimprint/api/internal/api"
	"net/http"
	"testing"
)

func TestCustomPersona_EmptyStudentCanChooseWithoutModel(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/personas"
	body := `{"label":"摄影社新同学","wants":"了解我拍的校园树木","keywords":["清晰","自然","清晰"]}`
	rec := siteReq(t, h, c, "POST", url, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	got := decodeSite(t, rec)
	if got["chosen"] != true || got["label"] != "摄影社新同学" || len(got["keywords"].([]any)) != 2 {
		t.Fatalf("choice: %v", got)
	}
	// A later student decision replaces the active choice, not the saved alternatives.
	rec = siteReq(t, h, c, "POST", url, `{"label":"班级同学","wants":"讨论拍摄地点","keywords":["简洁"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("second: %s", rec.Body)
	}
	rec = siteReq(t, h, c, "GET", url, "")
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	chosen := 0
	for _, row := range rows {
		if row["chosen"] == true {
			chosen++
			if row["label"] != "班级同学" {
				t.Fatal("old choice remains active")
			}
		}
	}
	if len(rows) != 2 || chosen != 1 {
		t.Fatalf("rows=%v", rows)
	}
	for _, invalid := range []string{`{}`, `{"label":"a","wants":"b","keywords":[]}`, `{"label":"a","wants":"b","keywords":[" "]}`} {
		if rec := siteReq(t, h, c, "POST", url, invalid); rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid accepted: %s", rec.Body)
		}
	}
	otherID := createStudent(t, pool, SeedSchoolID, "other-persona@demo.local")
	other := signInAs(t, pool, otherID)
	if rec := siteReq(t, h, other, "POST", url, body); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign create: %d", rec.Code)
	}
}
