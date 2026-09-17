package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 产品负责人 2026-09-17 逐字：
//
//	I don't want a chat-only page. just let the students be able to come back
//	to the reading page, the original reading page. they can even send
//	messages! to chat more.
//
// 完成页那三格（报告 · 对话 · 原文，2026-09-16）给的是一份**看**的东西。
// 这一条给的是那间房子本身。
//
// 🚨 守着的不是「接口返回 200」，是**那道 finished 写闸真的松开了**：
// TestFinishedReading_RejectsMutatingRequests 里那十一条路在完成之后一律 403，
// 而她按下「继续阅读」之后必须重新走得通 —— 否则她回到房间里，输入框还是哑的。

func readingStatus(t *testing.T, h http.Handler, cookie *http.Cookie, id string) (string, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET reading = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var got struct {
		Status     string  `json:"status"`
		FinishedAt *string `json:"finishedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode reading: %v; body=%s", err, rec.Body)
	}
	return got.Status, got.FinishedAt != nil
}

func TestReopenReading_LetsHerWriteToItAgain(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)
	finishReadingAtom(t, h, cookie, id)

	// 完成之后，改名这条路（和别的十条一样）是 403。
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/readings/"+id,
		strings.NewReader(`{"title":"改个名"}`)), cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("完成之后 PATCH = %d, want 403（那道闸没在守）", rec.Code)
	}
	if status, hasFinishedAt := readingStatus(t, h, cookie, id); status != "finished" || !hasFinishedAt {
		t.Fatalf("完成之后 status=%q finishedAt?=%v", status, hasFinishedAt)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/reopen",
		strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST reopen = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	// 🚨 这一条才是这个测试的意义：闸松开了。
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/readings/"+id,
		strings.NewReader(`{"title":"改个名"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("继续阅读之后 PATCH = %d, want 200 —— 她回到房间里，手还是被绑着的; body=%s",
			rec.Code, rec.Body)
	}
	// 屏幕上不该同时写着「进行中」和一个完成时间。
	status, hasFinishedAt := readingStatus(t, h, cookie, id)
	if status != "active" {
		t.Errorf("继续阅读之后 status=%q, want active", status)
	}
	if hasFinishedAt {
		t.Error("继续阅读之后还留着一个完成时间")
	}
}

// 再按一次「完成这篇」照常收得住，而且这一次是真的又完成了一次。
func TestReopenReading_CanBeFinishedAgain(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)
	finishReadingAtom(t, h, cookie, id)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/reopen", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST reopen = %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/finish",
		strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("第二次 POST finish = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if status, hasFinishedAt := readingStatus(t, h, cookie, id); status != "finished" || !hasFinishedAt {
		t.Fatalf("第二次完成之后 status=%q finishedAt?=%v", status, hasFinishedAt)
	}
}

// 没完成过的阅读上按它是 no-op，不会把什么状态洗掉。
func TestReopenReading_OnAnOpenReadingIsANoOp(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/reopen", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST reopen on an open reading = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if status, _ := readingStatus(t, h, cookie, id); status != "active" {
		t.Errorf("status=%q, want active", status)
	}
}

// 归属那一条不在这里测：reopen 走的是 loadOwnedReadingAtomRow，和 finishReading
// 同一个装载器，那条「别人的一律平 404」已经有自己的测试守着。
