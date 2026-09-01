package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

type reframeOut struct {
	ID          string  `json:"id"`
	Who         string  `json:"who"`
	Needs       string  `json:"needs"`
	Why         string  `json:"why"`
	HMW         string  `json:"hmw"`
	Supersedes  *string `json:"supersedes"`
	ConfirmedAt *string `json:"confirmedAt"`
}

func decodeReframe(t *testing.T, rec *httptest.ResponseRecorder) reframeOut {
	t.Helper()
	var out reframeOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode reframe: %v — body=%s", err, rec.Body)
	}
	return out
}

// 🚨 四格没填齐不给确认。填了一半的问题陈述读起来像"想清楚了"，而后面每一步
// 都会架在它上面。门槛必须在服务端——只活在按钮里的门槛是装饰。
func TestPblReframe_ConfirmNeedsAllFour(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/reframes"

	made := decodeReframe(t, pblPost(t, h, cookie, url, `{"who":"打饭的人"}`))
	if rec := pblPost(t, h, cookie, url+"/"+made.ID+"/confirm", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("confirm with one field = %d, want 400", rec.Code)
	}

	pblReq(t, h, cookie, "PATCH", url+"/"+made.ID,
		`{"needs":"知道还剩什么","why":"白跑一趟只能买面包"}`)
	if rec := pblPost(t, h, cookie, url+"/"+made.ID+"/confirm", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("confirm without the question = %d, want 400", rec.Code)
	}

	pblReq(t, h, cookie, "PATCH", url+"/"+made.ID, `{"hmw":"我们可以怎样让他出门前就知道"}`)
	rec := pblPost(t, h, cookie, url+"/"+made.ID+"/confirm", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm complete = %d; body=%s", rec.Code, rec.Body)
	}
	if decodeReframe(t, rec).ConfirmedAt == nil {
		t.Fatal("确认了却没有 confirmedAt")
	}
}

// 🚨 改写不覆盖旧的。问题被重新框定的那一刻正是这门课要教的事——覆盖掉就等
// 于把它删了。定过的那一版不能再改，只能开新的一版指回去。
func TestPblReframe_RewriteSupersedesInsteadOfOverwriting(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/reframes"

	first := decodeReframe(t, pblPost(t, h, cookie, url,
		`{"who":"打饭的人","needs":"知道还剩什么","why":"白跑一趟","hmw":"我们可以怎样提前告诉他"}`))
	pblPost(t, h, cookie, url+"/"+first.ID+"/confirm", "")

	// 定过之后不给改。
	if rec := pblReq(t, h, cookie, "PATCH", url+"/"+first.ID, `{"who":"别人"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("patch a confirmed version = %d, want 400", rec.Code)
	}

	second := decodeReframe(t, pblPost(t, h, cookie, url,
		`{"who":"做饭的阿姨","needs":"知道该做多少","why":"做多了要倒掉",
		  "hmw":"我们可以怎样让她提前知道今天来多少人","supersedes":"`+first.ID+`"}`))
	if second.Supersedes == nil || *second.Supersedes != first.ID {
		t.Fatalf("supersedes = %v, want %s", second.Supersedes, first.ID)
	}

	// 两版都还在。
	var all []reframeOut
	rec := pblReq(t, h, cookie, "GET", url, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &all); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("列表里 %d 版，want 2 —— 旧的那版被覆盖掉了", len(all))
	}
}

func TestPblReframe_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/reframes"
	made := decodeReframe(t, pblPost(t, h, cookie, url, `{"who":"打饭的人"}`))

	otherID := createStudent(t, pool, SeedSchoolID, "other-reframe@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblReq(t, h, other, "PATCH", url+"/"+made.ID, `{"who":"x"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign patch = %d, want 404", rec.Code)
	}
}
