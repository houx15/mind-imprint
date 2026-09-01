package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

func pblReq(t *testing.T, h http.Handler, c *http.Cookie, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(method, url, strings.NewReader(body)), c))
	return rec
}

type noteOut struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Body    string `json:"body"`
	Author  string `json:"author"`
	Edited  bool   `json:"edited"`
	Cluster string `json:"cluster"`
}

func decodeNotes(t *testing.T, rec *httptest.ResponseRecorder) []noteOut {
	t.Helper()
	var out []noteOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode notes: %v — body=%s", err, rec.Body)
	}
	return out
}

// 一次贴一批：她出门回来常常一口气写好几条。
func TestPblNotes_CreateBatchAndList(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/notes"

	rec := pblPost(t, h, cookie, url, `{"notes":[
	  {"kind":"observation","body":"中午十二点半，第三个桶已经满了"},
	  {"kind":"quote","body":"阿姨说「每天都这样」"},
	  {"kind":"assumption","body":"我猜剩的主要是米饭","author":"yinji"}
	]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d; body=%s", rec.Code, rec.Body)
	}
	if got := decodeNotes(t, rec); len(got) != 3 {
		t.Fatalf("created %d notes, want 3", len(got))
	}

	list := decodeNotes(t, pblReq(t, h, cookie, "GET", url, ""))
	if len(list) != 3 {
		t.Fatalf("listed %d, want 3", len(list))
	}
	// 🚨 印记写的和她写的要分得开，否则过程记录里 AI 的话会被算成她的。
	var byYinji int
	for _, n := range list {
		if n.Author == "yinji" {
			byYinji++
		}
	}
	if byYinji != 1 {
		t.Fatalf("印记写的便签 %d 张，want 1 —— author 没存住", byYinji)
	}
}

func TestPblNotes_RejectsEmptyAndUnknownKind(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/notes"

	for _, body := range []string{
		`{"notes":[]}`,
		`{"notes":[{"kind":"observation","body":"   "}]}`,
		`{"notes":[{"kind":"随便什么","body":"x"}]}`,
	} {
		if rec := pblPost(t, h, cookie, url, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("create %s = %d, want 400", body, rec.Code)
		}
	}
}

// 🚨 她改印记写的便签，edited 要留住——纠正是最强的过程信号之一。
func TestPblNotes_EditingYinjisNoteIsRecorded(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/notes"

	made := decodeNotes(t, pblPost(t, h, cookie, url,
		`{"notes":[{"kind":"assumption","body":"我猜是米饭","author":"yinji"},
		            {"kind":"observation","body":"第三个桶满了"}]}`))
	yinji, mine := made[0], made[1]

	rec := pblReq(t, h, cookie, "PATCH", url+"/"+yinji.ID, `{"body":"其实主要是菜汤"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch = %d; body=%s", rec.Code, rec.Body)
	}
	var got noteOut
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Edited {
		t.Fatal("她改了印记写的便签，edited 却是 false")
	}

	// 她自己写的便签，改了不算"纠正"。
	rec = pblReq(t, h, cookie, "PATCH", url+"/"+mine.ID, `{"body":"第三个桶十二点半就满了"}`)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Edited {
		t.Fatal("她改自己写的便签不该记成纠正")
	}
}

// 归堆就是"把哪些放一起"，这块板上唯一真正要动脑的操作。
func TestPblNotes_Clustering(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/notes"

	made := decodeNotes(t, pblPost(t, h, cookie, url,
		`{"notes":[{"kind":"observation","body":"第三个桶满了"}]}`))
	rec := pblReq(t, h, cookie, "PATCH", url+"/"+made[0].ID, `{"cluster":"打饭时间"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("cluster = %d; body=%s", rec.Code, rec.Body)
	}
	var got noteOut
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Cluster != "打饭时间" {
		t.Fatalf("cluster = %q", got.Cluster)
	}
	// 拿下来的便签不再出现在板上。
	if rec := pblReq(t, h, cookie, "DELETE", url+"/"+made[0].ID, ""); rec.Code != http.StatusOK {
		t.Fatalf("archive = %d", rec.Code)
	}
	if list := decodeNotes(t, pblReq(t, h, cookie, "GET", url, "")); len(list) != 0 {
		t.Fatalf("拿下来之后板上还剩 %d 张", len(list))
	}
}

// 别人的便签和不存在的便签，对外必须长得一样。
func TestPblNotes_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/notes"
	made := decodeNotes(t, pblPost(t, h, cookie, url,
		`{"notes":[{"kind":"observation","body":"第三个桶满了"}]}`))

	otherID := createStudent(t, pool, SeedSchoolID, "other-board@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblReq(t, h, other, "PATCH", url+"/"+made[0].ID, `{"body":"x"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign patch = %d, want 404", rec.Code)
	}
	if rec := pblReq(t, h, other, "GET", url, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign list = %d, want 404", rec.Code)
	}
}
