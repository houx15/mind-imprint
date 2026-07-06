# Slice 2a · Material Substrate — Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add the `material` entity and its HTTP API to the Go backend (+ the shared Zod contract): list, paste-create, fetch-from-seed (SSRF-guarded), and scratch update — the data substrate the Slice 3 card engine will anchor onto.

**Architecture:** Zod `Material` in `packages/contracts`; a dependency-free `internal/materialize` package (URL fetch + SSRF guard + HTML extraction + paragraph segmentation); a `0009_material.sql` migration + sqlc queries; `internal/api/material.go` handlers mirroring the existing `tasks.go`/`cards.go` ownership + boundary-validation idioms; the fetcher injected via a `Fetcher` interface on `Deps` so handler tests are deterministic.

**Tech Stack:** Go 1.26 (`net/http`, `golang.org/x/net/html`, pgx/v5, sqlc, goose, testcontainers). Backend commands run from `apps/api`.

## Global Constraints

- **Go + Postgres**; follow existing patterns. Ownership hidden as **404** (never 403), via `loadOwnedTask`.
- **No new external Go dependency** — HTML extraction uses `golang.org/x/net/html` (already in the module graph).
- **SSRF guard is required** on the seed fetch: https/http only; reject loopback/private/link-local/unspecified/non-global-unicast IPs at dial time (post-DNS, dialing the checked IP to defeat rebinding); ≤3 redirects; ≤2 MB body; 8 s timeout.
- **`from-seed` failure returns HTTP 422** with error code `material_fetch_failed` and `details.reason` ∈ `{no_seed, blocked, unreachable, bad_status, unsupported_content, too_large, empty}`; no material persisted on failure.
- **Material `kind`** ∈ `{article, draft}`; **`source`** ∈ `{fetched, pasted}`. PDF deferred.
- Backend tests use testcontainers (Docker up); run `make test` (or `go test ./...`) from `apps/api`; regenerate sqlc with `make sqlc`. `go test -short ./...` skips container tests.
- UI-facing strings are Chinese; code identifiers/comments English.

---

### Task 1: Zod `Material` contract

**Files:**
- Create: `packages/contracts/src/material.ts`
- Modify: `packages/contracts/src/index.ts`
- Test: `packages/contracts/src/material.test.ts`

**Interfaces:**
- Produces: `Material`, `MaterialBlock`, `MaterialKind`, `MaterialSource` Zod schemas + inferred types, exported from the package barrel.

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/src/material.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { Material } from "./material";

const valid = {
  id: "m1",
  task_id: "t1",
  kind: "article",
  source: "fetched",
  title: "卫星图看中国变绿",
  source_url: "https://example.com/a",
  blocks: [{ id: "b0", text: "第一段。" }, { id: "b1", text: "第二段。" }],
  scratch: "",
  created_at: "2026-07-06T00:00:00Z",
};

describe("Material contract", () => {
  it("parses a valid material", () => {
    expect(Material.parse(valid)).toMatchObject({ id: "m1", kind: "article" });
  });
  it("allows a null source_url (pasted draft)", () => {
    expect(Material.parse({ ...valid, source: "pasted", kind: "draft", source_url: null }).source_url).toBeNull();
  });
  it("rejects an unknown kind", () => {
    expect(Material.safeParse({ ...valid, kind: "pdf" }).success).toBe(false);
  });
  it("rejects an unknown source", () => {
    expect(Material.safeParse({ ...valid, source: "scraped" }).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd packages/contracts && npx vitest run src/material.test.ts`
Expected: FAIL — `Failed to resolve import "./material"`.

- [ ] **Step 3: Create the contract**

Create `packages/contracts/src/material.ts`:

```ts
import { z } from "zod";

export const MaterialKind = z.enum(["article", "draft"]);
export const MaterialSource = z.enum(["fetched", "pasted"]);

export const MaterialBlock = z.object({
  id: z.string(),
  text: z.string(),
});

export const Material = z.object({
  id: z.string(),
  task_id: z.string(),
  kind: MaterialKind,
  source: MaterialSource,
  title: z.string(),
  source_url: z.string().nullable(),
  blocks: z.array(MaterialBlock),
  scratch: z.string(),
  created_at: z.string(),
});

export type MaterialKind = z.infer<typeof MaterialKind>;
export type MaterialSource = z.infer<typeof MaterialSource>;
export type MaterialBlock = z.infer<typeof MaterialBlock>;
export type Material = z.infer<typeof Material>;
```

- [ ] **Step 4: Export from the barrel**

In `packages/contracts/src/index.ts`, add after the `export * from "./task";` line:

```ts
export * from "./material";
```

- [ ] **Step 5: Run — expect PASS**

Run: `cd packages/contracts && npx vitest run src/material.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/material.ts packages/contracts/src/material.test.ts packages/contracts/src/index.ts
git commit -m "feat(contracts): Material schema (blocks span model, article/draft)"
```

---

### Task 2: `internal/materialize` — fetch, SSRF guard, extract, segment

**Files:**
- Create: `apps/api/internal/materialize/segment.go`
- Create: `apps/api/internal/materialize/extract.go`
- Create: `apps/api/internal/materialize/guard.go`
- Create: `apps/api/internal/materialize/fetch.go`
- Test: `apps/api/internal/materialize/segment_test.go`
- Test: `apps/api/internal/materialize/guard_test.go`
- Test: `apps/api/internal/materialize/fetch_test.go`

**Interfaces:**
- Produces:
  - `type Block struct { ID string \`json:"id"\`; Text string \`json:"text"\` }`
  - `func Segment(text string) []Block`
  - `type FetchError struct { Reason string; Err error }` with `Error()`/`Unwrap()`
  - `type HTTPFetcher struct{...}`; `func NewFetcher() *HTTPFetcher`; `func (f *HTTPFetcher) FetchReadable(ctx context.Context, rawURL string) (title, text string, err error)` — errors are `*FetchError` carrying a machine `Reason`.
  - unexported `isBlockedIP(net.IP) bool`, `extractHTML([]byte) (title, text string)`.

- [ ] **Step 1: Write the segment + guard tests**

Create `apps/api/internal/materialize/segment_test.go`:

```go
package materialize

import "testing"

func TestSegmentSplitsParagraphsAndDropsBlanks(t *testing.T) {
	in := "第一段。\n\n  \n第二段，有点长。\n\n\n第三段。"
	got := Segment(in)
	if len(got) != 3 {
		t.Fatalf("want 3 blocks, got %d: %+v", len(got), got)
	}
	if got[0].ID != "b0" || got[1].ID != "b1" || got[2].ID != "b2" {
		t.Fatalf("ids not sequential: %+v", got)
	}
	if got[0].Text != "第一段。" || got[2].Text != "第三段。" {
		t.Fatalf("unexpected text: %+v", got)
	}
}

func TestSegmentCollapsesInternalWhitespace(t *testing.T) {
	got := Segment("a   b\n c\td")
	if len(got) != 1 || got[0].Text != "a b c d" {
		t.Fatalf("want single collapsed block 'a b c d', got %+v", got)
	}
}

func TestSegmentEmptyInputYieldsNone(t *testing.T) {
	if got := Segment("   \n\n  "); len(got) != 0 {
		t.Fatalf("want 0 blocks, got %+v", got)
	}
}
```

Create `apps/api/internal/materialize/guard_test.go`:

```go
package materialize

import (
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "::1", "10.0.0.5", "192.168.1.1", "172.16.0.1", "169.254.169.254", "0.0.0.0", "fe80::1", "fc00::1"}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}
	for _, s := range allowed {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
	if !isBlockedIP(nil) {
		t.Errorf("nil IP must be blocked")
	}
}
```

- [ ] **Step 2: Run — expect FAIL (compile error, undefined Segment/isBlockedIP)**

Run: `cd apps/api && go test ./internal/materialize/`
Expected: FAIL — build errors: undefined `Segment`, `isBlockedIP`.

- [ ] **Step 3: Implement segment + guard**

Create `apps/api/internal/materialize/segment.go`:

```go
// Package materialize fetches a URL's readable text (SSRF-guarded) and segments
// text into paragraph blocks. It has no DB or api dependency.
package materialize

import (
	"fmt"
	"regexp"
	"strings"
)

// Block is one paragraph of a material; ID is stable within a material.
type Block struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

// Segment splits text on blank-line boundaries into paragraph blocks, collapsing
// internal whitespace and dropping empties. Ids are b0, b1, … in order.
func Segment(text string) []Block {
	blocks := make([]Block, 0)
	for _, part := range blankLine.Split(text, -1) {
		collapsed := strings.Join(strings.Fields(part), " ")
		if collapsed == "" {
			continue
		}
		blocks = append(blocks, Block{ID: fmt.Sprintf("b%d", len(blocks)), Text: collapsed})
	}
	return blocks
}
```

Create `apps/api/internal/materialize/guard.go`:

```go
package materialize

import "net"

// isBlockedIP reports whether ip must never be dialed (SSRF protection):
// loopback, private, link-local (incl. 169.254.169.254 cloud metadata),
// unspecified, multicast, or anything not global-unicast.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	return !ip.IsGlobalUnicast()
}
```

- [ ] **Step 4: Run — expect segment + guard tests PASS**

Run: `cd apps/api && go test ./internal/materialize/`
Expected: PASS for the segment + guard tests (the fetch test does not exist yet).

- [ ] **Step 5: Write the fetch test**

Create `apps/api/internal/materialize/fetch_test.go`:

```go
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
	title, text, err := NewFetcher().FetchReadable(context.Background(), srv.URL)
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
	_, _, err := NewFetcher().FetchReadable(context.Background(), srv.URL)
	if reasonOf(t, err) != "unsupported_content" {
		t.Fatalf("want unsupported_content, got %v", err)
	}
}

func TestFetchReadableBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	_, _, err := NewFetcher().FetchReadable(context.Background(), srv.URL)
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
	_, _, err := newGuardedFetcher().FetchReadable(context.Background(), srv.URL)
	if reasonOf(t, err) != "blocked" {
		t.Fatalf("want blocked, got %v", err)
	}
}

func TestFetchReadableRejectsBadScheme(t *testing.T) {
	_, _, err := NewFetcher().FetchReadable(context.Background(), "file:///etc/passwd")
	if reasonOf(t, err) != "blocked" {
		t.Fatalf("want blocked, got %v", err)
	}
}
```

Note on the two fetcher constructors: `NewFetcher()` is the production fetcher whose dialer applies the SSRF guard. Because `httptest.Server` binds `127.0.0.1`, the success/non-html/bad-status tests must use a fetcher **without** the loopback block, and one test must confirm the guard **does** block loopback. Implement two constructors: `NewFetcher()` (guarded, production) and an unexported `newUnguardedFetcher()` used only by the success/non-html/bad-status tests; and `newGuardedFetcher()` == `NewFetcher()` aliased for the block test's readability. Update the three passing tests below to call `newUnguardedFetcher()`.

- [ ] **Step 6: Fix the test to use the right constructors**

Edit `fetch_test.go`: in `TestFetchReadableExtractsHTML`, `TestFetchReadableRejectsNonHTML`, and `TestFetchReadableBadStatus`, replace `NewFetcher()` with `newUnguardedFetcher()`. Leave `TestFetchReadableBlocksLoopbackByScheme` calling `newGuardedFetcher()` and `TestFetchReadableRejectsBadScheme` calling `NewFetcher()` (bad scheme is rejected before any dial, so the guarded fetcher is correct there).

- [ ] **Step 7: Run — expect FAIL (undefined fetcher/extract symbols)**

Run: `cd apps/api && go test ./internal/materialize/`
Expected: FAIL — undefined `NewFetcher`, `newUnguardedFetcher`, `newGuardedFetcher`, `FetchError`.

- [ ] **Step 8: Implement extract + fetch**

Create `apps/api/internal/materialize/extract.go`:

```go
package materialize

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

var skipTags = map[string]bool{"script": true, "style": true, "nav": true, "header": true, "footer": true, "aside": true, "noscript": true}
var blockTags = map[string]bool{"p": true, "h1": true, "h2": true, "h3": true, "li": true, "blockquote": true}

// extractHTML pulls the <title> and the concatenated text of block elements,
// skipping script/style/nav/header/footer/aside/noscript. Paragraphs are joined
// with a blank line so Segment can split them.
func extractHTML(body []byte) (title, text string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", ""
	}
	var paras []string
	var walk func(n *html.Node, inSkip bool)
	walk = func(n *html.Node, inSkip bool) {
		if n.Type == html.ElementNode {
			if n.Data == "title" && title == "" {
				title = textContent(n)
			}
			if skipTags[n.Data] {
				inSkip = true
			}
			if !inSkip && blockTags[n.Data] {
				if t := textContent(n); t != "" {
					paras = append(paras, t)
				}
				return // captured; don't double-count nested blocks
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inSkip)
		}
	}
	walk(doc, false)
	return title, strings.Join(paras, "\n\n")
}

// textContent returns the whitespace-collapsed text of a node's subtree.
func textContent(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
```

Create `apps/api/internal/materialize/fetch.go`:

```go
package materialize

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	fetchTimeout = 8 * time.Second
	maxBodyBytes = 2 << 20 // 2 MiB
	maxRedirects = 3
)

// FetchError carries a stable machine reason for a failed fetch.
type FetchError struct {
	Reason string
	Err    error
}

func (e *FetchError) Error() string {
	if e.Err != nil {
		return "material fetch failed (" + e.Reason + "): " + e.Err.Error()
	}
	return "material fetch failed (" + e.Reason + ")"
}
func (e *FetchError) Unwrap() error { return e.Err }

// HTTPFetcher fetches and extracts readable text. Zero value is not usable; use a constructor.
type HTTPFetcher struct{ client *http.Client }

// NewFetcher returns the production, SSRF-guarded fetcher.
func NewFetcher() *HTTPFetcher { return newFetcher(true) }

// newUnguardedFetcher (test-only) skips the IP guard so httptest loopback servers are reachable.
func newUnguardedFetcher() *HTTPFetcher { return newFetcher(false) }

// newGuardedFetcher is an alias for NewFetcher used for test readability.
func newGuardedFetcher() *HTTPFetcher { return NewFetcher() }

func newFetcher(guard bool) *HTTPFetcher {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, &FetchError{Reason: "unreachable", Err: err}
			}
			if guard {
				for _, ip := range ips {
					if isBlockedIP(ip) {
						return nil, &FetchError{Reason: "blocked", Err: fmt.Errorf("blocked ip %s", ip)}
					}
				}
			}
			var lastErr error
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, &FetchError{Reason: "unreachable", Err: lastErr}
		},
	}
	c := &http.Client{
		Timeout:   fetchTimeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return &FetchError{Reason: "unreachable", Err: errors.New("too many redirects")}
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return &FetchError{Reason: "blocked", Err: errors.New("bad redirect scheme")}
			}
			return nil
		},
	}
	return &HTTPFetcher{client: c}
}

// FetchReadable fetches rawURL and returns an extracted title + text, or a *FetchError.
func (f *HTTPFetcher) FetchReadable(ctx context.Context, rawURL string) (string, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", &FetchError{Reason: "blocked", Err: errors.New("unsupported url")}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", &FetchError{Reason: "blocked", Err: err}
	}
	req.Header.Set("User-Agent", "MindImprintBot/1.0 (+material)")
	resp, err := f.client.Do(req)
	if err != nil {
		var fe *FetchError
		if errors.As(err, &fe) {
			return "", "", fe
		}
		return "", "", &FetchError{Reason: "unreachable", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", &FetchError{Reason: "bad_status", Err: fmt.Errorf("status %d", resp.StatusCode)}
	}
	ct := resp.Header.Get("Content-Type")
	isHTML := strings.HasPrefix(ct, "text/html")
	isText := strings.HasPrefix(ct, "text/plain")
	if !isHTML && !isText {
		return "", "", &FetchError{Reason: "unsupported_content", Err: fmt.Errorf("content-type %q", ct)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return "", "", &FetchError{Reason: "unreachable", Err: err}
	}
	if len(body) > maxBodyBytes {
		return "", "", &FetchError{Reason: "too_large", Err: nil}
	}
	if isText {
		return "", string(body), nil
	}
	title, text := extractHTML(body)
	return title, text, nil
}
```

- [ ] **Step 9: Ensure `golang.org/x/net` is a direct dependency**

Run: `cd apps/api && go mod tidy`
Expected: `go.mod` now lists `golang.org/x/net` in the direct `require` block (it was indirect). No network download needed — it's already in the module cache.

- [ ] **Step 10: Run — expect ALL materialize tests PASS**

Run: `cd apps/api && go test ./internal/materialize/`
Expected: PASS (segment 3, guard 1, fetch 5).

- [ ] **Step 11: Commit**

```bash
git add apps/api/internal/materialize/ apps/api/go.mod apps/api/go.sum
git commit -m "feat(api): materialize — SSRF-guarded fetch + html extract + segment"
```

---

### Task 3: DB layer — migration, queries, sqlc, DTO

**Files:**
- Create: `apps/api/internal/store/migrations/0009_material.sql`
- Create: `apps/api/internal/store/queries/material.sql`
- Generated (via `make sqlc`): `apps/api/internal/store/sqlc/material.sql.go` + `Material` in `models.go`
- Modify: `apps/api/internal/api/dto.go` (add `materialDTO` + `toMaterialDTO`)
- Test: `apps/api/internal/api/material_store_test.go`

**Interfaces:**
- Produces (sqlc-generated): `sqlc.Material{ ID uuid.UUID; TaskID uuid.UUID; Kind string; Source string; Title string; SourceUrl *string; Blocks []byte; Scratch string; CreatedAt time.Time }`; `Queries.CreateMaterial(ctx, CreateMaterialParams{TaskID,Kind,Source,Title,SourceUrl,Blocks}) (Material, error)`; `ListMaterialsByTask(ctx, taskID) ([]Material, error)`; `GetMaterial(ctx, id) (Material, error)`; `UpdateMaterialScratch(ctx, UpdateMaterialScratchParams{ID,TaskID,Scratch}) (Material, error)`.
- Produces: `toMaterialDTO(sqlc.Material) materialDTO` where `materialDTO` JSON tags match the Zod `Material` (`kind, source, title, source_url, blocks, scratch, created_at`).

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0009_material.sql`:

```sql
-- +goose Up
CREATE TABLE material (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('article','draft')),
    source     text NOT NULL CHECK (source IN ('fetched','pasted')),
    title      text NOT NULL,
    source_url text,
    blocks     jsonb NOT NULL DEFAULT '[]',
    scratch    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX material_task_created_idx ON material (task_id, created_at);

-- +goose Down
DROP TABLE material;
```

- [ ] **Step 2: Write the queries**

Create `apps/api/internal/store/queries/material.sql`:

```sql
-- name: CreateMaterial :one
INSERT INTO material (task_id, kind, source, title, source_url, blocks)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListMaterialsByTask :many
SELECT * FROM material
WHERE task_id = $1
ORDER BY created_at;

-- name: GetMaterial :one
SELECT * FROM material WHERE id = $1;

-- name: UpdateMaterialScratch :one
UPDATE material SET scratch = $3
WHERE id = $1 AND task_id = $2
RETURNING *;
```

- [ ] **Step 3: Generate sqlc**

Run: `cd apps/api && make sqlc`
Expected: no errors; `internal/store/sqlc/material.sql.go` created and a `Material` struct added to `internal/store/sqlc/models.go`. Confirm the generated `Material.Blocks` is `[]byte` and `Material.SourceUrl` is `*string`.

- [ ] **Step 4: Add the DTO**

In `apps/api/internal/api/dto.go`, add (after `toTaskDTO`):

```go
type materialDTO struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"task_id"`
	Kind      string          `json:"kind"`
	Source    string          `json:"source"`
	Title     string          `json:"title"`
	SourceURL *string         `json:"source_url"`
	Blocks    json.RawMessage `json:"blocks"`
	Scratch   string          `json:"scratch"`
	CreatedAt string          `json:"created_at"`
}

func toMaterialDTO(m sqlc.Material) materialDTO {
	blocks := json.RawMessage(m.Blocks)
	if len(blocks) == 0 {
		blocks = json.RawMessage("[]")
	}
	return materialDTO{
		ID:        m.ID.String(),
		TaskID:    m.TaskID.String(),
		Kind:      m.Kind,
		Source:    m.Source,
		Title:     m.Title,
		SourceURL: m.SourceUrl,
		Blocks:    blocks,
		Scratch:   m.Scratch,
		CreatedAt: m.CreatedAt.Format(tsLayout),
	}
}

func toMaterialDTOs(rows []sqlc.Material) []materialDTO {
	out := make([]materialDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toMaterialDTO(m))
	}
	return out
}
```

- [ ] **Step 5: Write the round-trip test**

Create `apps/api/internal/api/material_store_test.go`:

```go
package api_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

func TestMaterialQueriesRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "material rt"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	url := "https://example.com/a"
	m, err := q.CreateMaterial(ctx, sqlc.CreateMaterialParams{
		TaskID: task.ID, Kind: "article", Source: "fetched", Title: "标题",
		SourceUrl: &url, Blocks: []byte(`[{"id":"b0","text":"第一段。"}]`),
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	if m.Scratch != "" || m.Kind != "article" {
		t.Fatalf("defaults wrong: %+v", m)
	}

	list, err := q.ListMaterialsByTask(ctx, task.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}

	upd, err := q.UpdateMaterialScratch(ctx, sqlc.UpdateMaterialScratchParams{ID: m.ID, TaskID: task.ID, Scratch: "记一笔"})
	if err != nil || upd.Scratch != "记一笔" {
		t.Fatalf("scratch update: %v scratch=%q", err, upd.Scratch)
	}

	// Ownership scope: wrong task id → no rows.
	if _, err := q.UpdateMaterialScratch(ctx, sqlc.UpdateMaterialScratchParams{ID: m.ID, TaskID: uuid.New(), Scratch: "x"}); err == nil {
		t.Fatalf("expected no-rows error for wrong task scope")
	}
}
```

- [ ] **Step 6: Run — expect PASS**

Run: `cd apps/api && go test ./internal/api/ -run TestMaterialQueriesRoundTrip`
Expected: PASS (Docker required; skips under `-short`).

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/store/migrations/0009_material.sql apps/api/internal/store/queries/material.sql apps/api/internal/store/sqlc/ apps/api/internal/api/dto.go apps/api/internal/api/material_store_test.go
git commit -m "feat(api): material table + sqlc queries + DTO"
```

---

### Task 4: HTTP handlers — list / paste / from-seed / scratch

**Files:**
- Create: `apps/api/internal/api/material.go`
- Modify: `apps/api/internal/api/api.go` (add `Fetcher` to `Deps`; register routes)
- Modify: `apps/api/cmd/api/main.go` (wire `Fetcher: materialize.NewFetcher()`)
- Test: `apps/api/internal/api/material_test.go`

**Interfaces:**
- Consumes: `materialize.Segment`, `materialize.FetchError`, `materialize.NewFetcher`; `Queries.CreateMaterial/ListMaterialsByTask/UpdateMaterialScratch`; `loadOwnedTask`; `toMaterialDTO(s)`.
- Produces: `Fetcher` interface on `Deps`; handlers `listMaterials`, `createMaterial`, `materialFromSeed`, `updateMaterialScratch`.

- [ ] **Step 1: Write the handler test**

Create `apps/api/internal/api/material_test.go`:

```go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

type fakeFetcher struct {
	title, text string
	err         error
}

func (f fakeFetcher) FetchReadable(_ context.Context, _ string) (string, string, error) {
	return f.title, f.text, f.err
}

func TestMaterialPasteCreateListScratch(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, err := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/tasks/" + task.ID.String() + "/materials"

	// paste create
	body, _ := json.Marshal(map[string]any{"kind": "draft", "title": "我的初稿", "text": "第一段。\n\n第二段。"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base, bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		Material struct {
			ID     string          `json:"id"`
			Source string          `json:"source"`
			Blocks json.RawMessage `json:"blocks"`
		} `json:"material"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Material.Source != "pasted" || !bytes.Contains(created.Material.Blocks, []byte(`"第一段。"`)) {
		t.Fatalf("unexpected material: %s", rec.Body)
	}

	// list
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", base, nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("我的初稿")) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}

	// scratch
	sbody, _ := json.Marshal(map[string]any{"scratch": "记一笔"})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", base+"/"+created.Material.ID+"/scratch", bytes.NewReader(sbody)), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("记一笔")) {
		t.Fatalf("scratch: %d %s", rec.Code, rec.Body)
	}
}

func TestMaterialPasteValidation(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/tasks/" + task.ID.String() + "/materials"
	for _, tc := range []map[string]any{
		{"kind": "pdf", "title": "x", "text": "y"},   // bad kind
		{"kind": "draft", "title": "x", "text": ""},  // empty text
		{"kind": "draft", "title": "x", "text": "  "}, // whitespace-only → no blocks
	} {
		body, _ := json.Marshal(tc)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", base, bytes.NewReader(body)), cookie))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400 for %v, got %d %s", tc, rec.Code, rec.Body)
		}
	}
}

func TestMaterialFromSeedNoSeed(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"}) // no seed
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/materials/from-seed", nil), cookie))
	if rec.Code != http.StatusUnprocessableEntity || !bytes.Contains(rec.Body.Bytes(), []byte("no_seed")) {
		t.Fatalf("want 422 no_seed, got %d %s", rec.Code, rec.Body)
	}
}

func TestMaterialFromSeedSuccess(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	seed := "https://example.com/a"
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t", Seed: &seed})
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{title: "标题", text: "第一段。\n\n第二段。"}}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/materials/from-seed", nil), cookie))
	if rec.Code != http.StatusCreated || !bytes.Contains(rec.Body.Bytes(), []byte(`"fetched"`)) || !bytes.Contains(rec.Body.Bytes(), []byte("第一段。")) {
		t.Fatalf("want 201 fetched material, got %d %s", rec.Code, rec.Body)
	}
}

func TestMaterialListOwnershipHiddenAs404(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	// A task owned by the admin, requested by the seed student → 404.
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedAdminID.String()), Title: "t"})
	h := New(Deps{Queries: q, Pool: pool, Fetcher: fakeFetcher{}}).Handler()
	cookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/"+task.ID.String()+"/materials", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rec.Code, rec.Body)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/api/ -run TestMaterial`
Expected: FAIL — `Deps` has no `Fetcher` field; handlers undefined.

- [ ] **Step 3: Add `Fetcher` to `Deps`**

In `apps/api/internal/api/api.go`, add to the imports `"context"` (already present) and add the interface + field. After the `Enqueuer` interface, add:

```go
// Fetcher retrieves readable text from a URL. materialize.HTTPFetcher is the
// production impl; tests inject a fake.
type Fetcher interface {
	FetchReadable(ctx context.Context, url string) (title, text string, err error)
}
```

In the `Deps` struct, add a field:

```go
	Fetcher Fetcher // fetches material text from a seed URL
```

- [ ] **Step 4: Register the routes**

In `apps/api/internal/api/api.go` `Handler()`, in the protected group (after the card routes), add:

```go
	mux.Handle("GET /api/v1/tasks/{id}/materials", protected(a.listMaterials))
	mux.Handle("POST /api/v1/tasks/{id}/materials", protected(a.createMaterial))
	mux.Handle("POST /api/v1/tasks/{id}/materials/from-seed", protected(a.materialFromSeed))
	mux.Handle("PUT /api/v1/tasks/{id}/materials/{mid}/scratch", protected(a.updateMaterialScratch))
```

- [ ] **Step 5: Implement the handlers**

Create `apps/api/internal/api/material.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
)

const (
	maxPasteBytes   = 500 * 1024
	maxScratchBytes = 20 * 1024
)

func writeMaterialOrNotFound(w http.ResponseWriter, r *http.Request, m sqlc.Material, err error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"material": toMaterialDTO(m)})
}

func writeFetchFailed(w http.ResponseWriter, reason string) {
	httpx.WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error": map[string]any{
			"code":    "material_fetch_failed",
			"message": "无法读取该链接",
			"details": map[string]any{"reason": reason},
		},
	})
}

func (a *API) listMaterials(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListMaterialsByTask(r.Context(), t.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"materials": toMaterialDTOs(rows)})
}

func (a *API) createMaterial(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Kind != "article" && body.Kind != "draft" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "kind 非法", nil))
		return
	}
	if len(body.Text) == 0 || len(body.Text) > maxPasteBytes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "text 长度非法", nil))
		return
	}
	blocks := materialize.Segment(body.Text)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "text 无有效内容", nil))
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = "未命名材料"
	}
	m, err := a.createMaterialRow(w, r, t.ID, body.Kind, "pasted", title, nil, blocks)
	if err != nil {
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"material": toMaterialDTO(m)})
}

func (a *API) materialFromSeed(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	if t.Seed == nil || strings.TrimSpace(*t.Seed) == "" {
		writeFetchFailed(w, "no_seed")
		return
	}
	title, text, err := a.d.Fetcher.FetchReadable(r.Context(), *t.Seed)
	if err != nil {
		reason := "unreachable"
		var fe *materialize.FetchError
		if errors.As(err, &fe) {
			reason = fe.Reason
		}
		writeFetchFailed(w, reason)
		return
	}
	blocks := materialize.Segment(text)
	if len(blocks) == 0 {
		writeFetchFailed(w, "empty")
		return
	}
	if strings.TrimSpace(title) == "" {
		title = "来自链接的材料"
	}
	seed := *t.Seed
	m, err := a.createMaterialRow(w, r, t.ID, "article", "fetched", title, &seed, blocks)
	if err != nil {
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"material": toMaterialDTO(m)})
}

// createMaterialRow marshals blocks and inserts; on error it writes the envelope
// and returns err (non-nil) so callers can bail.
func (a *API) createMaterialRow(w http.ResponseWriter, r *http.Request, taskID uuid.UUID, kind, source, title string, sourceURL *string, blocks []materialize.Block) (sqlc.Material, error) {
	raw, err := json.Marshal(blocks)
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Material{}, err
	}
	m, err := a.d.Queries.CreateMaterial(r.Context(), sqlc.CreateMaterialParams{
		TaskID: taskID, Kind: kind, Source: source, Title: title, SourceUrl: sourceURL, Blocks: raw,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Material{}, err
	}
	return m, nil
}

func (a *API) updateMaterialScratch(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var body struct {
		Scratch string `json:"scratch"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.Scratch) > maxScratchBytes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "scratch 过长", nil))
		return
	}
	m, err := a.d.Queries.UpdateMaterialScratch(r.Context(), sqlc.UpdateMaterialScratchParams{
		ID: mid, TaskID: t.ID, Scratch: body.Scratch,
	})
	writeMaterialOrNotFound(w, r, m, err)
}
```

- [ ] **Step 6: Wire the production fetcher in main**

In `apps/api/cmd/api/main.go`, add the import `"mindimprint/api/internal/materialize"`, and in the `api.Deps{...}` literal passed to `api.New(...)`, add the field:

```go
		Fetcher: materialize.NewFetcher(),
```

- [ ] **Step 7: Run — expect PASS**

Run: `cd apps/api && go test ./internal/api/ -run TestMaterial`
Expected: PASS (all `TestMaterial*` cases).

- [ ] **Step 8: Full backend build + vet + short tests**

Run: `cd apps/api && go build ./... && go vet ./... && go test -short ./...`
Expected: builds clean; vet clean; short tests pass (container tests skipped).

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/api/material.go apps/api/internal/api/api.go apps/api/internal/api/material_test.go apps/api/cmd/api/main.go
git commit -m "feat(api): material HTTP handlers — list/paste/from-seed/scratch"
```

---

## Self-Review

**Spec coverage:** Material entity (contract T1; table/queries/DTO T3) ✅; SSRF-guarded fetch + extraction + segmentation (T2) ✅; endpoints list/paste/from-seed/scratch with 422 `material_fetch_failed{reason}` (T4) ✅; ownership 404 (T3 query scope + T4 `loadOwnedTask`) ✅; injectable `Fetcher` for deterministic from-seed test (T4) ✅; kinds article/draft, PDF deferred ✅; no new external dep (x/net/html) ✅.

**Placeholder scan:** No TBD/TODO. One deliberate throwaway (`seedTaskWithSeed` stub) is explicitly deleted in T4 Step 1's closing note before compile. Complete code in every code step.

**Type consistency:** `Fetcher.FetchReadable(ctx, url) (title, text string, err error)` identical in the `Deps` interface (T4), the fake (T4 test), and `materialize.HTTPFetcher` (T2). `materialize.Block{ID,Text}` and `Segment` used consistently in T2/T4. `CreateMaterialParams{TaskID,Kind,Source,Title,SourceUrl,Blocks}` and `UpdateMaterialScratchParams{ID,TaskID,Scratch}` match the queries in T3. `materialDTO` JSON tags (`kind/source/title/source_url/blocks/scratch`) match the Zod `Material` (T1).

**Ordering:** T1 contract → T2 materialize (independent) → T3 DB (independent of T2) → T4 handlers (consumes T2 + T3). Frontend is a separate plan (2b).

## Notes for the executor

- `SeedUserID`/`SeedAdminID` are exported `uuid.UUID` consts in package `api`; the test helper `mustUUID(SeedUserID.String())` mirrors existing usage. If `sqlc.CreateTaskParams` uses `Seed *string`, pass `&seed`.
- If `make sqlc` names a generated param/field differently than assumed (e.g. `SourceUrl` vs `SourceURL`), adjust the handler/DTO/test references to the generated names — the generated code is the source of truth. sqlc lowercases `url` to `Url` by default (hence `SourceUrl`).
