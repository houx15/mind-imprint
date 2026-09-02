package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

const handOver = `{"kind":"draft","title":"给食堂的一页建议",
 "payload":{"body":"..."},
 "guessed":["我猜剩的主要是米饭"],
 "admits":["没算过每天到底剩多少斤"]}`

func artifactID(t *testing.T, body string) string {
	t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil || out.ID == "" {
		t.Fatalf("decode artifact: %v — body=%s", err, body)
	}
	return out.ID
}

// 🚨 印记 must name what it guessed AND what is still wrong. A handover that
// hides its assumptions can only be accepted, never reviewed.
func TestPblArtifact_RequiresDisclosure(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/artifacts"

	for _, body := range []string{
		`{"kind":"draft","guessed":[],"admits":["x"]}`,
		`{"kind":"draft","guessed":["x"],"admits":[]}`,
		`{"kind":"draft"}`,
	} {
		if rec := pblPost(t, h, cookie, url, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("handover %s = %d, want 400", body, rec.Code)
		}
	}
	if rec := pblPost(t, h, cookie, url, handOver); rec.Code != http.StatusCreated {
		t.Fatalf("full handover = %d; body=%s", rec.Code, rec.Body)
	}
}

// 🚨 理由只在「重新执行任务」这一档必填（产品负责人 2026-09-02）。
//
// 三档的门槛不一样，是有道理的：她没有意见时硬要她写一句，就是逼她编；她留了
// 意见时，那些意见本身就是给印记的指令；只有推倒重来必须说清方向——否则印记
// 只能再猜一遍，她会拿到第二份同样不对的东西。
func TestPblArtifact_OnlyRedoNeedsADirection(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	// 每一档都要一件新的成果：一件只定得了一次。
	settle := func(body string) int {
		rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/artifacts", handOver)
		if rec.Code != http.StatusCreated {
			t.Fatalf("handover = %d; body=%s", rec.Code, rec.Body)
		}
		aid := artifactID(t, rec.Body.String())
		return pblPost(t, h, cookie,
			"/api/v1/pbl/projects/"+pid+"/artifacts/"+aid+"/settle", body).Code
	}

	if got := settle(`{"verdict":"kept"}`); got != http.StatusOK {
		t.Fatalf("审核通过 = %d, want 200 —— 没有意见时不该逼她编一句", got)
	}
	if got := settle(`{"verdict":"revise"}`); got != http.StatusOK {
		t.Fatalf("执行修改 = %d, want 200 —— 她的意见已经逐条留在审核里了", got)
	}
	if got := settle(`{"verdict":"dropped","why":"   "}`); got != http.StatusBadRequest {
		t.Fatalf("没给方向的重做 = %d, want 400", got)
	}
	if got := settle(`{"verdict":"dropped","why":"这次只写食堂那一段，别扯到全校"}`); got != http.StatusOK {
		t.Fatalf("给了方向的重做 = %d, want 200", got)
	}
	if got := settle(`{"verdict":"perfect","why":"好"}`); got != http.StatusBadRequest {
		t.Fatalf("unknown verdict = %d, want 400", got)
	}
}

// 一件成果只定一次。
func TestPblArtifact_SettlesOnlyOnce(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/artifacts", handOver)
	aid := artifactID(t, rec.Body.String())
	url := "/api/v1/pbl/projects/" + pid + "/artifacts/" + aid + "/settle"

	if rec := pblPost(t, h, cookie, url, `{"verdict":"kept"}`); rec.Code != http.StatusOK {
		t.Fatalf("first settle = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := pblPost(t, h, cookie, url, `{"verdict":"revise"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("second settle = %d, want 400", rec.Code)
	}
}

// The tool endpoint is retained even though the interaction is deferred: an
// unexplained tool is an ambush, and a decline is a record, not an absence.
func TestPblTool_EndpointRecordsReasonAndDecline(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/tools"

	if rec := pblPost(t, h, cookie, url, `{"tool":"persona","reason":"  "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("reasonless summon = %d, want 400", rec.Code)
	}
	rec := pblPost(t, h, cookie, url,
		`{"tool":"persona","reason":"你说的「大家」得先变成一个具体的人"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("summon = %d; body=%s", rec.Code, rec.Body)
	}
	tid := artifactID(t, rec.Body.String())

	// 铁律②: she may decline, and declining is recorded.
	rec = pblPost(t, h, cookie, url+"/"+tid+"/resolve", `{"status":"declined"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("decline = %d; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"status":"declined"`) {
		t.Fatalf("decline not recorded: %s", rec.Body)
	}

	// The tool vocabulary is a FREE string — a new tool must not need a
	// migration, because the toolbox is open.
	if rec := pblPost(t, h, cookie, url,
		`{"tool":"某种还没设计出来的工具","reason":"因为此刻需要它"}`); rec.Code != http.StatusCreated {
		t.Fatalf("unknown tool name = %d, want 201 (the toolbox is open)", rec.Code)
	}
}

// Someone else's project is indistinguishable from one that does not exist.
func TestPblArtifact_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	otherID := createStudent(t, pool, SeedSchoolID, "other-artifact@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblPost(t, h, other, "/api/v1/pbl/projects/"+pid+"/artifacts", handOver); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign handover = %d, want 404", rec.Code)
	}
}
