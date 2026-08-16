# OSS CDN URL 鉴权 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make all OSS read URLs CDN-cacheable while keeping the bucket private, by switching the `SignDownload` seam from OSS-presigned URLs to Aliyun CDN URL鉴权 (Type A), and wire a course-asset batch-signing endpoint + sync lookup resolver so 2.0 course assets resolve to real cacheable URLs.

**Architecture:** One server seam (`oss.Service.SignDownload`) changes its output from an OSS presigned GET to a Type A `auth_key` URL when `OSS_CDN_AUTH_KEY` is configured (else unchanged presigned fallback, for safe rollout). A new `POST /courses/{slug}/asset-urls` endpoint signs a batch of a course's relative asset paths (client collects them from the definition; server maps `courses/<slug>/<path>` and signs). The frontend resolver becomes a pure map lookup; the host player fetches the map after loading the definition and refreshes it before the window lapses.

**Tech Stack:** Go (`net/http`, `crypto/md5`), TypeScript/Zod (`packages/course-contract`), React (`apps/web`), vitest.

**Design doc:** `docs/superpowers/specs/2026-08-16-oss-cdn-url-auth-design.md`.

## Global Constraints

- **Secrets are server-side only.** `OSS_CDN_AUTH_KEY` never reaches the frontend, logs, error bodies, or stored data. Only signed URLs cross the boundary.
- **Type A exactly:** `auth_key = <ts>-<rand>-<uid>-<md5>`, `md5 = md5("<uri>-<ts>-<rand>-<uid>-<privateKey>")` lowercase hex, `uri` = `"/" + objectKey` (decoded), `rand = "0"`, `uid = "0"`, `ts` = unix seconds at generation.
- **Safe rollout / fallback:** when `OSS_CDN_AUTH_KEY == ""`, `SignDownload` MUST return the pre-existing OSS presigned URL (no behavior change). Only when it is set does the URL鉴权 path activate.
- **Go border-validates only.** The server never parses course block internals to find assets; the client sends the relative paths. The server only validates each path's shape and maps it to `courses/<slug>/<path>`.
- **Determinism in pure code:** `signTypeA` takes `ts` as a parameter (no `time.Now()` inside); `collectAssetPaths` is a pure walk. Real time enters only at `SignDownload` and the host `RuntimeCoursePlayer`.
- **Key convention:** a course's asset relative path `p` maps to OSS key `courses/<slug>/<p>`.
- **Window:** console 验证时长 = 7200s; server mirror `OSS_CDN_AUTH_WINDOW` default `7200`.

---

### Task 1: Go — Type A signer, config, `SignDownload` branch, caller updates

**Files:**
- Modify: `apps/api/internal/config/config.go` (add two env fields)
- Modify: `apps/api/internal/oss/oss.go` (Service fields, `New` wiring, `signTypeA`, `SignDownload` branch, `DownloadWindow`, `NewSigner`)
- Modify: `apps/api/internal/api/oss.go:267` + expiresAt (`ossResolveURL`)
- Modify: `apps/api/internal/api/project_covers.go:86` (`resolveCoverURL`)
- Modify: `apps/api/internal/api/cards_catalog.go:134`
- Modify: `apps/api/internal/api/course_scene.go:197`
- Test: `apps/api/internal/oss/oss_test.go` (new)

**Interfaces:**
- Produces: `oss.Service.SignDownload(objectKey string) (string, error)` (TTL param removed); `oss.Service.DownloadWindow() time.Duration`; `oss.NewSigner(cdnDomain, cdnAuthKey string, window time.Duration) *oss.Service` (network-free constructor for tests/other callers).
- Consumes: existing `config.Config`, `withScheme` (already in oss.go).

- [ ] **Step 1: Add config fields.** In `config.go`, inside the OSS block (after `OSSAdminKey`):

```go
	// OSSCDNAuthKey is the Aliyun CDN URL鉴权 Type A 主KEY. When set, download
	// URLs are signed as CDN URL鉴权 links (cacheable, auth_key excluded from the
	// cache key) instead of OSS presigned URLs. Empty ⇒ presigned fallback.
	OSSCDNAuthKey string `env:"OSS_CDN_AUTH_KEY"`
	// OSSCDNAuthWindow is the URL鉴权 validity window in seconds; it must mirror
	// the console 验证时长. Used to report expiresAt / schedule client refresh.
	OSSCDNAuthWindow int `env:"OSS_CDN_AUTH_WINDOW" envDefault:"7200"`
```

- [ ] **Step 2: Write the failing signer test.** Create `apps/api/internal/oss/oss_test.go`:

```go
package oss

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSignTypeA_KnownVector(t *testing.T) {
	const domain = "mind-oss.uni-robot.cn"
	const key = "courses/x/assets/videos/case.mp4"
	const priv = "testprivatekey"
	var ts int64 = 1_700_000_000

	got := signTypeA(domain, priv, key, ts)

	uri := "/" + key
	want := md5.Sum([]byte(fmt.Sprintf("%s-%d-%s-%s-%s", uri, ts, "0", "0", priv)))
	wantHex := hex.EncodeToString(want[:])
	wantURL := fmt.Sprintf("https://%s%s?auth_key=%d-0-0-%s", domain, uri, ts, wantHex)
	if got != wantURL {
		t.Fatalf("signTypeA = %q, want %q", got, wantURL)
	}
	if !strings.Contains(got, "?auth_key=1700000000-0-0-") {
		t.Fatalf("auth_key shape wrong: %q", got)
	}
}

func TestSignDownload_AuthKeyBranch(t *testing.T) {
	s := NewSigner("mind-oss.uni-robot.cn", "k", 2*time.Hour)
	url, err := s.SignDownload("courses/x/a.png")
	if err != nil {
		t.Fatalf("SignDownload: %v", err)
	}
	if !strings.Contains(url, "auth_key=") || strings.Contains(url, "Signature=") {
		t.Fatalf("expected URL鉴权 link, got %q", url)
	}
	if s.DownloadWindow() != 2*time.Hour {
		t.Fatalf("DownloadWindow = %v, want 2h", s.DownloadWindow())
	}
}
```

- [ ] **Step 3: Run it — expect FAIL** (`signTypeA`/`NewSigner`/new `SignDownload` undefined).

Run: `go test ./internal/oss/ -run 'TestSignTypeA_KnownVector|TestSignDownload_AuthKeyBranch' -count=1`
Expected: FAIL (build error: undefined).

- [ ] **Step 4: Implement in `oss.go`.** Add imports `crypto/md5`, `encoding/hex`. Add fields to `Service`:

```go
type Service struct {
	origin        *alioss.Bucket
	cdn           *alioss.Bucket
	cdnDomain     string        // custom CDN host, no scheme (e.g. mind-oss.uni-robot.cn)
	cdnAuthKey    string        // URL鉴权 Type A 主KEY; "" ⇒ presigned fallback
	cdnAuthWindow time.Duration // mirror of console 验证时长
}
```

In `New`, after building `cdnBucket`, return with the new fields set:

```go
	window := time.Duration(cfg.OSSCDNAuthWindow) * time.Second
	return &Service{
		origin:        originBucket,
		cdn:           cdnBucket,
		cdnDomain:     cfg.OSSCDNDomain,
		cdnAuthKey:    cfg.OSSCDNAuthKey,
		cdnAuthWindow: window,
	}, nil
```

Add the constant, the constructor, the pure signer, the new `SignDownload`, and the accessor (replace the old `SignDownload`):

```go
// presignFallbackTTL bounds the OSS presigned GET used before URL鉴权 is
// configured (cdnAuthKey == ""). Generous enough for large audio/video reads.
const presignFallbackTTL = 15 * time.Minute

// NewSigner builds a Service that only signs URL鉴权 download links (no OSS
// origin/CDN clients). Used by tests and any caller that needs signing without
// network access; SignUpload/PutObject/GetObject/Exists must not be called on it.
func NewSigner(cdnDomain, cdnAuthKey string, window time.Duration) *Service {
	return &Service{cdnDomain: cdnDomain, cdnAuthKey: cdnAuthKey, cdnAuthWindow: window}
}

// signTypeA builds an Aliyun CDN URL鉴权 Type A link. Pure (ts is injected) so it
// is deterministic and unit-testable. objectKey is assumed path-safe ASCII
// (uuid/kebab/sanitized-ext by construction); the md5 is computed over the
// decoded URI, matching what the edge recomputes.
func signTypeA(cdnDomain, privateKey, objectKey string, ts int64) string {
	uri := "/" + objectKey
	const rand, uid = "0", "0"
	sum := md5.Sum([]byte(fmt.Sprintf("%s-%d-%s-%s-%s", uri, ts, rand, uid, privateKey)))
	authKey := fmt.Sprintf("%d-%s-%s-%s", ts, rand, uid, hex.EncodeToString(sum[:]))
	return fmt.Sprintf("%s%s?auth_key=%s", withScheme(cdnDomain), uri, authKey)
}

// SignDownload returns a cacheable CDN read URL. With a URL鉴权 主KEY configured it
// emits a Type A auth_key link (auth_key is excluded from the CDN cache key, so
// reads cache). Without one it falls back to the legacy OSS presigned GET, so
// this code can ship before the console is cut over to URL鉴权.
func (s *Service) SignDownload(objectKey string) (string, error) {
	if s.cdnAuthKey != "" {
		return signTypeA(s.cdnDomain, s.cdnAuthKey, objectKey, time.Now().Unix()), nil
	}
	return s.cdn.SignURL(objectKey, alioss.HTTPGet, int64(presignFallbackTTL.Seconds()))
}

// DownloadWindow is how long a SignDownload URL stays valid: the URL鉴权 window
// when configured, else the presigned fallback TTL. Callers use it to report
// expiresAt and schedule refresh.
func (s *Service) DownloadWindow() time.Duration {
	if s.cdnAuthKey != "" && s.cdnAuthWindow > 0 {
		return s.cdnAuthWindow
	}
	return presignFallbackTTL
}
```

- [ ] **Step 5: Run signer tests — expect PASS.**

Run: `go test ./internal/oss/ -count=1`
Expected: PASS.

- [ ] **Step 6: Update the four callers (drop the TTL arg).**
  - `apps/api/internal/api/oss.go`: `ossResolveURL` → `url, err := a.d.OSS.SignDownload(req.ObjectKey)`; change the response `ExpiresAt` to `time.Now().Add(a.d.OSS.DownloadWindow()).UTC().Format(time.RFC3339)`.
  - `apps/api/internal/api/project_covers.go:86`: `url, err := a.d.OSS.SignDownload(key)`.
  - `apps/api/internal/api/cards_catalog.go:134`: `if url, err := a.d.OSS.SignDownload(key); err == nil {`.
  - `apps/api/internal/api/course_scene.go:197`: `url, err := a.d.OSS.SignDownload(key)`.

- [ ] **Step 7: Remove now-dead TTL consts if unused.** After Step 6, `grep -rn 'ossDownloadTTL\|sceneAudioTTL' apps/api/internal` — if a constant has no remaining references, delete its declaration (`ossDownloadTTL` in `oss.go`, `sceneAudioTTL` wherever declared). Leave `ossUploadTTL` untouched.

- [ ] **Step 8: Build + vet the whole API module.**

Run: `go build ./... && go vet ./internal/oss/ ./internal/api/`
Expected: clean.

- [ ] **Step 9: Commit.**

```bash
git add apps/api/internal/config/config.go apps/api/internal/oss/oss.go apps/api/internal/oss/oss_test.go apps/api/internal/api/oss.go apps/api/internal/api/project_covers.go apps/api/internal/api/cards_catalog.go apps/api/internal/api/course_scene.go
git commit -m "feat(oss): CDN URL鉴权 (Type A) SignDownload with presigned fallback"
```

---

### Task 2: Go — course asset-URL batch endpoint

**Files:**
- Create: `apps/api/internal/api/course_asset_urls.go`
- Modify: `apps/api/internal/api/api.go:217` (register route after the session routes)
- Test: `apps/api/internal/api/course_asset_urls_test.go` (new)

**Interfaces:**
- Consumes: `oss.Service.SignDownload` / `DownloadWindow` (Task 1), `httpx.WriteJSON/WriteError/ErrOSSUnavailable/ErrBadRequest`, `decodeJSON`, `Deps.OSS`.
- Produces: `POST /api/v1/courses/{slug}/asset-urls` → `{ assetUrls: Record<path,url>, expiresAt }`. Pure helpers `validRelativeAssetPath(string) bool`, `courseAssetKey(slug, path string) string`.

- [ ] **Step 1: Write the failing test.** Create `apps/api/internal/api/course_asset_urls_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/oss"
)

func TestValidRelativeAssetPath(t *testing.T) {
	ok := []string{"assets/videos/case.mp4", "interactions/html/sim.html", "a/b/c.png"}
	bad := []string{"", "/leading", "../escape", "a/../b", "http://x/y.png", "https://x", "data:text/plain,hi"}
	for _, p := range ok {
		if !validRelativeAssetPath(p) {
			t.Errorf("validRelativeAssetPath(%q) = false, want true", p)
		}
	}
	for _, p := range bad {
		if validRelativeAssetPath(p) {
			t.Errorf("validRelativeAssetPath(%q) = true, want false", p)
		}
	}
}

func TestCourseAssetKey(t *testing.T) {
	if got := courseAssetKey("compare-claims", "assets/videos/case.mp4"); got != "courses/compare-claims/assets/videos/case.mp4" {
		t.Fatalf("courseAssetKey = %q", got)
	}
}

func newAssetURLsRequest(t *testing.T, slug, body string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/courses/"+slug+"/asset-urls", strings.NewReader(body))
	r.SetPathValue("slug", slug)
	return r
}

func TestPostCourseAssetURLs_Signs(t *testing.T) {
	a := &API{d: Deps{OSS: oss.NewSigner("mind-oss.uni-robot.cn", "k", 2*time.Hour)}}
	w := httptest.NewRecorder()
	a.postCourseAssetURLs(w, newAssetURLsRequest(t, "demo", `{"paths":["assets/a.png","assets/a.png","assets/v.mp4"]}`))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		AssetURLs map[string]string `json:"assetUrls"`
		ExpiresAt string            `json:"expiresAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.AssetURLs) != 2 {
		t.Fatalf("assetUrls len = %d, want 2 (deduped)", len(resp.AssetURLs))
	}
	got := resp.AssetURLs["assets/a.png"]
	if !strings.Contains(got, "/courses/demo/assets/a.png?auth_key=") {
		t.Fatalf("signed url = %q", got)
	}
	if resp.ExpiresAt == "" {
		t.Fatalf("expiresAt empty")
	}
}

func TestPostCourseAssetURLs_BadPath(t *testing.T) {
	a := &API{d: Deps{OSS: oss.NewSigner("d", "k", time.Hour)}}
	w := httptest.NewRecorder()
	a.postCourseAssetURLs(w, newAssetURLsRequest(t, "demo", `{"paths":["../secret"]}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPostCourseAssetURLs_OSSDisabled(t *testing.T) {
	a := &API{d: Deps{}}
	w := httptest.NewRecorder()
	a.postCourseAssetURLs(w, newAssetURLsRequest(t, "demo", `{"paths":["assets/a.png"]}`))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`postCourseAssetURLs`/helpers undefined).

Run: `go test ./internal/api/ -run 'TestValidRelativeAssetPath|TestCourseAssetKey|TestPostCourseAssetURLs' -count=1`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement `course_asset_urls.go`:**

```go
package api

// course_asset_urls.go — OSS CDN URL鉴权 slice. POST /api/v1/courses/{slug}/asset-urls
// signs a batch of a course's relative asset paths into cacheable CDN read URLs.
// The client (which holds the full CourseDefinition) collects the paths; the
// server maps each to the key courses/<slug>/<path> and signs it — so a caller
// can only obtain URLs within its own course's asset namespace, and the server
// never parses course block internals. The same endpoint is the refresh endpoint
// (re-POST the same paths when a session outlasts the URL鉴权 window).

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
)

const (
	assetURLsBodyLimit = 16 << 10 // 16 KB: a few hundred short relative paths
	assetURLsMaxPaths  = 256
)

type courseAssetURLsReq struct {
	Paths []string `json:"paths"`
}

type courseAssetURLsResp struct {
	AssetURLs map[string]string `json:"assetUrls"`
	ExpiresAt string            `json:"expiresAt"`
}

// validRelativeAssetPath mirrors the shape of packages/course-contract's
// relativeAssetPathSchema: non-empty, no leading slash, no ".." segment, no
// scheme. It is a border check, not a schema port.
func validRelativeAssetPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return false
	}
	lower := strings.ToLower(p)
	return !strings.HasPrefix(lower, "http://") &&
		!strings.HasPrefix(lower, "https://") &&
		!strings.HasPrefix(lower, "data:")
}

// courseAssetKey maps a course slug + a validated relative asset path to its OSS
// object key.
func courseAssetKey(slug, path string) string {
	return "courses/" + slug + "/" + path
}

func (a *API) postCourseAssetURLs(w http.ResponseWriter, r *http.Request) {
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	slug := r.PathValue("slug")
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, "/") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_slug", "无效的课程标识。", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, assetURLsBodyLimit)
	var req courseAssetURLsReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(req.Paths) > assetURLsMaxPaths {
		httpx.WriteError(w, r, httpx.ErrBadRequest("too_many_paths", "资源路径过多。", nil))
		return
	}
	urls := make(map[string]string, len(req.Paths))
	for _, p := range req.Paths {
		if !validRelativeAssetPath(p) {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_asset_path", "无效的资源路径。", nil))
			return
		}
		if _, done := urls[p]; done {
			continue
		}
		url, err := a.d.OSS.SignDownload(courseAssetKey(slug, p))
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		urls[p] = url
	}
	httpx.WriteJSON(w, http.StatusOK, courseAssetURLsResp{
		AssetURLs: urls,
		ExpiresAt: time.Now().Add(a.d.OSS.DownloadWindow()).UTC().Format(time.RFC3339),
	})
}
```

- [ ] **Step 4: Register the route.** In `api.go`, after the `PUT .../session` line (`:217`):

```go
	mux.Handle("POST /api/v1/courses/{slug}/asset-urls", protected(a.postCourseAssetURLs)) // OSS CDN URL鉴权
```

- [ ] **Step 5: Run tests — expect PASS.**

Run: `go test ./internal/api/ -run 'TestValidRelativeAssetPath|TestCourseAssetKey|TestPostCourseAssetURLs' -count=1`
Expected: PASS.

- [ ] **Step 6: Build.**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 7: Commit.**

```bash
git add apps/api/internal/api/course_asset_urls.go apps/api/internal/api/course_asset_urls_test.go apps/api/internal/api/api.go
git commit -m "feat(api): POST /courses/{slug}/asset-urls — batch-sign course assets"
```

---

### Task 3: Contract — `collectAssetPaths`

**Files:**
- Create: `packages/course-contract/src/assets.ts`
- Modify: `packages/course-contract/src/index.ts` (export it)
- Test: `packages/course-contract/test/assets.test.ts` (new)

**Interfaces:**
- Consumes: `CourseDefinitionDocument` type from `./course`, block union from `./blocks`.
- Produces: `collectAssetPaths(document: CourseDefinitionDocument): string[]` — deduped relative asset paths, excluding `http(s)://` and `data:`.

- [ ] **Step 1: Write the failing test.** Create `packages/course-contract/test/assets.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { collectAssetPaths } from "../src/assets";
import type { CourseDefinitionDocument } from "../src/course";

// A synthetic document exercising every relativeAssetPathSchema-typed field.
// It need not pass full validation — collectAssetPaths only walks structure.
const doc = {
  schemaVersion: "2.0",
  course: {
    id: "c",
    title: "t",
    language: "en",
    estimatedMinutes: 5,
    objectives: [{ id: "o1", text: "x", evidenceBlockIds: ["b1"] }],
    opening: {
      learningPreview: [],
      personalization: { enabled: false, allowedSignals: [] },
      fallback: { text: "hi", audio: "assets/audio/open.mp3" },
    },
    closing: {
      preparedSummary: "s",
      takeaways: [],
      transferApplications: [],
      personalization: { enabled: false, allowedSignals: [] },
      fallback: { text: "bye", audio: "assets/audio/close.mp3" },
    },
    parts: [
      {
        id: "p1",
        title: "P",
        objectiveIds: ["o1"],
        slices: [
          {
            id: "s1",
            title: "S",
            objectiveIds: ["o1"],
            estimatedSeconds: 30,
            narrations: [{ id: "n1", text: "x", audio: "assets/audio/n1.mp3" }],
            blocks: [
              { id: "b1", type: "text", content: "no asset" },
              { id: "b2", type: "images", presentation: "single", items: [
                { id: "i1", source: "assets/images/a.png", alt: "a" },
                { id: "i2", source: "https://cdn.example/x.png", alt: "abs" }, // excluded
              ] },
              { id: "b3", type: "pdf", title: "P", source: "assets/pdfs/p.pdf" },
              { id: "b4", type: "video", source: "assets/videos/v.mp4",
                poster: "assets/images/poster.jpg", captions: "assets/captions/v.vtt",
                interaction: { source: "interactions/video/v.json" } },
              { id: "b5", type: "interactiveHtml", source: "interactions/html/sim.html",
                protocolVersion: "1.0", aspectRatio: "4:3" },
              { id: "b6", type: "images", presentation: "single", items: [
                { id: "i3", source: "assets/images/a.png", alt: "dup" }, // dedup
                { id: "i4", source: "data:image/png;base64,AAAA", alt: "inline" }, // excluded
              ] },
            ],
            layout: { kind: "full", slots: [] },
            workflow: {} as never,
            navigation: {} as never,
          },
        ],
      },
    ],
  },
} as unknown as CourseDefinitionDocument;

describe("collectAssetPaths", () => {
  it("returns every relative asset path, deduped, excluding absolute and data URIs", () => {
    const got = collectAssetPaths(doc).sort();
    expect(got).toEqual(
      [
        "assets/audio/close.mp3",
        "assets/audio/n1.mp3",
        "assets/audio/open.mp3",
        "assets/captions/v.vtt",
        "assets/images/a.png",
        "assets/images/poster.jpg",
        "assets/pdfs/p.pdf",
        "assets/videos/v.mp4",
        "interactions/html/sim.html",
        "interactions/video/v.json",
      ].sort(),
    );
  });
});
```

- [ ] **Step 2: Run — expect FAIL** (module missing).

Run: `pnpm --filter @mind-imprint/course-contract test -- assets`
Expected: FAIL (cannot resolve `../src/assets`).

- [ ] **Step 3: Implement `src/assets.ts`:**

```ts
import type { CourseDefinitionDocument } from "./course";

// collectAssetPaths walks a CourseDefinition and returns the deduped set of
// relative asset paths it references — every field typed relativeAssetPathSchema
// in the contract (keep in sync with that schema's usages). Absolute
// (http(s)://) and inline (data:) references are excluded: they need no signing.
function isRelativeAsset(p: string | undefined): p is string {
  return !!p && !/^https?:\/\//i.test(p) && !p.startsWith("data:");
}

export function collectAssetPaths(document: CourseDefinitionDocument): string[] {
  const out = new Set<string>();
  const add = (p: string | undefined) => {
    if (isRelativeAsset(p)) out.add(p);
  };
  const { course } = document;

  add(course.opening.fallback.audio);
  add(course.closing.fallback.audio);

  for (const part of course.parts) {
    for (const slice of part.slices) {
      for (const n of slice.narrations) add(n.audio);
      for (const block of slice.blocks) {
        switch (block.type) {
          case "images":
            for (const item of block.items) add(item.source);
            break;
          case "pdf":
            add(block.source);
            break;
          case "video":
            add(block.source);
            add(block.poster);
            add(block.captions);
            add(block.interaction?.source);
            break;
          case "interactiveHtml":
            add(block.source);
            break;
          // text, fillBlank, singleChoice carry no assets
        }
      }
    }
  }
  return [...out];
}
```

- [ ] **Step 4: Export from `src/index.ts`.** Add:

```ts
export { collectAssetPaths } from "./assets";
```

- [ ] **Step 5: Run test + typecheck — expect PASS.**

Run: `pnpm --filter @mind-imprint/course-contract test -- assets && pnpm --filter @mind-imprint/course-contract typecheck`
Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add packages/course-contract/src/assets.ts packages/course-contract/src/index.ts packages/course-contract/test/assets.test.ts
git commit -m "feat(course-contract): collectAssetPaths — deduped relative asset paths"
```

---

### Task 4: Web — lookup resolver + asset-urls client

**Files:**
- Rewrite: `apps/web/src/course/assetResolver.ts`
- Create: `apps/web/src/api/courseAssetUrls.ts`
- Test: `apps/web/src/course/assetResolver.test.ts` (new)
- Test: `apps/web/src/api/courseAssetUrls.test.ts` (new)

**Interfaces:**
- Consumes: `AssetResolver` from `@mind-imprint/course-runtime`, `apiFetch` from `@/api/client`.
- Produces: `resolveAssetPath(assetUrls, relativePath): string`; `makeCdnAssetResolver(getAssetUrls: () => Record<string,string>): AssetResolver`; `fetchCourseAssetUrls(slug, paths): Promise<{ assetUrls: Record<string,string>; expiresAt: string }>`.

- [ ] **Step 1: Write the failing resolver test.** Create `apps/web/src/course/assetResolver.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { resolveAssetPath, makeCdnAssetResolver } from "./assetResolver";

describe("resolveAssetPath", () => {
  const map = { "assets/a.png": "https://cdn/courses/x/assets/a.png?auth_key=1-0-0-ab" };
  it("returns the mapped URL on a hit", () => {
    expect(resolveAssetPath(map, "assets/a.png")).toBe(map["assets/a.png"]);
  });
  it("returns the path unchanged on a miss", () => {
    expect(resolveAssetPath(map, "assets/missing.png")).toBe("assets/missing.png");
  });
  it("passes absolute and data URIs through untouched", () => {
    expect(resolveAssetPath(map, "https://x/y.png")).toBe("https://x/y.png");
    expect(resolveAssetPath(map, "data:image/png;base64,AAAA")).toBe("data:image/png;base64,AAAA");
  });
});

describe("makeCdnAssetResolver", () => {
  it("reads the map live via the getter", () => {
    let m: Record<string, string> = {};
    const r = makeCdnAssetResolver(() => m);
    expect(r.resolve("assets/a.png")).toBe("assets/a.png");
    m = { "assets/a.png": "https://cdn/a?auth_key=x" };
    expect(r.resolve("assets/a.png")).toBe("https://cdn/a?auth_key=x");
  });
});
```

- [ ] **Step 2: Run — expect FAIL.**

Run: `pnpm --filter web test -- assetResolver`
Expected: FAIL (exports missing). (If the web package name differs, use the name from `apps/web/package.json`; check with `grep '"name"' apps/web/package.json`.)

- [ ] **Step 3: Rewrite `assetResolver.ts`:**

```ts
import type { AssetResolver } from "@mind-imprint/course-runtime";

// assetResolver.ts — the production AssetResolver (Course Runtime §4). Course
// assets are served as CDN URL鉴权 links minted server-side (POST
// /courses/{slug}/asset-urls) and handed to the client as a { relativePath → url }
// map. The runtime resolves paths SYNCHRONOUSLY, so resolution is a pure map
// lookup: no awaiting, no per-object signing here. Absolute (http(s)://) and
// inline (data:) references pass through untouched.

/** Pure lookup: mapped URL on a hit, the path unchanged on a miss/pass-through. */
export function resolveAssetPath(assetUrls: Record<string, string>, relativePath: string): string {
  if (/^https?:\/\//i.test(relativePath) || relativePath.startsWith("data:")) {
    return relativePath;
  }
  return assetUrls[relativePath] ?? relativePath;
}

/**
 * makeCdnAssetResolver returns a synchronous AssetResolver backed by a live map.
 * The getter is read on every resolve() so the host can refresh the signed URLs
 * (before the URL鉴权 window lapses) without rebuilding the resolver or the
 * adapters that own the session.
 */
export function makeCdnAssetResolver(getAssetUrls: () => Record<string, string>): AssetResolver {
  return {
    resolve(relativePath: string): string {
      return resolveAssetPath(getAssetUrls(), relativePath);
    },
  };
}
```

- [ ] **Step 4: Write the failing client test.** Create `apps/web/src/api/courseAssetUrls.test.ts`, mirroring the fetch-mock setup used by the sibling api tests (inspect `apps/web/src/api/courseDefinition.test.ts` if present; otherwise mock `./client`'s `apiFetch`):

```ts
import { describe, it, expect, vi, beforeEach } from "vitest";

const apiFetch = vi.fn();
vi.mock("./client", () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }));

import { fetchCourseAssetUrls } from "./courseAssetUrls";

beforeEach(() => apiFetch.mockReset());

describe("fetchCourseAssetUrls", () => {
  it("POSTs the paths and returns the envelope", async () => {
    apiFetch.mockResolvedValue({ assetUrls: { "assets/a.png": "https://cdn/a?auth_key=x" }, expiresAt: "2026-08-16T12:00:00Z" });
    const out = await fetchCourseAssetUrls("demo", ["assets/a.png"]);
    expect(apiFetch).toHaveBeenCalledWith(
      "/api/v1/courses/demo/asset-urls",
      { method: "POST", body: JSON.stringify({ paths: ["assets/a.png"] }) },
    );
    expect(out.assetUrls["assets/a.png"]).toContain("auth_key=");
    expect(out.expiresAt).toBe("2026-08-16T12:00:00Z");
  });
});
```

- [ ] **Step 5: Implement `courseAssetUrls.ts`:**

```ts
import { apiFetch } from "./client";

// courseAssetUrls.ts — client for POST /api/v1/courses/{slug}/asset-urls. Sends
// the course's relative asset paths (collected client-side via
// collectAssetPaths) and gets back a { relativePath → signed CDN URL } map plus
// the window's expiry. Also the refresh endpoint: re-call with the same paths.

export interface CourseAssetUrls {
  assetUrls: Record<string, string>;
  expiresAt: string;
}

export async function fetchCourseAssetUrls(slug: string, paths: string[]): Promise<CourseAssetUrls> {
  return apiFetch<CourseAssetUrls>(`/api/v1/courses/${slug}/asset-urls`, {
    method: "POST",
    body: JSON.stringify({ paths }),
  });
}
```

- [ ] **Step 6: Run tests + typecheck — expect PASS.**

Run: `pnpm --filter web test -- assetResolver courseAssetUrls && pnpm --filter web typecheck`
Expected: PASS. (Substitute the real web package name if different.)

- [ ] **Step 7: Commit.**

```bash
git add apps/web/src/course/assetResolver.ts apps/web/src/api/courseAssetUrls.ts apps/web/src/course/assetResolver.test.ts apps/web/src/api/courseAssetUrls.test.ts
git commit -m "feat(web): lookup AssetResolver + course asset-urls client"
```

---

### Task 5: Web — host wiring + refresh in `RuntimeCoursePlayer`

**Files:**
- Modify: `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx`
- Test: `apps/web/src/shell/courses/RuntimeCoursePlayer.test.tsx` (new or extend existing)

**Interfaces:**
- Consumes: `getCourseDefinition` (`@/api/courseDefinition`), `collectAssetPaths` (`@mind-imprint/course-contract`), `fetchCourseAssetUrls` (`@/api/courseAssetUrls`), `makeCdnAssetResolver` (`@/course/assetResolver`).
- Produces: the mounted `CoursePlayer` whose `assetResolver` resolves the course's assets from the fetched map; a refresh timer that re-signs before `expiresAt`.

- [ ] **Step 1: Write the failing component test.** Create `apps/web/src/shell/courses/RuntimeCoursePlayer.test.tsx`. Mock the api + contract collector; assert the flow calls `fetchCourseAssetUrls` with the collected paths and mounts the player. Mirror the render/mocking harness of the existing course tests under `apps/web/src/shell/courses/` (check for a sibling `*.test.tsx` and reuse its setup):

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

const getCourseDefinition = vi.fn();
const fetchCourseAssetUrls = vi.fn();
const collectAssetPaths = vi.fn();

vi.mock("@/api/courseDefinition", () => ({ getCourseDefinition: (...a: unknown[]) => getCourseDefinition(...a) }));
vi.mock("@/api/courseAssetUrls", () => ({ fetchCourseAssetUrls: (...a: unknown[]) => fetchCourseAssetUrls(...a) }));
vi.mock("@mind-imprint/course-contract", () => ({ collectAssetPaths: (...a: unknown[]) => collectAssetPaths(...a) }));
// Stub the heavy renderer: assert it receives an assetResolver that resolves a known asset.
vi.mock("@mind-imprint/course-renderer", () => ({
  CoursePlayer: ({ adapters }: { adapters: { assetResolver: { resolve: (p: string) => string } } }) => (
    <div data-testid="player">{adapters.assetResolver.resolve("assets/a.png")}</div>
  ),
}));
vi.mock("@/course/apiSessionAdapter", () => ({ makeApiSessionAdapter: () => ({ setStatus: vi.fn() }) }));
vi.mock("@/course/apiSceneGenerator", () => ({ makeApiSceneGenerator: () => ({}) }));

import { RuntimeCoursePlayer } from "./RuntimeCoursePlayer";

beforeEach(() => {
  getCourseDefinition.mockReset();
  fetchCourseAssetUrls.mockReset();
  collectAssetPaths.mockReset();
});

describe("RuntimeCoursePlayer", () => {
  it("loads the definition, collects paths, fetches asset urls, and resolves via the map", async () => {
    getCourseDefinition.mockResolvedValue({ schemaVersion: "2.0", course: {} });
    collectAssetPaths.mockReturnValue(["assets/a.png"]);
    fetchCourseAssetUrls.mockResolvedValue({ assetUrls: { "assets/a.png": "https://cdn/a?auth_key=x" }, expiresAt: "2999-01-01T00:00:00Z" });

    render(<RuntimeCoursePlayer slug="demo" onExit={() => {}} onFinish={() => {}} />);

    await waitFor(() => expect(screen.getByTestId("player")).toBeInTheDocument());
    expect(fetchCourseAssetUrls).toHaveBeenCalledWith("demo", ["assets/a.png"]);
    expect(screen.getByTestId("player")).toHaveTextContent("https://cdn/a?auth_key=x");
  });

  it("skips the asset-urls fetch when the course references no assets", async () => {
    getCourseDefinition.mockResolvedValue({ schemaVersion: "2.0", course: {} });
    collectAssetPaths.mockReturnValue([]);
    render(<RuntimeCoursePlayer slug="demo" onExit={() => {}} onFinish={() => {}} />);
    await waitFor(() => expect(screen.getByTestId("player")).toBeInTheDocument());
    expect(fetchCourseAssetUrls).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run — expect FAIL** (current player doesn't call the collector/fetch; resolves via slug-join).

Run: `pnpm --filter web test -- RuntimeCoursePlayer`
Expected: FAIL.

- [ ] **Step 3: Rewrite the data-loading + adapters section of `RuntimeCoursePlayer.tsx`.** Keep the header/JSX shell and the `setStatus`→`onFinish` wrapping. Replace imports and the load/adapters logic:

Replace the import of `makeCdnAssetResolver` usage and add:

```tsx
import { collectAssetPaths } from "@mind-imprint/course-contract";
import type { CourseDefinitionDocument } from "@mind-imprint/course-contract";
import { fetchCourseAssetUrls } from "@/api/courseAssetUrls";
```

Inside the component, hold the signed map in a ref (so the resolver reads it live and the adapters never rebuild), plus a state flag to gate render and force a re-resolve after refresh:

```tsx
  const assetUrlsRef = useRef<Record<string, string>>({});

  // Adapters are built once per slug. The sessionAdapter holds authoritative
  // session state, so it must survive re-renders and asset-url refreshes; the
  // resolver reads assetUrlsRef live, so a refresh never rebuilds this object.
  const adapters = useMemo<CourseRuntimeAdapters>(() => {
    const base = makeApiSessionAdapter(slug);
    const sessionAdapter: SessionAdapter = {
      ...base,
      async setStatus(sessionId, status) {
        await base.setStatus(sessionId, status);
        if (status === "completed") onFinishRef.current();
      },
    };
    return {
      assetResolver: makeCdnAssetResolver(() => assetUrlsRef.current),
      sessionAdapter,
      openingGenerator: makeApiSceneGenerator(slug),
      closingGenerator: makeApiSceneGenerator(slug),
    };
  }, [slug]);
```

Replace the load effect: load the definition, collect paths, fetch the map (skip when empty), stash it in the ref, then reveal the player; schedule a refresh before `expiresAt`:

```tsx
  useEffect(() => {
    let cancelled = false;
    let refreshTimer: ReturnType<typeof setTimeout> | undefined;
    setDocument(null);
    setError(null);
    assetUrlsRef.current = {};

    const scheduleRefresh = (slug: string, paths: string[], expiresAt: string) => {
      const lead = new Date(expiresAt).getTime() - Date.now() - 5 * 60_000; // 5 min early
      const delay = Math.max(lead, 60_000);
      refreshTimer = setTimeout(async () => {
        try {
          const next = await fetchCourseAssetUrls(slug, paths);
          if (cancelled) return;
          assetUrlsRef.current = next.assetUrls;
          setRefreshTick((t) => t + 1); // re-render so renderers re-resolve
          scheduleRefresh(slug, paths, next.expiresAt);
        } catch {
          /* transient; the next asset load falls back to the (now-stale) map */
        }
      }, delay);
    };

    (async () => {
      try {
        const doc = (await getCourseDefinition(slug)) as CourseDefinitionDocument;
        if (cancelled) return;
        const paths = collectAssetPaths(doc);
        if (paths.length > 0) {
          const signed = await fetchCourseAssetUrls(slug, paths);
          if (cancelled) return;
          assetUrlsRef.current = signed.assetUrls;
          scheduleRefresh(slug, paths, signed.expiresAt);
        }
        setDocument(doc);
      } catch (e) {
        if (cancelled) return;
        setError(e instanceof ApiError ? e.message : "课程定义加载失败");
      }
    })();

    return () => {
      cancelled = true;
      if (refreshTimer) clearTimeout(refreshTimer);
    };
  }, [slug]);
```

Add the state used above near the other hooks: `const [, setRefreshTick] = useState(0);` (import `useState` already present). Keep `document`/`error` state and the existing JSX branch (`error ? … : document ? <CoursePlayer …/> : loading`). `collectAssetPaths` returning a typed doc: `getCourseDefinition` returns `unknown`; the `as CourseDefinitionDocument` cast is acceptable at the host boundary (the runtime deeply validates).

- [ ] **Step 4: Run test + typecheck — expect PASS.**

Run: `pnpm --filter web test -- RuntimeCoursePlayer && pnpm --filter web typecheck`
Expected: PASS.

- [ ] **Step 5: Full web suite (guard against regressions in the course shell).**

Run: `pnpm --filter web test`
Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add apps/web/src/shell/courses/RuntimeCoursePlayer.tsx apps/web/src/shell/courses/RuntimeCoursePlayer.test.tsx
git commit -m "feat(web): RuntimeCoursePlayer fetches + refreshes signed asset URLs"
```

---

## Final verification (after all tasks)

- [ ] Go: `cd apps/api && go build ./... && go test ./internal/oss/ ./internal/api/ -short -count=1` (the new tests are DB-free; `-short` skips testcontainers).
- [ ] Contract: `pnpm --filter @mind-imprint/course-contract test && pnpm --filter @mind-imprint/course-contract typecheck`.
- [ ] Web: `pnpm --filter web test && pnpm --filter web typecheck`.
- [ ] Confirm no secret leaked: `grep -rn "OSS_CDN_AUTH_KEY" apps/web` returns nothing (frontend never references it).
- [ ] Update `apps/api/.env.example` with `OSS_CDN_AUTH_KEY=` and `OSS_CDN_AUTH_WINDOW=7200` (documentation only; no secret value). Update `docs/2026-07-27-oss-storage-developer-guide.md` Keys section to mention the new URL鉴权 secret + window and the 私有Bucket回源 requirement.
```
