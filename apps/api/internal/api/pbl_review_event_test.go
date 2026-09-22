package api_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestReviewCompletionCarriesAuthoritativeRevision(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"请查看修改意见"}`),
		evidenceScript(`{"reply":"请查看修改意见"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"开始测试"}`)
	aid := uuid.New()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO pbl_artifact(id,atom_id,kind,title,payload,guessed,admits,verdict,why,settled_at) VALUES($1,$2,'site','观察主页','{}','[]','[]','revise','移除三个空照片模块',now())`, aid, pid)
	if err != nil {
		t.Fatal(err)
	}
	// The client result lies about approval: only the saved artifact is authoritative.
	result, _ := json.Marshal(map[string]string{"artifactId": aid.String(), "verdict": "kept"})
	_, err = pool.Exec(ctx, `INSERT INTO pbl_tool_instance(atom_id,tool,reason,kind,status,result,resolved_at) VALUES($1,'review','审查主页','thinking','done',$2,now())`, pid, result)
	if err != nil {
		t.Fatal(err)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":""}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if len(provider.Requests) != 3 {
		t.Fatalf("expected two coach calls and one evidence check, got %d", len(provider.Requests))
	}
	b, _ := json.Marshal(provider.Requests[1])
	if !strings.Contains(string(b), "但未通过成果") || !strings.Contains(string(b), "移除三个空照片模块") {
		t.Fatalf("revision missing from actual model input: %s", b)
	}
}

func TestExplicitCompletionWinsOverNewerTool(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"请开始"}`),
		evidenceScript(`{"reply":"请记录一次真实试用反馈"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"开始测试"}`)
	completed := uuid.New()
	_, err := pool.Exec(context.Background(), `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status,result,student_note,resolved_at) VALUES($1,$2,'ship','展示主页','thinking','done','{}','指定完成事件',now()-interval '1 minute'),($3,$2,'ideas','其他事件','thinking','done','{}','较新的无关事件',now())`, completed, pid, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"","completedToolId":"`+completed.String()+`"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if len(provider.Requests) < 2 {
		t.Fatal("missing coach request")
	}
	b, _ := json.Marshal(provider.Requests[1])
	start := strings.LastIndex(string(b), "【学生刚做完这件事】")
	if start < 0 {
		t.Fatal("missing event")
	}
	event := string(b)[start:]
	if !strings.Contains(event, "指定完成事件") || strings.Contains(event, "较新的无关事件") || !strings.Contains(event, "不要自动重做历史修改") {
		t.Fatalf("wrong completion: %s", event)
	}
}

func TestObservationCompletionCarriesSubmittedQuestion(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"开始"}`), evidenceScript(`{"reply":"请给访客一个查找制作过程的任务"}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"开始"}`)
	tid := uuid.New()
	_, err := pool.Exec(context.Background(), `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status,resolved_at) VALUES($1,$2,'observe','观察','world','done',now())`, tid, pid)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO pbl_observation_draft(tool_id,submitted_revision,submitted_notes) VALUES($1,1,'[{"kind":"question","body":"尚未试用，应该给访客什么任务？"}]')`, tid)
	if err != nil {
		t.Fatal(err)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"","completedToolId":"`+tid.String()+`"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	b, _ := json.Marshal(provider.Requests[1])
	start := strings.LastIndex(string(b), "【学生刚做完这件事】")
	if start < 0 {
		t.Fatal("missing event")
	}
	event := string(b)[start:]
	if !strings.Contains(event, "尚未试用，应该给访客什么任务") || !strings.Contains(event, "类型：question") || !strings.Contains(event, "不代表已外出") {
		t.Fatal(event)
	}
}

// The independent checker must receive the same completion event as the coach;
// otherwise it may repair a valid continuation back into an obsolete request.
func TestBoardCompletionReachesCoachAndEvidenceChecker(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"开始"}`),
		evidenceScript(`{"reply":"这仍是假设，接下来比较另一种可能。"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"开始"}`)
	tid := uuid.New()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status,result,resolved_at) VALUES($1,$2,'board','比较预测','thinking','done','{}',now())`, tid, pid)
	if err != nil {
		t.Fatal(err)
	}
	prediction := "如果杯口偏离，我猜接水台可能有水滴；尚未现场观察。"
	_, err = pool.Exec(ctx, `INSERT INTO pbl_note(atom_id,kind,body,author) VALUES($1,'assumption',$2,'student')`, pid, prediction)
	if err != nil {
		t.Fatal(err)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"","completedToolId":"`+tid.String()+`"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if len(provider.Requests) != 3 {
		t.Fatalf("unexpected requests: %d", len(provider.Requests))
	}
	for _, index := range []int{1, 2} {
		raw, _ := json.Marshal(provider.Requests[index])
		if !strings.Contains(string(raw), "这是思考板完成后的回流") || !strings.Contains(string(raw), prediction) {
			t.Fatalf("request %d lost completion or prediction: %s", index, raw)
		}
	}
}

func TestEndedObservationRevisionRepairsToNewTask(t *testing.T) {
	old := uuid.New()
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"已更新旧清单","mission_target":"`+old.String()+`","mission":[{"prompt":"观察水的来源","want_kind":"observation"}]}`),
		evidenceScript(`{"reply":"已准备新的观察任务","tool":"observe","tool_reason":"继续观察","mission":[{"prompt":"观察水的来源，看不清请记录看不清","want_kind":"observation"}]}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	_, err := pool.Exec(context.Background(), `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status,result,resolved_at) VALUES($1,$2,'observe','旧观察','world','done','{}',now())`, old, pid)
	if err != nil {
		t.Fatal(err)
	}
	rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"请修改清单供下次观察"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if len(provider.Requests) != 3 {
		t.Fatalf("expected draft, repair, check; got %d", len(provider.Requests))
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM pbl_tool_instance WHERE id=$1`, old).Scan(&status); err != nil || status != "done" {
		t.Fatalf("old task changed: %s %v", status, err)
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pbl_tool_instance WHERE atom_id=$1 AND tool='observe' AND status='summoned'`, pid).Scan(&count); err != nil || count != 1 {
		t.Fatalf("new task missing: %d %v", count, err)
	}
	thread := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/thread", "")
	if strings.Contains(thread.Body.String(), "已更新旧清单") {
		t.Fatal("invalid success was published")
	}
}
