package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"mindimprint/api/internal/gateway"
)

type decisionRevisionProvider struct {
	payload     string
	calls       int
	beforeFirst func()
}

func TestDecisionRevisionAndConfirmationRace(t *testing.T) {
	provider := &decisionRevisionProvider{}
	h, cookie, _, _ := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, cookie)
	base := "/api/v1/pbl/projects/" + pid + "/decisions"
	d := decodeDecision(t, pblPost(t, h, cookie, base, twoRoads))
	provider.payload = fmt.Sprintf(`{"decisionId":%q,"version":0,"reason":"补充条件","options":[{"id":%q,"label":"先给食堂","description":"新的候选内容"}]}`, d.ID, d.Options[0].ID)
	var wg sync.WaitGroup
	var settledStatus int
	wg.Add(2)
	go func() { defer wg.Done(); turnProducing(t, h, cookie, pid) }()
	go func() {
		defer wg.Done()
		settledStatus = siteReq(t, h, cookie, "POST", base+"/"+d.ID+"/settle", `{"contentVersion":0,"choice":"先给食堂","why":"原候选理由","whyNot":"原候选比较"}`).Code
	}()
	wg.Wait()
	var rows []struct {
		ContentVersion int     `json:"contentVersion"`
		SettledAt      *string `json:"settledAt"`
	}
	r := siteReq(t, h, cookie, "GET", base, "")
	if err := json.Unmarshal(r.Body.Bytes(), &rows); err != nil || len(rows) != 1 {
		t.Fatal(r.Body)
	}
	if settledStatus == 200 {
		if rows[0].ContentVersion != 0 || rows[0].SettledAt == nil {
			t.Fatal("revision changed confirmed content", r.Body)
		}
	} else if settledStatus == 409 {
		if rows[0].ContentVersion != 1 || rows[0].SettledAt != nil {
			t.Fatal("stale confirmation committed", r.Body)
		}
	} else {
		t.Fatal("unexpected confirmation status", settledStatus)
	}
}

func (p *decisionRevisionProvider) Stream(ctx context.Context, resolved gateway.Resolved, request gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	text := coachProducing("decision", p.payload)
	p.calls++
	if p.calls == 1 && p.beforeFirst != nil {
		p.beforeFirst()
	}
	if p.calls%2 == 0 {
		text = `{"supported":true,"issues":[]}`
	}
	return pblCoachSaying(text).Stream(ctx, resolved, request)
}

func TestDecisionRevisionUsesVersionCapturedBeforeModelCall(t *testing.T) {
	started, resume := make(chan struct{}), make(chan struct{})
	provider := &decisionRevisionProvider{beforeFirst: func() { close(started); <-resume }}
	h, cookie, _, pool := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, cookie)
	base := "/api/v1/pbl/projects/" + pid + "/decisions"
	d := decodeDecision(t, pblPost(t, h, cookie, base, twoRoads))
	provider.payload = fmt.Sprintf(`{"decisionId":%q,"reason":"基于旧快照修改","options":[{"id":%q,"label":"先给食堂","description":"不应覆盖并发更新"}]}`, d.ID, d.Options[0].ID)
	finished := make(chan int, 1)
	go func() {
		finished <- pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"请修改"}`).Code
	}()
	<-started
	// Another writer commits while the model is still producing its response.
	_, err := pool.Exec(context.Background(), "UPDATE pbl_decision SET content_version=content_version+1 WHERE id=$1", d.ID)
	close(resume)
	status := <-finished
	if err != nil || status != 200 {
		t.Fatal(err, status)
	}
	got := siteReq(t, h, cookie, "GET", base, "").Body.String()
	if strings.Contains(got, "不应覆盖并发更新") || !strings.Contains(got, `"contentVersion":1`) || !strings.Contains(got, `"revisionHistory":[]`) {
		t.Fatal("model response overwrote a newer snapshot", got)
	}
}

func TestDecisionRevisionPreservesDraftAndRejectsStaleConfirmation(t *testing.T) {
	provider := &decisionRevisionProvider{}
	h, cookie, _, _ := liteHandlerWithProvider(t, provider)
	pid := newProjectViaAPI(t, h, cookie)
	base := "/api/v1/pbl/projects/" + pid + "/decisions"
	d := decodeDecision(t, pblPost(t, h, cookie, base, twoRoads))
	draftURL := base + "/" + d.ID + "/draft"
	if r := siteReq(t, h, cookie, "PUT", draftURL, `{"revision":0,"draft":{"contentVersion":0,"why":"原来的个人草稿"}}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	payload := func(version int, optionID string) string {
		return fmt.Sprintf(`{"decisionId":%q,"version":%d,"reason":"纠正过度归因","options":[{"id":%q,"label":"先给食堂","description":"先了解食堂是否能够调整，不保证会改变菜量"}]}`, d.ID, version, optionID)
	}
	// The model's guessed version is ignored; the server uses the snapshot
	// captured before the model call. Student confirmation still sends a version.
	provider.payload = payload(999, d.Options[0].ID)
	turnProducing(t, h, cookie, pid)
	read := func() string { return siteReq(t, h, cookie, "GET", base, "").Body.String() }
	updated := read()
	var rows []struct {
		ID              string            `json:"id"`
		ContentVersion  int               `json:"contentVersion"`
		RevisionHistory []json.RawMessage `json:"revisionHistory"`
	}
	if err := json.Unmarshal([]byte(updated), &rows); err != nil || len(rows) != 1 || rows[0].ID != d.ID || rows[0].ContentVersion != 1 || len(rows[0].RevisionHistory) != 1 {
		t.Fatal(updated, err)
	}
	for _, required := range []string{"先了解食堂是否能够调整", "他们能直接改菜量，但要等排期", "当天就有反馈，但改不了任何事"} {
		if !strings.Contains(updated, required) {
			t.Fatal("missing current/previous content", updated)
		}
	}
	if got := siteReq(t, h, cookie, "GET", draftURL, ""); !strings.Contains(got.Body.String(), "原来的个人草稿") || !strings.Contains(got.Body.String(), `"contentVersion":0`) {
		t.Fatal(got.Body)
	}
	settle := base + "/" + d.ID + "/settle"
	if r := siteReq(t, h, cookie, "POST", settle, `{"choice":"先给食堂","why":"旧理由","whyNot":"旧比较","contentVersion":0}`); r.Code != 409 {
		t.Fatal("stale confirmation accepted", r.Body)
	}
	// Replayed revision, a foreign project target and a student-authored option
	// must not mutate the already revised candidate or append history.
	turnProducing(t, h, cookie, pid)
	if read() != updated {
		t.Fatal("stale revision changed decision")
	}
	otherPID := newProjectViaAPI(t, h, cookie)
	provider.payload = payload(1, d.Options[0].ID)
	turnProducing(t, h, cookie, otherPID)
	if read() != updated {
		t.Fatal("cross-project revision changed decision")
	}
	added := decodeDecision(t, pblPost(t, h, cookie, base+"/"+d.ID+"/options", `{"label":"自己调查","description":"学生自己的方法"}`))
	provider.payload = payload(1, added.Options[len(added.Options)-1].ID)
	beforeStudent := read()
	turnProducing(t, h, cookie, pid)
	if read() != beforeStudent {
		t.Fatal("AI overwrote a student option")
	}
	if r := siteReq(t, h, cookie, "POST", settle, `{"choice":"先给食堂","why":"核对过新条件","whyNot":"其他方式无法直接询问","contentVersion":1}`); r.Code != 200 {
		t.Fatal(r.Body)
	}
	provider.payload = payload(1, d.Options[0].ID)
	beforeFinal := read()
	turnProducing(t, h, cookie, pid)
	if read() != beforeFinal {
		t.Fatal("AI revised a settled decision")
	}
}
