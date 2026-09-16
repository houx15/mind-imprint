package api_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
)

func TestObservationDraftIsolationConflictAndIdempotentSubmit(t *testing.T) {
	h, c, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, c)
	tid := uuid.New()
	_, err := pool.Exec(context.Background(), `INSERT INTO pbl_tool_instance(id,atom_id,tool,reason,kind,status) VALUES($1,$2,'observe','记录观察','world','accepted')`, tid, pid)
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/pbl/projects/" + pid + "/tools/" + tid.String() + "/observation-draft"
	body := `{"revision":0,"document":[{"kind":"assumption","body":"这是待验证的猜测","imageKey":""}]}`
	saved := siteReq(t, h, c, "PUT", base, body)
	if saved.Code != 200 {
		t.Fatal(saved.Body)
	}
	var n int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM pbl_note WHERE atom_id=$1`, pid).Scan(&n)
	if n != 0 {
		t.Fatal("draft leaked into notes")
	}
	got := siteReq(t, h, c, "GET", base, "")
	var draft struct {
		Revision int                           `json:"revision"`
		Document []struct{ Kind, Body string } `json:"document"`
	}
	if err = json.Unmarshal(got.Body.Bytes(), &draft); err != nil || draft.Revision != 1 || len(draft.Document) != 1 || draft.Document[0].Kind != "assumption" {
		t.Fatal(got.Body)
	}
	if stale := siteReq(t, h, c, "PUT", base, body); stale.Code != 409 {
		t.Fatal(stale.Body)
	}
	first := siteReq(t, h, c, "POST", base+"/submit", `{"revision":1}`)
	if first.Code != 201 {
		t.Fatal(first.Body)
	}
	retry := siteReq(t, h, c, "POST", base+"/submit", `{"revision":1}`)
	if retry.Code != 200 {
		t.Fatal(retry.Body)
	}
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM pbl_note WHERE atom_id=$1`, pid).Scan(&n)
	if n != 1 {
		t.Fatalf("duplicate notes: %d", n)
	}
	other := newProjectViaAPI(t, h, c)
	if cross := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+other+"/tools/"+tid.String()+"/observation-draft", ""); cross.Code != 404 {
		t.Fatal(cross.Body)
	}
	if invalid := siteReq(t, h, c, "PUT", base, `{"revision":2,"document":[{"kind":"observation","body":"照片","imageKey":"users/another/images/photo.png"}]}`); invalid.Code != 400 {
		t.Fatal(invalid.Body)
	}
}
