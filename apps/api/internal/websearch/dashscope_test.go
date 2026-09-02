package websearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// parseHits is where this package earns its keep: the tool's payload shape is
// not contractual, so the parser has to survive the vendor changing it. These
// are the shapes it has actually been seen to use, plus the two ways a parser
// like this usually breaks — a bare array and prose.
func TestParseHitsAcceptsTheShapesTheToolActuallyReturns(t *testing.T) {
	cases := map[string]struct {
		in       string
		wantN    int
		wantHead string
	}{
		"bare array":   {`[{"title":"a","url":"u","snippet":"s"}]`, 1, "a"},
		"results key":  {`{"results":[{"title":"b","url":"u"},{"title":"c"}]}`, 2, "b"},
		"data key":     {`{"data":[{"title":"d"}]}`, 1, "d"},
		"nested value": {`{"webPages":{"value":[{"title":"e"}]}}`, 1, "e"},
		// Prose comes back as one result rather than nothing: the caller then
		// still has something to show, which beats a silent empty list it
		// cannot tell from "no hits".
		"plain prose": {`没有找到相关结果。`, 1, ""},
		"empty":       {``, 0, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := parseHits(tc.in, 10)
			if len(got) != tc.wantN {
				t.Fatalf("got %d results, want %d (%#v)", len(got), tc.wantN, got)
			}
			if tc.wantHead != "" && got[0].Title != tc.wantHead {
				t.Errorf("first title = %q, want %q", got[0].Title, tc.wantHead)
			}
		})
	}
}

func TestParseHitsRespectsCount(t *testing.T) {
	got := parseHits(`{"results":[{"title":"a"},{"title":"b"},{"title":"c"}]}`, 2)
	if len(got) != 2 {
		t.Fatalf("got %d, want the count cap of 2", len(got))
	}
}

// The transport must accept BOTH answer formats. MCP's streamable HTTP lets a
// server reply with plain JSON or upgrade to SSE, and which one it picks is not
// something the client gets to decide — a client that only reads JSON works
// until the day a slow query gets streamed, and then fails as "unparseable".
func TestReadsBothPlainJSONAndSSEReplies(t *testing.T) {
	hits := `{"content":[{"type":"text","text":"{\"results\":[{\"title\":\"ok\"}]}"}]}`

	for _, mode := range []string{"json", "sse"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer k" {
					t.Errorf("Authorization = %q", got)
				}
				var body map[string]any
				_ = decode(r, &body)
				if body["method"] == "initialize" {
					w.Header().Set("Mcp-Session-Id", "sess-1")
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
					return
				}
				if got := r.Header.Get("Mcp-Session-Id"); got != "sess-1" {
					t.Errorf("session id not echoed back, got %q", got)
				}
				payload := `{"jsonrpc":"2.0","id":2,"result":` + hits + `}`
				if mode == "sse" {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = w.Write([]byte("event: message\ndata: " + payload + "\n\n"))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(payload))
			}))
			defer srv.Close()

			c := &Client{Endpoint: srv.URL, APIKey: "k", HTTP: srv.Client()}
			got, err := c.Search(context.Background(), "中国 可再生能源 实际减排", 3)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Title != "ok" {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

// A missing key must be a named, harmless error, not a call. Search is an
// enrichment: a reading room that dies because a search key is absent is worse
// than one that quietly has no results.
func TestMissingKeyIsANamedErrorAndNeverCalls(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()
	c := &Client{Endpoint: srv.URL, HTTP: srv.Client()}
	if _, err := c.Search(context.Background(), "q", 3); err != ErrNotConfigured {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
	if called {
		t.Error("an unconfigured client must not reach the network")
	}
}

// An upstream failure must not echo the request back. The key travels in a
// header, and an error that quotes the whole exchange is how a secret reaches a
// log (AGENTS.md: 密钥绝不进日志、抛出或渲染的错误).
func TestUpstreamErrorNeverCarriesTheKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()
	c := &Client{Endpoint: srv.URL, APIKey: "super-secret-key", HTTP: srv.Client()}
	_, err := c.Search(context.Background(), "q", 3)
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), "super-secret-key") {
		t.Fatalf("the key leaked into the error: %v", err)
	}
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// TestLiveWebSearch hits the real Bailian MCP service. Skipped unless
// LIVE_LLM=1 and a key is present — the free tier is 2000 calls, so a suite
// that ran this on every commit would spend it.
//
//	LIVE_LLM=1 DASHSCOPE_API_KEY=… go test ./internal/websearch -run TestLive -v
func TestLiveWebSearchReturnsRealHits(t *testing.T) {
	if os.Getenv("LIVE_LLM") != "1" {
		t.Skip("set LIVE_LLM=1 to call the real search service")
	}
	key := os.Getenv("DASHSCOPE_API_KEY")
	if key == "" {
		t.Skip("DASHSCOPE_API_KEY not set")
	}
	got, err := New(key).Search(context.Background(), "中国 可再生能源 装机量 实际减排", 3)
	if err != nil {
		t.Fatalf("live search failed: %v", err)
	}
	t.Logf("%d results", len(got))
	for _, r := range got {
		t.Logf("  %s — %s", r.Title, r.URL)
	}
	if len(got) == 0 {
		t.Error("no results for a query that certainly has some")
	}
}
