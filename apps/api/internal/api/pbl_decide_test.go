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
	Flip      string  `json:"flip"`
	SettledAt *string `json:"settledAt"`
	Options   []struct {
		ID     string `json:"id"`
		Label  string `json:"label"`
		Wins   string `json:"wins"`
		Author string `json:"author"`
	} `json:"options"`
	Criteria []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"criteria"`
}

func decodeDecision(t *testing.T, rec *httptest.ResponseRecorder) decisionOut {
	t.Helper()
	var out decisionOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode decision: %v — body=%s", err, rec.Body)
	}
	return out
}

func openDecisionViaAPI(t *testing.T, h http.Handler, c *http.Cookie, pid string) decisionOut {
	t.Helper()
	rec := pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/decisions",
		`{"subject":"先给食堂还是先发班群"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open decision = %d; body=%s", rec.Code, rec.Body)
	}
	return decodeDecision(t, rec)
}

// 🚨 一个选项不叫决定，叫已经定了。
func TestPblDecision_NeedsTwoOptions(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	d := openDecisionViaAPI(t, h, cookie, pid)
	settle := "/api/v1/pbl/projects/" + pid + "/decisions/" + d.ID + "/settle"
	full := `{"choice":"先给食堂","why":"他们能直接改","flip":"如果他们说早试过了"}`

	if rec := pblPost(t, h, cookie, settle, full); rec.Code != http.StatusBadRequest {
		t.Fatalf("settle with no options = %d, want 400", rec.Code)
	}
	pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/decisions/"+d.ID+"/options",
		`{"label":"先给食堂"}`)
	if rec := pblPost(t, h, cookie, settle, full); rec.Code != http.StatusBadRequest {
		t.Fatalf("settle with one option = %d, want 400 —— 一个选项不叫选", rec.Code)
	}
}

// 🚨 最常被跳过的一步。不说清什么重要，所谓的比较只是在挑一个看起来顺眼的。
func TestPblDecision_NeedsCriteriaBeforeSettling(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	d := openDecisionViaAPI(t, h, cookie, pid)
	opts := "/api/v1/pbl/projects/" + pid + "/decisions/" + d.ID + "/options"
	pblPost(t, h, cookie, opts, `{"label":"先给食堂"}`)
	pblPost(t, h, cookie, opts, `{"label":"先发班群"}`)

	settle := "/api/v1/pbl/projects/" + pid + "/decisions/" + d.ID + "/settle"
	if rec := pblPost(t, h, cookie, settle,
		`{"choice":"先给食堂","why":"他们能直接改","flip":"如果他们说早试过了"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("settle without criteria = %d, want 400", rec.Code)
	}
}

// 🚨「什么会让你改主意」是整件事的关键格：写得出它，这才是一个能被推翻的判断，
// 复盘的时候才有东西可以回头看。
func TestPblDecision_NeedsWhatWouldChangeHerMind(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	d := openDecisionViaAPI(t, h, cookie, pid)
	base := "/api/v1/pbl/projects/" + pid + "/decisions/" + d.ID
	pblPost(t, h, cookie, base+"/options", `{"label":"先给食堂"}`)
	pblPost(t, h, cookie, base+"/options", `{"label":"先发班群"}`)
	pblPost(t, h, cookie, base+"/criteria", `{"label":"这周之内能有回应"}`)

	if rec := pblPost(t, h, cookie, base+"/settle",
		`{"choice":"先给食堂","why":"他们能直接改","flip":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("settle without the flip = %d, want 400", rec.Code)
	}
	if rec := pblPost(t, h, cookie, base+"/settle",
		`{"choice":"先给食堂","why":"","flip":"如果他们说早试过了"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("settle without a reason = %d, want 400", rec.Code)
	}

	rec := pblPost(t, h, cookie, base+"/settle",
		`{"choice":"先给食堂","why":"他们能直接改菜量","gaveUp":"班群里能更快听到同学怎么说",
		  "flip":"如果食堂说他们早就试过了"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete settle = %d; body=%s", rec.Code, rec.Body)
	}
	got := decodeDecision(t, rec)
	if got.SettledAt == nil || got.Flip == "" {
		t.Fatalf("定了却没存住：settledAt=%v flip=%q", got.SettledAt, got.Flip)
	}

	// 定过之后不给再定，也不给改选项。
	if rec := pblPost(t, h, cookie, base+"/settle",
		`{"choice":"先发班群","why":"改主意了","flip":"x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("second settle = %d, want 400", rec.Code)
	}
	if rec := pblReq(t, h, cookie, "PATCH",
		"/api/v1/pbl/projects/"+pid+"/options/"+got.Options[0].ID,
		`{"wins":"改一改"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("editing an option after settling = %d, want 400", rec.Code)
	}
}

// 印记提的选项和她自己写的要分得开。
func TestPblDecision_RecordsWhoProposedWhat(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/decisions", `{
	  "subject":"先给食堂还是先发班群",
	  "options":[{"label":"先给食堂","author":"yinji"},{"label":"先发班群"}],
	  "criteria":[{"label":"这周之内能有回应"}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open with options = %d; body=%s", rec.Code, rec.Body)
	}
	d := decodeDecision(t, rec)
	if len(d.Options) != 2 || len(d.Criteria) != 1 {
		t.Fatalf("options=%d criteria=%d", len(d.Options), len(d.Criteria))
	}
	if d.Options[0].Author != "yinji" || d.Options[1].Author != "student" {
		t.Fatalf("作者记错了：%q / %q", d.Options[0].Author, d.Options[1].Author)
	}
}

func TestPblDecision_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	d := openDecisionViaAPI(t, h, cookie, pid)

	otherID := createStudent(t, pool, SeedSchoolID, "other-decide@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblPost(t, h, other, "/api/v1/pbl/projects/"+pid+"/decisions/"+d.ID+"/options",
		`{"label":"插一脚"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign option = %d, want 404", rec.Code)
	}
}
