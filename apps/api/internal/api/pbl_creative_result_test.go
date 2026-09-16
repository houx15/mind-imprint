package api_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTeacherFiltersLegacyCreativeDraftFields(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atomID := seedLiteWebsiteProject(t, pool, studentID)
	_, err := pool.Exec(t.Context(), `INSERT INTO pbl_tool_instance(atom_id,tool,kind,reason,status,result) VALUES($1,'creative','thinking','test','done','{"feeling":"confirmed choice","pendingMotif":"PRIVATE_MOTIF","responseDrafts":{"v":{"feedback":"PRIVATE_RESPONSE"}}}')`, atomID)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	path := "/api/v1/lite/teacher/classes/" + classID + "/students/" + studentID.String() + "/items/" + atomID.String()
	if code := getJSON(t, h, teacher, path, &result); code != 200 {
		t.Fatal(code)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "PRIVATE_") || !strings.Contains(string(raw), "confirmed choice") {
		t.Fatal(string(raw))
	}
}

func TestCreativeToolResultExcludesUnsubmittedDrafts(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	url := "/api/v1/pbl/projects/" + id + "/tools"
	r := pblPost(t, h, c, url, `{"tool":"creative","reason":"确认创作方向"}`)
	if r.Code != 201 {
		t.Fatal(r.Body)
	}
	tid := artifactID(t, r.Body.String())
	r = pblPost(t, h, c, url+"/"+tid+"/resolve", `{"status":"done","result":{"feeling":"已选择的风格","pendingMotif":"PRIVATE_MOTIF","responseDrafts":{"version":{"feedback":"PRIVATE_RESPONSE"}},"trial":{"observation":"已提交的试用判断"}}}`)
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	var result string
	if err := pool.QueryRow(t.Context(), `SELECT result::text FROM pbl_tool_instance WHERE id=$1`, tid).Scan(&result); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{result, r.Body.String()} {
		if strings.Contains(value, "PRIVATE_") || strings.Contains(value, "responseDrafts") || strings.Contains(value, "pendingMotif") {
			t.Fatal("draft escaped", value)
		}
		if !strings.Contains(value, "已选择的风格") || !strings.Contains(value, "已提交的试用判断") {
			t.Fatal("confirmed work lost", value)
		}
	}
}
