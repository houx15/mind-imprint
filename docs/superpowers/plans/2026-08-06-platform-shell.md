# Part 2 · Platform Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Rebuild the student platform shell against the Part-1 design system — a new icon nav, a brand-new **homepage** (greeting + recent projects + course recommendations), a projects page with square gradient-cover cards, and restyled 图鉴 / 成长报告 / 设置 / auth — with the accent picker persisted server-side.

**Architecture:** The shell keeps its non-router tab-switch model but gains a new default landing (`home`) and a 52px icon rail (`首页/项目/图鉴/我`). Covers are deterministic macaron gradients (no backend). Recent projects come from the existing `listProjects()` (already ordered `last_active_at DESC`). Course recommendations reuse the existing flat course list. The accent choice persists to the existing unused `users.avatar_color` column via one new `PUT` endpoint; `AccentProvider` initializes from `MeUser.avatar_color`.

**Tech Stack:** React + Vite + TS + Tailwind + the `apps/web/src/ui/` design-system library (Part 1); Go `net/http` + sqlc for the one backend endpoint. Tests: Vitest/jsdom (web, `apps/web/test/**`), Go `httptest`/testcontainers (api).

## Global Constraints

- **Consume Part 1, don't reinvent.** All chrome uses `apps/web/src/ui/` primitives (`Button`, `Surface`/`Card`/`CompactRow`, forms, `Badge`/`Tabs`/`Segmented`/`Progress`/`Stepper`, `Modal`/`Drawer`/`Menu`/`toast`, `Skeleton*`, `Pebble`, `PebbleProgress`/loaders, `Illustration`/`EmptyState`, `Icon`). Colors via `mk-*` tokens only — **no hardcoded hex** in new/rewritten code (except spec-sanctioned literals already used inside `ui/`).
- **Tailwind rule (from Part 1):** same-property utilities emit alphabetically — exactly ONE class per CSS property per element (prop→single-class map; copy `cx()` from `ui/Card.tsx`).
- **Scope = student platform only.** Do NOT touch `console/` (teacher/admin). Do NOT deploy (single deploy is Part 4).
- **Covers = macaron gradients**, deterministic by a seed string (title/slug). No image upload, no backend cover fields.
- **Accent persists to `users.avatar_color`** (repurposed to hold an accent preset id). Validate server-side against the 8 preset ids. `avatar_color` is currently unread by any UI, so repurposing is safe.
- **Voice & Tone (spec §19):** onboarding/empty/home copy is warm and lively (e.g. "快来创建你的第一个写作项目吧！"); no jargon ("agentic") in student copy. Coach copy is out of scope here.
- **No mobile, no dark mode** this round (desktop-first).
- **Don't regress existing behavior:** deep-links (project→growth, card→course), project creation, course play, evaluation report open must still work.
- Web verify per task: `npm --prefix apps/web test`, `npm --prefix apps/web run typecheck`, `npm --prefix apps/web run build`. Go verify: `cd apps/api && go test ./...` (needs Docker for testcontainers) or the specific package; `go build ./...`.

---

## File Structure

- `apps/api/internal/store/queries/users.sql` — **Modify:** add `SetUserAvatarColor`.
- `apps/api/internal/store/sqlc/users.sql.go` — **Regenerate** (sqlc, pin v1.27.0, CGO_ENABLED=0).
- `apps/api/internal/api/users_accent.go` — **Create:** `putUserAccent` handler + preset-id allowlist.
- `apps/api/internal/api/api.go` — **Modify:** route `PUT /api/v1/users/me/accent`.
- `apps/api/internal/api/users_accent_test.go` — **Create.**
- `apps/web/src/api/auth.ts` — **Modify:** add `setAccent(id)` client fn; `MeUser` already has `avatar_color`.
- `apps/web/src/ui/cover.ts` — **Create:** `coverGradient(seed)`.
- `apps/web/src/ui/accent.tsx` — **Modify:** accept `initialAccent` + `onPersist`.
- `apps/web/src/shell/Nav.tsx` — **Create:** the 52px icon rail (replaces `LeftRail`).
- `apps/web/src/shell/StudentApp.tsx` — **Modify:** new tab set + default `home`; wrap in `AccentProvider` seeded from user.
- `apps/web/src/shell/home/HomePage.tsx` — **Create.**
- `apps/web/src/shell/home/ProjectSnapshotCard.tsx`, `CourseRecCard.tsx` — **Create** (or inline).
- `apps/web/src/workspace/Directory.tsx` — **Modify:** restyle to gradient-cover card grid; extract create flow to `CreateProjectDrawer`.
- `apps/web/src/workspace/CreateProjectDrawer.tsx` — **Create.**
- `apps/web/src/shell/growth/*`, `settings/SettingsView.tsx`, `auth/AuthScreen.tsx`, `growth/ToolkitCards.tsx` — **Modify:** restyle to tokens; settings gains the accent picker; 图鉴 promoted to a top-level nav tab.
- `apps/web/test/shell/**`, `apps/web/test/ui/cover.test.ts` — **Create/Modify** tests.

---

### Task 1: Backend — persist accent to `users.avatar_color`

**Files:**
- Modify: `apps/api/internal/store/queries/users.sql`
- Regenerate: `apps/api/internal/store/sqlc/users.sql.go`
- Create: `apps/api/internal/api/users_accent.go`, `apps/api/internal/api/users_accent_test.go`
- Modify: `apps/api/internal/api/api.go`

**Interfaces:**
- Produces: `PUT /api/v1/users/me/accent` body `{"accent":"<presetId>"}` → 200 `{"accent":"<presetId>"}`. Rejects unknown ids with 400. Writes the id string into `users.avatar_color` for the authenticated user.

- [ ] **Step 1: Add the sqlc query** to `apps/api/internal/store/queries/users.sql`:

```sql
-- name: SetUserAvatarColor :exec
UPDATE users SET avatar_color = $1 WHERE id = $2;
```

- [ ] **Step 2: Regenerate sqlc** — from `apps/api`: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate` with `CGO_ENABLED=0` (matches the repo's pinned version; a different version reformats unrelated structs). Confirm `SetUserAvatarColor` + `SetUserAvatarColorParams{AvatarColor string; ID uuid}` appear in `users.sql.go` and nothing else changed.
- [ ] **Step 3: Write the failing handler test** `users_accent_test.go` — using the existing api test harness (see `cards_theme.go`'s test for the pattern): a seeded student PUTs `{"accent":"teal"}` → 200 + body echoes; then `getMe` returns `avatar_color == "teal"`; an unknown id `{"accent":"chartreuse"}` → 400; unauthenticated → 401.
- [ ] **Step 4: Run → FAIL** (`cd apps/api && go test ./internal/api/ -run Accent`).
- [ ] **Step 5: Implement `users_accent.go`:**

```go
package api

import "net/http"

// The 8 accent preset ids the web design system ships (ui/accent.tsx). Kept as
// a server-side allowlist so avatar_color can only hold a known preset.
var accentPresetIDs = map[string]bool{
	"vermilion": true, "clay": true, "tangerine": true, "bamboo": true,
	"teal": true, "indigo": true, "violet": true, "rose": true,
}

type setAccentReq struct{ Accent string `json:"accent"` }

func (a *API) putUserAccent(w http.ResponseWriter, r *http.Request) {
	uid, ok := userID(r) // use the same auth-context helper cards_theme.go uses
	if !ok { httpError(w, http.StatusUnauthorized, "unauthorized"); return }
	var req setAccentReq
	if err := decodeJSON(r, &req); err != nil || !accentPresetIDs[req.Accent] {
		httpError(w, http.StatusBadRequest, "invalid accent"); return
	}
	if err := a.d.Queries.SetUserAvatarColor(r.Context(), sqlc.SetUserAvatarColorParams{
		AvatarColor: req.Accent, ID: uid,
	}); err != nil { httpError(w, http.StatusInternalServerError, "failed"); return }
	writeJSON(w, http.StatusOK, map[string]string{"accent": req.Accent})
}
```
(Use the EXACT helper names this codebase uses — mirror `cards_theme.go` for `userID`/decode/error/write helpers and the `sqlc` import path. Adjust `AvatarColor`/`ID` field names to the regenerated struct.)

- [ ] **Step 6: Route it** in `api.go` next to the cards theme route: `mux.Handle("PUT /api/v1/users/me/accent", protected(a.putUserAccent))`.
- [ ] **Step 7: Run → PASS**; `go build ./...`.
- [ ] **Step 8: Commit** — `git add apps/api/internal/store/queries/users.sql apps/api/internal/store/sqlc/users.sql.go apps/api/internal/api/users_accent.go apps/api/internal/api/users_accent_test.go apps/api/internal/api/api.go && git commit -m "feat(api): persist accent preset to users.avatar_color"`

---

### Task 2: `coverGradient` util

**Files:**
- Create: `apps/web/src/ui/cover.ts`, `apps/web/test/ui/cover.test.ts`
- Modify: `apps/web/src/ui/index.ts` (export)

**Interfaces:**
- Produces: `coverGradient(seed: string): { background: string; macaron: MacaronName }` — deterministic: hash the seed → pick one of the 7 macarons → return a `linear-gradient(135deg, <base>, <a lighter/darker sibling>)` CSS string using that macaron's tokens. Same seed → same gradient. Also `coverGradientStyle(seed): React.CSSProperties`.

- [ ] **Step 1: Failing test** — `coverGradient("中国是否让地球更可持续")` is deterministic (equal across calls); different seeds usually differ; the chosen macaron is one of the 7 names; the background is a `linear-gradient(...)` string.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — small string hash (e.g. FNV-ish sum of char codes) → `MACARONS` key by index; gradient from `MACARONS[name].base` → a mix toward `MACARONS[name].bg`. Import `MACARONS` from `./tokens`.
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): coverGradient util (deterministic macaron covers)"`

---

### Task 3: `AccentProvider` ↔ server wiring

**Files:**
- Modify: `apps/web/src/ui/accent.tsx`, `apps/web/src/api/auth.ts`
- Test: `apps/web/test/ui/accent.test.tsx` (extend)

**Interfaces:**
- `api/auth.ts`: add `setAccent(accent: AccentId): Promise<void>` → `PUT /api/v1/users/me/accent`.
- `accent.tsx`: `AccentProvider({ initialAccent?, onPersist?, children })`. On mount: if `initialAccent` is a valid preset id use it (else fall back to `localStorage` then `vermilion`), apply to `document.documentElement`. `setAccent` applies + writes localStorage AND calls `onPersist?.(id)` (fire-and-forget; ignore network error, keep the local change). Keep the existing localStorage behavior intact for the `?ds` gallery (which passes no `initialAccent`/`onPersist`).

- [ ] **Step 1: Failing test** — `AccentProvider initialAccent="indigo"` applies indigo on mount (ignores localStorage); `setAccent("rose")` calls the provided `onPersist` with `"rose"` and still persists to localStorage; with no `initialAccent`, behavior is the old localStorage-first path (existing tests still pass).
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — thread `initialAccent`/`onPersist` through; guard `initialAccent` against the preset list. Add `setAccent` to `api/auth.ts` + the `ApiClient` interface in `api/index.ts`.
- [ ] **Step 4: Run → PASS; typecheck.**
- [ ] **Step 5: Commit** — `git commit -m "feat(ds): AccentProvider server-persist seam (initialAccent + onPersist)"`

---

### Task 4: Nav rail + shell wiring (new default landing = 首页)

**Files:**
- Create: `apps/web/src/shell/Nav.tsx`
- Modify: `apps/web/src/shell/StudentApp.tsx`
- Delete/retire: `apps/web/src/shell/LeftRail.tsx` (remove after Nav replaces it)
- Test: `apps/web/test/shell/Nav.test.tsx`, update `StudentApp` tests if any.

**Interfaces:**
- `Nav({ tab, onTab, user })` — 52px vertical icon rail: **首页 (home) / 项目 (projects) / 图鉴 (gallery) / 我 (me)**. Lucide icons (`Home`, `FolderKanban`/`LayoutGrid`, `Sparkles`/`LibraryBig`, user avatar). Active = `bg-mk-accent-50` tile + accent icon; inactive = muted. Bottom: a small Pebble or the user's initial as the 我 entry. Brand mark at top (small Pebble in accent).
- `StudentApp`: `type Tab = "home" | "projects" | "gallery" | "me"` plus the internal course/growth deep-link surfaces. Default `useState<Tab>("home")`. Wrap the whole shell in `<AccentProvider initialAccent={user?.avatar_color as AccentId} onPersist={api.setAccent}>`. Route bodies:
  - `home` → `HomePage` (Task 5) with callbacks: open project, open course, create project, go projects, go gallery.
  - `projects` → the projects page (Task 6, the restyled `Directory`/`WorkspaceContainer` entry). Opening a project still swaps into the studio four-room shell (unchanged internally).
  - `gallery` → `ToolkitCards` (Task 7), promoted to top level.
  - `me` → a small hub: 成长报告 (Task 8) + 设置 (Task 9) as a `Segmented` or two sections. (Keep growth's `initialScopeId` deep-link + course deep-link working.)
  - Course play + evaluation report remain reachable (course opened from home/gallery; growth report from a finished project).

Preserve the `onFinished(projectId)` → open growth report flow, and `courseFocus`/`growthFocus` deep-links.

- [ ] **Step 1: Failing test** — `Nav` renders 4 entries, active tab has the accent tile, clicking an entry calls `onTab(key)`. `StudentApp` mounts on `home` by default and renders `HomePage`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** `Nav` + rewire `StudentApp`. Consolidate growth+settings under `me`. Remove the old `LeftRail` import; delete the file once nothing references it. The dead `chat` tab/`ChatContainer` branch is dropped from the shell (leave the component + backend; just remove the nav-less branch).
- [ ] **Step 4: Run → PASS (full web suite); typecheck; build.**
- [ ] **Step 5: Commit** — `git commit -m "feat(shell): 52px icon nav + home default landing + me hub"`

---

### Task 5: HomePage (greeting + recent projects + course recs)

**Files:**
- Create: `apps/web/src/shell/home/HomePage.tsx` (+ small card subcomponents inline or in the same folder)
- Test: `apps/web/test/shell/HomePage.test.tsx`

**Interfaces:**
- `HomePage({ user, onOpenProject, onOpenCourse, onCreateProject, onGoProjects, onGoGallery })`.
- Fetches `api.listProjects()` and `api.listCourses()` (+ optionally per-course progress, but keep it lean — progress is nice-to-have, load lazily or omit on home). Shows `Skeleton*` while loading.
- **Greeting:** `Display`/`H1` "你好，{display_name} 👋" + a warm one-liner (lively voice §19, no jargon). Time-of-day optional.
- **最近项目 row:** horizontal snapshot; **first tile = 新建** (dashed hairline `＋` tile → `onCreateProject`), then recent projects (first ~6 from `listProjects`, already recency-ordered) as square gradient-cover cards (`coverGradient(project.title)`, title, status `Badge`, `qualLabel`). Clicking a project → `onOpenProject(id)`. A "查看全部 →" affordance → `onGoProjects`. If zero projects: `EmptyState` (illustration `emptyProjects`, title "快来创建你的第一个写作项目吧！", action → create).
- **推荐课程 row:** the course list as gradient-cover cards (`coverGradient(course.slug)`, title, `blurb`, `time_label`, `branch` badge, `step_count`). Click → `onOpenCourse(slug)`. Heading copy e.g. "好的写作，建立在阅读之上哦 · 推荐课程".
- Use `ui/` cards + `Illustration`/`EmptyState`; warm paper page bg; max content width ~1200.

- [ ] **Step 1: Failing test** — with a mocked `api` returning 2 projects + 2 courses: HomePage renders the greeting with `display_name`, a 新建 tile that fires `onCreateProject`, project cards that fire `onOpenProject(id)`, course cards that fire `onOpenCourse(slug)`; with 0 projects it renders the EmptyState create action. (Mock `api` module.)
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Handle loading (skeletons) + error (soft inline message, not a crash).
- [ ] **Step 4: Run → PASS; typecheck; build.**
- [ ] **Step 5: Commit** — `git commit -m "feat(shell): homepage — greeting + recent projects + course recs"`

---

### Task 6: Projects page — gradient-cover card grid + create drawer

**Files:**
- Modify: `apps/web/src/workspace/Directory.tsx`
- Create: `apps/web/src/workspace/CreateProjectDrawer.tsx`
- Test: `apps/web/test/workspace/Directory.test.tsx` (update), `CreateProjectDrawer.test.tsx`

**Interfaces:**
- `Directory` becomes a **square-card grid** (spec §10/§11): `minmax(~200px,1fr)` auto-fill; each project = `Card` (shadow-md, radius 10) with an 84px gradient cover (`coverGradient(title)`), a status `Badge` overlay (top-left), a hover-only `Menu` (⋯: 重命名/归档/删除 — wire only what exists today; hide unimplemented actions), title (16/600, 2-line clamp), meta (`qualLabel` · status · station), and the phase `Stepper`/mini-progress. **First tile = 新建** (dashed hairline `＋` tile) → opens `CreateProjectDrawer`.
- `CreateProjectDrawer({ open, onClose, onCreated })` — right `Drawer` wrapping the EXISTING create fields (project-type selector, writing-language selector, prompt `Textarea`) using `ui/` form controls; calls `api.createProject(...)`; on success `onCreated(id)` (opens the project). Preserve `titleFromPrompt` behavior.
- Keep the `evaluating`-status 15s polling and the "查看评估报告" action on done projects (→ growth deep-link) working.

- [ ] **Step 1: Failing test** — Directory renders a grid of project cards with covers + status badges + a 新建 tile; clicking 新建 opens the drawer; the drawer's create submits via a mocked `api.createProject` and fires `onCreated`. Existing open-project behavior preserved.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** Reuse `coverGradient`, `Card`, `Badge`, `Menu`, `Drawer`, form controls. Do NOT change `WorkspaceContainer`'s open→four-room internals.
- [ ] **Step 4: Run → PASS; typecheck; build.**
- [ ] **Step 5: Commit** — `git commit -m "feat(shell): projects page — gradient-cover cards + create drawer"`

---

### Task 7: 图鉴 top-level + restyle

**Files:**
- Modify: `apps/web/src/shell/growth/ToolkitCards.tsx` (restyle; it's now a top-level `gallery` tab, not nested in growth)
- Test: update as needed.

**Interfaces:** Keep the existing catalog data flow (`getCardsCatalog`, `setCardTheme`, proficiency stars, encountered greyscale, jump-to-course). Restyle to `ui/` tokens/cards/badges. The card-cover **colorway** picker (`CoverTheme`, 4 options) stays as-is (it's a distinct feature from the UI accent — leave it). Remove the growth-internal tab coupling now that it's top-level.

- [ ] **Step 1:** If a test exists, update it; else add a light render test (catalog mocked → cards render, encountered vs not, star rating shows).
- [ ] **Step 2–4:** Restyle, run tests/typecheck/build green.
- [ ] **Step 5: Commit** — `git commit -m "feat(shell): 图鉴 top-level + token restyle"`

---

### Task 8: 成长报告 restyle

**Files:**
- Modify: `apps/web/src/shell/growth/GrowthReport.tsx`, `LearningRecord`/`AbilityModel` as touched.
- Test: update/add light render tests.

**Interfaces:** Preserve data flow (`getGrowthHistory`, `initialScopeId` expand, `onOpenCourse`). Restyle container + rows + the expanded `DualAxisReport` to `ui/` tokens (Surfaces, Badges, Tabs/Segmented for the internal 学习记录/工具卡 split — but note 工具卡 moved to top-level gallery in Task 7, so growth now shows just 学习记录, optionally re-add 能力素养 later). Keep it token-clean; do not rewrite the DualAxisReport chart internals beyond color tokens.

- [ ] **Steps:** restyle → tests/typecheck/build green → commit `git commit -m "feat(shell): 成长报告 token restyle"`.

---

### Task 9: 设置 restyle + accent picker (server-persisted)

**Files:**
- Modify: `apps/web/src/shell/settings/SettingsView.tsx`
- Test: `apps/web/test/shell/SettingsView.test.tsx`

**Interfaces:**
- Restyle to `ui/` tokens. **Identity section shows REAL data:** `user.display_name`, `user.email`, `user.school.name`, and `user.classes[].name` (drop the hardcoded "IB DP1 · A 班 · TOK").
- **Accent picker** replaces the old "AI 形象" 4-hex swatches: render the 8 `ACCENT_PRESETS` as swatches (each showing a small `Pebble` in that accent), current = `useAccent().id`. Clicking → `useAccent().setAccent(id)` (which applies + localStorage + `onPersist`→server via Task 3). Show a live `Pebble` preview retinting.
- Keep 退出登录 (`onLogout`). The three cosmetic toggles: keep as local `Toggle`s (or drop — reviewer's call; they're non-persisted).

- [ ] **Step 1: Failing test** — SettingsView renders display_name/email/school/class from a mock user; renders 8 accent swatches; clicking one calls `useAccent().setAccent` (wrap in `AccentProvider` with a spy `onPersist`, assert it fires with the id).
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run → PASS; typecheck; build.**
- [ ] **Step 5: Commit** — `git commit -m "feat(shell): settings restyle + server-persisted accent picker"`

---

### Task 10: Auth restyle

**Files:**
- Modify: `apps/web/src/shell/auth/AuthScreen.tsx`
- Test: update as needed.

**Interfaces:** Keep the 3-step machine (login/register/bind + join-code) and API calls unchanged. Restyle to `ui/` tokens: warm paper split layout, an `Illustration` (e.g. `bookLover`/`standing20`) + a small brand `Pebble` on one side, the form (`Input`/`Button`) on the other; error states via the form `error` prop; primary `Button` for submit. Lively welcome copy (§19).

- [ ] **Steps:** restyle → login/register/bind still submit (test the happy path with mocked client) → tests/typecheck/build green → commit `git commit -m "feat(shell): auth screen restyle"`.

---

### Task 11: Integration wire-up + accent boot

**Files:**
- Modify: `apps/web/src/shell/AppShell.tsx` (ensure `MeUser.avatar_color` flows into `StudentApp`→`AccentProvider`; the boot placeholder bg uses `--mk-paper` not the old `#F3F4F8`)
- Modify: `apps/web/src/ui/index.ts` if new exports; any barrel/import cleanup.
- Test: a shell smoke test — booting as a student lands on `home` with the user's accent applied.

**Interfaces:** Confirm end-to-end: `getMe` → `avatar_color` (a preset id, or empty → default vermilion) → `AccentProvider initialAccent`. `setAccent` in settings persists to server + updates the live UI. Replace any remaining old-palette inline colors in the boot/loading states (`AppShell` placeholder, `AuthScreen` fallback) with tokens.

- [ ] **Step 1: Failing test** — a student boot (mock `getMe` returning `avatar_color:"teal"`) renders the shell on `home` with `--mk-accent-500` = teal on the root. (Or assert AccentProvider received it.)
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement + sweep** any stray old-palette hex in the shell boot path.
- [ ] **Step 4: Run FULL web suite + typecheck + build green; `go build ./...` green.**
- [ ] **Step 5: Commit** — `git commit -m "feat(shell): accent boot wire-up + palette sweep"`

---

## Self-Review Notes

- **Spec coverage:** subproject-2 (首页 greeting + recent projects snapshot + course recs → T5; 项目 square cover cards → T6; nav → T4; 图鉴 → T7; growth/settings/auth restyle → T8/T9/T10). Accent model persistence (spec §1.1 "复用/泛化 card_theme" → refined to `avatar_color` since card_theme is a 4-value cover enum; documented). Covers = macaron gradient (spec §10). Voice §19 in home/empty/auth copy.
- **Decisions locked with user:** covers = macaron gradients; accent = server-persisted via `avatar_color`.
- **Deferred (not gaps):** real cover-image upload; course recommendation ranking (recs = the flat list for now); undraw recolor (Part 4 / open item); full copy sweep (Part 4); studio internals (Part 3); the `me` hub could later split growth/settings into distinct nav entries.
- **Type consistency:** `AccentId` (from `ui/accent`) used in T1 allowlist (string ids must match), T3, T9, T11. `MacaronName` in T2. Tab union `home|projects|gallery|me` in T4/T5/T11.
- **Risk:** T1 sqlc regen can reformat structs — pin v1.27.0 and diff carefully (Part-1/prior gotcha). T4 rewrites the shell entry — run the FULL web suite, not a subset.
