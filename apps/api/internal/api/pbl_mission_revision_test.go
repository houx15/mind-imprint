package api_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

type missionMutationProvider struct {
	gateway.Provider
	before func()
	calls  int
}

func (p *missionMutationProvider) Stream(ctx context.Context, resolved gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.calls++
	if p.calls == 2 && p.before != nil {
		p.before()
	}
	return p.Provider.Stream(ctx, resolved, req)
}

func TestMissionRevisionRejectsForeignAndChangedTargets(t *testing.T) {
	for _, scenario := range []string{"foreign", "changed"} {
		t.Run(scenario, func(t *testing.T) {
			tid, mid := uuid.New(), uuid.New()
			revision := evidenceScript(`{"reply":"修订","mission_target":"` + tid.String() + `","mission":[{"prompt":"新任务","want_kind":"observation"}]}`)
			if scenario == "changed" {
				revision = evidenceScript(`{"reply":"已修改观察清单和方案","hook":"请使用新版","mission_target":"` + tid.String() + `","mission":[{"prompt":"新任务","want_kind":"observation"}],"produce":{"kind":"artifact","payload":{"kind":"draft","title":"依赖新任务的方案","body":"请执行新任务","marks":[],"dimensions":[]}}}`)
			}
			third := evidenceScript(`{"supported":true,"issues":[]}`)
			if scenario == "foreign" {
				third = revision
			}
			provider := &missionMutationProvider{Provider: gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"开始"}`), revision, third)}
			h, c, _, pool := liteHandlerWithProvider(t, provider)
			pid := newProjectViaAPI(t, h, c)
			base := "/api/v1/pbl/projects/" + pid
			siteReq(t, h, c, "POST", base+"/turn", `{"text":"开始"}`)
			owner := pid
			if scenario == "foreign" {
				owner = newProjectViaAPI(t, h, c)
			}
			ctx := context.Background()
			if _, err := pool.Exec(ctx, `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status) VALUES($1,$2,'observe','观察','world','accepted')`, tid, owner); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO pbl_mission_item(id,tool_id,prompt,want_kind,ordinal) VALUES($1,$2,'原任务','observation',0)`, mid, tid); err != nil {
				t.Fatal(err)
			}
			if scenario == "changed" {
				provider.before = func() {
					if rec := siteReq(t, h, c, "PATCH", base+"/mission/"+mid.String(), `{"done":true}`); rec.Code != 200 {
						t.Fatal(rec.Body)
					}
				}
			}
			wantStatus := 200
			if scenario == "foreign" {
				wantStatus = 502
			}
			turn := siteReq(t, h, c, "POST", base+"/turn", `{"text":"修改任务"}`)
			if rec := turn; rec.Code != wantStatus {
				t.Fatal(rec.Body)
			}
			if scenario == "changed" {
				var dto struct{ Reply, Hook string }
				if err := json.Unmarshal(turn.Body.Bytes(), &dto); err != nil {
					t.Fatal(err)
				}
				if dto.Reply != "本轮操作未完成，请查看下方错误信息。" || dto.Hook != "" {
					t.Fatal("stale success displayed", dto)
				}
				var produced int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_artifact WHERE atom_id=$1`, pid).Scan(&produced); err != nil || produced != 0 {
					t.Fatal("dependent artifact survived rejected mission", produced, err)
				}
				var savedReply string
				if err := pool.QueryRow(ctx, `SELECT content FROM atom_message WHERE atom_id=$1 AND role='ai' ORDER BY seq DESC LIMIT 1`, pid).Scan(&savedReply); err != nil || savedReply != dto.Reply {
					t.Fatal("history kept success claim", savedReply, err)
				}
			}
			var count, archived int
			if err := pool.QueryRow(ctx, `SELECT count(*),count(superseded_at) FROM pbl_mission_item WHERE tool_id=$1`, tid).Scan(&count, &archived); err != nil || count != 1 || archived != 0 {
				t.Fatalf("unexpected overwrite %d %d %v", count, archived, err)
			}
			rec := siteReq(t, h, c, "GET", base+"/thread", "")
			if scenario == "changed" && !strings.Contains(rec.Body.String(), "观察清单更新失败") {
				t.Fatal("failure not visible", rec.Body)
			}
		})
	}
}

func TestMissionRevisionPreservesHistoryAndDraft(t *testing.T) {
	tid := uuid.New()
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"开始"}`),
		evidenceScript(`{"reply":"请查看修订清单","mission_target":"`+tid.String()+`","mission":[{"prompt":"五分钟内记录看清的餐盘，时间到即停","want_kind":"observation"}]}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, c)
	base := "/api/v1/pbl/projects/" + pid
	siteReq(t, h, c, "POST", base+"/turn", `{"text":"开始"}`)
	ctx := context.Background()
	old := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status) VALUES($1,$2,'observe','观察','world','accepted')`, tid, pid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO pbl_mission_item(id,tool_id,prompt,want_kind,ordinal,done_at) VALUES($1,$2,'旧任务','observation',0,now())`, old, tid); err != nil {
		t.Fatal(err)
	}
	draft := base + "/tools/" + tid.String() + "/observation-draft"
	if rec := siteReq(t, h, c, "PUT", draft, `{"revision":0,"document":[{"kind":"question","body":"原任务的记录","from":"旧任务","imageKey":""}]}`); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if rec := siteReq(t, h, c, "POST", base+"/turn", `{"text":"请调整清单"}`); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	rec := siteReq(t, h, c, "GET", base+"/tools/"+tid.String()+"/mission", "")
	var items []struct {
		ID                   string
		DoneAt, SupersededAt *string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil || len(items) != 2 {
		t.Fatal(rec.Body)
	}
	active, history := 0, 0
	for _, item := range items {
		if item.SupersededAt != nil {
			history++
			if item.ID != old.String() || item.DoneAt == nil {
				t.Fatal("original completion lost")
			}
		} else {
			active++
			if item.DoneAt != nil {
				t.Fatal("new task inherited completion")
			}
		}
	}
	if active != 1 || history != 1 {
		t.Fatal(items)
	}
	if rec = siteReq(t, h, c, "PATCH", base+"/mission/"+old.String(), `{"done":false}`); rec.Code != 409 {
		t.Fatal(rec.Body)
	}
	rec = siteReq(t, h, c, "GET", draft, "")
	var saved struct{ Document []struct{ Body, From string } }
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil || len(saved.Document) != 1 || saved.Document[0].Body != "原任务的记录" || saved.Document[0].From != "旧任务" {
		t.Fatal(rec.Body)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_tool_instance WHERE atom_id=$1 AND tool='observe'`, pid).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate tool: %d %v", count, err)
	}
}
