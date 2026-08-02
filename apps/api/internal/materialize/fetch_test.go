package materialize

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func reasonOf(t *testing.T, err error) string {
	t.Helper()
	var fe *FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("want *FetchError, got %T: %v", err, err)
	}
	return fe.Reason
}

func TestFetchReadableExtractsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>标题</title></head><body><nav>菜单</nav><p>第一段。</p><script>ignore()</script><p>第二段。</p></body></html>`))
	}))
	defer srv.Close()
	title, text, _, err := newUnguardedFetcher().FetchReadable(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if title != "标题" {
		t.Errorf("title = %q", title)
	}
	if !strings.Contains(text, "第一段。") || !strings.Contains(text, "第二段。") || strings.Contains(text, "菜单") || strings.Contains(text, "ignore") {
		t.Errorf("text extraction wrong: %q", text)
	}
}

func TestFetchReadableRejectsNonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"x":1}`))
	}))
	defer srv.Close()
	_, _, _, err := newUnguardedFetcher().FetchReadable(context.Background(), srv.URL)
	if reasonOf(t, err) != "unsupported_content" {
		t.Fatalf("want unsupported_content, got %v", err)
	}
}

func TestFetchReadableBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	_, _, _, err := newUnguardedFetcher().FetchReadable(context.Background(), srv.URL)
	if reasonOf(t, err) != "bad_status" {
		t.Fatalf("want bad_status, got %v", err)
	}
}

func TestFetchReadableBlocksLoopbackByScheme(t *testing.T) {
	// httptest servers listen on 127.0.0.1 — the SSRF guard must block the dial.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<p>hi</p>"))
	}))
	defer srv.Close()
	// Force the guard by pointing NewGuardedFetcher at it — see note below.
	_, _, _, err := newGuardedFetcher().FetchReadable(context.Background(), srv.URL)
	if reasonOf(t, err) != "blocked" {
		t.Fatalf("want blocked, got %v", err)
	}
}

func TestFetchReadableRejectsBadScheme(t *testing.T) {
	_, _, _, err := NewFetcher().FetchReadable(context.Background(), "file:///etc/passwd")
	if reasonOf(t, err) != "blocked" {
		t.Fatalf("want blocked, got %v", err)
	}
}
