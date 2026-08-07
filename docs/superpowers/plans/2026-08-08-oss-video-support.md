# OSS video resource support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make the OSS upload/resolve API + the developer docs **video-compatible** — accept `video/mp4`, `video/webm`, `video/quicktime` in the appropriate scope(s) with a video-sized cap, map their extensions, add a real body-size guard on the presign path, and (optional last task) let a course render a `video` asset.

**Architecture:** OSS is a pure presigner (`apps/api/internal/oss/oss.go`); all allowlist/size policy lives one layer up in `apps/api/internal/api/oss.go` in the `ossScopes` registry. Adding video = extend that registry's `allowedTypes` + `maxBytes`, add `extByContentType` entries, and (defense-in-depth) cap the presign request body. Content-Type is signed into the PUT URL, so the client must send the exact type — already handled by `apps/web/src/api/oss.ts`. Course rendering is a separate consumer (`CourseAssetType` enum + `AssetView`), split into its own task so it can be dropped.

**Tech Stack:** Go (`apps/api`) · Zod contracts (`packages/contracts`) · React/TS (`apps/web`). Branch `oss-video` off `main` (AFTER the project-covers deploy lands).

## Global Constraints
- **The video-carrying scope is `course_material`** (`courses/` prefix, admin-key gated). Videos are course resources uploaded by admins/backend scripts, not student uploads — do NOT add video to `user_image` or `web_resource`. Raise ONLY `course_material.maxBytes`.
- **Video MIME allowlist (exact):** `video/mp4`, `video/webm`, `video/quicktime`. **Extension map:** `video/mp4`→`.mp4`, `video/webm`→`.webm`, `video/quicktime`→`.mov`.
- **Video size cap:** `500 << 20` (500 MB) for `course_material`. (Images/pdf in that scope keep working under the same, now-larger, cap — that's fine; the cap is a ceiling.)
- **Content-Type is signed into the PUT** (`SignUpload` binds `alioss.ContentType`); the client MUST send the exact `requiredContentType`. No new frontend work needed for upload — `oss.ts` already forwards `file.type`.
- **Soft-cap caveat:** `size` is client-declared and validated before signing; a presigned PUT can't hard-enforce bytes. Task 2 adds an `http.MaxBytesReader` on the presign *request* body (tiny JSON) — that guards the endpoint, NOT the OSS PUT itself; keep the doc's "soft check" note and extend it for video.
- **Design principles (course render task only):** any new/edited UI ≥14px body, 12px only for meta/hints; never `bg-mk-<token>/<opacity>`; one Tailwind class per property. `<video>` uses `controls`, `w-full`, `max-w-full`.
- **Go on macOS:** `CGO_ENABLED=0`; Go tests FOREGROUND (docker) — scoped `-run`. Pre-existing `TestWeeklyReportForSeededClass` flaky (ignore). Web: pnpm; tests `apps/web/test/**`; tsc 0 + vitest. Never `git add -A`.

---

## File Structure
- `apps/api/internal/api/oss.go` — `ossScopes` (video types + raised cap), `extByContentType` (video exts) (T1); presign body guard (T2).
- `apps/api/internal/api/oss_test.go` (or the existing OSS test file) — video-type accepted, oversized rejected, ext mapping (T1/T2).
- `docs/2026-07-27-oss-storage-developer-guide.md` + `docs/superpowers/specs/2026-07-27-oss-storage-infra-design.md` — video in the scopes table, size caps, gotchas (T3).
- `packages/contracts/src/course.ts` — `CourseAssetType` gains `"video"` (T4).
- `apps/web/src/shell/courses/AssetView.tsx` — `video` render branch (T4).

---

## Task 1: Backend — accept video MIME types + extensions in course_material

**Files:** `apps/api/internal/api/oss.go`; Test scoped `internal/api` (docker foreground).

**Interfaces — Produces:** `course_material` scope `allowedTypes` includes the 3 video types; `maxBytes` = `500 << 20`; `extByContentType` maps the 3 video types → `.mp4`/`.webm`/`.mov`. A presign request `{scope:"course_material", contentType:"video/mp4", size:<≤500MB>}` succeeds and returns a `.mp4` object key.

- [ ] **Step 1:** In `ossScopes` (`oss.go:49-68`), extend `course_material.allowedTypes` (currently `typeSet("image/png","image/jpeg","image/webp","application/pdf")`) to also include `"video/mp4","video/webm","video/quicktime"`; change its `maxBytes` from `50 << 20` to `500 << 20`.
- [ ] **Step 2:** In `extByContentType` (`oss.go:74-80`), add `"video/mp4": ".mp4"`, `"video/webm": ".webm"`, `"video/quicktime": ".mov"`.
- [ ] **Step 3: Test (docker foreground, scoped):** a `course_material` presign with `contentType:"video/mp4"`, `size: 200<<20` returns 200 with an object key ending `.mp4`; `contentType:"video/mp4"`, `size: 600<<20` returns the `file_too_large` error; a `web_resource` presign with `contentType:"video/mp4"` still returns `unsupported_type` (video NOT allowed outside course_material). `CGO_ENABLED=0 go test ./internal/api/ -run 'TestOss|TestUpload' -count=1` foreground (grep the existing OSS test name first).
- [ ] **Step 4: Commit** `feat(api): accept video (mp4/webm/mov) in course_material OSS scope + ext mapping`.

---

## Task 2: Backend — cap the presign request body (defense-in-depth)

**Files:** `apps/api/internal/api/oss.go` (`writeUploadURL` / the two handlers); Test.

**Interfaces — Produces:** the presign JSON request body is read through an `http.MaxBytesReader` (e.g. 4 KB) so a malformed/huge body is rejected with a clean 4xx, not buffered. (This guards the endpoint; the actual OSS PUT stays a soft cap — documented.)

- [ ] **Step 1:** In the two upload handlers (`ossAdminUploadURL` `oss.go:137`, `ossUserUploadURL` `oss.go:160`) OR in `writeUploadURL` before `decodeJSON`, wrap `r.Body = http.MaxBytesReader(w, r.Body, 4<<10)`. Confirm `decodeJSON` surfaces the resulting error as a 400 (grep `decodeJSON` in `dto.go:33-38`; if it returns a generic decode error, that's fine — assert 400).
- [ ] **Step 2: Test (scoped, foreground):** a presign request with a >4 KB body returns 400. Keep it minimal; reuse the OSS test harness. `CGO_ENABLED=0 go test ./internal/api/ -run 'TestOss|TestUpload' -count=1`.
- [ ] **Step 3: Commit** `feat(api): cap OSS presign request body with MaxBytesReader`.

---

## Task 3: Docs — video in the OSS developer guide + infra spec

**Files:** `docs/2026-07-27-oss-storage-developer-guide.md`, `docs/superpowers/specs/2026-07-27-oss-storage-infra-design.md`. No tests (docs).

**Interfaces — Produces:** the scopes table + gotchas + media list mention video.

- [ ] **Step 1:** In the developer guide, update the Scopes table (`docs/2026-07-27-oss-storage-developer-guide.md:82-86`) — `course_material` allowed types become `png, jpeg, webp, pdf, mp4, webm, mov` and its cap `500 MB`. Update the frontend example comment (~L100) if it enumerates types. Update "Where course JSON goes" media list (~L191) to include video.
- [ ] **Step 2:** In the Gotchas section (~L171-180), extend the "Size cap is a soft check" note: video caps are much larger, so the soft-check caveat matters more; mention the new presign-body `MaxBytesReader` (guards the endpoint, not the PUT). Add a one-line "Video Content-Type must be exact (`video/mp4` etc.), same signature rule as images."
- [ ] **Step 3:** In the infra spec (`docs/superpowers/specs/2026-07-27-oss-storage-infra-design.md`), add a short note in its scopes/types section that `course_material` carries video (mp4/webm/mov, 500 MB) — mirror the guide, keep it brief.
- [ ] **Step 4: Commit** `docs(oss): document video resources in course_material scope`.

---

## Task 4 (optional — course video rendering): CourseAssetType + AssetView video branch

**Files:** `packages/contracts/src/course.ts`, `apps/web/src/shell/courses/AssetView.tsx`; Tests. **Skip if the user wants OSS-API-only.**

**Interfaces — Consumes:** an OSS-hosted video object key resolved via `api.resolveUrl`. **Produces:** `CourseAssetType` includes `"video"`; `AssetView` renders a `<video controls>` for video assets.

- [ ] **Step 1:** `packages/contracts/src/course.ts:3` — add `"video"` to `CourseAssetType = z.enum([...])`. Check whether the asset schema needs a resolvable-key field (it already carries the same shape image assets use — reuse it). Contracts build green.
- [ ] **Step 2:** `AssetView.tsx` (the `switch`/branches at ~L67-152) — add a `case "video"` rendering `<video src={resolvedUrl} controls className="w-full max-w-full rounded-mk-sm" />` (resolve the key the same way the `image` branch does). No autoplay; `controls` on.
- [ ] **Step 3: Test:** a course asset `{type:"video", ...}` renders a `<video>` element with the resolved src; full web suite + tsc green.
- [ ] **Step 4: Commit** `feat(courses): render video course assets (<video controls>)`.

---

## Self-Review Checklist
- Video accepted ONLY in `course_material` (not user/web); cap 500 MB ✓.
- 3 MIME types + 3 ext mappings, symmetric ✓.
- Presign body guarded; PUT stays a documented soft cap ✓.
- Docs (guide + spec) enumerate video + caveats ✓.
- (T4) `CourseAssetType` + `AssetView` render `<video controls>` ✓.
- Green: scoped `internal/api` foreground; web tsc 0 + vitest; docs proofread.
- **NOTE:** deploy = `full` (api + web; no migration in this plan).
