package api_test

import (
	"context"
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func evidenceScript(raw string) []gateway.StreamEvent {
	return []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: raw}, {Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 20}}, {Kind: gateway.EventDone, StopReason: gateway.StopStop}}
}

func TestEvidenceGuardRepairsMalformedVerdictWithoutChangingCandidate(t *testing.T) {
	for _, repaired := range []bool{true, false} {
		t.Run(map[bool]string{true: "repair", false: "bounded_failure"}[repaired], func(t *testing.T) {
			invalid := `{"supported":false,"issues":[{"quote":"候选没有这句话","reason":"无法定位"}]}`
			last := invalid
			if repaired {
				last = `{"supported":true,"issues":[]}`
			}
			provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"还没有现场记录，只能讨论准备工作。"}`), evidenceScript(invalid), evidenceScript(last))
			h, c, _, pool := liteHandlerWithProvider(t, provider)
			id := newProjectViaAPI(t, h, c)
			if _, err := pool.Exec(context.Background(), `INSERT INTO pbl_note(atom_id,kind,body,author) VALUES($1,'question','还没去现场，如何准备？','student')`, id); err != nil {
				t.Fatal(err)
			}
			r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"怎么开始"}`)
			if (r.Code == 200) != repaired {
				t.Fatal(r.Code, r.Body)
			}
			if provider.Calls != 3 {
				t.Fatalf("format repair must be bounded, calls=%d", provider.Calls)
			}
			if provider.Requests[1].Messages[1].Content != provider.Requests[2].Messages[1].Content {
				t.Fatal("repair changed candidate or sources")
			}
			if !strings.Contains(provider.LastRequest.Messages[len(provider.LastRequest.Messages)-1].Content, "单个字符串值逐字复制") {
				t.Fatal("missing quote repair guidance")
			}
			var count int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM llm_call WHERE atom_id=$1`, id).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 3 {
				t.Fatalf("unmetered check: %d", count)
			}
			thread := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
			if strings.Contains(thread.Body.String(), "还没有现场记录，只能讨论准备工作。") != repaired {
				t.Fatal("invalid verdict allowed persistence or valid reply lost")
			}
		})
	}
}

func TestEvidenceGuardRepairsBeforePersistence(t *testing.T) {
	for _, repaired := range []bool{true, false} {
		t.Run(map[bool]string{true: "repair", false: "reject"}[repaired], func(t *testing.T) {
			bad := `{"reply":"这是现场事实"}`
			checkBad := `{"supported":false,"issues":[{"quote":"这是现场事实","reason":"原文明确为虚构"}]}`
			fixed, check := bad, checkBad
			if repaired {
				fixed = `{"reply":"这是虚构演练，不能证明真实社区情况"}`
				check = `{"supported":true,"issues":[]}`
			}
			provider := gateway.NewSequenceStubProvider(evidenceScript(bad), evidenceScript(checkBad), evidenceScript(fixed), evidenceScript(check))
			h, c, _, pool := liteHandlerWithProvider(t, provider)
			id := newProjectViaAPI(t, h, c)
			original := "【虚构测试记录，未进行实地观察】模拟情境：公告栏十分钟未发现信息。"
			_, err := pool.Exec(context.Background(), `INSERT INTO pbl_note(atom_id,kind,body,author) VALUES($1,'observation',$2,'student')`, id, original)
			if err != nil {
				t.Fatal(err)
			}
			r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"能下什么结论"}`)
			if repaired && r.Code != 200 {
				t.Fatal(r.Body)
			}
			if !repaired && r.Code == 200 {
				t.Fatal("unsupported reply accepted")
			}
			if provider.Calls != 4 {
				t.Fatalf("expected draft/check/repair/check, got %d", provider.Calls)
			}
			input, _ := json.Marshal(provider.LastRequest)
			if !strings.Contains(string(input), original) {
				t.Fatal("source qualifier missing from checker")
			}
			thread := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
			if strings.Contains(thread.Body.String(), "这是现场事实") {
				t.Fatal("rejected claim persisted")
			}
			if repaired && !strings.Contains(thread.Body.String(), "这是虚构演练") {
				t.Fatal("repair not persisted")
			}
			var calls int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM llm_call WHERE atom_id=$1`, id).Scan(&calls); err != nil {
				t.Fatal(err)
			}
			if calls != 4 {
				t.Fatalf("calls not all metered: %d", calls)
			}
		})
	}
}

func TestEvidenceGuardIncludesStudentReviewAnswers(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"将按你的审核意见修改。"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, cookie, _, pool := liteHandlerWithProvider(t, provider)
	id := newProjectViaAPI(t, h, cookie)
	other := newProjectViaAPI(t, h, cookie)
	for _, project := range []string{id, other} {
		var artifact string
		if err := pool.QueryRow(t.Context(), `INSERT INTO pbl_artifact(atom_id,kind,title) VALUES($1,'draft','记录表') RETURNING id`, project).Scan(&artifact); err != nil {
			t.Fatal(err)
		}
		answer := "请按一平汤匙标准记录不确定类别"
		if project == other {
			answer = "另一项目的审核答案不得进入"
		}
		if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_review_mark(artifact_id,question,answer) VALUES($1,'标准是什么',$2)`, artifact, answer); err != nil {
			t.Fatal(err)
		}
		verdictReason := "请区分原话与概括，并提供单人记录方法"
		if project == other {
			verdictReason = "另一项目的重做方向不得进入"
		}
		if _, err := pool.Exec(t.Context(), `UPDATE pbl_artifact SET verdict='revise', why=$2, settled_at=now() WHERE id=$1`, artifact, verdictReason); err != nil {
			t.Fatal(err)
		}
		if project == id {
			if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_review_dimension(artifact_id,prompt,answer) VALUES($1,'分工','A计总数与米饭，B计蔬菜与肉类')`, artifact); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_artifact(atom_id,kind,title) VALUES($1,'draft','修订后的待审核版')`, id); err != nil {
		t.Fatal(err)
	}
	r := siteReq(t, h, cookie, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"请执行已保存的审核修改"}`)
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	if provider.Calls != 2 {
		t.Fatalf("expected draft and evidence check, got %d", provider.Calls)
	}
	input, _ := json.Marshal(provider.LastRequest)
	if !strings.Contains(provider.LastRequest.Messages[1].Content, `\"verdict\":\"pending\"`) || !strings.Contains(string(input), "修订后的待审核版") {
		t.Fatal("pending artifact state missing when no review answer exists")
	}
	for _, wanted := range []string{"请按一平汤匙标准记录不确定类别", "A计总数与米饭，B计蔬菜与肉类", "设计判断，不是现场证据", "请区分原话与概括，并提供单人记录方法", "学生审核结论：revise"} {
		if !strings.Contains(string(input), wanted) {
			t.Errorf("missing review source %q", wanted)
		}
	}
	if strings.Contains(string(input), "另一项目的审核答案不得进入") || strings.Contains(string(input), "另一项目的重做方向不得进入") {
		t.Fatal("cross-project review leaked")
	}
}

func TestEvidenceGuardChecksFirstPlanWithoutNotes(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"活动之间必须完成纸样审核","produce":{"kind":"plan","payload":{}}}`),
		evidenceScript(`{"supported":false,"issues":[{"quote":"活动之间必须完成纸样审核","reason":"学生只有两次90分钟，没有活动间时间"}]}`),
		evidenceScript(`{"reply":"将纸样制作和审核放进第二次活动，不安排活动外任务。"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, cookie, _, _ := liteHandlerWithProvider(t, provider)
	id := newProjectViaAPI(t, h, cookie)
	rec := siteReq(t, h, cookie, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"只有两次90分钟，没有活动间时间。"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if provider.Calls != 4 {
		t.Fatalf("unchecked first plan: calls=%d", provider.Calls)
	}
	input, _ := json.Marshal(provider.LastRequest)
	if !strings.Contains(string(input), "只有两次90分钟") {
		t.Fatal("student constraint omitted")
	}
	thread := siteReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
	if strings.Contains(thread.Body.String(), "活动之间必须完成纸样审核") {
		t.Fatal("rejected plan claim persisted")
	}
}

func TestEvidenceGuardChecksDecisionWithoutNotes(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"重复厨房标签证明能独立查找","produce":{"kind":"decision","payload":{"subject":"选择试用方法","options":[{"label":"方法A","description":"重复厨房标签证明能独立查找"},{"label":"方法B","description":"给出物品名称独立查找"}]}}}`),
		evidenceScript(`{"supported":false,"issues":[{"quote":"重复厨房标签证明能独立查找","reason":"提示答案不支持独立查找结论"}]}`),
		evidenceScript(`{"reply":"给出物品名称，请参与者独立查找，记录首次选择。","tool":"decide","tool_reason":"比较两种方法","produce":{"kind":"decision","payload":{"subject":"选择试用方法","options":[{"label":"实际操作","description":"给出物品名称，记录首次选择"},{"label":"口头预期","description":"记录预计查找的页面，不当作实际操作"}]}}}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, cookie, _, _ := liteHandlerWithProvider(t, provider)
	id := newProjectViaAPI(t, h, cookie)
	rec := siteReq(t, h, cookie, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"请比较两种不诱导的查找测试方案。"}`)
	if rec.Code != 200 || provider.Calls != 4 {
		t.Fatalf("decision was not checked and repaired: status=%d calls=%d body=%s", rec.Code, provider.Calls, rec.Body)
	}
	thread := siteReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
	if strings.Contains(thread.Body.String(), "重复厨房标签证明能独立查找") || !strings.Contains(thread.Body.String(), "记录首次选择") {
		t.Fatal("decision repair was not persisted correctly")
	}
	decisions := siteReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+id+"/decisions", "")
	if strings.Contains(decisions.Body.String(), "重复厨房标签") || !strings.Contains(decisions.Body.String(), "口头预期") {
		t.Fatal("repaired decision options were lost or rejected options persisted")
	}
	var turn struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &turn); err != nil || turn.Tool != "decide" {
		t.Fatal("repaired decision did not retain its tool invitation")
	}
}

func TestEvidenceGuardSeesConfirmedDecision(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"你已选择先给食堂。"}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, cookie, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, cookie)
	base := "/api/v1/pbl/projects/" + pid
	decision := decodeDecision(t, pblPost(t, h, cookie, base+"/decisions", twoRoads))
	settled := pblPost(t, h, cookie, base+"/decisions/"+decision.ID+"/settle", `{"choice":"先给食堂","why":"只有他们能调整供应","whyNot":"班群无法调整供应","flip":"如果食堂不能接收建议，就调整对象"}`)
	if settled.Code != 200 {
		t.Fatal(settled.Body)
	}
	// An unconfirmed private draft must remain outside the reviewer sources.
	if _, err := pool.Exec(context.Background(), `INSERT INTO pbl_decision(atom_id,subject,choice,why,why_not) VALUES($1,'尚未确认','私密草稿选项','私密草稿理由','私密草稿取舍')`, pid); err != nil {
		t.Fatal(err)
	}
	rec := pblPost(t, h, cookie, base+"/turn", `{"text":"下一步"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if provider.Calls != 2 {
		t.Fatalf("expected draft and check, got %d", provider.Calls)
	}
	raw, _ := json.Marshal(provider.Requests[1])
	for _, value := range []string{"学生在理性决策卡中已确认", "只有他们能调整供应", "班群无法调整供应", "如果食堂不能接收建议，就调整对象"} {
		if !strings.Contains(string(raw), value) {
			t.Fatalf("missing %s", value)
		}
	}
	if strings.Contains(string(raw), "私密草稿理由") {
		t.Fatal("private draft leaked to reviewer")
	}
}

func TestEvidenceGuardIncludesUntestedReturnRecord(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"这只是试用计划，尚无实际结果。"}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, cookie, _, pool := liteHandlerWithProvider(t, provider)
	id := newProjectViaAPI(t, h, cookie)
	other := newProjectViaAPI(t, h, cookie)
	for _, row := range []struct{ id, body string }{{id, "尚未测试，没有参与者，仅计划试读筛选问题"}, {other, "另一项目试用记录不得进入"}} {
		if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_keep_entry(atom_id,kind,body,stage) VALUES($1,'thought',$2,'change')`, row.id, row.body); err != nil {
			t.Fatal(err)
		}
	}
	rec := siteReq(t, h, cookie, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"这份试用计划可以怎样验证"}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	input, _ := json.Marshal(provider.LastRequest)
	if !strings.Contains(string(input), "尚未测试，没有参与者，仅计划试读筛选问题") || !strings.Contains(string(input), "thought是想法或计划") {
		t.Fatal("return record or its qualifier missing")
	}
	if strings.Contains(string(input), "另一项目试用记录不得进入") {
		t.Fatal("cross-project record leaked")
	}
}

func TestIterationOpeningUsesSelectedRecordNotOldMainRequest(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"收到旧任务。"}`), evidenceScript(`{"reply":"这条记录仍是未测试计划，先讨论怎样试读。"}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, cookie, _, _ := liteHandlerWithProvider(t, provider)
	id := newProjectViaAPI(t, h, cookie)
	base := "/api/v1/pbl/projects/" + id
	if r := siteReq(t, h, cookie, "POST", base+"/turn", `{"text":"旧主线专用标记：请重写旧计划"}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	r := siteReq(t, h, cookie, "POST", base+"/keep", `{"kind":"thought","stage":"change","body":"当前选中记录：尚未测试，准备试读筛选问题"}`)
	var entry struct {
		ID string `json:"id"`
	}
	if r.Code != 201 {
		t.Fatal(r.Body)
	}
	if err := json.Unmarshal(r.Body.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	r = siteReq(t, h, cookie, "POST", base+"/keep/"+entry.ID+"/session", ``)
	var session struct {
		ID string `json:"sessionId"`
	}
	if r.Code != 201 {
		t.Fatal(r.Body)
	}
	if err := json.Unmarshal(r.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"sessionId": session.ID})
	r = siteReq(t, h, cookie, "POST", base+"/turn", string(body))
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	if len(provider.Requests) != 3 {
		t.Fatalf("unexpected model calls: %d", len(provider.Requests))
	}
	context, _ := json.Marshal(provider.Requests[1])
	if strings.Contains(string(context), "旧主线专用标记") || !strings.Contains(string(context), "当前选中记录：尚未测试") {
		t.Fatal("iteration opening did not isolate its selected record")
	}
}
