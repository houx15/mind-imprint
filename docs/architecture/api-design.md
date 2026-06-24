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
