# Course Authoring & Publish Lifecycle — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the course generator create/modify `CourseDefinition 2.0` courses over the API + upload their media, land them as an admin-only `preview` draft, and ship (publish) them with a script that generates narration TTS, sets the cover, and flips status.

**Architecture:** Two new columns on `course` (`status`, `cover`); three OSS_ADMIN_KEY-gated write endpoints (definition upsert, media asset-upload-url, ship); a `role='admin'` preview-visibility gate on the read/serve endpoints; a `.deploy-local` ship wrapper. No new tables; reuses existing OSS/Voice/store/validation helpers.

**Tech Stack:** Go (`net/http`, sqlc/goose/pgx), Postgres, React (cover card), the `@mind-imprint/course-contract` shapes on the wire.

**Design doc:** `docs/superpowers/specs/2026-08-16-course-authoring-publish-lifecycle-design.md`.

## Global Constraints

- **Go border-validates only** — never re-implement the TS Zod/referential/workflow validation. For a definition write, check `schemaVersion == "2.0"`, `course.id` non-empty and `== {slug}`, valid JSON, `cardIds` resolve. Nothing deeper.
- **Two gates, both existing:** generator writes use the `OSS_ADMIN_KEY` bearer (`a.ossAdminAuthed`); preview visibility uses the session user's `role == "admin"`.
- **Status semantics:** column default `'published'` (existing/seeded rows stay live). The definition-upsert endpoint sets `'preview'` **only on insert**; on update, status is **preserved** (never silently (un)published). Ship is the only `preview → published` transition.
- **Deterministic key convention:** a course's asset `X` lives at OSS key `courses/<slug>/X`. Media upload and ship's audio upload both honor it; it matches what `/asset-urls` signs for playback.
- **Secrets server-side only** (`OSS_ADMIN_KEY`, OSS/Voice creds) — never logged/echoed.
- **sqlc:** regenerate with the pinned `sqlc@v1.27.0`, `CGO_ENABLED=0` on macOS. Run the FULL Go package builds/tests, not `-run` subsets. Go tests are testcontainers-backed — run FOREGROUND with `-timeout 1800s`; the pure-helper tests are `-short`-safe.

---

### Task 1: Schema + queries foundation (migration 0072 + sqlc)

**Files:**
- Create: `apps/api/internal/store/migrations/0072_course_status_cover.sql`
- Modify: `apps/api/internal/store/queries/course.sql`
- Regenerate: `apps/api/internal/store/sqlc/*` (via `sqlc generate`)

**Interfaces produced (later tasks consume):**
- sqlc: `ListCourseRows` now returns `status, cover` and takes an `include_preview bool` arg; `GetCourseBySlug` returns `status, cover`; `GetCourseDefinition` returns `(course_definition, status)`; new `GetCourseStatusBySlug(slug) -> status`; new `UpsertCourseDefinition(...)` (preview-on-insert, preserves status); new `SetCourseStatusAndCover(slug, status, cover)`.

- [ ] **Step 1: Write the migration.** `0072_course_status_cover.sql`:

```sql
-- +goose Up
-- Course authoring/publish lifecycle: `status` gates catalog/play visibility
-- (rows created via the admin definition-upsert API land as 'preview'; existing
-- and seeded rows default to 'published' so nothing already live is hidden).
-- `cover` is a catalog cover id (e.g. a stock 'img:N'), set at ship time.
ALTER TABLE course
  ADD COLUMN status text NOT NULL DEFAULT 'published'
    CHECK (status IN ('preview','published')),
  ADD COLUMN cover  text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE course DROP COLUMN IF EXISTS cover;
ALTER TABLE course DROP COLUMN IF EXISTS status;
```

- [ ] **Step 2: Update `course.sql`.** Replace `ListCourseRows`, `GetCourseBySlug`, `GetCourseDefinition`, and add three new queries:

```sql
-- name: ListCourseRows :many
-- Preview courses are visible only when include_preview is true (the caller is
-- an admin). Students (false) see 'published' only.
SELECT slug, branch, title, blurb, time_label, card_ids, step_count, status, cover
FROM course
WHERE status = 'published' OR sqlc.arg(include_preview)::bool
ORDER BY branch, title;

-- name: GetCourseBySlug :one
SELECT id, slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, audio_manifest, status, cover
FROM course WHERE slug = $1;

-- name: GetCourseDefinition :one
SELECT course_definition, status FROM course WHERE slug = $1;

-- name: GetCourseStatusBySlug :one
SELECT status FROM course WHERE slug = $1;

-- name: UpsertCourseDefinition :one
-- Course authoring: create/modify a 2.0 course. status is set to 'preview' ONLY
-- on insert (EXCLUDED is not applied on conflict), so re-posting a definition
-- never (un)publishes an existing course. structure/render_cache are the empty
-- object for 2.0 courses (they use course_definition, not the legacy blobs).
INSERT INTO course (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, course_definition, status, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,0,'{}','{}',$7,'preview', now())
ON CONFLICT (slug) DO UPDATE SET
  branch = EXCLUDED.branch, title = EXCLUDED.title, blurb = EXCLUDED.blurb,
  time_label = EXCLUDED.time_label, card_ids = EXCLUDED.card_ids,
  course_definition = EXCLUDED.course_definition, updated_at = now()
RETURNING slug, status;

-- name: SetCourseStatusAndCover :exec
UPDATE course SET status = $2, cover = $3, updated_at = now() WHERE slug = $1;
```

(Leave `UpsertCourse` untouched — the legacy publish path + seed keep using it; their inserts get `status='published'` from the column default, updates preserve it.)

- [ ] **Step 3: Regenerate sqlc.**

Run (from `apps/api`): `CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`
Expected: `internal/store/sqlc/course.sql.go` + `models.go` regenerate; `Course` model gains `Status string` + `Cover string`; new query methods exist.

- [ ] **Step 4: Thread the new columns through the store rows.** In `apps/api/internal/agent/coursestore.go`:
  - Add `Status string` and `Cover string` to `CourseSummaryRow` (the `ListCourses` row type) and populate them from `ListCourseRows`.
  - `ListCourses(ctx)` currently calls `ListCourseRows(ctx)` with no arg; it now needs `include_preview`. Change the store method to `ListCourses(ctx context.Context, includePreview bool)` and pass it through.
  - Add `Status string` to the `GetCoursePayload` return payload struct (so handlers can gate). If `GetCoursePayload` uses `GetCourseBySlug`, map the new `Status`/`Cover`.
  - Add a thin `CourseStatus(ctx, slug) (string, error)` store method over `GetCourseStatusBySlug` (used by the endpoints that don't load the full payload).

- [ ] **Step 5: Fix compile fallout + run store tests.** `ListCourses` now takes `includePreview` — the one existing caller (`listCourses` handler, Task 2 will finalize it) must pass a value; for this task pass `true` temporarily OR let Task 2 own it (mark the handler with `false` to keep students' current behavior). Build the module.

Run: `cd apps/api && go build ./... && go test ./internal/store/ ./internal/agent/ -run Course -count=1 -timeout 1800s`
Expected: PASS (migration applies; seeded rows read back `status='published'`, `cover=''`).

- [ ] **Step 6: Commit.**

```bash
git add apps/api/internal/store/migrations/0072_course_status_cover.sql apps/api/internal/store/queries/course.sql apps/api/internal/store/sqlc apps/api/internal/agent/coursestore.go
git commit -m "feat(course): status+cover columns, preview-aware queries"
```

---

### Task 2: Preview-visibility gate on the read/serve endpoints

**Files:**
- Modify: `apps/api/internal/api/course.go` (`listCourses`, `getCourse`)
- Modify: `apps/api/internal/api/course_definition.go` (`getCourseDefinition`)
- Modify: `apps/api/internal/api/course_session.go` (`postCourseSession`)
- Modify: `apps/api/internal/api/course_asset_urls.go` (`postCourseAssetURLs`)
- Modify: `apps/api/internal/api/course_scene.go` (`sceneForCourse`)
- Create: `apps/api/internal/api/course_visibility.go` (the shared helper)
- Test: `apps/api/internal/api/course_visibility_test.go`

**Interfaces:**
- Consumes: `UserFromContext`, `store.CourseStatus`, `store.ListCourses(ctx, includePreview)`.
- Produces: `func isAdmin(ctx context.Context) bool`; `func (a *API) requireVisibleCourse(w, r, slug) bool` — writes 404 + returns false when the course is `preview` and the caller is not admin.

- [ ] **Step 1: Write the failing test.** `course_visibility_test.go` — table test of `isAdmin` (admin user in ctx → true; student → false; no user → false). (Pure; construct a context with a `User` via the same helper the auth tests use — mirror an existing `*_test.go` that builds an authed context.)

- [ ] **Step 2: Implement `course_visibility.go`:**

```go
package api

import (
	"context"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// isAdmin reports whether the session user in ctx is an admin. Preview courses
// are visible/playable only to admins; students see published only.
func isAdmin(ctx context.Context) bool {
	u, ok := UserFromContext(ctx)
	return ok && u.Role == "admin"
}

// requireVisibleCourse enforces the preview gate for a by-slug course read: if
// the course is 'preview' and the caller is not an admin it writes a 404 (a
// draft is indistinguishable from a nonexistent course, so it never leaks) and
// returns false. A published course, an admin caller, or an unknown slug (left
// to the downstream handler's own 404) returns true.
func (a *API) requireVisibleCourse(w http.ResponseWriter, r *http.Request, slug string) bool {
	if isAdmin(r.Context()) {
		return true
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	status, err := store.CourseStatus(r.Context(), slug)
	if err != nil {
		return true // unknown slug / db error → let the downstream handler map it (404 etc.)
	}
	if status == "preview" {
		httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
		return false
	}
	return true
}
```

- [ ] **Step 3: Wire the gate.**
  - `listCourses`: `rows, err := store.ListCourses(r.Context(), isAdmin(r.Context()))`.
  - `getCourse`, `getCourseDefinition`, `postCourseSession`, `postCourseAssetURLs`, `sceneForCourse`: at the top (after reading `slug := r.PathValue("slug")`), add `if !a.requireVisibleCourse(w, r, slug) { return }`.

- [ ] **Step 4: Run tests.**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run 'Visibility|Course' -count=1 -timeout 1800s`
Expected: PASS; add/confirm a test that a `preview` course 404s for a student and 200s for an admin on `getCourseDefinition` (mirror `course_definition_test.go`'s setup, then flip status via `SetCourseStatusAndCover`).

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/api/course_visibility.go apps/api/internal/api/course_visibility_test.go apps/api/internal/api/course.go apps/api/internal/api/course_definition.go apps/api/internal/api/course_session.go apps/api/internal/api/course_asset_urls.go apps/api/internal/api/course_scene.go
git commit -m "feat(api): preview-visibility gate (admins see draft courses)"
```

---

### Task 3: `PUT /admin/courses/{slug}/definition` — create/modify

**Files:**
- Create: `apps/api/internal/api/course_definition_admin.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/course_definition_admin_test.go`

**Interfaces:**
- Consumes: `a.ossAdminAuthed`, `decodeJSON`, `cards.ByID`, `agent.NewSqlcAgentStore`, the new `store.UpsertCourseDefinition`.
- Produces: `PUT /api/v1/admin/courses/{slug}/definition` → `{ slug, status }`.

- [ ] **Step 1: Write the failing test** (`course_definition_admin_test.go`): mirror `course_definition_test.go` + `postAdminUploadCourse` test harness. Cases: valid doc → 200, row created with `status='preview'`, definition readable; missing/blank admin key → 401; `schemaVersion != "2.0"` → 422; `course.id != {slug}` → 400; unknown `cardId` → 400; re-PUT of a published course preserves `status='published'` (create → `SetCourseStatusAndCover(...,'published',...)` → PUT again → still published).

- [ ] **Step 2: Implement the handler:**

```go
package api

// course_definition_admin.go — the course generator's create/modify endpoint.
// PUT /api/v1/admin/courses/{slug}/definition (OSS_ADMIN_KEY bearer) upserts a
// CourseDefinition 2.0 course. Border-validation only: schemaVersion "2.0",
// course.id present and == {slug}, cardIds resolve. Deep validation is the
// generator's job (the TS course-contract). New courses land as 'preview'.

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
)

type putCourseDefinitionReq struct {
	Definition json.RawMessage `json:"definition"` // the whole { schemaVersion, course } document
	Blurb      string          `json:"blurb"`
	CardIDs    []string        `json:"cardIds"`
}

// putCourseDefinitionDoc is the border slice of the document.
type putCourseDefinitionDoc struct {
	SchemaVersion string `json:"schemaVersion"`
	Course        struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		EstimatedMinutes int    `json:"estimatedMinutes"`
	} `json:"course"`
}

func (a *API) putCourseDefinition(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	slug := r.PathValue("slug")
	var body putCourseDefinitionReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var doc putCourseDefinitionDoc
	if err := json.Unmarshal(body.Definition, &doc); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "definition 不是合法 JSON", nil))
		return
	}
	if doc.SchemaVersion != "2.0" {
		httpx.WriteError(w, r, &httpx.APIError{Status: http.StatusUnprocessableEntity, Code: "invalid_course_definition", Message: "schemaVersion 必须为 2.0"})
		return
	}
	if doc.Course.ID == "" || doc.Course.Title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "course.id / course.title 不能为空", nil))
		return
	}
	if doc.Course.ID != slug {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "course.id 必须与 slug 一致", nil))
		return
	}
	for _, id := range body.CardIDs {
		if _, ok := cards.ByID(id); !ok {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "未知的工具卡: "+id, nil))
			return
		}
	}
	cardIDs := body.CardIDs
	if cardIDs == nil {
		cardIDs = []string{}
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	status, err := store.UpsertCourseDefinition(r.Context(), agent.UpsertCourseDefinitionInput{
		Slug:       slug,
		Branch:     "Runtime",
		Title:      doc.Course.Title,
		Blurb:      body.Blurb,
		TimeLabel:  minutesLabel(doc.Course.EstimatedMinutes),
		CardIDs:    cardIDs,
		Definition: body.Definition,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"slug": slug, "status": status})
}

// minutesLabel formats the catalog time label, mirroring seedGoldenCourse.
func minutesLabel(m int) string {
	if m <= 0 {
		return ""
	}
	return "约 " + itoaMinutes(m) + " 分钟"
}
```

Add to `coursestore.go`: `UpsertCourseDefinitionInput{ Slug, Branch, Title, Blurb, TimeLabel string; CardIDs []string; Definition []byte }` and a `func (s *sqlcAgentStore) UpsertCourseDefinition(ctx, in) (string, error)` calling the new sqlc `UpsertCourseDefinition` (returns the row's `status`). For `itoaMinutes`, use `strconv.Itoa` (or reuse the existing minutes formatting in `seed_courses.go`'s `fmt.Sprintf("约 %d 分钟", …)` — prefer `fmt.Sprintf`, drop the helper).

- [ ] **Step 3: Register the route** in `api.go` near the other admin course route (after line 220):

```go
	mux.Handle("PUT /api/v1/admin/courses/{slug}/definition", http.HandlerFunc(a.putCourseDefinition)) // course generator create/modify
```

- [ ] **Step 4: Run tests.** `cd apps/api && go build ./... && go test ./internal/api/ -run CourseDefinitionAdmin -count=1 -timeout 1800s` → PASS.

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/api/course_definition_admin.go apps/api/internal/api/course_definition_admin_test.go apps/api/internal/api/api.go apps/api/internal/agent/coursestore.go
git commit -m "feat(api): PUT /admin/courses/{slug}/definition (create/modify, lands preview)"
```

---

### Task 4: `POST /admin/courses/{slug}/asset-upload-url` — media upload

**Files:**
- Create: `apps/api/internal/api/course_asset_upload.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/course_asset_upload_test.go`

**Interfaces:**
- Consumes: `a.ossAdminAuthed`, `decodeJSON`, `ossScopes["course_material"]` (allowed types + 500 MB cap), `validRelativeAssetPath` (Task 2's file / existing in `course_asset_urls.go`), `a.d.OSS.SignUpload`.
- Produces: `POST /api/v1/admin/courses/{slug}/asset-upload-url` → `{ putUrl, objectKey, requiredContentType, maxBytes, expiresAt }`.

- [ ] **Step 1: Write the failing test** (`course_asset_upload_test.go`): valid `{relativePath, contentType:"video/mp4", size}` → 200, `objectKey == "courses/<slug>/<relativePath>"`, `putUrl` non-empty; `..`/leading-slash/scheme relativePath → 400; slug with `..` → 400; disallowed contentType → 400; size over 500 MB → 400; nil OSS → 503. Build the API with `oss.NewSigner(...)`? — no: `SignUpload` needs the real presign path, so construct the OSS `Service` via `oss.New(cfg)` in the test only if creds are present; otherwise assert the validation branches (400s) with a `NewSigner`-built service (which has a nil origin bucket — so guard: the validation 400s all fire BEFORE `SignUpload`, so they're testable without a real origin). For the 200 path, gate the test on OSS creds being configured (skip like the OSS live tests do) OR assert only that validation passes and the objectKey is built correctly by extracting the key-construction into a pure helper `courseAssetUploadKey(slug, rel)` and unit-testing that.

- [ ] **Step 2: Implement the handler:**

```go
package api

// course_asset_upload.go — the course generator's media upload endpoint.
// POST /api/v1/admin/courses/{slug}/asset-upload-url (OSS_ADMIN_KEY bearer)
// signs a presigned PUT to the DETERMINISTIC key courses/<slug>/<relativePath>
// — the exact path the CourseDefinition references and /asset-urls signs for
// playback. Used for video/pdf/images/interactiveHtml (narration audio is
// generated at ship, not uploaded).

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
)

type courseAssetUploadReq struct {
	RelativePath string `json:"relativePath"`
	ContentType  string `json:"contentType"`
	Size         int64  `json:"size"`
}

func (a *API) postCourseAssetUploadURL(w http.ResponseWriter, r *http.Request) {
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	slug := r.PathValue("slug")
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, "/") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_slug", "无效的课程标识。", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, ossPresignBodyLimit)
	var req courseAssetUploadReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !validRelativeAssetPath(req.RelativePath) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_asset_path", "无效的资源路径。", nil))
		return
	}
	sc := ossScopes["course_material"]
	if !sc.allowedTypes[req.ContentType] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unsupported_type", "不支持的文件类型。", nil))
		return
	}
	if req.Size <= 0 || req.Size > sc.maxBytes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("file_too_large", "文件超出大小限制。", nil))
		return
	}
	objectKey := courseAssetUploadKey(slug, req.RelativePath)
	putURL, err := a.d.OSS.SignUpload(objectKey, req.ContentType, ossUploadTTL)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ossUploadResp{
		PutURL:              putURL,
		ObjectKey:           objectKey,
		RequiredContentType: req.ContentType,
		MaxBytes:            sc.maxBytes,
		ExpiresAt:           time.Now().Add(ossUploadTTL).UTC().Format(time.RFC3339),
	})
}

func courseAssetUploadKey(slug, relativePath string) string {
	return "courses/" + slug + "/" + relativePath
}
```

- [ ] **Step 3: Register the route** in `api.go`:

```go
	mux.Handle("POST /api/v1/admin/courses/{slug}/asset-upload-url", http.HandlerFunc(a.postCourseAssetUploadURL)) // course generator media upload
```

- [ ] **Step 4: Run tests.** `cd apps/api && go build ./... && go test ./internal/api/ -run CourseAssetUpload -count=1 -timeout 1800s` → PASS.

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/api/course_asset_upload.go apps/api/internal/api/course_asset_upload_test.go apps/api/internal/api/api.go
git commit -m "feat(api): POST /admin/courses/{slug}/asset-upload-url (deterministic course path)"
```

---

### Task 5: Ship — definition audio-gen + endpoint + wrapper script

**Files:**
- Create: `apps/api/internal/agent/course_definition_audio.go`
- Create: `apps/api/internal/api/course_ship.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Create: `deploy/ship-course.sh` (committed; holds no secret) + a note to add `.deploy-local/ship-course.sh` wrapper
- Test: `apps/api/internal/agent/course_definition_audio_test.go`, `apps/api/internal/api/course_ship_test.go`

**Interfaces:**
- Consumes: `agent.CourseAudioSynth`/`CourseAudioStore` (existing, from `course_audio.go`), the stored definition (`store.GetCourseDefinition`... returns `(def, status)` now), `store.SetCourseStatusAndCover`.
- Produces: `agent.GenerateDefinitionAudio(ctx, synth, store, slug string, definition []byte) (int, error)` (returns count generated); `POST /api/v1/admin/courses/{slug}/ship`.

- [ ] **Step 1: Write the failing audio-gen test** (`course_definition_audio_test.go`): feed a small definition with 2 narrations (+ opening.fallback.audio) and stub synth/store (reuse `stubCourseAudioSynth`/`stubCourseAudioStore` from `seed_courses_test.go` — copy them into this package's test or reference). Assert `PutObject` called once per authored audio path with key `courses/<slug>/<audio path>`; a nil synth/store degrades to `0, nil` (no error).

- [ ] **Step 2: Implement `course_definition_audio.go`:**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// definitionAudioDoc is the border slice of a CourseDefinition 2.0 needed to
// generate narration audio: every narration's text+audio path, plus the
// opening/closing fallback audio. Deep structure is irrelevant here.
type definitionAudioDoc struct {
	Course struct {
		Opening struct{ Fallback struct{ Text, Audio string } `json:"fallback"` } `json:"opening"`
		Closing struct{ Fallback struct{ Text, Audio string } `json:"fallback"` } `json:"closing"`
		Parts   []struct {
			Slices []struct {
				Narrations []struct {
					Text  string `json:"text"`
					Audio string `json:"audio"`
				} `json:"narrations"`
			} `json:"slices"`
		} `json:"parts"`
	} `json:"course"`
}

// GenerateDefinitionAudio synthesizes TTS for each narration (and opening/
// closing fallback that has an audio path) in a CourseDefinition 2.0 and uploads
// the mp3 to courses/<slug>/<authored audio path> — the exact path the document
// references. Returns the number of clips generated. synth/store nil (voice/OSS
// unconfigured) degrades to (0, nil): ship still flips status, just without
// audio. Idempotent in effect: same text -> same bytes at the same key.
func GenerateDefinitionAudio(ctx context.Context, synth CourseAudioSynth, store CourseAudioStore, slug string, definition []byte) (int, error) {
	if synth == nil || store == nil {
		return 0, nil
	}
	var doc definitionAudioDoc
	if err := json.Unmarshal(definition, &doc); err != nil {
		return 0, fmt.Errorf("definition audio: parse: %w", err)
	}
	type clip struct{ text, audio string }
	var clips []clip
	add := func(text, audio string) {
		if text != "" && audio != "" {
			clips = append(clips, clip{text, audio})
		}
	}
	add(doc.Course.Opening.Fallback.Text, doc.Course.Opening.Fallback.Audio)
	add(doc.Course.Closing.Fallback.Text, doc.Course.Closing.Fallback.Audio)
	for _, p := range doc.Course.Parts {
		for _, s := range p.Slices {
			for _, n := range s.Narrations {
				add(n.Text, n.Audio)
			}
		}
	}
	n := 0
	for _, c := range clips {
		key := "courses/" + slug + "/" + c.audio
		audio, err := synth.Synthesize(ctx, c.text, 1.0)
		if err != nil {
			return n, fmt.Errorf("definition audio: synth %q: %w", c.audio, err)
		}
		if err := store.PutObject(ctx, key, "audio/mpeg", audio); err != nil {
			return n, fmt.Errorf("definition audio: put %q: %w", key, err)
		}
		n++
	}
	return n, nil
}
```

- [ ] **Step 3: Implement the ship handler** `course_ship.go`:

```go
package api

// course_ship.go — publish a preview course. POST /api/v1/admin/courses/{slug}/ship
// (OSS_ADMIN_KEY bearer): generate narration TTS from the stored definition,
// set the cover, flip status preview -> published. The one publish transition.

import (
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

type courseShipReq struct {
	Cover string `json:"cover"` // e.g. "img:3" (a stock catalog cover id)
}

func (a *API) postCourseShip(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	slug := r.PathValue("slug")
	var body courseShipReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	def, _, err := store.GetCourseDefinition(r.Context(), slug) // (definition, status, err); 404 if none
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(def) == 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("该课程没有 2.0 定义"))
		return
	}

	// Nil-guard voice/OSS the same way postAdminUploadCourse does (typed-nil-in-
	// interface): both must be explicitly nil-checked before entering the
	// interface params or a nil *oss.Service won't compare equal to nil inside.
	var synth agent.CourseAudioSynth
	if a.d.Voice != nil {
		synth = a.d.Voice
	}
	var audioStore agent.CourseAudioStore
	if a.d.OSS != nil {
		audioStore = a.d.OSS
	}
	generated, err := agent.GenerateDefinitionAudio(r.Context(), synth, audioStore, slug, def)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if err := store.SetCourseStatusAndCover(r.Context(), slug, "published", body.Cover); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"slug": slug, "status": "published", "narrationsGenerated": generated})
}
```

Add `SetCourseStatusAndCover(ctx, slug, status, cover string) error` and adjust `GetCourseDefinition(ctx, slug) ([]byte, string, error)` in `coursestore.go` to return the new `(definition, status)` from the updated query (Task 1). Update the existing `getCourseDefinition` handler (course_definition.go) to the new 2-value+err signature (it already ignores status → `def, _, err := ...`).

- [ ] **Step 4: Register route + write the wrapper script.**

`api.go`:
```go
	mux.Handle("POST /api/v1/admin/courses/{slug}/ship", http.HandlerFunc(a.postCourseShip)) // publish a preview course
```

`deploy/ship-course.sh` (committed, no secret — mirrors `deploy/remote-deploy.sh`'s "committed body, secrets injected by the local wrapper" split):
```bash
#!/usr/bin/env bash
# Publish a preview course: POST /admin/courses/<slug>/ship. Drive it from the
# local wrapper .deploy-local/ship-course.sh which injects API + OSS_ADMIN_KEY.
#   usage (on caller): API=… ADMIN_KEY=… bash ship-course.sh <slug> [cover]
set -euo pipefail
SLUG="${1:?usage: ship-course.sh <slug> [cover]}"
COVER="${2:-}"
: "${API:?set API}"; : "${ADMIN_KEY:?set ADMIN_KEY}"
curl -fsS -X POST "$API/api/v1/admin/courses/$SLUG/ship" \
  -H "Authorization: Bearer $ADMIN_KEY" -H "Content-Type: application/json" \
  -d "{\"cover\":\"$COVER\"}"
echo
```
Add (documentation step, not committed) a `.deploy-local/ship-course.sh` that sets `API=https://mind-api.uni-robot.cn` + `ADMIN_KEY=…` from the deploy env and calls the committed script — the same pattern as `.deploy-local/deploy.sh`.

- [ ] **Step 5: Run tests.** `cd apps/api && go build ./... && go test ./internal/agent/ ./internal/api/ -run 'DefinitionAudio|CourseShip' -count=1 -timeout 1800s` → PASS.

- [ ] **Step 6: Commit.**

```bash
git add apps/api/internal/agent/course_definition_audio.go apps/api/internal/agent/course_definition_audio_test.go apps/api/internal/api/course_ship.go apps/api/internal/api/course_ship_test.go apps/api/internal/api/course_definition.go apps/api/internal/api/api.go apps/api/internal/agent/coursestore.go deploy/ship-course.sh
git commit -m "feat(api): POST /admin/courses/{slug}/ship (TTS + cover + publish) + wrapper"
```

---

### Task 6: Course cover in the catalog (DTO + card)

**Files:**
- Modify: `apps/api/internal/api/<course DTO file>` (`courseSummaryDTO` + `toCourseSummaryDTO`) — find with `grep -rn "courseSummaryDTO\|toCourseSummaryDTO" apps/api/internal/api`
- Modify: the web course-catalog card component (find with `grep -rln "listCourses\|/courses\b" apps/web/src` → the catalog view/card)
- Test: extend the nearest existing DTO/catalog test

**Interfaces:**
- Consumes: `resolveCoverURL` (the existing project-cover resolver in `project_covers.go`), the `Cover` field now on `CourseSummaryRow` (Task 1).
- Produces: `coverUrl` on the course-summary JSON; the catalog card renders it.

- [ ] **Step 1: Backend — add `CoverURL` to `courseSummaryDTO`** and set it in `toCourseSummaryDTO` from the row's `Cover` via `a.resolveCoverURL(cover)`. NOTE: `resolveCoverURL` is a method on `*API`; if `toCourseSummaryDTO` is a free function, thread the resolver in (pass `a.resolveCoverURL` or make it a method) — mirror how `projects.go` builds `CoverURL: a.resolveCoverURL(cover)`. Empty `cover` → empty string (no cover). Add the field to the JSON tag `coverUrl`.

- [ ] **Step 2: Backend test** — extend the catalog/DTO test: a course with `cover="img:3"` surfaces a non-empty `coverUrl`; `cover=""` → empty.

- [ ] **Step 3: Frontend — render the cover** on the course catalog card (image when `coverUrl` present, otherwise the existing fallback visual). Keep it minimal and match the card's existing styles/tokens.

- [ ] **Step 4: Frontend test** — extend the nearest course-catalog component test to assert the cover renders when `coverUrl` is present.

- [ ] **Step 5: Run + commit.** `cd apps/api && go build ./... && go test ./internal/api/ -run Course -count=1 -timeout 1800s`; `pnpm --filter web test -- <catalog test>` + `pnpm --filter web typecheck`.

```bash
git add -p   # stage only the touched DTO + web card + tests (never git add -A)
git commit -m "feat: show course cover in the catalog"
```

---

## Final verification (after all tasks)

- [ ] `cd apps/api && go build ./... && go vet ./internal/... && go test ./internal/store/ ./internal/agent/ ./internal/api/ -count=1 -timeout 1800s` (full, testcontainers).
- [ ] `pnpm --filter web test && pnpm --filter web typecheck`.
- [ ] Manual contract sanity: a `PUT …/definition` with the golden `coverage-course.json` (as `{definition: <that doc>, blurb:"…"}`) → row created `preview`; a student's `GET /courses` omits it; an admin's includes it and `GET …/definition` 200s; `POST …/ship` with `{cover:"img:3"}` → `published`, narration audio objects exist under `courses/<slug>/…`.
- [ ] No secret in the committed `deploy/ship-course.sh` (reads `ADMIN_KEY` from env only).
