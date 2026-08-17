# Course Authoring API — Handover for the Teacher-Side Production Workflow

**Date:** 2026-08-17
**Pinned at commit:** `0f144dfef6a4417d7e41b003cefece0f6a7590e5` (`main`, short `0f144dfe`) — this is the exact revision live on production.
**Audience:** the team building the teacher-side end-to-end course production tool (local materials → compile to `CourseDefinition` → validate → local preview with the real student renderer → annotate → AI-assisted revision → upload orchestration → submit).

> **Read this first — honesty note.** Your request (items 3 and 4) asks us to *confirm* several guarantees: optimistic concurrency, idempotent publishing, SHA-256 dedup, and upload support for all six media types. **Some of these do not exist in the current backend.** Rather than confirm them falsely, this document states plainly what exists today, what does not, and — for each gap — the workaround or the backend change you should request. The gaps are collected in §7 ("Gap register") so your planning can account for them up front.

---

## 1. Packages, preview host, and the golden example

### 1.1 The three shared packages

All three are **workspace packages with version `0.0.0`** — they are not independently semver-published. The *git commit is the version*. Pin to `0f144dfe` (or, better, ask us to cut a git tag you can reference — see "How to consume the pin" below).

| Package | Name | Version | Runtime deps | Purpose |
|---|---|---|---|---|
| `packages/course-contract` | `@mind-imprint/course-contract` | `0.0.0` | `zod@^3.23.0` | Zod schemas + validators; single source of truth for the `CourseDefinition` and `VideoInteractionDocument` shapes. |
| `packages/course-runtime` | `@mind-imprint/course-runtime` | `0.0.0` | `@mind-imprint/course-contract` (workspace) | Headless (no React) deterministic session state machine + host-injection adapter contracts + `InMemorySessionAdapter`. |
| `packages/course-renderer` | `@mind-imprint/course-renderer` | `0.0.0` | contract + runtime (workspace), `react-markdown@^10.1.0`, `remark-gfm@^4.0.1`; peer `react@^18.3.0`, `react-dom@^18.3.0` | The **real student React renderer** (`CoursePlayer`) you will preview against. Chrome-agnostic. |

**How to consume the pin.** These packages are `workspace:*` and are not published to any registry. Two options:
- **Git submodule / subtree / vendored copy** of `packages/course-contract`, `packages/course-runtime`, `packages/course-renderer` at commit `0f144dfe`. They are self-contained (only external deps are `zod`, `react-markdown`, `remark-gfm`, `react`).
- **Ask us for a git tag** (e.g. `course-authoring-handover-2026-08-17`) pointing at `0f144dfe` so you have a stable, named reference instead of a raw SHA. We can create it on request.

Version constants are exported so you can assert at runtime: `COURSE_CONTRACT_VERSION`, `COURSE_RUNTIME_VERSION`, `COURSE_RENDERER_VERSION` (all `"0.0.0"` today).

### 1.2 Reference preview host

The production host boundary is `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx`. Read it as the reference for *what a host must provide* — but note it wires every adapter to the **live backend** (signed CDN assets, server session, live Opening/Closing AI, SSE coach). For **local preview you do not need any of that** — see §6 for the minimal offline host recipe. Do not confuse the renderer's `CoursePlayer` (from `@mind-imprint/course-renderer`) with the legacy `apps/web/src/shell/courses/CoursePlayer.tsx`, which is the old linear player and unrelated.

### 1.3 Golden `CourseDefinition` example

A complete, valid `schemaVersion: "2.0"` document that exercises **all seven block types** lives at:

```
apps/api/internal/store/seed/courses/coverage-course.json
```

- Course `id: "evidence-comparability"`, `title: "Can These Two Claims Be Compared?"`, `language: "en"`, `estimatedMinutes: 6`, one objective, a personalized `opening` and `closing`.
- One part (`part-check-comparability`) with four slices covering `video` (with a video-interaction + `video-ended-and-interactions-completed` completion), `singleChoice` (graded, `submit-correct-or-exhausted`), `text`, `images` (side-by-side), `fillBlank` (`submit-any`), `pdf`, and `interactiveHtml`. Layout presets `split-horizontal 2:1`, `full`, `split-vertical`. Every slice carries a real workflow graph.

Its companion assets (real, not lorem) are in `docs/reference/mock_assets/coverage/`: `case.mp4`, `case-poster.jpg`, `case.en.vtt`, `original-chart.png`, `cropped-chart.png`, and `case-video.json` (the matching `VideoInteractionDocument`, `schemaVersion: "1.1"`, one graded singleChoice cue at 4.5s).

> ⚠️ **Known inconsistency in the example** — treat as an authoring lesson, not a contract fact: in `coverage-course.json` the video block declares `durationSeconds: 90`, while its `case-video.json` interaction document declares `durationSeconds: 9.4`. `validateVideoInteraction` bounds cue times against the **interaction document's** own duration, so this passes validation, but the two numbers should agree. Your compiler should keep the block duration and the interaction-doc duration in sync.

### 1.4 The `CourseDefinition` shape (authoritative summary)

Defined in `packages/course-contract/src/course.ts` (all `.strict()`):

```
CourseDefinitionDocument = { schemaVersion: "2.0", course: CourseDefinition }

CourseDefinition = {
  id, title, language (BCP-47), estimatedMinutes (>0),
  objectives:  [{ id, text, evidenceBlockIds[] }]            // ≥1
  opening:     { learningPreview[], personalization{enabled, allowedSignals}, fallback{text, audio?} }
  parts:       [{ id, title, objectiveIds[], slices[] }]     // ≥1
  closing:     { preparedSummary, takeaways[], transferApplications[], personalization{...}, fallback{text, audio?} }
}

SliceDefinition = { id, title, objectiveIds[], estimatedSeconds (>0),
                    blocks[≥1], layout, narrations[], workflow, navigation }
```

Block union (discriminated on `type`, `packages/course-contract/src/blocks.ts`): `text`, `images` (`single|side-by-side|gallery`), `pdf`, `video` (optional `interaction{source}`, `completion.rule ∈ video-ended | video-ended-and-interactions-completed`), `interactiveHtml` (`protocolVersion:"1.0"`, `aspectRatio 1:1|4:3`, `capabilities.audio?`), `fillBlank` (assessment = `graded` or `reflection`), `singleChoice` (assessment = `graded` or `survey`). Assessment completion rules: `submit-any | submit-correct | submit-correct-or-exhausted{maxAttempts}`.

`VideoInteractionDocument` (`packages/course-contract/src/videoInteraction.ts`): `{ schemaVersion:"1.1", video:{ blockId, source, durationSeconds, cues[] } }`; each cue `{ id, atSeconds≥0, pauseVideo, required, prompt, activity }`, activity reusing the same singleChoice/fillBlank assessment sub-schemas.

**Validators you should run in your compile step** (`packages/course-contract/src/validate/`):
- `validateCourseDefinition(input: unknown): ValidateResult` — runs structural (Zod) → referential → quality → per-slice workflow layers. Returns `{ok:false, issues[]}` if any issue has severity ≠ `"warn"`; warnings ride along on `ok:true`. `ValidationIssue = { path, message, layer, severity? }`.
- `validateVideoInteraction(doc, videoBlock): ValidationIssue[]` — cue-id uniqueness, strictly-increasing `atSeconds`, cue times within `durationSeconds`, and `blockId`/`source` matching the owning block.

There is also `collectAssetPaths(...)` exported from the contract package — use it to enumerate every relative asset path a definition references (drives your upload manifest).

---

## 2. Authoring API — endpoints, auth, request/response

**Base URL (production, the only environment — see §5):** `https://mind-api.uni-robot.cn`
**Auth for every `/admin/courses/...` route:** `Authorization: Bearer <OSS_ADMIN_KEY>` (see §5 for delivery). A blank/misconfigured server key never authenticates. On failure: **401**, body message `需要有效的管理密钥`, key never echoed.

All four authoring endpoints are under `/api/v1/admin/courses`. Course identity is the **stable `slug`** (a URL-safe string). `slug` is the conflict key everywhere.

### 2.1 Create or update the draft definition — `PUT /api/v1/admin/courses/{slug}/definition`

Handler: `apps/api/internal/api/course_definition_admin.go:54`.

**Request body:**
```jsonc
{
  "definition": { "schemaVersion": "2.0", "course": { ... } },  // the whole document, stored verbatim as jsonb
  "blurb":   "one-line course description",
  "cardIds": ["data-literacy", "opcvl", "craap"]                // tool cards granted on completion; each must exist in the card registry
}
```

**Border validation (server checks only the outer envelope; the deep shape is your responsibility via the contract package):**
- `definition` must be valid JSON → else **400** `validation_failed`.
- `course.schemaVersion` must be `"2.0"` → else **422** `invalid_course_definition`.
- `course.id` and `course.title` non-empty, and **`course.id` must equal `{slug}`** → else **400** `validation_failed`.
- Every `cardIds` entry must be a known card id → else **400** `validation_failed`.

**Response 200:** `{ "slug": "<slug>", "status": "<status>" }`. `status` is `"preview"` on a fresh insert; on a re-PUT it is the course's **existing** status (PUT never changes status — see §3.2).

### 2.2 Upload-URL planning — `POST /api/v1/admin/courses/{slug}/asset-upload-url`

Handler: `apps/api/internal/api/course_asset_upload.go:24`. (Full asset mechanics in §4.)

**Request:** `{ "relativePath": "assets/videos/case.mp4", "contentType": "video/mp4", "size": 4900000 }`
**Response:** `{ "putUrl", "objectKey", "requiredContentType", "maxBytes", "expiresAt" }` — presigned **PUT**, TTL 10 minutes, `objectKey = courses/<slug>/<relativePath>`. **503** if OSS is not configured.

### 2.3 Publish / ship — `POST /api/v1/admin/courses/{slug}/ship`

Handler: `apps/api/internal/api/course_ship.go:26`.

**Request:** `{ "cover": "img:3" }` (a stock cover catalog id; empty string allowed → preserves existing cover).
**Behavior:** loads the stored definition (**404** if slug unknown or no 2.0 definition), runs a pre-publish asset self-containment gate (**422** `asset_validation_failed` with `{issues:[...]}` if any referenced asset is missing/blocking), **generates narration TTS**, then flips `status → published` and sets the cover.
**Response 200:** `{ "slug", "status": "published", "narrationsGenerated": <int> }`.

> ⚠️ Ship **re-runs TTS on every call** and has no "already published" short-circuit — see §3.3. Budget for this.

### 2.4 Admin course listing — reuse the runtime list endpoint

There is **no dedicated `/admin/courses` list endpoint**. Admin discovery of draft (`preview`) courses goes through the **session-authenticated** runtime endpoint:

- `GET /api/v1/courses` → `{ "courses": [ { slug, branch, title, blurb, time_label, card_ids[], step_count, coverUrl } ] }`.
- This endpoint gates on **session role**, not the `OSS_ADMIN_KEY` bearer: an admin *session* sees `preview` + `published`; a student session sees `published` only. The list DTO exposes `coverUrl` but **not** the raw `status`.

**Implication for your tool:** the `OSS_ADMIN_KEY` bearer authorizes *writes* (PUT/ship/upload) but does **not** by itself let you *list or read back* draft courses — those readback routes are session-role gated (see §2.5, §3.1). If your workflow needs headless discovery/readback with only the bearer, that's a backend gap to raise (§7, G7).

### 2.5 Draft readback — `GET /api/v1/courses/{slug}/definition`

Handler: `apps/api/internal/api/course_definition.go:35`. **Session-authenticated** (a `preview` course 404s for non-admin sessions).
**Response 200:** `{ "definition": <raw stored jsonb>, "hash": "<sha256 hex of the exact stored bytes>" }`.

The `hash` is the sha256 of the stored definition bytes. It is a **session-staleness signal** (the runtime uses it to detect a course edited under an in-flight learner session). **It is not enforced as a write precondition on PUT** — see §3.1.

### 2.6 Bearer-gated readback for the generator (added 2026-08-17)

The endpoints in §2.4/§2.5 are **session-role gated** — the admin bearer key can write but cannot read a draft back without an admin login session. To let a headless generator round-trip the drafts it created **using only the `OSS_ADMIN_KEY` bearer**, two read endpoints were added. Both bypass the preview-visibility gate (that gate protects students, not the key holder) and neither mutates anything.

- **`GET /api/v1/admin/courses`** (bearer) → `{ "courses": [ { slug, branch, title, blurb, time_label, card_ids[], step_count, coverUrl, **status** } ] }`. Unlike the student list (§2.4), this includes `preview` drafts and exposes each course's `status`.
- **`GET /api/v1/admin/courses/{slug}/definition`** (bearer) → `{ "definition": <raw stored jsonb>, "hash": "<sha256 hex>", "status": "preview"|"published" }` for a course of **any** status. **404** for unknown slug or no 2.0 definition; **422** if the stored blob's `schemaVersion != "2.0"`.

Use these for the generator's "re-open a course I created earlier" flow. (The session-gated §2.5 route still exists and is what the student runtime uses.)

---

## 3. Item 3 — course identity, draft/published, concurrency, idempotency, recovery

This section answers your item 3 point by point. **✅ = exists as asked. ⚠️ = partial. ❌ = does not exist.**

### 3.1 Stable course identity — ✅ YES
Courses are keyed by a stable external `slug`. All authoring queries address the row `WHERE slug = $1`. (`course.id` is an internal UUID for a legacy FK, but the PUT handler enforces `course.id == slug`, so from your side the slug is the single identity.)

### 3.2 Draft / published separation — ⚠️ PARTIAL (status flag only, not content isolation)
- There is a single `status text NOT NULL DEFAULT 'published' CHECK (status IN ('preview','published'))` column (migration `0072_course_status_cover.sql`). Lifecycle is `preview → published`. New rows created via PUT land `preview`; publishing flips to `published`. PUT deliberately never changes `status`, so re-editing a course cannot accidentally (un)publish it.
- **But there is only ONE definition column** (`course_definition jsonb`, migration `0070`). There is **no separate `draft_definition` vs `published_definition`.** Editing the definition of an already-`published` course **mutates the live published bytes in place.** There is no content-level draft/published isolation and no snapshot-on-publish.
- **Consequence for your tool:** if teachers must edit a published course without the changes going live until re-ship, that isolation does not exist server-side today. Either (a) treat "published" as immutable in your UX and require a new slug for revisions, or (b) request a backend change to snapshot published content separately (§7, G4).

### 3.3 Optimistic concurrency (revision / definition hash) — ❌ DOES NOT EXIST
`PUT .../definition` is a **blind last-writer-wins UPSERT** (`INSERT ... ON CONFLICT (slug) DO UPDATE SET course_definition = EXCLUDED.course_definition, ...`). There is:
- no `revision`/`version` column,
- no `updated_at` precondition,
- no `If-Match`/ETag,
- no comparison of the `hash` from §2.5 on write.

Two authors racing on the same slug **silently clobber each other; the later write wins with no 409 and no detection.** The `hash` returned by GET is *not* accepted as a write precondition — the server offers no conditional-write path.
- **Consequence:** you cannot detect a concurrent overwrite through the API today. If your tool has multiple teachers or multiple tabs, you must serialize writes client-side, or request a backend `If-Match: <hash>` precondition on PUT (§7, G3). This is the single most important guarantee in your item 3 that is **absent**.

### 3.4 Idempotent publishing — ⚠️ PARTIAL (DB convergent, side effects not)
- The status write itself is an idempotent `UPDATE ... WHERE slug=$1` with `cover = COALESCE(NULLIF($3,''), cover)`, so the DB end-state converges (re-ship with empty cover preserves the existing cover; the row never duplicates or corrupts).
- **But ship has no `if status == 'published' return` guard and re-generates narration TTS unconditionally on every call.** Re-shipping an already-published course re-runs the full TTS pass (cost + latency; `narrationsGenerated` reflects a fresh run). There is no idempotency key and no request-dedup.
- **Consequence:** publishing is safe to repeat (won't corrupt), but it is **not cheap to repeat.** Do not treat re-ship as a free no-op. (Note: `GenerateDefinitionAudio` may internally skip audio it has already synthesized by content hash — that lives in `apps/api/internal/agent/`, below the handler — but the handler imposes no guard, so assume a re-ship does real work.)

### 3.5 Safe recovery after an ambiguous timeout — ⚠️ SAFE FOR CORRECTNESS
- **PUT** is naturally idempotent by slug (`ON CONFLICT (slug)`). Retrying the *same* body after an ambiguous timeout converges to the same row — no duplicate, no corruption. Caveat: because there is no concurrency guard (§3.3), a blind retry will also clobber any *intervening third-party edit*.
- **Ship** is safe for correctness on retry (terminal write is an idempotent UPDATE by slug), but a retry re-runs TTS (§3.4) — wasteful, not harmful.
- **Bottom line:** both writes are row-level idempotent by slug, so retry-after-timeout will not duplicate or corrupt a course. But there is **no concurrency control** and **no publish-side-effect guard.** Build your recovery on "retry is safe but may overwrite a concurrent edit and may repeat TTS," not on "the server will reject a stale or duplicate write."

---

## 4. Item 4 — asset upload: dedup, media types, readback

### 4.1 Upload flow
`POST /api/v1/admin/courses/{slug}/asset-upload-url` (bearer auth) returns a presigned **PUT** URL. You then `PUT` the bytes directly to OSS with the exact `Content-Type` from `requiredContentType` (the presign binds the content-type; sending a different one fails the PUT). Key formula is deterministic: **`courses/<slug>/<relativePath>`** — write and read share one function, so paths agree by construction. `relativePath` is passed through verbatim after validation (rejects `..` and absolute paths); there is **no UUID rename**. `maxBytes` for the course scope is **500 MB**; note the size check is a soft client-declared check, not enforced by OSS on the PUT.

### 4.2 SHA-256 / content-addressed dedup — ❌ DOES NOT EXIST (for teacher uploads)
The upload request DTO has **no `sha256`/checksum field**, and the handler never checks object existence. Every call unconditionally signs a fresh presigned PUT. Because the key is deterministic, **re-uploading the same `relativePath` silently overwrites** the object — there is no "already present, skip" response.
- A `Service.Exists(key)` primitive **does** exist in `apps/api/internal/oss/oss.go`, and content-addressed keying exists for *server-generated narration audio* (`sha256(voice+"\n"+text)`), but **neither is wired into the teacher upload path.**
- **Consequence:** the "send SHA-256 → server says already-present → skip re-upload" capability you asked for must be **built.** The `oss.Service.Exists` primitive is available to build on, but the upload handler and request DTO do not use it today (§7, G1). Until then, your tool can (a) track uploaded assets locally and skip re-planning, and/or (b) rely on overwrite-idempotence (same path → same object). Note there is no cheap server-side "does `courses/<slug>/<path>` already exist?" probe exposed to your bearer either.

### 4.3 Media-type allowlist — ⚠️ FOUR OF SIX (JSON + VTT rejected)
The `course_material` upload scope allowlist is exactly these 8 MIME types (`apps/api/internal/api/oss.go:61`):
`image/png`, `image/jpeg`, `image/webp`, `application/pdf`, `video/mp4`, `video/webm`, `video/quicktime`, `text/html`.

| Your required type | MIME | Allowed? |
|---|---|---|
| Images (png/jpg) | `image/png`, `image/jpeg` (+`image/webp`) | ✅ yes |
| PDF | `application/pdf` | ✅ yes |
| Video | `video/mp4` (+`video/webm`, `video/quicktime`) | ✅ yes |
| Interactive HTML | `text/html` | ✅ yes |
| **Video-interaction JSON** | `application/json` | ✅ **now allowed** (added 2026-08-17) |
| **WebVTT captions** | `text/vtt` | ✅ **now allowed** (added 2026-08-17) |

**RESOLVED 2026-08-17:** `application/json` and `text/vtt` were added to the `course_material` allowlist (`typeSet(...)` at `oss.go:63`) and `extByContentType` (`oss.go:90`), with a test (`TestPostCourseAssetUploadURL_InteractionTypes`). All six required media types now upload through the sanctioned path — the `text/html` stop-gap used during the release gate is no longer needed. (This change lands in the next backend deploy; until deployed on prod, keep using the `text/html` stop-gap.)

### 4.4 Asset readback — ✅ signed URLs (Type-A CDN or presigned GET)
There is **no public/un-signed CDN URL** for course assets — every read is signed via a `SignDownload` seam. Two ways to obtain URLs:
- **Batch (session-auth):** `POST /api/v1/courses/{slug}/asset-urls` with `{ "paths": ["assets/videos/case.mp4", ...] }` (≤256 paths) → `{ "assetUrls": { "<path>": "<signed url>" }, "expiresAt": "<RFC3339>" }`. Re-POST to refresh when a session outlasts the window.
- **Single object (session OR bearer):** `POST /api/v1/oss/resolve-url` with `{ "objectKey": "courses/<slug>/<path>" }` → a signed URL. Object key must sit under `web/`, `courses/`, or `users/`.

When a CDN URL鉴权 主KEY is configured the signed URL is an Aliyun Type-A `auth_key` link (cacheable — `auth_key` is excluded from the cache key); otherwise it falls back to a 15-minute OSS presigned GET. **For local preview you do not use any of this** — you resolve assets to local file/blob URLs (§6).

---

## 5. Item 5 — environment, credential, namespace

### 5.1 There is no dedicated staging environment — ❗ PROD ONLY
Honest statement: a repo-wide search found **no staging/preprod host**. The only environment is production:

| | URL |
|---|---|
| API | `https://mind-api.uni-robot.cn` |
| Web (SPA) | `https://mind-web.uni-robot.cn` |

Both resolve to a single ECS host `47.93.151.131` (docker-compose: `db` + `api` + `web`; host nginx role-routes both domains; TLS via certbot). "Test isolation" today is achieved **by a dedicated sandbox course slug on prod**, not by a separate environment.

- **Recommendation:** if you need a true staging environment for the teacher tool, that is a request to us (stand up a second compose stack + subdomain, or a namespaced tenant). Until then, use the sandbox slug below and treat every write as hitting production.

### 5.2 Credential
- **Env-var NAME:** `OSS_ADMIN_KEY` (declared `apps/api/internal/config/config.go`, passed through `deploy/docker-compose.prod.yml`, template `deploy/env.prod.example`). It is a static bearer secret; the server compares it in constant time.
- **Header format:** `Authorization: Bearer <OSS_ADMIN_KEY>`.
- **Secure delivery (per your instruction — not via chat, not in the repo):** we will hand you the value out-of-band through a secrets channel (e.g. a shared password manager entry or an encrypted 1-1 handoff). In your tool, read it from an env var named `OSS_ADMIN_KEY` sourced from a git-ignored `.env`/secrets store — mirror our own scripts, which source it from a git-ignored `env.prod` and never commit it. **Do not** inline the literal in any committed file.
  - Housekeeping note on our side: one local wrapper script currently inlines the key literally on disk (git-ignored, not in history). We plan to rotate and move it into the env file. If the key is rotated, we will re-deliver it out-of-band.

### 5.3 Test course namespace
- **Sandbox course slug:** `evidence-comparability` (the "coverage course"). Its definition is the golden example in §1.3; its assets live under `courses/evidence-comparability/...` on prod OSS. Use it as the safe target for end-to-end testing.
- **Asset namespace formula:** `courses/<slug>/<relativePath>`. Relative-path conventions in use: `assets/videos/…`, `assets/images/…`, `assets/captions/…`, `assets/pdfs/…`, `interactions/html/…`, `interactions/video/…`. (Server-generated narration audio uses a separate `courses/audio/<slug>/…` namespace — you do not write there.)
- **Recommendation:** pick a distinct slug prefix for teacher-tool test courses (e.g. `sandbox-<name>`) so you never overwrite `evidence-comparability` or any real course. Remember: same slug + same relativePath overwrites (§4.2), and there is no draft isolation for a published slug (§3.2).

---

## 6. Local preview recipe (no login / no server session / no live AI)

You asked to preview with the **real student renderer** without the full student app, login, production session adapter, or live Opening/Closing AI. That is fully supported — the renderer is chrome-agnostic and takes all its coupling through injected adapters. Mount `CoursePlayer` from `@mind-imprint/course-renderer` with these props:

**Required:** `document` (your compiled definition, `unknown` — validated internally), `adapters: CourseRuntimeAdapters`, `studentId: string`, `idFactory: IdFactory`, `clock: Clock`.
**Optional (skip for preview):** `definitionHash`, `sessionId`, `onBusReady`, `signalResolver`, `onProgress`, `onComplete`.

The four adapters (`CourseRuntimeAdapters = { assetResolver, sessionAdapter, openingGenerator, closingGenerator }`), all satisfiable offline:

- `sessionAdapter` → **`InMemorySessionAdapter`** from `@mind-imprint/course-runtime` (`adapters.ts`). No backend.
- `assetResolver` → a static `{ resolve(relativePath) { return localUrlFor(relativePath) } }` mapping relative paths to local file/`blob:`/dev-server URLs. No signed CDN.
- `openingGenerator` / `closingGenerator` → **stubs that return the course's own `fallback.text`** as a `RuntimeSceneResult` with `fallbackUsed: true`. No AI, no network. (The renderer only calls the generators when a scene isn't already persisted; returning the authored fallback is exactly the "no personalization" path.)
- `idFactory` → `() => crypto.randomUUID()`; `clock` → `() => new Date().toISOString()`.

**If the course has video-interaction blocks:** also wrap the tree in `<InteractionLoaderProvider value={loader}>`, where `loader` reads your local video-interaction JSON (the same document `validateVideoInteraction` checks) and returns it. Everything else in the production host (`RuntimeCoursePlayer.tsx`) — the top progress bar, the `AskPanel` SSE coach, pagehide/visibility flush, signed-URL fetches — is host chrome you omit.

This gives teachers a pixel-accurate preview through the exact renderer students use, entirely offline.

---

## 7. Gap register — what to request from us before/during build

Each row below carries the product owner's decision (2026-08-17 review).

| # | Gap | Decision | Detail |
|---|---|---|---|
| **G1** | No SHA-256 asset dedup on the upload path. | ✅ **Not needed — won't build.** | The real concern is not re-creating the same *course*, solved client-side: the generator **records the slug/id it created and reuses it** (PUT is keyed on slug → updates in place, never duplicates). Re-uploading the same relativePath overwrites the same object, so files don't pile up either. No backend change. |
| **G2** | `application/json` + `text/vtt` not in the upload allowlist. | ✅ **DONE (2026-08-17).** | Both MIMEs added to `oss.go:63` + `extByContentType`, with a test. All six media types now upload cleanly. Lands next deploy. |
| **G3** | No optimistic concurrency on PUT (last-writer-wins, silent). | ⏸️ **Skip for now.** | Only needed if two editors touch the *same* course concurrently. The generator is single-editor-per-course, so blind LWW is acceptable. Revisit only if shared editing appears. (Fix, if ever: `If-Match: <hash>` precondition on PUT.) |
| **G4** | Draft/published content isolation. | ✅ **Status flag is sufficient.** | Students already see **published only** (list filters `status='published'`; a preview course 404s for students). Separation via the status flag is the chosen model — no second definition column. Caveat: editing an already-published course goes live on save (before re-ship); acceptable given save-then-ship happen together. If stricter "hidden until re-publish" isolation is later required, request it. |
| **G5** | Ship regenerates TTS every call. | ✅ **Intended behavior.** | Publishing to prod is *defined as* "generate narration TTS + go live" — this is desired. Only note: don't press publish repeatedly, since each press re-runs TTS. |
| **G6** | No staging environment. | ✅ **Prod-only by choice.** | No separate staging tier. Test against prod using a throwaway slug (e.g. `sandbox-*`); every write hits the live system. |
| **G7** | Bearer authorizes writes but not headless list/readback of drafts. | ✅ **DONE (2026-08-17).** | Two bearer-gated read endpoints added (see §2.6): `GET /admin/courses` (all courses incl. preview, with status) and `GET /admin/courses/{slug}/definition` (`{definition, hash, status}`, any status). DB-backed tests prove the key reads back a preview draft. Lands next deploy. |

Net: backend changes made are **G2 (done)** and **G7 (done)**. G1/G3/G4/G5/G6 are resolved by design decisions (no backend work). No open items remain.

---

## 8. Quick reference — end-to-end authoring sequence

1. **Compile** local materials → `CourseDefinition` document (`{ schemaVersion:"2.0", course:{...} }`), `course.id === slug`.
2. **Validate** locally: `validateCourseDefinition(doc)` + `validateVideoInteraction(...)` per video. Fail on any non-`warn` issue.
3. **Preview** with the real renderer via the offline host recipe (§6).
4. **Plan uploads:** `collectAssetPaths(doc)` → for each asset `POST .../asset-upload-url` → `PUT` bytes to the presigned URL with the required content-type. (Mind G1/G2.)
5. **Save draft:** `PUT .../definition` with `{ definition, blurb, cardIds }` → course lands/stays `preview`. (No concurrency guard — G3.)
6. **Read back** (bearer, headless): `GET /admin/courses/{slug}/definition` → `{ definition, hash, status }` (§2.6); or discover all your courses via `GET /admin/courses`.
7. **Publish:** `POST .../ship` with `{ cover }` → asset gate + TTS + `status:published`. (Re-ship re-TTSes — G5.)
8. Students then see the course via `GET /courses` (published) and play it through `RuntimeCoursePlayer`.

All writes target `https://mind-api.uni-robot.cn` with `Authorization: Bearer <OSS_ADMIN_KEY>`. Sandbox on slug `evidence-comparability` (or a fresh `sandbox-*` slug). Everything is production — there is no staging.
