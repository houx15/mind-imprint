package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

type lookbackOut struct {
	ID         string `json:"id"`
	Prompt     string `json:"prompt"`
	AnchorKind string `json:"anchorKind"`
	Answer     string `json:"answer"`
}

func decodeLookback(t *testing.T, rec *httptest.ResponseRecorder) []lookbackOut {
	t.Helper()
	var out []lookbackOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode lookback: %v — body=%s", err, rec.Body)
	}
	return out
}

// 🚨 复盘的问题从这个项目**真的发生过的事**里长出来，不是一张空表。
// 「it can be a form」——是表单，但问「你学到了什么」的空格，和被否掉的那种
// 卡片是同一件东西。
func TestPblLookback_AsksAboutWhatActuallyHappened(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	// 她做过一个带「什么会让我改主意」的决定。
	d := openDecisionViaAPI(t, h, cookie, pid)
	base := "/api/v1/pbl/projects/" + pid + "/decisions/" + d.ID
	pblPost(t, h, cookie, base+"/options", `{"label":"先给食堂"}`)
	pblPost(t, h, cookie, base+"/options", `{"label":"先发班群"}`)
	pblPost(t, h, cookie, base+"/criteria", `{"label":"这周之内能有回应"}`)
	pblPost(t, h, cookie, base+"/settle", `{"choice":"先给食堂","why":"他们能直接改",
	  "flip":"食堂说他们早就试过了"}`)

	// 她退回了一份东西。
	aid := newArtifactViaAPI(t, h, cookie, pid)
	pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/settle",
		`{"verdict":"revise","why":"第二段把我的话改成了它自己的说法"}`)

	got := decodeLookback(t, pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/lookback", ""))
	if len(got) < 3 {
		t.Fatalf("只生成了 %d 问：%+v", len(got), got)
	}

	var sawDecision, sawArtifact bool
	for _, p := range got {
		switch p.AnchorKind {
		case "decision":
			sawDecision = true
			// 做决定时写下的那一句，到这里才兑现。
			if !strings.Contains(p.Prompt, "食堂说他们早就试过了") {
				t.Fatalf("决定那一问没把「什么会让我改主意」问回来：%q", p.Prompt)
			}
		case "artifact":
			sawArtifact = true
			if !strings.Contains(p.Prompt, "第二段把我的话改成了它自己的说法") {
				t.Fatalf("退回那一问没带上她当时的理由：%q", p.Prompt)
			}
		}
	}
	if !sawDecision {
		t.Fatal("她做过一个决定，复盘里却没问到")
	}
	if !sawArtifact {
		t.Fatal("她退回过一份东西，复盘里却没问到")
	}
}

// 🚨 只生成一次。再生成一遍会把她答过的冲掉，而复盘本来就是隔几天回来慢慢
// 写的。
func TestPblLookback_GeneratedOnceAndKeepsHerAnswers(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/lookback"

	first := decodeLookback(t, pblReq(t, h, cookie, "GET", url, ""))
	if len(first) == 0 {
		t.Fatal("一个还没定过什么的项目也该有得问")
	}
	if rec := pblReq(t, h, cookie, "PATCH", url+"/"+first[0].ID,
		`{"answer":"最难的是承认第一个问题问错了"}`); rec.Code != http.StatusOK {
		t.Fatalf("answer = %d; body=%s", rec.Code, rec.Body)
	}

	second := decodeLookback(t, pblReq(t, h, cookie, "GET", url, ""))
	if len(second) != len(first) {
		t.Fatalf("第二次打开变成了 %d 问（原来 %d 问）——重新生成了", len(second), len(first))
	}
	if second[0].ID != first[0].ID || second[0].Answer == "" {
		t.Fatalf("她答过的被冲掉了：%+v", second[0])
	}
}

func TestPblLookback_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	otherID := createStudent(t, pool, SeedSchoolID, "other-lookback@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblReq(t, h, other, "GET", "/api/v1/pbl/projects/"+pid+"/lookback", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign lookback = %d, want 404", rec.Code)
	}
}

/* ── 上线之后 ───────────────────────────────────────────────────────────── */

// 🚨 一条数据长出一轮新的思考——这一下就是维持和归档的分界。
// 「so one project may have several sessions」
func TestPblKeep_AnEntryGrowsANewSession(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/keep"

	rec := pblPost(t, h, cookie, url, `{"kind":"stat","body":"这周有 12 个人打开过","stage":"observe"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add entry = %d; body=%s", rec.Code, rec.Body)
	}
	var entry struct {
		ID        string  `json:"id"`
		SessionID *string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &entry)
	if entry.SessionID != nil {
		t.Fatal("刚记下的一条不该已经有一轮思考")
	}

	rec = pblPost(t, h, cookie, url+"/"+entry.ID+"/session", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("open session = %d; body=%s", rec.Code, rec.Body)
	}
	var opened struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &opened)
	if opened.SessionID == "" {
		t.Fatal("没开出会话来")
	}

	// 再点一次应该回到同一轮，而不是又开一条平行的线。
	rec = pblPost(t, h, cookie, url+"/"+entry.ID+"/session", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("second open = %d, want 200", rec.Code)
	}
	var again struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &again)
	if again.SessionID != opened.SessionID {
		t.Fatalf("又开了一轮：%s vs %s", again.SessionID, opened.SessionID)
	}

	// 这一轮要真的在项目的会话列表里。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/sessions", "")
	var sessions []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	var found bool
	for _, s := range sessions {
		if s.ID == opened.SessionID && s.Kind == "keeping" {
			found = true
		}
	}
	if !found {
		t.Fatalf("开出来的那一轮不在会话列表里：%+v", sessions)
	}
}

func TestPblKeep_RejectsJunk(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/keep"

	for _, body := range []string{
		`{"kind":"随便","body":"x","stage":"observe"}`,
		`{"kind":"stat","body":"   ","stage":"observe"}`,
	} {
		if rec := pblPost(t, h, cookie, url, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("add %s = %d, want 400", body, rec.Code)
		}
	}
	// 认不出的阶段退回"看数据"，而不是报错——阶段是提示，不是门槛。
	if rec := pblPost(t, h, cookie, url,
		`{"kind":"thought","body":"也许该换个标题","stage":"什么阶段"}`); rec.Code != http.StatusCreated {
		t.Fatalf("unknown stage = %d, want 201", rec.Code)
	}
}
