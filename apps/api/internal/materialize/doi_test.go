package materialize

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestExtractDOI(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://doi.org/10.1038/s41586-020-2649-2", "10.1038/s41586-020-2649-2", true},
		{"http://dx.doi.org/10.1234/abc.def", "10.1234/abc.def", true},
		{"https://www.nature.com/articles/10.1038/s41586-020-2649-2", "10.1038/s41586-020-2649-2", true},
		{"https://example.org/some/article", "", false},
		{"https://doi.org/10.1038/s41586-020-2649-2.", "10.1038/s41586-020-2649-2", true}, // trailing dot trimmed
	}
	for _, c := range cases {
		u, _ := url.Parse(c.in)
		got, ok := extractDOI(u)
		if ok != c.ok || got != c.want {
			t.Errorf("extractDOI(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// A DOI URL resolves via Crossref to the publisher landing page, which is then
// fetched directly; the Crossref title backfills when the page has no <title>.
func TestFetchReadable_DOIResolvesViaCrossref(t *testing.T) {
	// article body well over the main-content floor so the <article> subtree is
	// trusted; no <title> forces the Crossref-title fallback.
	longBody := strings.Repeat("出版社正文第一段，足够长以越过 main 阈值。", 20)
	landing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><article><p>` + longBody + `</p></article><div class="related"><p>相关推荐噪音</p></div></body></html>`))
	}))
	defer landing.Close()

	crossref := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"title":["真正的论文标题"],"resource":{"primary":{"URL":"` + landing.URL + `"}}}}`))
	}))
	defer crossref.Close()

	old := crossrefBase
	crossrefBase = crossref.URL + "/"
	defer func() { crossrefBase = old }()

	title, text, err := newUnguardedFetcher().FetchReadable(context.Background(), "https://doi.org/10.1234/abc")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if title != "真正的论文标题" {
		t.Errorf("title = %q, want Crossref fallback", title)
	}
	if !strings.Contains(text, "出版社正文第一段") {
		t.Errorf("landing body not fetched: %q", text)
	}
	if strings.Contains(text, "相关推荐噪音") {
		t.Errorf("out-of-article chrome should be dropped: %q", text)
	}
}

// When Crossref can't resolve (non-200), the original URL is fetched unchanged —
// DOI resolution only ever helps.
func TestFetchReadable_DOICrossrefFailFallsBack(t *testing.T) {
	crossref := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer crossref.Close()
	old := crossrefBase
	crossrefBase = crossref.URL + "/"
	defer func() { crossrefBase = old }()

	// the "DOI" host is a loopback httptest server serving HTML — with Crossref
	// failing, FetchReadable must fetch this original URL directly.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>原始页</title></head><body><p>原始正文。</p></body></html>`))
	}))
	defer origin.Close()
	// embed a DOI in the path so extractDOI fires, but keep the loopback host
	_, text, err := newUnguardedFetcher().FetchReadable(context.Background(), origin.URL+"/10.1234/xyz")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(text, "原始正文") {
		t.Errorf("fallback fetch of original URL failed: %q", text)
	}
}

func TestFetchReadable_SendsBrowserUA(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>x</p></body></html>`))
	}))
	defer srv.Close()
	if _, _, err := newUnguardedFetcher().FetchReadable(context.Background(), srv.URL); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(gotUA, "Mozilla/5.0") || !strings.Contains(gotUA, "MindImprint") {
		t.Errorf("UA = %q, want browser-like self-identifying UA", gotUA)
	}
}

// The <article>/<main> subtree is preferred over whole-page chrome when rich
// enough; a thin/empty wrapper falls back to the full-page scrape.
func TestExtractPrefersMainContent(t *testing.T) {
	long := strings.Repeat("正文内容很长。", 40)
	body := []byte(`<html><head><title>T</title></head><body><nav><p>导航</p></nav><main><p>` + long + `</p></main><footer><p>页脚噪音</p></footer><div class="sidebar"><p>侧栏推荐</p></div></body></html>`)
	title, text := extractHTML(body)
	if title != "T" {
		t.Errorf("title = %q", title)
	}
	if !strings.Contains(text, "正文内容很长") {
		t.Errorf("main content missing: %q", text)
	}
	if strings.Contains(text, "侧栏推荐") || strings.Contains(text, "页脚噪音") {
		t.Errorf("out-of-main chrome should be dropped: %q", text)
	}
}
