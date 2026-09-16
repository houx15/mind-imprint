package api_test

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHomepageProgressRequiresSavedWorkAndKeepsPlanStatus(t *testing.T) {
	h, c, _, pool := liteHandler(t)
	pid := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	read := func() []struct{ Status, Progress string } {
		t.Helper()
		rec := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/plan", "")
		var result struct {
			Plan struct {
				Steps []struct{ Status, Progress string }
			}
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Plan.Steps) != 5 {
			t.Fatal(rec.Body)
		}
		return result.Plan.Steps
	}
	if got := read()[0].Progress; got != "todo" {
		t.Fatalf("empty account: %s", got)
	}
	_, err := pool.Exec(context.Background(), `INSERT INTO pbl_tool_instance(atom_id,tool,reason,kind,status,resolved_at) VALUES($1,'persona','确认受众','thinking','done',now()),($1,'sites','采集','thinking','done',now())`, pid)
	if err != nil {
		t.Fatal(err)
	}
	if got := read()[0].Progress; got != "doing" {
		t.Fatalf("tool completion alone must not complete step: %s", got)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/personas", `{"label":"摄影社同学","wants":"观察树木","keywords":["自然"]}`)
	if rec.Code != 201 && rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	steps := read()
	if steps[0].Progress != "done" || steps[0].Status != "tentative" {
		t.Fatalf("progress must not overwrite status: %+v", steps[0])
	}
	if steps[1].Progress != "doing" {
		t.Fatal("creative exploration should follow confirmed audience")
	}
	if steps[2].Progress != "todo" {
		t.Fatal("structure should wait for creative trial")
	}
	rec = siteReq(t, h, c, "GET", "/api/v1/pbl/projects", "")
	var projects []struct {
		ID, CurrentStep       string
		StepsDone, StepsTotal int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &projects); err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].ID != pid || projects[0].StepsDone != 1 || projects[0].StepsTotal != 5 || projects[0].CurrentStep != "构思与试用第一幕" {
		t.Fatalf("project card disagrees with saved homepage progress: %s", rec.Body)
	}
}
