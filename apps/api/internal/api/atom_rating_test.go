package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 报告底部那五颗星（0107）。
//
// 🚨 方向：**她评我们**。这不是对她的评分，不是等第，不进评估，也不喂模型。
// 所以下面钉住的三件事，每一件都是「读反了会怎样」：
//
//  1. 1–5 之外一律拒绝——包括 0。前端漏传字段会解出 0，把它当成一分存下去，
//     就是把她没做过的动作记成了最低分。
//  2. 没打星 = null，不是 0。「她没说」和「她给了一星」必须分得开。
//  3. 已完成的阅读必须还能打星。这正是她被问到的时刻。

func putRating(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT",
		"/api/v1/readings/"+id+"/rating", strings.NewReader(body)), cookie))
	return rec
}

func TestPutReadingRating_StoresAndOverwrites(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := putRating(t, h, cookie, id, `{"rating":4}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT rating = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var got struct {
		Rating *int `json:"rating"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body)
	}
	if got.Rating == nil || *got.Rating != 4 {
		t.Fatalf("rating = %v, want 4", got.Rating)
	}

	// She changes her mind: the star is an answer, not an append-only event.
	if rec := putRating(t, h, cookie, id, `{"rating":2}`); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"rating":2`) {
		t.Fatalf("second PUT = %d %s, want 200 with rating 2", rec.Code, rec.Body)
	}
}

func TestPutReadingRating_RefusesOutOfRange(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	// `{}` is the one that matters: a frontend that forgets the field decodes
	// to 0, and storing that would record a one-star she never gave.
	for _, body := range []string{`{}`, `{"rating":0}`, `{"rating":6}`, `{"rating":-1}`, `{"rating":null}`} {
		rec := putRating(t, h, cookie, id, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("PUT %s = %d, want 400; body=%s", body, rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "invalid_rating") {
			t.Fatalf("PUT %s: want invalid_rating, got %s", body, rec.Body)
		}
	}
}

func TestPutReadingRating_WorksOnAFinishedReading(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/finish", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("finish = %d; body=%s", rec.Code, rec.Body)
	}

	// The finished-reading write gate refuses edits to the RECORD. Rating the
	// experience is not editing the record — and it is asked for precisely
	// here, at the foot of the report, after finishing.
	if rec := putRating(t, h, cookie, id, `{"rating":5}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT rating on a finished reading = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}
