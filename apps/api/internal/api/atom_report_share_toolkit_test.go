package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

// 产品负责人 2026-09-17：阅读报告上「段落工具」那一节（有她自己写的仿写）
// 和对话一样，**单独勾选**才跟着分享链接公开。默认不公开；撤销再重开不继承。

func shareWithHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string, transcript, toolkit bool) {
	t.Helper()
	body, _ := json.Marshal(map[string]bool{"includeTranscript": transcript, "includeToolkit": toolkit})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(http.MethodPost,
		"/api/v1/readings/"+id+"/report/share", strings.NewReader(string(body))), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("share = %d; body=%s", rec.Code, rec.Body)
	}
}

func publicHasToolkit(t *testing.T, h http.Handler, token string) bool {
	t.Helper()
	rec := getPublicReportHTTP(h, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("public GET = %d; body=%s", rec.Code, rec.Body)
	}
	var env struct {
		Report map[string]json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	_, ok := env.Report["toolkit"]
	return ok
}

func TestSharedToolkitIsASeparateOptIn(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtomHTTP(t, h, cookie, "一篇关于羽毛的文章")
	putReadingTakeawayHTTP(t, h, cookie, id, "我学到了保暖羽毛的作用")
	// 她在一段上开过结构解析（这一篇是中文的，翻译是英文专用的工具，
	// 报告里不会列它）—— 报告因此有「段落工具」那一节。
	if _, err := q.InsertReadingBlockNote(t.Context(), sqlc.InsertReadingBlockNoteParams{
		AtomID: uuid.MustParse(id), BlockID: "b1", Tool: "structure", Body: "这一段在承接上文",
	}); err != nil {
		t.Fatal(err)
	}
	finishReadingHTTP(t, h, cookie, id)
	own := getReadingReportHTTP(h, cookie, id)
	if !strings.Contains(own.Body.String(), `"toolkit"`) {
		t.Fatalf("她自己的报告里应该有段落工具那一节：%s", own.Body)
	}

	// 1. 默认不公开。
	shareWithHTTP(t, h, cookie, id, false, false)
	token, _ := decodeShareResponse(t, shareReportHTTP(t, h, cookie, "readings", id))
	if publicHasToolkit(t, h, token) {
		t.Error("没勾「公开段落工具」，访客却看到了那一节")
	}

	// 2. 勾了才公开，而且她自己那一侧读得回这个状态。
	shareWithHTTP(t, h, cookie, id, false, true)
	if !publicHasToolkit(t, h, token) {
		t.Error("勾了「公开段落工具」，访客还是看不到")
	}
	var env struct {
		IncludeToolkit bool `json:"includeToolkit"`
	}
	_ = json.Unmarshal(getReadingReportHTTP(h, cookie, id).Body.Bytes(), &env)
	if !env.IncludeToolkit {
		t.Error("她自己那一侧读不回「已勾选」—— 勾选框重开会显示成没勾")
	}

	// 3. 撤销再重开，不继承上一次的公开范围。
	if rec := revokeReportHTTP(t, h, cookie, "readings", id); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d", rec.Code)
	}
	token2, _ := decodeShareResponse(t, shareReportHTTP(t, h, cookie, "readings", id))
	if publicHasToolkit(t, h, token2) {
		t.Error("撤销再重开的链接继承了上一次的「公开段落工具」")
	}
}
