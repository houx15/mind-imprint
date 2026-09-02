package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

type decisionOut struct {
	ID        string  `json:"id"`
	Subject   string  `json:"subject"`
	Choice    string  `json:"choice"`
	Why       string  `json:"why"`
	WhyNot    string  `json:"whyNot"`
	SettledAt *string `json:"settledAt"`
	Options   []struct {
		ID          string `json:"id"`
		Label       string `json:"label"`
		Description string `json:"description"`
	} `json:"options"`
}

func decodeDecision(t *testing.T, rec *httptest.ResponseRecorder) decisionOut {
	t.Helper()
	var out decisionOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode decision: %v — body=%s", err, rec.Body)
	}
	return out
}

const twoRoads = `{
  "subject":"这份建议先给食堂，还是先发在班群里",
  "options":[
    {"label":"先给食堂","description":"他们能直接改菜量，但要等排期"},
    {"label":"先发班群","description":"当天就有反馈，但改不了任何事"}
  ]}`

// 🚨 确认一个决定要回答两个问题：为什么选它，为什么不选别的。
//
// 产品负责人 2026-09-02：「they need to answer two small questions: why this,
// and why not others.」第二句才是这件工具真正教的东西——选中一个不难（顺眼就
// 点了），说得出为什么放掉另外几个，才说明她真的把它们放在一起比过。
func TestPblDecision_NeedsBothWhyAndWhyNot(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/decisions", twoRoads)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open decision = %d; body=%s", rec.Code, rec.Body)
	}
	d := decodeDecision(t, rec)
	if len(d.Options) != 2 {
		t.Fatalf("options = %d, want 2", len(d.Options))
	}
	// 卡片上要有说明：光一个标题她判断不了。
	if d.Options[0].Description == "" {
		t.Fatal("选项没有说明")
	}

	settle := "/api/v1/pbl/projects/" + pid + "/decisions/" + d.ID + "/settle"
	for _, body := range []string{
		`{"choice":"","why":"a","whyNot":"b"}`,
		`{"choice":"先给食堂","why":"  ","whyNot":"b"}`,
		`{"choice":"先给食堂","why":"a","whyNot":"   "}`,
	} {
		if rec := pblPost(t, h, cookie, settle, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("settle %s = %d, want 400", body, rec.Code)
		}
	}

	rec = pblPost(t, h, cookie, settle, `{"choice":"先给食堂",
	  "why":"只有他们能真的把菜量改了",
	  "whyNot":"班群反馈快，但同学说了也改不了食堂的量"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete settle = %d; body=%s", rec.Code, rec.Body)
	}
	got := decodeDecision(t, rec)
	if got.SettledAt == nil || got.WhyNot == "" {
		t.Fatalf("定了却没存住：settledAt=%v whyNot=%q", got.SettledAt, got.WhyNot)
	}

	// 一个决定只定一次。
	if rec := pblPost(t, h, cookie, settle,
		`{"choice":"先发班群","why":"x","whyNot":"y"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("second settle = %d, want 400", rec.Code)
	}
}

// 一个选项不叫决定。印记提的时候就该给出几条路。
func TestPblDecision_NeedsTwoOptions(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/decisions",
		`{"subject":"要不要做","options":[{"label":"做"}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open = %d; body=%s", rec.Code, rec.Body)
	}
	d := decodeDecision(t, rec)
	if rec := pblPost(t, h, cookie,
		"/api/v1/pbl/projects/"+pid+"/decisions/"+d.ID+"/settle",
		`{"choice":"做","why":"a","whyNot":"b"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("settle with one option = %d, want 400", rec.Code)
	}
}

func TestPblDecision_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	d := decodeDecision(t, pblPost(t, h, cookie,
		"/api/v1/pbl/projects/"+pid+"/decisions", twoRoads))

	otherID := createStudent(t, pool, SeedSchoolID, "other-decide@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblPost(t, h, other,
		"/api/v1/pbl/projects/"+pid+"/decisions/"+d.ID+"/settle",
		`{"choice":"先给食堂","why":"a","whyNot":"b"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign settle = %d, want 404", rec.Code)
	}
}
