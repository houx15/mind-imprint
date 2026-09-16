# API Design

> **Reference doc.** The HTTP API for the 思维印记 backend platform. Authored
> 2026-06-24. Companion to the
> [architecture spec](../superpowers/specs/2026-06-24-backend-platform-architecture-design.md)
> and [database-schema.md](./database-schema.md).
>
> **Style:** REST + JSON, one Server-Sent-Events streaming endpoint for the agent
> turn. Cookie-authenticated (P2). Base path `/api/v1`.
>
> **Phase tags** mark when an endpoint becomes real. P1 runs as a seeded mock
> student (a dev "act-as-seed-user" middleware) so every route works before auth
> exists; P2 replaces that middleware with real sessions — no route signatures change.

## Conventions

- **Content type:** `application/json` for everything except `POST /tasks/:id/turn`
  (`text/event-stream`).
- **Auth:** an httpOnly/Secure/SameSite=Lax session cookie. State-changing requests
  also carry a double-submit CSRF token (header `X-CSRF-Token`) once P2 lands.
- **IDs** are UUID strings.
- **Timestamps** are ISO-8601 UTC strings.
- **Errors** use one envelope (see below); 2xx bodies are the resource or `{}`.

### Error envelope

Every non-2xx response:

```json
{ "error": { "code": "string_code", "message": "human-readable, safe to show", "details": {} } }
```

- `code` is a stable machine string (`validation_failed`, `unauthorized`,
  `email_unverified`, `invalid_join_code`, `not_entitled`, `not_found`,
  `rate_limited`, `internal`).
- `message` never leaks internals; real error detail goes to `slog` server-side only.
- Status mapping: `400` validation, `401` unauthenticated, `403` forbidden,
  `404` not found, `409` conflict (e.g. email taken), `429` rate-limited,
  `500` internal.

---

## Health

```
GET /healthz   → 200 "ok"                liveness
GET /readyz    → 200 when DB reachable    readiness
```

---

## Auth  `[P2]` (P1: seeded student + dev login stub)

### `POST /api/v1/auth/signup`
Creates an org-bound account. **A valid class join code is required** — without one,
registration is rejected (the organization invariant).

Request:
```json
{ "email": "phoebe@example.com", "password": "…", "display_name": "Phoebe", "join_code": "SY-7Q2K" }
```
Behavior: one atomic transaction creates the `users` row (with `school_id` taken
from the join code's `class.school_id`) **and** the `enrollments` row binding it to
the join code's class. Generates a verification token (stores only its hash, 24h
TTL) and emails the raw token via the `Mailer`.

Responses: `201 {}` (verification email sent) · `409 email_taken` ·
`400 invalid_join_code` · `400 validation_failed` · `429 rate_limited`.

### `POST /api/v1/auth/verify-email`
Request: `{ "token": "…" }` → marks `email_verified_at`, consumes the token, opens a
session (sets cookie). Responses: `200 {user}` · `400 token_invalid_or_expired`.

### `POST /api/v1/auth/signin`
Request: `{ "email": "…", "password": "…" }`. Verifies argon2id; **rejects
unverified accounts**. Sets the session cookie.
Responses: `200 {user}` · `401 invalid_credentials` · `403 email_unverified` ·
`429 rate_limited`.

### `POST /api/v1/auth/signout`
Deletes the session row, clears the cookie. `204`.

### `GET /api/v1/auth/me`
```json
200 { "user": { "id":"…", "email":"…", "display_name":"…", "role":"student",
                "avatar_color":"…",
                "school": { "id":"…", "name":"…" },
                "classes": [ { "id":"…", "name":"…", "role_in_class":"student" } ] } }
```
`401 unauthorized`. The school is always present (invariant); `classes` is an array
(one for a student, several for a teacher, empty for an admin).

---

## Tasks  `[P1]`

### `GET /api/v1/tasks`
`200 { "tasks": [ { id, title, seed, status, created_at, last_active_at } ] }` — the
caller's tasks, most-recently-active first.

### `POST /api/v1/tasks`
Request: `{ "title": "…", "seed": "https://…" | null }` → `201 { task }`.

### `GET /api/v1/tasks/:id`
`200 { task, messages: [...], cards: [...] }` — the task plus its transcript and
card instances (the client builds the process-tree projection from `cards` +
`messages`). `404 not_found` if not owned by the caller.

---

## The agent turn (SSE)  `[P1]`

### `POST /api/v1/tasks/:id/turn`
Request: `{ "user_input": "…" }`. Response: `text/event-stream`.

Gated by `HasEntitlement(ctx, user)` (a token-spending endpoint). When it returns
false the request is rejected before any model call: `403 not_entitled`. Today the
seam returns true for everyone.

The server builds the system prompt (克制阶梯) + the card catalog as the
`summon_card` tool from the embedded card JSON, resolves the key
(`keyResolver(ctx)`), calls the provider, and streams:

```
event: text
data: {"delta":"…assistant prose tokens…"}

event: card
data: {"card_instance_id":"…","card_id":"sift_craap","nudge_text":"…"}

event: done
data: {"message_id":"…"}
```

- `text` events stream assistant prose to render live.
- A `card` event means the model emitted `summon_card`: the server has persisted a
  `proposed` card_instance and stored the tool_call id on the assistant message, and
  **the turn ends** — the tool *result* is the human filling the card.
- `event: error` carries the standard error envelope and ends the stream.

Streaming hygiene: flush per event, `X-Accel-Buffering: no`, the upstream call bound
to the request context (client disconnect cancels it), heartbeat comments. On completion the
per-turn usage (tier/tokens/cost) is written onto the assistant `messages` row (no
separate ledger table).

**One card per turn** is enforced server-side (一次只问一个).

---

## Card envelope lifecycle  `[P1]` — the human-executed tool

A proposed card is created by the turn stream. The client then drives it through the
envelope states. Close ≠ skip: merely closing the sheet records nothing; only an
explicit skip records a skip.

### `PATCH /api/v1/tasks/:id/cards/:cid`
Request: `{ "status": "active" }` → marks the card opened (so the live process tree
reflects it). `200 { card }`.

### `PUT /api/v1/tasks/:id/cards/:cid`
Submit the completed envelope:
```json
{ "field_values": { … }, "event_trace": [ … ], "status": "completed" }
```
The server validates the **outer** envelope (legal status, well-formed trace,
object field_values), persists, sets `completed_at`, and — on the **next** `/turn` —
refeeds the serialized card outcome as the `summon_card` tool result. `200 { card }`.

### `POST /api/v1/tasks/:id/cards/:cid/skip`
Request: `{ "event_trace": [ … ] }` → status `skipped`, recorded as process signal.
`200 { card }`.

---

## Evaluation  `[P1 sync → P4 async]`

### `POST /api/v1/tasks/:id/evaluate`
Triggers evaluation. Gated by `HasEntitlement` (`403 not_entitled` when false).
**P1:** runs inline (flagship model, never downgraded), writes the `evaluations`
row, returns `201 { evaluation }` with `status:"done"`. **P4:** enqueues a river
job, returns `202 { evaluation_id, status:"queued" }`.

> The evaluation response shape is **provisional** — the eval data model gets a
> dedicated design pass (see database-schema.md). P1 returns the current SOLO-rubric
> contract (`scores[]` + `narrative`).

### `GET /api/v1/tasks/:id/evaluation`
`200 { evaluation: { id, status, scores, narrative, model, created_at, completed_at } }`.
The frontend polls this until `status` is `done` or `failed` (P4). `failed` never
exposes `error` internals to the client.

---

## Card specs (no endpoint)

The 33 card JSON specs are **not** served at runtime. The frontend imports them at
build time from `packages/contracts/cards/`; Go `go:embed`s the same files. One
shared source, no drift, no fetch.

---

## CORS

The SPA origin is allowed explicitly (`rs/cors`, exact origin, `credentials: true`)
so the session cookie flows. In production the SPA and API are served under the same
registrable domain to keep SameSite=Lax effective.

## PBL artifact trial drafts (2026-09-15)

All three endpoints use the normal Lite session and check ownership of both project and artifact. A same-user artifact under a different project path is also rejected with 404. No endpoint calls a model.

- `GET /pbl/projects/{id}/artifacts/{aid}/trial-draft`: returns `{document, revision}`. Missing row returns an empty draft at revision 0 without creating feedback.
- `PUT /pbl/projects/{id}/artifacts/{aid}/trial-draft`: accepts `{document, revision}`. Document fields: `mode` (`self` / `other` / `not_tested`), `version`, `task`, `expected`, `actual`, `next`; text fields allow unfinished content and are capped at 1500 Unicode code points each. Saves increment revision. Stale revision returns 409 without replacing the saved document.
- `POST /pbl/projects/{id}/artifacts/{aid}/trial-draft/submit`: accepts `{revision}`. Requires version, task, expected, plus actual for a claimed test. Creates one keep entry and clears the draft in one transaction. Untested input becomes thought/change; tested input becomes feedback/observe. Untested output never reuses a hidden actual result. Returns `{entry, draft}`. A retry of the latest submitted revision returns the same entry and the current draft; it does not erase a later autosave. Older or conflicting revisions return 409.

Drafts do not enter the coach, evidence checker, feedback history, or completion claims. Only explicit submission produces a keep entry; opening its discussion remains a separate action. The browser autosaves after 400 ms, drains pending writes before supported navigation/closing/version switching, shows unsaved status and prevents refresh without the browser's unsaved-changes warning. Conflict recovery compares local and saved drafts before choosing which to retain.

### Optional PBL inspiration bookmarks (2026-09-15)

`POST /pbl/projects/{id}/sites/bookmark` accepts `{url}` and saves a normalized HTTP(S) link after checking project ownership. It does not fetch the URL or call a model; title/analysis remain empty for a new bookmark. Re-collecting the same URL preserves its student judgment and any existing AI analysis. Query parameters and fragments are retained because interactive examples can use them to select a specific effect. A bookmark makes no claim that its destination exists, was viewed, or was tested.

The existing `POST /pbl/projects/{id}/sites` remains the separate, explicitly requested AI text-analysis path. Failed extraction leaves a previously saved bookmark and student note intact. Its prompt accepts useful content from tutorials, interactive-demo descriptions, art/science pages or personal websites, and requires quoted textual evidence; it cannot claim to have seen or operated visual effects. Student observations remain distinct from AI analysis in refeed. The UI allows zero references and keeps the optional inspiration entry available before the first bookmark.

### Versioned creative homepage publication (2026-09-15)

`POST /pbl/site/publish` accepts optional `{versionId}`. A supplied version must belong to the user's homepage project, contain saved student page content (process notes alone are insufficient), preserve that content in its HTML, and match the currently retained trial version with a nonempty observation. Publication writes the selected version pointer and share token in one transaction; creative direction is locked before the site, matching generation's lock order. An existing share token is retained during updates. New private versions and content edits never change the selected published version.

`GET /pbl/projects/{id}/code-versions` returns version metadata plus `publicationMissing`, without exposing brief JSON in the list. These reasons inform the publish screen before the action; the POST independently enforces them. `GET /pbl/site` includes the owner's `publishedVersionId` when available. Legacy template-only publication remains for projects without a version publication; omission of a version after publishing a generated version is rejected rather than silently switching back to a template.

Creation direction may opt into `includeComparison` only with `includeProcess` and a retained trial. Content composition resolves that trial's parent within the same project before calling the model, then stores `processComparison` (`beforeVersionId`, `afterVersionId`, `feedback`, `observation`) in the immutable code-version brief. A trial without a parent returns `400 missing_comparison_parent` without a model call. Ordinary revisions inherit their base version's comparison snapshot; later edits to the creative draft do not change an existing version. This snapshot does not itself grant public access to either historical version. The student-facing opt-in is shown with the retained process. Owner version metadata and published site metadata expose only selected comparison text; public metadata also gives an opaque render key for the publication (no internal version IDs). Rendering accepts `comparison=before|after` resolved exclusively from that root version and verifies the actual parent relationship within the same project. Public rendering optionally takes `publication=<expected-render-key>` and rejects a changed publication instead of mixing versions. Both sides use unchanged code-preview isolation headers and the active site share token; revoking the token disables both. No arbitrary history-ID public route is added.

For a generated publication, anonymous `GET /public/sites/{token}` returns only `{generated:true}`. `GET /public/sites/{token}/render` resolves the active share token and immutable version, renders its owned image asset using the same isolated renderer as private previews, and sends CSP sandbox, no-store and noindex headers. It never returns private version IDs, prompts, trial records or other draft metadata. `DELETE /pbl/site/publish` removes the active token, making both public routes inaccessible; previously loaded browser content cannot be remotely erased. The version pointer remains available to the owner for explicit re-publication.

### PBL 观察草稿（2026-09-15）

`GET/PUT /api/v1/pbl/projects/{id}/tools/{tid}/observation-draft` 读取或保存 `{document: [{kind,body,imageKey,from?}], revision}`。只允许当前用户项目的 observe 工具；图片 key 必须归属当前账号。PUT 使用 revision 乐观锁，冲突返回409。未提交草稿不进入便签或教练上下文。

`POST .../observation-draft/submit` 接收 `{revision}`，在事务中将非空记录生成 student 便签并清空草稿，返回 `{notes,draft}`。同一 submitted revision 的重复请求返回原便签结果，不重复创建；新的草稿不被旧提交重试清空。

观察清单读取返回每项的 `supersededAt`，非空表示历史任务。勾选历史项返回409。教练通过 `mission_target`（本轮上下文中的当前项目未结束observe工具ID）加完整 `mission` 调用现有清单修订，可与计划同轮产出。服务端保留生成前快照并在事务内比较，期间发生勾选或修订则拒绝覆盖，在对话中报告失败；不接受模型提供的版本号。前端在教练回合结束后重新读取清单，历史任务独立折叠展示。

`POST /pbl/projects/{id}/turn` 的 `observation:{toolId,revision}` 表示讨论该工具刚提交的一批记录，不结束工具。不能与text或completedToolId同时发送；校验工具为当前项目/当前讨论的accepted观察工具，并与服务端submitted_revision匹配。只读取持久化回执正文，陈旧回执或错误归属返回409。观察界面“提交记录并讨论”使用此事件，明确“结束本次观察”才走已有工具完成流程；未提交正文须先提交，草稿与便签不因结束被删除。

### 学生直接编辑观察任务

`PUT /api/v1/pbl/projects/{id}/mission/{mid}` 接收 `{prompt, version}`。version为清单GET返回的只读行指纹，prompt为1至2000字。仅限本人项目中未结束观察工具的当前任务；项目与任务归属不匹配返回404，历史任务、过期指纹或工具结束返回409。成功返回包含历史的完整任务数组，字段增加version与editedByStudent。只修订指定项并保留旧记录，不调用模型，不标记任务已完成。后续教练请求读取学生修改后的任务及来源。
