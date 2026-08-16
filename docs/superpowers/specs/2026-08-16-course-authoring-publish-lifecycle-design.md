# Course Authoring & Publish Lifecycle — Design

> **Status:** design, ready for plan.
> **Goal:** let the (out-of-repo) **course generator** create/modify `CourseDefinition 2.0` courses over the API and upload their media, land them as an offline **draft (`preview`)** that only an **admin** can preview through the real student player, and **ship (publish)** them with a single script that generates narration TTS, sets the cover, and flips the status.
> **Builds on:** [[course-runtime-2.0-build-2026-08-16]] (the renderer/contract), [[course-asset-auth-slice-2026-08-16]] (URL鉴权 asset serving), and the guide `docs/2026-08-16-course-renderer-and-contract-guide.md`.

## Problem

Today a 2.0 course can only reach prod by committing JSON to the repo + deploying (`seedGoldenCourse`). There is **no API to create/modify a course definition**, **no way to upload a course's media to its `courses/<slug>/<path>` namespace** (the admin upload assigns a random uuid key), **no draft state** (every row in `course` is live), and **no publish step**. The generator needs all four.

## Non-goals

- No admin authoring **UI** (publish is a script; preview reuses the student player).
- No preview **tokens / share links** (preview = an admin account).
- No change to the read/serve path built in the URL鉴权 slice (definition GET, `/asset-urls`, sessions) beyond the `preview`-visibility gate.
- The generator itself, and its deep (Zod) validation, stay out of repo — Go only **border-validates**.

## Data model (one migration)

Add two columns to `course` (both with safe defaults so existing rows are unaffected):

- `status text NOT NULL DEFAULT 'published' CHECK (status IN ('preview','published'))` — existing seeded/published courses stay live; **rows created via the new API default to `preview`** (set explicitly by the handler, not the column default).
- `cover text NOT NULL DEFAULT ''` — set at ship time; empty renders no cover.

No other schema changes — `course_definition` (jsonb) already exists.

## Auth model

Two distinct gates, both already present in the codebase:

- **Generator write endpoints** (`PUT …/definition`, `POST …/asset-upload-url`, `POST …/ship`) — the **`OSS_ADMIN_KEY` bearer**, exactly like `postAdminUploadCourse` / `ossAdminUploadURL`. The generator is a trusted backend tool holding that key.
- **Preview visibility** (reading/playing a `preview` course) — a **logged-in user whose `role == "admin"`**. Students only ever see `published`.

## APIs

### 1. `PUT /api/v1/admin/courses/{slug}/definition` — create / modify (admin-key)

Request:
```jsonc
{
  "definition": { "schemaVersion": "2.0", "course": { … } },  // the whole document
  "blurb": "…",                 // catalog card blurb (definition has no blurb field)
  "cardIds": ["craap", …]       // optional: tool cards this course summons (default [])
}
```
Behavior (mirrors `seedGoldenCourse`, exposed over HTTP):
- Border-validate: `definition.schemaVersion == "2.0"`, `course.id` non-empty, valid JSON, `cardIds` all resolve in the registry. Reject → 400/422. (Deep structural/referential/workflow validation is the generator's job via the TS contract — Go does not re-implement it.)
- Slug: `{slug}` must equal `definition.course.id` (400 on mismatch) — one stable identity.
- Upsert the `course` row: `title`/`estimatedMinutes→time_label` derived from the definition; `branch = "Runtime"`; `blurb`/`cardIds` from the body; `structure`/`render_cache = "{}"`, `step_count = 0` (2.0 courses don't use the legacy blobs); attach the definition via `SetCourseDefinition`.
- **On create, `status = 'preview'`. On update, status is preserved** (re-posting a definition to an already-published course does not silently unpublish it, and does not republish a draft).
- Response: `{ slug, status }`.

### 2. `POST /api/v1/admin/courses/{slug}/asset-upload-url` — media upload (admin-key)

Request: `{ "relativePath": "assets/videos/case.mp4", "contentType": "video/mp4", "size": 12345 }`
Behavior:
- Validate `relativePath` with the **same rule as `validRelativeAssetPath`** (no leading `/`, no `..` segment, no scheme, no backslash) and `slug` (no `/`/`..`). Validate `contentType`/`size` against the `course_material` scope caps (the existing images/pdf/**video** allow-list, 500 MB ceiling).
- Sign a presigned **PUT to the OSS origin** for the key **`courses/<slug>/<relativePath>`** (deterministic — the exact path the definition references and `/asset-urls` signs for playback). Reuse `oss.SignUpload`.
- Response: same shape as the existing upload endpoints (`{ putUrl, objectKey, requiredContentType, maxBytes, expiresAt }`).
- The generator uses this for **video / pdf / images / interactiveHtml**. It does **not** upload narration audio (ship generates that).

### 3. `POST /api/v1/admin/courses/{slug}/ship` — publish (admin-key)

Behavior:
1. Load the stored `CourseDefinition`.
2. **Narration TTS:** for every `narration` (and any `opening.fallback.audio` / `closing.fallback.audio`), synthesize the `text` via the Voice service and `PutObject` the mp3 to `courses/<slug>/<that audio path>`. Idempotent by content hash (skip if the object exists), reusing the tts cache + the scheme today's `GenerateCourseAudio` uses. Nil-guard Voice/OSS (degrade to skip, never fail the ship) consistent with the rest of the codebase.
3. **Cover:** set `course.cover` from the request (`{ "cover": "img:3" }` — a stock `web/` cover id, resolved for the catalog the same way project covers are via `resolveCoverURL`). A course-specific uploaded cover image can come later; stock ids are the simplest start.
4. **Publish:** set `status = 'published'`.
5. Response: `{ slug, status: "published", narrationsGenerated: N }`.

A thin wrapper `.deploy-local/ship-course.sh <slug> [cover]` (mirrors `deploy.sh`: holds the admin key + host, `curl`s this endpoint) is what gets run — so "ship preview course xxx" is one command.

## Preview visibility (the filter)

Every student-facing course read gates on the caller's role:
- `listCourses` — `WHERE status = 'published' OR $callerIsAdmin`. (Add a `status` filter to the catalog query; pass whether the session user is admin.)
- `getCourse`, `getCourseDefinition`, `POST …/session`, `POST …/asset-urls`, `POST …/scene` — if the course is `preview` and the caller is **not** admin → `404` (indistinguishable from a nonexistent course, so drafts don't leak). Admins pass through.

So an admin logs in normally, sees preview courses in the catalog, and plays them through the **unchanged** `RuntimeCoursePlayer` — real assets, real URL鉴权, real session. WYSIWYG with what students will get after ship.

## Testing

- Migration applies; existing rows become `status='published'`, `cover=''`.
- `PUT …/definition`: create → row exists, `status='preview'`, definition stored, slug/id-mismatch → 400, bad `schemaVersion` → 422, unknown `cardId` → 400, re-PUT preserves status.
- `POST …/asset-upload-url`: signs a PUT to `courses/<slug>/<relativePath>`; `..`/scheme/leading-slash/oversize/wrong-type → 400; nil OSS → 503.
- Preview gate: a `preview` course is absent from a student's `listCourses` and 404s on the by-slug reads; an admin sees + plays it. A `published` course is visible to all.
- `POST …/ship`: generates audio for each narration (stub Voice/OSS like the seed tests), sets cover, flips status; idempotent re-ship.
- Pure helpers (path/slug validation, catalog-field derivation) unit-tested without DB; handler tests use `oss.NewSigner` / stub Voice where signing/TTS is exercised.

## Out of scope (later)

- Course-specific uploaded cover images (start with stock ids).
- Un-publish / archive transitions (only `preview → published` now).
- Bulk/CI publish, versioning/rollback of a course definition.
