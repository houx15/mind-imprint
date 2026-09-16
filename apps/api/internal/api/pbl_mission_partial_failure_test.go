package api_test

import (
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

// Inject a storage failure after the mission commit, then retry only the failed
// artifact. This checks real handlers and DB state, not a simulated UI receipt.
func TestMissionSuccessArtifactFailureCanRecoverWithoutRepeatingMission(t *testing.T) {
	tid := uuid.New()
	artifact := `{"kind":"artifact","payload":{"kind":"draft","title":"方案","body":"每盘只在对应类别画一笔","marks":[],"dimensions":[]}}`
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"开始"}`),
		evidenceScript(`{"reply":"清单和方案已更新","hook":"使用新方案","mission_target":"`+tid.String()+`","mission":[{"prompt":"每盘只在对应类别画一笔","want_kind":"observation"}],"produce":`+artifact+`}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
		evidenceScript(`{"reply":"方案已保存","produce":`+artifact+`}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	base := "/api/v1/pbl/projects/" + pid
	if r := siteReq(t, h, c, "POST", base+"/turn", `{"text":"开始"}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status) VALUES($1,$2,'observe','观察','world','accepted')`, tid, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_mission_item(tool_id,prompt,want_kind,ordinal) VALUES($1,'原任务','observation',0)`, tid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `CREATE FUNCTION test_artifact_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test artifact storage unavailable'; END $$;
 CREATE TRIGGER test_artifact_failure BEFORE INSERT ON pbl_artifact FOR EACH ROW EXECUTE FUNCTION test_artifact_failure();`); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(t.Context(), `DROP TRIGGER IF EXISTS test_artifact_failure ON pbl_artifact; DROP FUNCTION IF EXISTS test_artifact_failure();`)
	response := siteReq(t, h, c, "POST", base+"/turn", `{"text":"同步修改清单和方案"}`)
	var result struct{ Reply, Hook string }
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || result.Reply != "本轮操作未完成，请查看下方错误信息。" || result.Hook != "" {
		t.Fatal(response.Code, result)
	}
	thread := siteReq(t, h, c, "GET", base+"/thread", "")
	for _, notice := range []string{"观察清单已更新", "成果保存失败"} {
		if !strings.Contains(thread.Body.String(), notice) {
			t.Fatal("missing partial-state receipt", notice, thread.Body)
		}
	}
	var missions, artifacts, active, done int
	counts := func() {
		t.Helper()
		if err := pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER(WHERE superseded_at IS NULL),count(done_at) FROM pbl_mission_item WHERE tool_id=$1`, tid).Scan(&missions, &active, &done); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM pbl_artifact WHERE atom_id=$1`, pid).Scan(&artifacts); err != nil {
			t.Fatal(err)
		}
	}
	counts()
	if missions != 2 || active != 1 || done != 0 || artifacts != 0 {
		t.Fatal("wrong partial state", missions, active, done, artifacts)
	}
	if _, err := pool.Exec(t.Context(), `DROP TRIGGER test_artifact_failure ON pbl_artifact; DROP FUNCTION test_artifact_failure();`); err != nil {
		t.Fatal(err)
	}
	response = siteReq(t, h, c, "POST", base+"/turn", `{"text":"清单已经更新，只重试保存方案"}`)
	if response.Code != 200 {
		t.Fatal(response.Body)
	}
	counts()
	if missions != 2 || active != 1 || done != 0 || artifacts != 1 {
		t.Fatal("retry repeated successful operation", missions, active, done, artifacts)
	}
}
