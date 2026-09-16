package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

func TestStudentMissionEditPreservesHistoryAndRejectsStaleWrites(t *testing.T) {
	provider := gateway.NewSequenceStubProvider()
	h, cookie, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, cookie)
	base := "/api/v1/pbl/projects/" + pid
	ctx := context.Background()
	type row struct {
		ID, Prompt, Version, WantKind string
		Ordinal                       int
		DoneAt, SupersededAt          *string
		EditedByStudent               bool
	}
	seed := func() (string, []row) {
		tid := uuid.New().String()
		if _, err := pool.Exec(ctx, `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status) VALUES($1,$2,'observe','观察','world','accepted')`, tid, pid); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO pbl_mission_item(tool_id,prompt,want_kind,ordinal,done_at) VALUES($1,'第一项','observation',0,now()),($1,'第二项','question',1,now())`, tid); err != nil {
			t.Fatal(err)
		}
		r := siteReq(t, h, cookie, "GET", base+"/tools/"+tid+"/mission", "")
		var rows []row
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &rows) != nil {
			t.Fatal(r.Body)
		}
		return tid, rows
	}
	body := func(r row, text string) string {
		b, _ := json.Marshal(map[string]string{"prompt": text, "version": r.Version})
		return string(b)
	}
	tid, original := seed()
	path := base + "/mission/" + original[0].ID
	unchanged := siteReq(t, h, cookie, "PUT", path, body(original[0], "第一项"))
	if unchanged.Code != 200 {
		t.Fatal(unchanged.Body)
	}
	r := siteReq(t, h, cookie, "PUT", path, body(original[0], "修改后的第一项"))
	var rows []row
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &rows) != nil || len(rows) != 3 {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
	var fresh row
	for _, item := range rows {
		switch item.ID {
		case original[0].ID:
			if item.SupersededAt == nil || item.DoneAt == nil || item.Prompt != "第一项" {
				t.Fatal("history lost", item)
			}
		case original[1].ID:
			if item.Version != original[1].Version {
				t.Fatal("unrelated task changed", item)
			}
		default:
			fresh = item
		}
	}
	if fresh.Prompt != "修改后的第一项" || fresh.DoneAt != nil || fresh.SupersededAt != nil || !fresh.EditedByStudent || fresh.Ordinal != 0 || fresh.WantKind != "observation" {
		t.Fatal("invalid new task", fresh)
	}
	if r := siteReq(t, h, cookie, "PUT", path, body(original[0], "过期覆盖")); r.Code != 409 {
		t.Fatal("accepted old ID", r.Body)
	}
	if r := siteReq(t, h, cookie, "PATCH", base+"/mission/"+fresh.ID, `{"done":true}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	if r := siteReq(t, h, cookie, "PUT", base+"/mission/"+fresh.ID, body(fresh, "覆盖勾选")); r.Code != 409 {
		t.Fatal("ignored concurrent tick", r.Body)
	}
	otherProject := newProjectViaAPI(t, h, cookie)
	if r := siteReq(t, h, cookie, "PUT", "/api/v1/pbl/projects/"+otherProject+"/mission/"+original[1].ID, body(original[1], "跨项目")); r.Code != 404 {
		t.Fatal("cross project", r.Body)
	}
	other := signInAs(t, pool, createStudent(t, pool, api.SeedSchoolID, "mission-edit-other@demo.local"))
	if r := siteReq(t, h, other, "PUT", base+"/mission/"+original[1].ID, body(original[1], "他人修改")); r.Code != 404 {
		t.Fatal("cross owner", r.Body)
	}
	for _, text := range []string{" ", strings.Repeat("字", 2001)} {
		if r := siteReq(t, h, cookie, "PUT", base+"/mission/"+original[1].ID, body(original[1], text)); r.Code != 400 {
			t.Fatal("invalid text", r.Body)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE pbl_tool_instance SET status='done' WHERE id=$1`, tid); err != nil {
		t.Fatal(err)
	}
	if r := siteReq(t, h, cookie, "PUT", base+"/mission/"+original[1].ID, body(original[1], "已结束")); r.Code != 409 {
		t.Fatal("closed tool edited", r.Body)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_mission_item WHERE tool_id=$1`, tid).Scan(&count); err != nil || count != 3 {
		t.Fatal("rejected/no-op writes created history", count, err)
	}
	if provider.Calls != 0 {
		t.Fatal("student edit called model", provider.Calls)
	}
}
