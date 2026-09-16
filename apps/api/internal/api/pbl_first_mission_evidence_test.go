package api_test

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// A fresh project has no notes yet. Its first field task still needs review:
// rejected instructions must not become an invitation or a saved checklist.
func TestEvidenceGuardChecksFirstMissionWithoutNotes(t *testing.T) {
	good := `{"reply":"请查看五分钟观察任务","tool":"observe","tool_reason":"记录可见动作","mission":[{"prompt":"五分钟内记录看清的投放动作，看不清记不确定","want_kind":"observation"}]}`
	bad := `{"reply":"请观察","tool":"observe","tool_reason":"观察","mission":[{"prompt":"必须采访十个人","want_kind":"observation"}]}`
	accepted := `{"supported":true,"issues":[]}`
	rejected := `{"supported":false,"issues":[{"quote":"必须采访十个人","reason":"学生不采访且只有五分钟"}]}`
	for _, mode := range []string{"accepted", "repaired", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			sequence := [][]gateway.StreamEvent{evidenceScript(good), evidenceScript(accepted)}
			wantStatus, wantSaved := 200, 1
			if mode == "repaired" {
				sequence = [][]gateway.StreamEvent{evidenceScript(bad), evidenceScript(rejected), evidenceScript(good), evidenceScript(accepted)}
			} else if mode == "rejected" {
				sequence = [][]gateway.StreamEvent{evidenceScript(bad), evidenceScript(rejected), evidenceScript(bad), evidenceScript(rejected)}
				wantStatus, wantSaved = 502, 0
			}
			provider := gateway.NewSequenceStubProvider(sequence...)
			h, cookie, _, pool := liteHandlerWithProvider(t, provider)
			id := newProjectViaAPI(t, h, cookie)
			r := siteReq(t, h, cookie, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"只有五分钟，不采访，请给我观察任务。"}`)
			if r.Code != wantStatus || provider.Calls != len(sequence) {
				t.Fatalf("status=%d calls=%d body=%s", r.Code, provider.Calls, r.Body)
			}
			if !strings.Contains(provider.Requests[1].Messages[1].Content, "只有五分钟，不采访") {
				t.Fatal("first checklist review omitted student constraints")
			}
			var tools, items, invalid, calls int
			ctx := context.Background()
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_tool_instance WHERE atom_id=$1`, id).Scan(&tools); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE m.prompt='必须采访十个人') FROM pbl_mission_item m JOIN pbl_tool_instance t ON t.id=m.tool_id WHERE t.atom_id=$1`, id).Scan(&items, &invalid); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM llm_call WHERE atom_id=$1`, id).Scan(&calls); err != nil {
				t.Fatal(err)
			}
			if tools != wantSaved || items != wantSaved || invalid != 0 || calls != len(sequence) {
				t.Fatalf("tools=%d items=%d invalid=%d calls=%d", tools, items, invalid, calls)
			}
		})
	}
}

func TestEvidenceGuardChecksClaimsWhileMissionIsOpen(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"观察清单已经修改"}`),
		evidenceScript(`{"supported":false,"issues":[{"quote":"观察清单已经修改","reason":"没有修改操作","category":"missing_action"}]}`),
		evidenceScript(`{"reply":"清单尚未修改，请明确需要调整的内容。"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, cookie, _, pool := liteHandlerWithProvider(t, provider)
	id := newProjectViaAPI(t, h, cookie)
	ctx := context.Background()
	var toolID string
	if err := pool.QueryRow(ctx, `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status) VALUES(gen_random_uuid(),$1,'observe','短观察','world','accepted') RETURNING id`, id).Scan(&toolID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pbl_mission_item(id,tool_id,prompt,want_kind,ordinal) VALUES(gen_random_uuid(),$1,'原清单','observation',0)`, toolID); err != nil {
		t.Fatal(err)
	}
	r := siteReq(t, h, cookie, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"请把任务缩短到五分钟"}`)
	if r.Code != 200 || provider.Calls != 4 {
		t.Fatalf("unchecked claim: %d %d %s", r.Code, provider.Calls, r.Body)
	}
	for _, index := range []int{1, 3} {
		input := provider.Requests[index].Messages[1].Content
		if !strings.Contains(input, toolID) || !strings.Contains(input, "原清单") {
			t.Fatal("review omitted authoritative pre-turn mission", index)
		}
	}
	thread := siteReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
	if strings.Contains(thread.Body.String(), "观察清单已经修改") || !strings.Contains(thread.Body.String(), "清单尚未修改") {
		t.Fatal("false completion persisted", thread.Body)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_mission_item WHERE tool_id=$1 AND prompt='原清单' AND superseded_at IS NULL AND done_at IS NULL`, toolID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("original mission changed: %d %v", count, err)
	}
}
