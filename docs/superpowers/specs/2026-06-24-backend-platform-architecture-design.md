# Backend Platform Architecture — Design Spec

> **Status:** Approved design (north-star). Authored 2026-06-24.
> **Scope:** The architecture that governs the move from a pure-frontend SPA to a
> frontend/backend-separated platform with a Go backend, a real database, real
> accounts, and async evaluation. This document is the *north-star*; each
> implementation phase (P1–P4) gets its own spec → plan → build cycle.
>
> **Companion documents:**
> - [`docs/architecture/database-schema.md`](../../architecture/database-schema.md) — full table-by-table schema.
> - [`docs/architecture/api-design.md`](../../architecture/api-design.md) — every endpoint + the SSE protocol.
> - [`docs/architecture/chat-history-storage.md`](../../architecture/chat-history-storage.md) — message storage, growth analysis, partitioning.
> - [`docs/architecture/go-backend-best-practices.md`](../../architecture/go-backend-best-practices.md) — Go stack reference.

---

## 1. Goal

Turn 思维印记 from a browser-only demo (all state in `localStorage`, the browser
calling the model directly) into a real platform: a Go backend that owns the
database, the model, and the agent brain; a React SPA that renders and calls
the API; real accounts gated by organization membership; and asynchronous
process evaluation.

This is **greenfield backend work** — there is no existing Node/Express server to
refactor. The Node+Express gateway in the original PRD was always "future state"
and was never built.

## 2. Non-negotiable inheritance (the product DNA)

The platform rewrite must not erode what the product *is*. These survive verbatim:

**四条设计铁律 (the four design laws):**
1. **AI 克制** — the AI never concludes for the student; the 克制阶梯 system prompt
   is the core of this and moves server-side **verbatim** (pinned by a parity test).
2. **不操纵** — no slot-machine retention. `summon_card` only *proposes*; opening a
   card is always a human action; **close ≠ skip** (only an explicit 跳过 records a skip).
3. **一次只问一个** — one question, one card per turn. Enforced in the turn loop.
4. **过程即数据** — skips, opens, and field changes are signal, recorded in the
   envelope's `event_trace`; now durably in Postgres, with per-turn tier/token/cost
   recorded on each assistant message as first-class process data.

**The standard envelope (`CardInstance`) is the load-bearing floor.** Its shape
(`field_values`, `event_trace`, `status`) is unchanged. It is the common ground of
the process tree, usage counts, and evaluation.

**Card = a tool in the tool-use loop that a human executes.** `summon_card(card_id)`
remains the single seam between the decision layer and the Card Runtime.

**New card = new JSON, no renderer change.** This law now extends to the backend:
the same card JSON is read by Go (prompt/refeed/catalog) and by the frontend
(rendering). One source, zero drift, no code change to add a card.

## 3. Architecture overview

Three deployable units in one git repo (the existing pnpm workspace):

```
mind-imprint/
├─ apps/web/            React+Vite SPA — pure renderer + API client (no localStorage store, no model calls)
├─ apps/api/            NEW · Go service (own go.mod) — the ONLY unit that holds secrets, talks to DB or model
│  ├─ cmd/api/main.go          wiring + graceful shutdown
│  ├─ internal/
│  │  ├─ http/                 ServeMux router, middleware, handlers, JSON error envelope
│  │  ├─ agent/                the brain: prompt builder, summon_card tool, turn loop, refeed (ported from TS)
│  │  ├─ gateway/              provider adapters (deepseek/anthropic) + SSE proxy + keyResolver seam
│  │  ├─ eval/                 SOLO-rubric evaluation (inline P1 → river worker P4)
│  │  ├─ auth/                 signup/verify/signin, sessions, argon2id        [P2]
│  │  ├─ org/                  schools/classes/enrollments + HasEntitlement seam [P2 minimal → P3 rich]
│  │  ├─ store/                sqlc-generated queries + pgxpool
│  │  └─ cards/                go:embed of the shared card JSON + spec types
│  ├─ migrations/              goose SQL migrations
│  └─ Dockerfile               multi-stage → distroless
├─ packages/contracts/         Zod schemas STAY (frontend's truth for rendering)
└─ packages/contracts/cards/   the 33 card JSON specs — single shared asset (TS imports; Go go:embed)
```

**The rule that makes the split real:** `apps/api` is the only unit holding secrets
(LLM key, DB DSN, session secret, SMTP creds) and the only unit that talks to the
model or the database. `apps/web` holds nothing sensitive.

**Tech stack** (rationale in the Go best-practices doc):

| Concern | Choice |
|---|---|
| HTTP | `net/http` ServeMux (Go 1.22+ method + wildcard routing) |
| DB driver / queries | `pgx/v5` + `pgxpool` + **sqlc** (type-safe SQL) |
| Migrations | **goose** |
| Auth | **argon2id** + server-side sessions in Postgres (httpOnly/Secure/SameSite cookie) |
| Email | `Mailer` interface — dev = log; prod = Aliyun DirectMail / Tencent SES |
| Async jobs | **river** (Postgres-backed; no Redis) |
| Streaming | hand-written SSE proxy |
| Config | `caarlos0/env` typed struct + `godotenv` (local only) |
| Logging / CORS / validation | `slog` · `rs/cors` · `go-playground/validator` |
| Tests | table-driven + `httptest` + `testcontainers-go` (real Postgres) |

**Region:** China-first. Email and model providers sit behind interfaces; default
model is DeepSeek (Anthropic selectable, via relay where network requires).
Global / Resend / Anthropic-default is a later config swap, not a rewrite.

## 4. The smart gateway (agent brain, server-side)

The backend owns the system prompt, the `summon_card` tool, the turn loop, and the
refeed logic. The frontend submits user input and card envelopes and renders the
stream. `summon_card` works as a **human-executed tool**:

```
POST /tasks/:id/turn  (Server-Sent Events)
  1. load task history; build system prompt (克制阶梯) + card catalog as the summon_card tool (from go:embed JSON)
  2. keyResolver(ctx) → {provider, model, key, orgID?}   (platform env now; org-billing later)
  3. stream provider deltas to the browser:
        event: text  → assistant prose tokens
        event: card  → summon_card tool_call: persist a `proposed` card_instance,
                       store the tool_call id, emit {card_instance_id, card_id, nudge_text}, end turn
        event: done
  4. the turn ENDS after summon_card — the tool RESULT is the human filling the card
  5. client submits the envelope (PUT) or skips (POST)
  6. the NEXT /turn refeeds the serialized card outcome (ported refeed logic) and continues
```

A card proposal cleanly closes one model turn; the human's envelope reopens the
loop as the tool result. **Text-and-card-in-one-response** is native: `text` events
and the `card` event arrive in the same stream.

**The key-resolver seam:** `keyResolver(ctx) → (provider, model, key, orgID?)`.
Today it returns the platform env secret. For future per-org billing it resolves
the caller's school/class key; org attribution comes from the user→school relation
at rollup time (the `llm_usage` view). Nothing else in the gateway changes.

**Streaming hygiene:** `Flush` per event; `X-Accel-Buffering: no`; the upstream
call is bound to `r.Context()` so a client disconnect cancels the provider call;
heartbeat comments keep the connection alive; on completion the per-turn usage
(tier + tokens + cost) is written onto the assistant `messages` row (no separate
ledger table).

## 5. Data model

Full detail in [`database-schema.md`](../../architecture/database-schema.md). Summary:

- **Identity & org:** `users` (role enum, `school_id` NOT NULL),
  `email_verification_tokens`, `sessions`, `schools`, `classes` (with `join_code`),
  `enrollments` (user↔class join).
- **Core domain:** `tasks`, `messages` (with per-turn usage), `card_instances`,
  `evaluations` (provisional).

Five deliberate choices:
1. **The envelope is stored as JSONB, validated at the edge only.** `field_values`
   and `event_trace` are `jsonb`. Go validates the *outer* envelope (status enum,
   required ids); the inner shape's deep truth stays the Zod contract. A new card
   adds zero columns.
2. **No `process_node` table.** The process tree is a read-time projection from
   `card_instances.parent_node_id` + messages (matches current behavior).
3. **Structural belonging vs entitlement are separate.** School/class belonging is
   real data (`schools`/`classes`/`enrollments`/`users.school_id`). Paid access
   ("membership") is **not** a table — it is a stubbed backend seam
   `HasEntitlement(ctx, user)` (period-subscription vs token-balance is undecided),
   checked before token-spending endpoints.
4. **Per-turn usage lives on the turn, not in a ledger.** One model call = one
   assistant `messages` row, so `model/tier/tokens/cost` sit on that row (and on
   `evaluations` for eval calls). **No `llm_calls` table.** Org cost rollups use an
   `llm_usage` UNION view; a real ledger is a later additive migration if billing
   needs it. Message storage rationale + growth analysis:
   [`chat-history-storage.md`](../../architecture/chat-history-storage.md)
   (row-per-message, append-only; partition later, not now).
5. **The evaluation data model is provisional.** P1 keeps the working SOLO-rubric
   contract; the richer shape gets its own design pass (see §7).

## 6. Authentication & the organization invariant

**Hard invariant (Global Constraint): every account belongs to a school; students
and teachers also belong to ≥1 class. No orgless registration — ever.** (Admins
belong to a school but no class.)

This couples auth and org, so a minimal org slice lands with **P2**:

- **Signup requires a valid class join code.** One atomic transaction creates the
  `users` row (its `school_id` taken from `class.school_id`) **and** the
  `enrollments` row binding it to the code's class — or the whole signup fails. No
  code path produces an orgless user.
- **Entitlement is a separate, deferred concept.** "Membership" as *paid access* is
  not modeled as data yet; the `HasEntitlement(ctx, user)` seam (stubbed `true`)
  gates token-spending endpoints (`/turn`, `/evaluate`) and is where a future
  period-subscription or token-balance model plugs in — no call sites change.
- Password hashing: **argon2id**. Email verification: a random token, only its
  **hash** stored, 24h TTL; raw token emailed via the `Mailer`. Signin **rejects
  unverified** accounts.
- Session: opaque random token in an httpOnly/Secure/SameSite=Lax cookie; the hash
  is looked up in `sessions` per request; logout deletes the row (revocable, no JWT).
- CSRF: SameSite=Lax + a double-submit token on state-changing POSTs. Rate-limit
  signup/signin/verify per-IP and per-email.
- Classes + join codes in P2 are **admin-seeded out-of-band** (seed script / SQL) —
  no teacher UI yet. Rich org management (teacher/admin login, class creation,
  rosters, school aggregation) is **P3**.

## 7. Evaluation (sync → async)

> **The evaluation data model is provisional.** "Tree of nodes" vs "flat scored
> items" vs the current SOLO rubric are materially different shapes; we deliberately
> do **not** lock one in. A dedicated evaluation-design brainstorm precedes any
> further investment. P1 keeps the existing, working SOLO-rubric contract
> (`scores[]` + narrative).

- **P1:** `POST /tasks/:id/evaluate` runs the SOLO-rubric evaluation **inline**
  (flagship model, **never downgraded**) and writes the `evaluations` row.
- **P4:** the same endpoint **enqueues a `river` job** and returns `{status:"queued"}`;
  a worker runs the eval (`queued→running→done/failed`); the frontend polls
  `GET …/evaluation`. River provides retries + idempotency. The evaluation contract
  (scores + narrative JSONB) is identical in both, so this is a call-site swap.

## 8. Frontend integration

`apps/web` becomes a pure renderer + API client:
- Delete the `localStorage` store (`mk.store`) and the browser LLM adapters
  (`anthropicAdapter`/`openaiAdapter`, `mk.llmConfig`). No secrets, no model calls
  in the browser.
- New `apiClient` module over the REST endpoints; a `useTurnStream` hook consumes
  the `/turn` SSE (`text` → prose, `card` → open the sheet).
- `CardSheetHost`, the renderers, and the teaching modals are **untouched** — they
  already speak the standard envelope; they submit via `PUT`/skip-`POST` instead of
  writing to the local store.
- Auth screens (signup with **join code**, email-verify landing, signin) replace the
  mock `AuthScreen`.

## 9. Phasing

Each phase is its own spec → plan → build and leaves the gate green.

| Phase | Delivers | End state |
|---|---|---|
| **P1** | Go service, Postgres+goose, envelope persistence, smart gateway + SSE, ported prompt/refeed/loop, per-turn usage on `messages`, seeded school+class+student, `HasEntitlement` stub, **inline** eval; frontend rewired | Today's app, real backend, one mock user |
| **P2** | Signup/verify/signin, argon2id, sessions, `Mailer`, **minimal org** (admin-seeded schools/classes + join-code-gated signup, `enrollments`), per-user tasks | Real multi-user, every account org-bound |
| **P3** | Teacher/admin roles, class creation + roster management, school aggregation | Full org product |
| **P4** | `river` job + worker, eval enqueue + poll | Non-blocking evaluation |

Beyond P4 (out of scope here): object storage / OSS, multimodal, more cards, richer
evaluation — all build cleanly on this architecture.

## 10. Testing strategy

The gate stays green at every phase:
`pnpm -r typecheck && pnpm -r test && (cd apps/api && go vet ./... && go test ./...)`

- **Go:** table-driven + `httptest` for handlers; `testcontainers-go` (real Postgres)
  for the store/sqlc layer; the turn loop tested against a **stubbed provider**
  (deterministic, no live key).
- **Parity test:** the Go-ported refeed + prompt output must match the TS originals
  for the Phoebe scenario, so the port does not silently drift.
- **Contracts:** existing Zod tests stay.
- **Web:** Vitest; the envelope/renderer guardrail tests carry over unchanged.

## 11. Security constraints (binding)

- Secrets (LLM key, DB DSN, session secret, SMTP creds) live only in `apps/api`
  server-side config, loaded from env; never in git, logs, thrown/rendered errors,
  the data store, or eval payloads.
- The browser never holds a provider key and never calls a model directly.
- The flagship evaluation model is **never downgraded**.
- The pre-existing uncommitted `package.json` change is not part of this work and
  stays out of all commits.

## 12. AGENTS.md update

The current "明确不做（本期）" list forbids real auth, teacher concepts, and the
client→model proxy. This refactor deliberately moves those into scope. AGENTS.md
must be rewritten to reflect the new phase while preserving the 四条设计铁律, the
envelope law, and the new organization invariant.
