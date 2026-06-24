# Database Schema

> **Reference doc.** The authoritative Postgres schema for the 思维印记 backend
> platform. Authored 2026-06-24. Companion to the
> [architecture spec](../superpowers/specs/2026-06-24-backend-platform-architecture-design.md),
> [api-design.md](./api-design.md), and [chat-history-storage.md](./chat-history-storage.md).
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

Two distinct concepts that were once conflated, now kept separate:

- **Structural belonging** — which school / class a user is in. Recorded as real
  tables (`schools`, `classes`, `enrollments`, plus `users.school_id`).
- **Entitlement ("membership" = paid access)** — whether a user/org may spend
  tokens. **No table yet** (period-subscription vs token-balance is undecided). It
  is a backend seam, `HasEntitlement(ctx, user) → bool`, stubbed to `true`. See
  "Entitlement seam" below.

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
| `school_id` | uuid FK → `schools(id)` on delete restrict | a class belongs to one school |
| `name` | text | |
| `join_code` | text | **unique**; consumed at signup to bind a new user |
| `created_at` | timestamptz | |

Index: `(join_code)` unique. Index: `(school_id)`.

### `users`  `[P1 stub → P2 real]`
The account. P1 seeds exactly one `student`. P2 makes signup/verify real.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `email` | text | **unique**, citext-lowered at app layer |
| `email_verified_at` | timestamptz null | null = unverified; signin rejects null |
| `password_hash` | text | argon2id encoded string |
| `role` | text | `CHECK (role IN ('student','teacher','admin'))`, default `'student'` |
| `school_id` | uuid FK → `schools(id)` on delete restrict, **NOT NULL** | every account belongs to a school, incl. admins |
| `display_name` | text | |
| `avatar_color` | text | carried from the current mock session concept |
| `created_at` | timestamptz | |

Index: `(school_id)`.

**Organization invariant** (enforced in the signup transaction): every user has a
`school_id`; students and teachers additionally have ≥1 `enrollments` row. No code
path produces a user without a school. For students, signup binds school + class
atomically from the join code (`class.school_id` supplies the school).

### `enrollments`  `[P2 minimal → P3 rich]`
Join table binding a user to a class. (Renamed from the earlier "memberships" — that
word is now reserved for the paid-entitlement concept, which is *not* a table.) A
join table — not a column on `users` — faithfully records that a student maps to one
class while a teacher may map to several (P3); an admin maps to none.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `user_id` | uuid FK → `users(id)` on delete cascade | |
| `class_id` | uuid FK → `classes(id)` on delete restrict | |
| `role_in_class` | text | `CHECK (role_in_class IN ('student','teacher'))`, default `'student'` |
| `created_at` | timestamptz | |

Index: `(user_id)`. Index: `(class_id)`. Unique: `(user_id, class_id)`.
Consistency rule (app-enforced): an enrollment's `class.school_id` must equal the
user's `school_id`.

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

### Entitlement seam (no table yet)
The paid-access concept ("membership") is deliberately **not** modeled as data yet.
A single backend function is the seam where billing later plugs in:

```go
// entitlement.go — the one place future billing reads from.
func (s *Service) HasEntitlement(ctx context.Context, u *User) (bool, error) {
    return true, nil // stub: open for everyone for now
}
```

Checked at the two token-spending gates: before `POST /tasks/:id/turn` and before
`POST /tasks/:id/evaluate`. When billing is designed, this reads a
period-subscription or token-balance model (likely keyed by school/org) without
changing any call site — mirrors the gateway's `keyResolver` seam.

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

### `messages`  `[P1]` — the conversation + per-turn usage
Row-per-message, append-only (see [chat-history-storage.md](./chat-history-storage.md)
for the rationale and growth analysis). One model call == one assistant message, so
token usage lives **on the assistant row** rather than in a separate ledger table.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `task_id` | uuid FK → `tasks(id)` on delete cascade | |
| `role` | text | `CHECK (role IN ('user','assistant','system','summary'))` |
| `content` | text | lz4-compressed by TOAST for long bodies |
| `tool_call` | jsonb null | the `summon_card` call args + tool_call id, when present |
| `provider` | text null | set on assistant rows produced by a model call |
| `model` | text null | " |
| `tier` | text null | chaperone vs flagship |
| `prompt_tokens` | integer null | " |
| `completion_tokens` | integer null | " |
| `cost_estimate` | numeric(12,6) null | provider-priced estimate |
| `created_at` | timestamptz | |

Index: `(task_id, created_at, id)` — covers "load a task's history in order" and
keyset pagination. **Not partitioned now**; if/when the table nears ~50–100M rows,
migrate to RANGE partitioning on `created_at` (online, via pg_partman). The
`'summary'` role is reserved for the future running-summary / context-compaction
pattern; nothing writes it in P1.

### `card_instances`  `[P1]` — the standard envelope
The load-bearing floor. Stored as columns for the outer envelope + JSONB for the
inner contract. **Unchanged** by the chat-history and usage-merge decisions.

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

### `evaluations`  `[P1 sync → P4 async]` — **PROVISIONAL**
> ⚠️ The evaluation **data model is provisional** and intentionally minimal. The
> richer shape (a node tree? flat scored items? the current SOLO rubric?) gets a
> dedicated design pass before further investment. P1 keeps the existing rubric
> contract (`scores[]` + narrative), which already works end-to-end.

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `task_id` | uuid FK → `tasks(id)` on delete cascade | |
| `scores` | jsonb | array of `{dim_id, level, note}` (current SOLO rubric) |
| `narrative` | text | "你的思维印记" |
| `model` | text | the flagship model used (never downgraded) |
| `tier` | text | flagship |
| `prompt_tokens` | integer null | costed call usage (usage lives here, not in a ledger) |
| `completion_tokens` | integer null | " |
| `cost_estimate` | numeric(12,6) null | " |
| `status` | text | `CHECK (status IN ('queued','running','done','failed'))` |
| `error` | text null | failure detail (server-side only; never leaked to client) |
| `created_at` | timestamptz | |
| `completed_at` | timestamptz null | |

Index: `(task_id, created_at desc)`. In P1 rows are written directly as `done`;
in P4 they transition `queued→running→done/failed` under a river job.

---

## Cost rollups (no `llm_calls` table)

There is **no separate `llm_calls` ledger**. Per-turn usage lives on `messages`
(assistant rows) and per-eval usage on `evaluations`. For per-organization cost
rollups, define a read-only view that unions both:

```sql
CREATE VIEW llm_usage AS
  SELECT m.id, t.user_id, u.school_id, 'chat'::text AS kind,
         m.provider, m.model, m.tier,
         m.prompt_tokens, m.completion_tokens, m.cost_estimate, m.created_at
    FROM messages m JOIN tasks t ON t.id = m.task_id JOIN users u ON u.id = t.user_id
   WHERE m.role = 'assistant' AND m.model IS NOT NULL
  UNION ALL
  SELECT e.id, t.user_id, u.school_id, 'eval'::text AS kind,
         NULL, e.model, e.tier,
         e.prompt_tokens, e.completion_tokens, e.cost_estimate, e.created_at
    FROM evaluations e JOIN tasks t ON t.id = e.task_id JOIN users u ON u.id = t.user_id;
```

If real billing later needs an immutable, queryable ledger, adding a dedicated
table is a purely additive migration (no backfill of historical rows required).

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
- P1 creates `schools`, `classes`, `users` (seeded single student), `enrollments`,
  `tasks`, `messages`, `card_instances`, `evaluations` — plus a seed of one school +
  one class + the student enrolled in it, so the seeded student is org-bound from
  day one (honoring the invariant before P2's real signup).
- P2 adds `email_verification_tokens`, `sessions`, and the join-code signup flow.
- `river` creates its own job tables via its migration tooling in P4.
