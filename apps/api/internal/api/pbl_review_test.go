package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	. "mindimprint/api/internal/api"
)

const reviewable = `{"kind":"draft","title":"给食堂的一页建议",
 "payload":{"body":"中国的碳排放总量全球第一。人均排放低于美国。"},
 "guessed":["我猜剩的主要是米饭"],
 "admits":["没算过每天到底剩多少斤"]}`

func newArtifactViaAPI(t *testing.T, h http.Handler, c *http.Cookie, pid string) string {
	t.Helper()
	rec := pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/artifacts", reviewable)
	if rec.Code != http.StatusCreated {
		t.Fatalf("handover = %d; body=%s", rec.Code, rec.Body)
	}
	return artifactID(t, rec.Body.String())
}

type reviewPlanOut struct {
	Marks []struct {
		ID        string  `json:"id"`
		Quote     string  `json:"quote"`
		Question  string  `json:"question"`
		Answer    string  `json:"answer"`
		SessionID *string `json:"sessionId"`
		Mine      bool    `json:"mine"`
	} `json:"marks"`
	Dimensions []struct {
		ID     string `json:"id"`
		Prompt string `json:"prompt"`
		Answer string `json:"answer"`
	} `json:"dimensions"`
}

// 🚨「点开任何一个词就能问」必须是一次动作：划一条、开一条会话线、接上，
// 一个请求做完。拆成三次往返，这个动作就没人用了。
func TestPblReview_AskOpensAChatLineInOneRequest(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	aid := newArtifactViaAPI(t, h, cookie, pid)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/ask",
		`{"quote":"人均排放低于美国。"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ask = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Mark struct {
			ID        string  `json:"id"`
			Quote     string  `json:"quote"`
			SessionID *string `json:"sessionId"`
			Mine      bool    `json:"mine"`
		} `json:"mark"`
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.SessionID == "" {
		t.Fatal("问出去了却没开会话线")
	}
	if out.Mark.SessionID == nil || *out.Mark.SessionID != out.SessionID {
		t.Fatalf("划出来的句子没和会话线接上：%v vs %s", out.Mark.SessionID, out.SessionID)
	}
	// 她自己问的，不该变成她要交的作业。
	if !out.Mark.Mine {
		t.Fatal("她自己选中问的句子应该标成 mine")
	}

	// 会话线要真的在项目的会话列表里，否则她点进去是死路。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/sessions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list sessions = %d", rec.Code)
	}
	var sessions []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	var found bool
	for _, s := range sessions {
		if s.ID == out.SessionID {
			found = true
			if s.Kind != "review" {
				t.Fatalf("会话线的类型是 %q，want review", s.Kind)
			}
		}
	}
	if !found {
		t.Fatal("开出来的会话线不在项目的会话列表里")
	}
}

func TestPblReview_AskNeedsAQuote(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	aid := newArtifactViaAPI(t, h, cookie, pid)
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/ask",
		`{"quote":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("ask with no quote = %d, want 400", rec.Code)
	}
}

// 🚨 划一句话出来却不问什么，只是在把字标黄。
func TestPblReview_MarkNeedsAQuestion(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	aid := newArtifactViaAPI(t, h, cookie, pid)
	url := "/api/v1/pbl/projects/" + pid + "/artifacts/" + aid + "/review"

	if rec := pblPost(t, h, cookie, url,
		`{"marks":[{"quote":"人均排放低于美国。","question":"  "}]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("mark without a question = %d, want 400", rec.Code)
	}
	if rec := pblPost(t, h, cookie, url, `{"marks":[],"dimensions":[]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty review plan = %d, want 400", rec.Code)
	}
}

func TestPblReview_AnswerMarksAndDimensions(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	aid := newArtifactViaAPI(t, h, cookie, pid)
	url := "/api/v1/pbl/projects/" + pid + "/artifacts/" + aid + "/review"

	rec := pblPost(t, h, cookie, url, `{
	  "marks":[{"part":"论证","partNote":"这一段要看证据撑不撑得住",
	            "quote":"人均排放低于美国。","question":"这和上一句是同一个口径吗？"}],
	  "dimensions":[{"prompt":"来源查得到吗","why":"查不到的数字等于没有"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create review plan = %d; body=%s", rec.Code, rec.Body)
	}
	var plan reviewPlanOut
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(plan.Marks) != 1 || len(plan.Dimensions) != 1 {
		t.Fatalf("plan = %d marks / %d dims", len(plan.Marks), len(plan.Dimensions))
	}
	// 印记划的不是"她问的"。
	if plan.Marks[0].Mine {
		t.Fatal("印记划的句子被标成了她问的")
	}

	if rec := pblReq(t, h, cookie, "PATCH", "/api/v1/pbl/projects/"+pid+"/marks/"+plan.Marks[0].ID,
		`{"answer":"不是，一个是总量一个是人均"}`); rec.Code != http.StatusOK {
		t.Fatalf("answer mark = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := pblReq(t, h, cookie, "PATCH",
		"/api/v1/pbl/projects/"+pid+"/dimensions/"+plan.Dimensions[0].ID,
		`{"answer":"查了 NASA 那一份"}`); rec.Code != http.StatusOK {
		t.Fatalf("answer dimension = %d; body=%s", rec.Code, rec.Body)
	}
}

func TestPblReview_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	aid := newArtifactViaAPI(t, h, cookie, pid)

	otherID := createStudent(t, pool, SeedSchoolID, "other-review@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblPost(t, h, other, "/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/ask",
		`{"quote":"人均排放低于美国。"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign ask = %d, want 404", rec.Code)
	}
}
