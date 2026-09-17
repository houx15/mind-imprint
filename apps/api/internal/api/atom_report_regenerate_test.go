package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 产品负责人 2026-09-17：「the report is real time or a saved data?… it is saved.
// then regenerated would change the content. but we need to let students know.」
//
// 报告是存下来的。她「继续阅读」再完成之后，报告按更全的记录重新生成 ——
// 分享过的也一样：链接不变，内容换新，而且报告上标出这是第几版。

type reportVersionJSON struct {
	Revision   int    `json:"revision"`
	RevisedAt  string `json:"revisedAt"`
	StaleSince string `json:"staleSince"`
	Stats      []struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
	} `json:"stats"`
}

func readReportVersion(t *testing.T, rec *httptest.ResponseRecorder) reportVersionJSON {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("report GET = %d; body=%s", rec.Code, rec.Body)
	}
	var env struct {
		Report reportVersionJSON `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	return env.Report
}

func chatTurnsOf(r reportVersionJSON) int {
	for _, s := range r.Stats {
		if s.Key == "chatTurns" {
			return s.Value
		}
	}
	return -1
}

func TestReopenedReadingRegeneratesItsSharedReport(t *testing.T) {
	prov := &countingProvider{inner: reportStubProvider()}
	h, cookie, q, _ := liteHandlerWithProvider(t, prov)
	id := finishedReadingID(t, h, cookie, q, "一篇关于气候变化的文章")

	v1 := readReportVersion(t, getReadingReportHTTP(h, cookie, id))
	if v1.Revision != 0 || v1.RevisedAt != "" {
		t.Fatalf("第一份报告不该带版本号：%+v", v1)
	}
	turnsBefore := chatTurnsOf(v1)

	shareRec := shareReportHTTP(t, h, cookie, "readings", id)
	if shareRec.Code != http.StatusOK {
		t.Fatalf("share = %d; body=%s", shareRec.Code, shareRec.Body)
	}
	token, _ := decodeShareResponse(t, shareRec)

	// 继续阅读，又说了一句话，再完成一次。
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/reopen", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen = %d; body=%s", rec.Code, rec.Body)
	}

	// 🚨 重新打开期间，分享链接仍然打得开，而且访客看不到记账字段。
	pub := getPublicReportHTTP(h, token)
	if pub.Code != http.StatusOK {
		t.Fatalf("重新打开期间公开链接 = %d，want 200（链接不该被悄悄撤掉）", pub.Code)
	}
	if strings.Contains(pub.Body.String(), "staleSince") {
		t.Errorf("访客看到了记账字段 staleSince：%s", pub.Body)
	}

	atomID := mustUUID(id)
	seq, err := q.NextAtomMessageSeq(context.Background(), atomID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.AppendAtomMessage(context.Background(), sqlc.AppendAtomMessageParams{
		AtomID: atomID, Seq: seq, Role: "student", Content: "回来之后我又想到一点：样本只有一个城市。",
	}); err != nil {
		t.Fatal(err)
	}
	finishReadingHTTP(t, h, cookie, id)

	v2 := readReportVersion(t, getReadingReportHTTP(h, cookie, id))
	if v2.Revision != 2 || v2.RevisedAt == "" {
		t.Fatalf("重新生成的那一份应该是第 2 版并带更新时间：%+v", v2)
	}
	if v2.StaleSince != "" {
		t.Errorf("新的一份不该还带着 staleSince：%+v", v2)
	}
	if got := chatTurnsOf(v2); got != turnsBefore+1 {
		t.Errorf("对话轮数 = %d，want %d —— 报告没有按新的记录重新生成", got, turnsBefore+1)
	}

	// 同一条分享链接，现在是新的内容。
	pub2 := getPublicReportHTTP(h, token)
	if pub2.Code != http.StatusOK {
		t.Fatalf("重新生成之后公开链接 = %d，want 200", pub2.Code)
	}
	if !strings.Contains(pub2.Body.String(), `"revision":2`) {
		t.Errorf("公开链接上还是旧的那一份：%s", pub2.Body)
	}

	// 第二次读报告不再重新生成（不是每次打开都花一次钱）。
	v2again := readReportVersion(t, getReadingReportHTTP(h, cookie, id))
	if v2again.Revision != 2 || v2again.RevisedAt != v2.RevisedAt {
		t.Errorf("第二次打开又重新生成了：%+v vs %+v", v2again, v2)
	}
}
