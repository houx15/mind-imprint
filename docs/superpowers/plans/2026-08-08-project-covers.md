# Project covers — full-bleed, selectable, randomized Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Project covers become a **no-padding, full-bleed** picture (from 15 uploaded webp assets) OR a color gradient — **selectable at project creation**, **randomized by default**. Replaces today's inset title-hash gradient.

**Architecture:** A `project.cover` text descriptor (`img:1..15` → an OSS-hosted webp from the embedded manifest, or `grad:<macaron>` → the existing `coverGradient`). Uploaded assets already live in OSS (`apps/api/internal/api/project_covers.json` maps index→objectKey). The backend randomizes a cover at create when the client sends none, emits `cover` + a signed `coverUrl` on project DTOs, and serves the 15 signed cover URLs for the create-drawer picker. The frontend adds a cover picker (15 images + 7 gradients, random default) and renders the cover full-bleed (no card padding).

**Tech Stack:** Go (`apps/api`, sqlc, goose) · Zod contracts · React/TS/Tailwind (`apps/web`). Branch `project-covers` (off main @`2157901`).

## Global Constraints
- **Assets already uploaded to prod OSS** — 15 webp under `web/` scope, mapped in `apps/api/internal/api/project_covers.json` (keys "1".."15" → objectKey). Embed it (`go:embed`); do NOT re-upload.
- **Cover value format:** `img:<1..15>` (a manifest index) OR `grad:<macaronName>` (one of the 7 `MACARONS`/`MACARON_NAMES` from `apps/web/src/ui/tokens.ts`). Nullable in DB; a null/absent cover falls back to today's title-hash gradient (deterministic) so old projects render unchanged.
- **Randomized default:** the create drawer preselects a RANDOM image cover on open (student sees + can change); if the client sends no cover, the server randomizes (`img:<rand 1..15>`) as a safety net. (Go `math/rand` is fine.)
- **OSS read pattern:** resolve `img:N` → manifest objectKey → `a.d.OSS.SignDownload(key, ossDownloadTTL)` → short-lived signed CDN URL, mirroring `apps/api/internal/api/cards_catalog.go:129-135`.
- **Design principles:** any new/edited UI ≥14px body, 12px only for meta/hints; never `bg-mk-<token>/<opacity>` / `border-mk-<token>/<NN>`; one class per property. Full-bleed images `object-cover w-full`, `max-width:100%`.
- **Go on macOS:** `CGO_ENABLED=0`; sqlc pin `@v1.27.0`; goose migration = next number `0058`; Go tests FOREGROUND (docker) — scoped `-run`. Pre-existing `TestWeeklyReportForSeededClass` flaky (ignore). Web: pnpm; tests `apps/web/test/**`; never `git add -A`.

---

## File Structure
- `apps/api/internal/store/migrations/0058_project_cover.sql` (NEW) — `ALTER TABLE project ADD COLUMN cover text` (T1).
- `apps/api/internal/store/queries/*.sql` — `CreateProject` gains `cover`; project read queries select `cover` (T1) → sqlc regen.
- `apps/api/internal/api/project_covers.go` (NEW) — `go:embed project_covers.json`; `projectCoverKey(idx)`, `randomProjectCover()`, `resolveCoverURL(cover)`; `GET /project-covers` handler (T1/T2).
- `apps/api/internal/api/project_create.go` — accept + store `cover` (randomize if empty) (T1).
- `apps/api/internal/studio/dto.go` + `packages/contracts` — project header/card DTO gains `cover` + `coverUrl` (T2).
- `apps/web/src/api/*` — `createProject` sends `cover`; `getProjectCovers()` client (T2/T3).
- `apps/web/src/workspace/CreateProjectDrawer.tsx` — cover picker (T3).
- `apps/web/src/workspace/Directory.tsx` + `apps/web/src/shell/home/HomePage.tsx` + `apps/web/src/ui/cover.ts` — full-bleed cover render (T4).

---

## Task 1: Backend — cover column, manifest helper, create-with-cover, /project-covers

**Files:** migration `0058_project_cover.sql`, `queries/*.sql` (+ regen), `project_covers.go` (new), `project_create.go`, `api.go`; Test scoped `internal/api` (docker foreground).

**Interfaces — Produces:** `project.cover text` (nullable). `projectCoverKey(idx int) (string, bool)` (manifest lookup). `randomProjectCover() string` (`"img:<1..N>"`). `GET /api/v1/project-covers` → `{covers:[{key:"img:1", url:<signed>}, …15]}` for the picker. `createProject` stores `cover` from the request (validated `img:N`/`grad:name`), or `randomProjectCover()` if empty.

- [ ] **Step 1:** Migration `0058_project_cover.sql` (goose Up: `ALTER TABLE project ADD COLUMN cover text;` Down: `ALTER TABLE project DROP COLUMN cover;`).
- [ ] **Step 2:** `project_covers.go`: `//go:embed project_covers.json` into a `map[string]string` (index→objectKey); `projectCoverManifest` sorted count `N`; `projectCoverKey(idx)` returns the objectKey; `randomProjectCover()` = `fmt.Sprintf("img:%d", rand.Intn(N)+1)`; `resolveCoverURL(cover string) string` — if `cover` starts `img:` and the index resolves, `a.d.OSS.SignDownload(key, ossDownloadTTL)` (guard nil OSS → ""); else "". `GET /project-covers` handler: for each manifest index, sign the url → `{covers:[{key,url}]}`. Route in `api.go` (session-protected; mirror an existing GET list route).
- [ ] **Step 3:** `project_create.go`: add `Cover string \`json:"cover"\`` to the request; validate (`img:<1..N>` or `grad:<known macaron>`); if empty/invalid → `randomProjectCover()`; thread into `CreateProjectParams` (add `cover` to the `CreateProject` sqlc query + regen). 
- [ ] **Step 4: Test (docker foreground, scoped):** creating a project with `cover:"img:3"` stores it; creating with no cover stores a valid `img:N`; `GET /project-covers` returns 15 entries with non-empty urls (when OSS configured; in tests OSS may be nil → assert the keys regardless). `CGO_ENABLED=0 go test ./internal/api/ -run 'TestProject|TestCover' -count=1` foreground.
- [ ] **Step 5: Commit** `feat(api): project.cover column + manifest + randomized default + GET /project-covers`.

---

## Task 2: Backend DTO + contracts — cover + coverUrl on project lists

**Files:** `apps/api/internal/studio/dto.go` (+ wherever the home/directory project list DTO is built), `packages/contracts`; Test.

**Interfaces — Produces:** the project header / card-list DTO gains `cover string` + `coverUrl string` (coverUrl = `resolveCoverURL(cover)` for `img:` covers, "" for `grad:`/null). Contracts mirror.

- [ ] **Step 1:** Add `Cover` + `CoverURL` to the project DTO(s) the home + directory consume (grep the project-list handler + `studio/dto.go`); populate `CoverURL` via `resolveCoverURL`. Add `cover`/`coverUrl` to the matching `packages/contracts` project schema.
- [ ] **Step 2: Test:** a project-list response includes `cover` + (for img covers, when OSS set) a `coverUrl`. Web contract test if applicable. tsc/build green.
- [ ] **Step 3: Commit** `feat(api,contracts): project cover + coverUrl on the project list DTO`.

---

## Task 3: Frontend — cover picker in the create drawer (randomized default)

**Files:** `apps/web/src/api/*` (`createProject` + `getProjectCovers`), `apps/web/src/workspace/CreateProjectDrawer.tsx`; Test.

**Interfaces — Consumes:** `GET /project-covers`. **Produces:** a cover section in the drawer — a grid of the 15 image thumbnails + the 7 gradient swatches; a RANDOM image preselected on open; the selected `cover` key sent in `createProject`.

- [ ] **Step 1:** `getProjectCovers(): Promise<{key:string;url:string}[]>` client. `createProject` request gains `cover?: string`.
- [ ] **Step 2:** In `CreateProjectDrawer`, add a "封面" section: fetch covers on open; render 15 `<img src={url} class="object-cover ...">` thumbs + 7 gradient swatches (from `coverGradientStyle`/macaron names). `useState` selected cover, defaulting to a random `img:N` chosen once on mount. Selecting one highlights it. On 开始, include `cover: selected`. **Design ≥14px labels; thumbs full-bleed object-cover; selection ring solid token.**
- [ ] **Step 3: Test:** the drawer shows cover options; a default is preselected; creating sends the chosen `cover`. Full web suite + tsc green.
- [ ] **Step 4: Commit** `feat(studio): cover picker in the create-project drawer (randomized default)`.

---

## Task 4: Frontend — full-bleed no-padding cover render

**Files:** `apps/web/src/ui/cover.ts` (a `CoverImage`/render helper), `apps/web/src/workspace/Directory.tsx`, `apps/web/src/shell/home/HomePage.tsx`; Test.

**Interfaces — Consumes:** `project.cover` + `project.coverUrl`. **Produces:** the card cover is FULL-BLEED (no `p-4` inset) — `img:` covers render `<img src={coverUrl} class="h-[120px] w-full object-cover">`, `grad:`/null covers render the gradient div; top corners rounded, flush to the card edges.

- [ ] **Step 1:** A shared render helper in `cover.ts`, e.g. `coverProps(project)` returning either `{img:url}` or `{gradientStyle}`, plus a `<ProjectCover project className/>` component that renders the full-bleed image-or-gradient with `rounded-t-mk-sm` and `overflow-hidden`.
- [ ] **Step 2:** `Directory.tsx` (cover at ~L96, inside `<Card ... p-4>`) + `HomePage.tsx` (cover at ~L88): make the cover FLUSH — either restructure the card so the cover sits outside the `p-4` content padding, or apply `-mx-4 -mt-4` + `rounded-t-mk-sm` to the cover so it bleeds to the card edges. Both surfaces change identically. `img:` → `<img object-cover>` via `coverUrl`; `grad:`/absent → gradient (fallback to `coverGradientStyle(title)` when cover is null). Keep the status Badge overlay.
- [ ] **Step 3: Test:** a project with an `img` cover + coverUrl renders an `<img>` full-bleed (no inset); a `grad`/null cover renders the gradient. Full web suite + tsc green.
- [ ] **Step 4: Commit** `feat(studio): full-bleed no-padding project covers (image or gradient)`.

---

## Self-Review Checklist
- Cover **selectable** at create (picker) · **randomized** default (server + drawer) · **full-bleed no-padding** (img or gradient) ✓.
- 15 webp served from OSS via signed urls (assets pre-uploaded; manifest embedded) ✓.
- Old projects (null cover) render the deterministic title-hash gradient — no break ✓.
- Design: images object-cover full-bleed, labels ≥14px, no sub-12px, no `bg-mk-<token>/<opacity>` ✓.
- Green: migration applies; Go build/scoped-api foreground; web tsc 0 + vitest.
- **NOTE:** deploy = `full` (migration 0058 + api + web).
