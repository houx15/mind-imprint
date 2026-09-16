package api_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestObservationSubmissionKeepsTaskOpenAndUsesReceipt(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"开始"}`), evidenceScript(`{"reply":"请先判断这个指标能说明什么"}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	base := "/api/v1/pbl/projects/" + pid
	siteReq(t, h, c, "POST", base+"/turn", `{"text":"开始"}`)
	tid := uuid.New()
	if _, err := pool.Exec(context.Background(), `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status) VALUES($1,$2,'observe','观察','world','accepted')`, tid, pid); err != nil {
		t.Fatal(err)
	}
	draft := base + "/tools/" + tid.String() + "/observation-draft"
	if rec := siteReq(t, h, c, "PUT", draft, `{"revision":0,"document":[{"kind":"question","body":"尚未观察，数字能帮助决定做什么吗？","imageKey":""}]}`); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if rec := siteReq(t, h, c, "POST", draft+"/submit", `{"revision":1}`); rec.Code != 201 {
		t.Fatal(rec.Body)
	}
	for _, body := range []string{
		`{"observation":{"toolId":"` + tid.String() + `","revision":0}}`,
		`{"text":"另一条消息","observation":{"toolId":"` + tid.String() + `","revision":1}}`,
	} {
		if rec := siteReq(t, h, c, "POST", base+"/turn", body); rec.Code < 400 {
			t.Fatal("invalid event accepted", rec.Body)
		}
	}
	other := newProjectViaAPI(t, h, c)
	event := `{"observation":{"toolId":"` + tid.String() + `","revision":1}}`
	if rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+other+"/turn", event); rec.Code != 409 {
		t.Fatal(rec.Body)
	}
	if rec := siteReq(t, h, c, "POST", base+"/turn", event); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM pbl_tool_instance WHERE id=$1`, tid).Scan(&status); err != nil || status != "accepted" {
		t.Fatal("submission ended task", status, err)
	}
	if len(provider.Requests) != 3 {
		t.Fatalf("expected coach and evidence calls: %d", len(provider.Requests))
	}
	coach, _ := json.Marshal(provider.Requests[1])
	review, _ := json.Marshal(provider.Requests[2])
	for _, raw := range []string{string(coach), string(review)} {
		if !strings.Contains(raw, "尚未观察，数字能帮助决定做什么吗？") {
			t.Fatal("question omitted from model evidence", raw)
		}
	}
	if !strings.Contains(string(coach), "观察任务仍在进行") {
		t.Fatal("missing ongoing status")
	}
}

func TestExplicitObservationEndDoesNotReplaySubmission(t *testing.T) {
	// 🚨 结束观察那一轮**也会过一次证据核对**，所以脚本要三条，不是两条 ——
	// 少的那一条不会报「脚本用完了」，它会让核对连着两次解析失败，整轮变成
	// ai_dialogue_failed。上面那条用例（第 13 行）给的就是三条。
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"开始"}`),
		evidenceScript(`{"reply":"本次任务已结束，下一步需要确定要验证的问题"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	base := "/api/v1/pbl/projects/" + pid
	siteReq(t, h, c, "POST", base+"/turn", `{"text":"开始"}`)
	tid := uuid.New()
	if _, err := pool.Exec(context.Background(), `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status,result,resolved_at) VALUES($1,$2,'observe','观察','world','done','{"endObservation":true}',now())`, tid, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO pbl_observation_draft(tool_id,submitted_revision,submitted_notes) VALUES($1,1,'[{"kind":"question","body":"上次已经讨论的问题"}]')`, tid); err != nil {
		t.Fatal(err)
	}
	if rec := siteReq(t, h, c, "POST", base+"/turn", `{"completedToolId":"`+tid.String()+`"}`); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	raw, _ := json.Marshal(provider.Requests[1])
	start := strings.LastIndex(string(raw), "【她刚做完这件事】")
	if start < 0 {
		t.Fatal("missing event")
	}
	event := string(raw)[start:]
	if !strings.Contains(event, "学生明确结束了本次观察任务") || strings.Contains(event, "上次已经讨论的问题") {
		t.Fatal(event)
	}
}
