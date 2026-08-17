# Slice 9 — Pre-Publish Asset Gate (HTML self-containment core)

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development.

**Goal:** ship refuses to publish a course whose interactive-HTML assets are missing or not self-contained (make external network requests). Closes the high-value, tractable core of review **P1-12** + the publish-time half of **P1-10** (HTML self-containment). Deep binary media validation (video H.264/AAC/faststart, PDF page-tree, WebVTT) + a server-served runtime CSP header are DEFERRED (need real assets + heavy binary parsing / OSS header config) — documented as follow-ups.

**Depends on:** the ship endpoint (`apps/api/internal/api/course_ship.go`) + `oss.Service.GetObject`.

## Task 1 — HTML self-containment validator + ship gate (apps/api)

- **Pure validator** — new `apps/api/internal/api/html_selfcontain.go` (package `api`) or a small `internal/htmlcheck` package: `ValidateHtmlSelfContained(html []byte) []SelfContainIssue` — flags external network dependencies in the HTML text: absolute `http(s)://` in `src=`/`href=`, `fetch(` / `XMLHttpRequest`, `import ... "http(s)://...`, `url(http(s)://...)`, `@import "http(s)://...`, external `<script src=http…>` / `<link href=http…>`. `data:` / `blob:` and no-network references are fine. Pure, thoroughly unit-tested (no I/O).
- **Wire into `postCourseShip`** (BEFORE flipping to published): border-walk the stored definition for every `interactiveHtml.source` path; for each, `a.d.OSS.GetObject` the object key `courses/<slug>/<path>` — a missing object is a blocking issue; otherwise run `ValidateHtmlSelfContained` and collect blocking issues. If ANY blocking issue → respond `422` with `{ issues: [{ assetPath, blockId?, message }] }` and do NOT publish. **Guard:** skip the whole stage when `a.d.OSS == nil` (unconfigured — like the audio step) so existing flows/tests aren't broken; a course with no interactiveHtml has nothing to check.
- **Tests:** `ValidateHtmlSelfContained` positive (a self-contained `<html>` with inline script/style/`data:` image passes) + negative (a `fetch("https://…")`, an external `<script src="https://…">`, a remote `<img src="http://…">`, a `@import url(https://…)` each fail) as pure unit tests. A ship handler test with a stub OSS: a definition whose interactiveHtml asset is self-contained → publishes; one whose asset does `fetch` external → `422`, status stays `preview`. **MUST keep the existing Part-1 `CourseShip` tests green** — run `go test ./internal/api/ -run CourseShip -count=1 -timeout 1800s`.

## Deferred (documented follow-ups — need real assets + heavy work)
- Video container/H.264/AAC/faststart, PDF page-tree/page-count, WebVTT structure validation (binary inspection; reuse `internal/docextract` for PDF).
- Runtime CSP: serve a restrictive `Content-Security-Policy` response header on HTML assets (OSS object metadata / a serving proxy) — infra config.
- The iframe expired-URL(403) recovery note from Slice 6.
