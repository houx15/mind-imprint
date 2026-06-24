# Database Schema

> **Reference doc.** The authoritative Postgres schema for the 思维印记 backend
> platform. Authored 2026-06-24. Companion to the
> [architecture spec](../superpowers/specs/2026-06-24-backend-platform-architecture-design.md)
> and [api-design.md](./api-design.md).
>
> **Engine:** PostgreSQL. **Access:** `pgx/v5` + `pgxpool`, queries via **sqlc**,
> migrations via **goose** (`apps/api/migrations/`).
>
> **Phase tags** (`[P1]`…`[P4]`) mark which phase creates the table. The schema is
> designed in full now so later phases never reshape it.

## Conventions

- Primary keys: `id uuid` default `gen_random_uuid()` (pgcrypto) unless noted.
- Timestamps: `timestamptz`, UTC. `created_at` defaults to `now()`.
- Soft references that may be absent use nullable FKs; hard relationships use
  `not null` + `on delete` policy stated per table.
- Enums are Postgres `text` columns with a `CHECK` constraint (simpler migrations
  than native enum types; sqlc maps them as strings).
- JSONB columns hold contract-shaped data validated at the application edge; see
  "The envelope contract" below.

---

## Identity & organization

### `users`  `[P1 stub → P2 real]`
The account. P1 seeds exactly one `student`. P2 makes signup/verify real.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `email` | text | **unique**, citext-lowered at app layer |
| `email_verified_at` | timestamptz null | null = unverified; signin rejects null |
| `password_hash` | text | argon2id encoded string |
| `role` | text | `CHECK (role IN ('student','teacher','admin'))`, default `'student'` |
| `display_name` | text | |
| `avatar_color` | text | carried from the current mock session concept |
| `created_at` | timestamptz | |

Invariant (enforced in the signup transaction, not by a column): every user has at
least one `memberships` row. See `memberships`.

### `email_verification_tokens`  `[P2]`

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → `users(id)` on delete cascade | |
| `token_hash` | text | SHA-256 of the raw token; **raw token never stored** |
| `expires_at` | timestamptz | 24h TTL |
| `consumed_at` | timestamptz null | set on successful verify; single-use |
| `created_at` | timestamptz | |

Index: `(token_hash)` unique. Index: `(user_id)`.

### `sessions`  `[P2]`
Server-side, revocable sessions. The cookie carries an opaque random token; the
server looks up its hash here every request.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → `users(id)` on delete cascade | |
| `token_hash` | text | SHA-256 of the cookie token |
| `expires_at` | timestamptz | sliding or fixed TTL |
| `user_agent` | text null | audit |
| `ip` | inet null | audit |
| `created_at` | timestamptz | |

Index: `(token_hash)` unique. Index: `(user_id)`.

### `schools`  `[P2 minimal → P3 rich]`

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `name` | text | |
| `created_at` | timestamptz | |

### `classes`  `[P2 minimal → P3 rich]`

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `school_id` | uuid FK → `schools(id)` on delete restrict | |
| `name` | text | |
| `join_code` | text | **unique**; consumed at signup to bind a student |
| `created_at` | timestamptz | |

Index: `(join_code)` unique. Index: `(school_id)`.

### `memberships`  `[P2 minimal → P3 rich]`
Join table binding a user to a class. A join table (not a column on `users`) keeps
the model flexible: a student maps to a class; a teacher may map to several. The
**organization invariant** ("no orgless account") is enforced by always creating a
membership inside the signup transaction.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → `users(id)` on delete cascade | |
| `class_id` | uuid FK → `classes(id)` on delete restrict | |
| `role_in_class` | text | `CHECK (role_in_class IN ('student','teacher'))`, default `'student'` |
| `created_at` | timestamptz | |

Index: `(user_id)`. Index: `(class_id)`. Unique: `(user_id, class_id)`.

---

## Core domain

### `tasks`  `[P1]`
A student's piece of work (their real task brought into the platform).

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → `users(id)` on delete cascade | owner |
| `title` | text | |
| `seed` | text null | pasted article link / starting context |
| `status` | text | `CHECK (status IN ('active','evaluated'))`, default `'active'` |
| `created_at` | timestamptz | |
| `last_active_at` | timestamptz | |

Index: `(user_id, last_active_at desc)`.

### `messages`  `[P1]`
The chaperone conversation transcript.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `task_id` | uuid FK → `tasks(id)` on delete cascade | |
| `role` | text | `CHECK (role IN ('user','assistant','system'))` |
| `content` | text | |
| `tool_call` | jsonb null | the `summon_card` call args + tool_call id, when present |
| `created_at` | timestamptz | |

Index: `(task_id, created_at)`.

### `card_instances`  `[P1]` — the standard envelope
The load-bearing floor. Stored as columns for the outer envelope + JSONB for the
inner contract.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `card_id` | text | references a card JSON spec id (not an FK; specs are file-embedded) |
| `task_id` | uuid FK → `tasks(id)` on delete cascade | |
| `parent_node_id` | uuid null | self-reference forming the process tree |
| `status` | text | `CHECK (status IN ('proposed','active','completed','skipped'))` |
| `field_values` | jsonb | nested by `step.key` then `field.key`; default `'{}'` |
| `event_trace` | jsonb | array of trace events; default `'[]'` |
| `rubric_tags` | text[] | |
| `created_at` | timestamptz | |
| `completed_at` | timestamptz null | |

Index: `(task_id, created_at)`. Index: `(parent_node_id)`.

### `evaluations`  `[P1 sync → P4 async]`

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `task_id` | uuid FK → `tasks(id)` on delete cascade | |
| `scores` | jsonb | array of `{dim_id, level, note}` |
| `narrative` | text | "你的思维印记" |
| `model` | text | the flagship model used (never downgraded) |
| `status` | text | `CHECK (status IN ('queued','running','done','failed'))` |
| `error` | text null | failure detail (server-side only; never leaked to client) |
| `created_at` | timestamptz | |
| `completed_at` | timestamptz null | |

Index: `(task_id, created_at desc)`. In P1 rows are written directly as `done`;
in P4 they transition `queued→running→done/failed` under a river job.

### `llm_calls`  `[P1]` — the cost spine
One row per model call (chat turn or eval). The usage ledger and the future
per-organization billing read model.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → `users(id)` null on delete set null | |
| `task_id` | uuid FK → `tasks(id)` null on delete set null | |
| `org_id` | uuid null | class/school attribution; written by the keyResolver seam |
| `kind` | text | `CHECK (kind IN ('chat','eval'))` |
| `provider` | text | `'deepseek'` / `'anthropic'` |
| `model` | text | |
| `tier` | text | chaperone vs flagship |
| `prompt_tokens` | integer | |
| `completion_tokens` | integer | |
| `cost_estimate` | numeric(12,6) | provider-priced estimate |
| `created_at` | timestamptz | |

Index: `(user_id, created_at desc)`. Index: `(org_id, created_at desc)`.

---

## The envelope contract (JSONB validation policy)

`card_instances.field_values` and `event_trace` are stored as JSONB and validated
**at the edge only**:

- Go validates the **outer** envelope on write: `status` is a legal enum, required
  ids are present and well-formed, `field_values` is an object, `event_trace` is an
  array of objects with a known `kind`.
- The **deep** shape of `field_values` (keyed by `step.key` → `field.key`) and each
  `TraceEvent` variant remains owned by the Zod contracts in `packages/contracts`
  (`envelope.ts`). The backend is a faithful pass-through for the inner structure.

This is what lets "new card = new JSON, no schema/column change" hold on the
backend as well as the frontend.

`TraceEvent` kinds (unchanged from the current contract): `field_change`,
`step_expand`, `note_open`, `skip`, `submit`.

---

## Process tree

There is **no `process_node` table**. The right-hand process tree is a read-time
projection built from `card_instances.parent_node_id` (self-referential) plus the
`messages` timeline for a task. This matches the current implementation and avoids
a redundant denormalized store.

---

## Migrations

- Tool: **goose**, SQL migrations under `apps/api/migrations/`.
- Each phase adds only its tagged tables; P1 creates `users` (seeded single
  student), `tasks`, `messages`, `card_instances`, `evaluations`, `llm_calls`, plus
  a minimal `schools`/`classes`/`memberships` seed so the seeded student is
  org-bound from day one (honoring the invariant even before P2's real signup).
- `river` creates its own job tables via its migration tooling in P4.
