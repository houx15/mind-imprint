# Slice 11 — Chat surface + coach-alone + card-as-offer (keystone) · Design

> Whole-product refactor #2. Builds on Slices 0–10. One superpowers loop:
> this spec → plan → subagent-driven TDD → whole-branch review → merge.

**Authority:** agent-spec §5.4 (Chat policy) + §5.6 (normative Chat trace) are the
technical source of truth; product-spec §2.2 + DEC-12 govern the surface, posture,
and the on-record disclosure; the Claude Design `docs/design/思维印记_工作区.dc.html`
lines 527–666 are the **binding** Chat UI. Where the old dc.html memory note calls the
chat workspace "stale", the CHAT block at 527–666 is the current binding surface and is
followed here.

---

## 1. Goal & scope

Make the free **Chat (聊天)** surface real: a standalone top-level tab where a student
converses with the coach **alone** (planner off), the coach **replies** in a guiding-not-
answering posture, and — when the student pastes a link — the classifier raises a
card-moment and the coach **offers a source-evaluation card (CRAAP) in-thread**, capped at
I3, reading as an offer not an interruption. Accepting opens the live annotate card runtime
in the chat bubble; completing it updates `card_competence` globally. Chat events enter the
event stream at supplementary weight, and the surface carries the binding on-record
disclosure.

This is the **keystone** — the thinnest honest vertical slice that reproduces the agent-spec
§5.6 Chat trace end-to-end. It deliberately defers the rest of Slice 11 (§9).

### The normative trace this keystone reproduces (agent-spec §5.6)

```
student: pastes news link + "this basically proves my point"   → T-A
classifier: URL detected → thread material minted;
            card_moment(source_evaluation) flag (CRAAP, I3)
coach (ctx: conversation + thread graph + flag)
     → reply  (engages the point, guiding-not-answering)
     → surface_card(craap, material, I3)  as an offer, in-thread
student accepts → annotate primitive opens in the chat bubble
card completes → card_instance persisted (status=completed); chat events → stream
```

No project, no gates, no planner, no anchored intervention. It reuses the CRAAP card
runtime, enforcement, and the annotate primitive — thread-scoped.

> The agent-spec §5.6 trace ends "competence updated globally." `card_competence`
> is a **dormant table with no update site anywhere in the platform today** (the
> Studio does not write it either). Wiring competence is a separate cross-cutting
> feature, not part of this keystone — it is deferred (§9), and the in-thread card
> behaves exactly as the Studio card does today: it persists its answers and flips
> to `completed`, no more.

---

## 2. Decisions (locked in brainstorming)

- **DEC-11.1 — Keystone = coach + card-as-offer.** Include the thread-scoped graph
  (paste → material), classifier card-moment detection (link → CRAAP), `surface_card`@I3
  in-thread, and the in-thread card runtime (persist answers + flip to `completed`).
  Defer multimodal, chat→project seeding, off-record control, semantic non-link
  moments, competence wiring, and thread graph_effects/evidence nodes.
- **DEC-11.2 — Thread graph via additive `thread_id` scope.** Add nullable `thread_id`
  to the two tables Chat's card runtime writes (`material`, `card_instances`) with a
  scope CHECK, exactly as Slice 10 (migration 0021) added project scope to
  `evaluations`. Chat reuses the SAME card runtime + enforcement, just thread-scoped.
  Thread and project stay clean siblings (so deferred project-seeding remains a real
  copy operation, not an in-place reinterpretation). `intervention.thread_id` is NOT
  added — the keystone writes no thread-scoped interventions (the coach `reply` is a
  chat_message, not an anchored intervention; the CRAAP card completes without one).
  Defer `graph_node`/`graph_edge` thread-scoping (thread evidence nodes).
- **DEC-11.3 — `reply` becomes a typed output.** `reply` is already in the C3 `Verb`
  enum but absent from the `AgentOutput` union. Add `{ type: "reply", body }`
  (anchor-free) to the Zod union + the Go mirror + `ValidateOutput`. Banned-phrasing
  runs on the reply body; echo `OutputCheck` is a no-op in chat (no draft).
- **DEC-11.4 — Composer multimodal deferred by inert rendering.** Render the composer
  per the binding design including the attach/image/voice icons, but wire only text
  send; the three multimodal controls are inert placeholders this slice.

---

## 3. Data model — migration `0022_chat_thread_scope.sql` (additive)

Additive only. Project/task rows and their queries are untouched.

```sql
-- +goose Up
-- Slice 11: give Chat's card runtime a thread-scoped home. Additive, mirrors
-- 0021's evaluations scope pattern. A material / card_instance belongs to
-- exactly one owner: a project, a chat thread, or a legacy task.

ALTER TABLE material       ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;
ALTER TABLE card_instances ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;

CREATE INDEX material_thread_created_idx       ON material (thread_id, created_at);
CREATE INDEX card_instances_thread_created_idx ON card_instances (thread_id, created_at);

-- Scope discipline: at least one owner set. Existing rows (project_id or task_id
-- set, thread_id NULL) satisfy these unchanged; a chat row sets thread_id only.
ALTER TABLE material       ADD CONSTRAINT material_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
ALTER TABLE card_instances ADD CONSTRAINT card_instances_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
-- +goose Down
-- (drop the two CHECKs, the two indexes, and the two columns)
```

> `material.task_id`/`project_id` and `card_instances.task_id`/`project_id` are all
> nullable as of migrations 0016/0020, so a thread-only row (both NULL, `thread_id`
> set) is already legal against existing FKs — only the new CHECK governs it. The
> plan's first step verifies these nullabilities against 0016/0020/0021 before
> writing the ALTERs. `intervention` is intentionally NOT touched (DEC-11.2).

New standalone-thread queries (`apps/api/internal/store/queries/chat.sql`, additive —
existing project-keyed queries stay for the Studio coach):

- `ListThreadsByUser :many` — `WHERE user_id = $1 ORDER BY created_at DESC`.
- `CreateStandaloneThread :one` — `INSERT (user_id, title) ... RETURNING *`
  (seeded_project_id NULL).
- `GetThread :one` — by id (handler checks `user_id` ownership).
- `ListMessagesByThread :many` — `WHERE thread_id = $1 ORDER BY created_at, id`.
- The existing `CreateChatMessage :one` query is **already thread-keyed** (inserts by
  `thread_id`); only the agentstore *wrapper* is project-keyed. The chat handler calls the
  sqlc query directly with the thread id — no new insert query needed.
- `ListMaterialsByThread` / `CreateThreadMaterial` / thread card_instance reads + the thin
  completion update, as the card runtime needs them (plan pins exact shapes against the
  existing project queries).

---

## 4. The chat turn loop — `RunChatStep` (agent package, new)

`RunAgentStep` is coupled to the project graph (loads graph, builds gate/card candidates
from it). Chat gets its own small step. Pure/injectable seams mirror the studio loop.

**4.1 Classifier (pure, no model).** `DetectChatMoments(message string, threadMaterials) ...`:
- Extract the first URL from the student message (a keyless regex; no fetch — the material
  is `source='pasted'`, blocks empty, title = host). Mint a thread `material`
  (kind=`article`).
- Run the existing `SurfaceCardCandidates`-style predicate over the thread's materials to
  yield a CRAAP `surface_card` candidate, **forced to Level I3**. Suppressed when the
  material already has a card_instance `proposed`/`active`/`completed` OR `skipped`
  (a declined offer is not re-raised in the same thread).

**4.2 Coach (flagship, model, never downgraded).** New `BuildChatContext(history,
threadGraph, flag)` — the chat-variant coach prompt:
- Posture: guiding-not-answering, **one notch more permissive** than Studio (may explain
  and inform, scoped — product §2.2), one question at a time, restraint ladder.
- Context recipe (agent-spec §5.5): conversation history + thread graph summary + the
  card-moment flag + per-card competence.
- Emits a **`reply`** typed output (`{ type:"reply", body }`).

**4.3 Emit (one SSE stream, mirrors `studioturn.go` + `streamAction`).**
1. The handler persists the student message and appends one `prompt_sent` event
   (`surface='chat'`, thread-scoped) BEFORE the step — the student's turn is the event,
   exactly as `postProjectTurn` does for the studio.
2. Coach call → `reply`. Meter via `RecordLLMCall{Surface:"chat", Purpose:"coach"}`
   whenever the call reported usage — **including on enforcement reject** (a rejected
   reply still cost money). On accept: banned-phrasing check → persist as an `assistant`
   chat_message → stream the reply. On reject: log server-side, stay silent (no message
   emitted).
3. If a fresh card candidate exists: `SurfaceCardThread` mints a `proposed` card_instance
   (thread-scoped) + streams the offer envelope (same shape studio uses). Both reply and
   offer ride the one stream.

**Coach discretion over the offer** (agent-spec: "the coach decides whether to act") is
approximated in the keystone by the in-flight + decline suppression above; explicit
coach-suppression of a flagged offer is deferred. Noted as a keystone simplification.

---

## 5. The `reply` typed output (C3 contract change)

`packages/contracts/src/agentOutput.ts`: add a sixth member to the `AgentOutput`
discriminated union:

```ts
z.object({ type: z.literal("reply"), body: z.string().min(1) }),
```

`apps/api/internal/agent/enforcement/enforcement.go`: add `"reply": true` to
`validOutputTypes` and a `case "reply"` in `ValidateOutput` requiring a non-empty `body`
and **no** anchor/criterion. Banned-phrasing (`BannedPhrasing`, incl. the
`rewritten-sentence-zh` ghostwriting rule) runs on the reply body. The existing
five types are unchanged; `rubric.test.ts`-style contract tests stay green.

---

## 6. Endpoints, DTO, client

**Endpoints** (`apps/api/internal/api/`, registered `protected(...)`):
- `GET /api/v1/chat/threads` → `ListThreadsByUser` → `[]ChatThreadDTO`.
- `POST /api/v1/chat/threads` → `CreateStandaloneThread` → `ChatThreadDTO`.
- `GET /api/v1/chat/threads/{id}/messages` → ownership check → `[]ChatMessageDTO`.
- `POST /api/v1/chat/threads/{id}/turn` (SSE) → ownership + `HasEntitlement` gate →
  persist student message → `RunChatStep` → stream reply (+ optional offer) →
  `RecordLLMCall` even on reject. Mirrors `postProjectTurn`.
- In-thread card **submit**: a **thin thread-scoped completion path**, NOT a reuse of
  `submitProjectCard`. That handler is project-coupled — it runs `CompleteCard` (which mints
  project `graph_effects`/evidence nodes) and refeeds a project `RunAgentStep`, both of which
  the keystone defers for threads. The chat submit instead: validates the field values +
  anchors (reuse `validateFieldValues`/`validateAnchors`), persists `field_values` +
  `event_trace` on the thread card_instance, and flips its status to `completed` — no
  graph_effects, no refeed, no competence write (all deferred). Ownership is by thread.

**DTOs** (`apps/api/internal/api/` or `studio` mirror + Zod in contracts, parity test):
- `ChatThreadDTO { id, title, createdAt }`
- `ChatMessageDTO { id, role, content, modality, createdAt }` (+ an optional `card`
  offer field on assistant messages carrying the surfaced card_instance id + spec id,
  so the client renders the in-thread offer).

**Client** (`apps/web/src/api/chat.ts` + barrel): `listThreads`, `createThread`,
`getMessages`, and the SSE `chatTurn` consumer (reuse the studio SSE client seam).

---

## 7. The Chat surface UI — binding dc.html 527–666

New top-level **聊天** tab. The rail slot exists (LeftRail comment: "聊天 … its surface is
Slice 11"); add the `chat` tab key, the binding chat SVG icon (dc.html line 117), and
mount a `ChatSurface` in `StudentApp`.

Three columns (rail is column 0):
1. **History sidebar** (288px, `#fff`): `新对话` primary button + `对话历史` list of the
   user's threads (title · preview · time), click to open. Binding styles from 527–549.
2. **Thread column** (`#F3F4F8`):
   - **Header** (60px): Bean avatar + active thread title + subtitle
     **「自由对话 · AI 只提问，不替你下结论」** + the binding green disclosure pill
     **「计入成长评估」** with tooltip **「这些对话会成为你成长评估的一部分」** (553–565).
   - **Chat log** (max-width 760px): message rows — AI messages carry the Bean avatar
     (left), student messages right-aligned; bubbles per 567–612. The **card offer**
     renders in-thread inside the AI message column (the `hasCard` block 589–608): a
     white card with the tag/title/desc and numbered steps; a primary **接受** action opens
     the live annotate card runtime in place (the existing card component), and a
     dismiss/skip marks the card_instance `skipped` (suppresses re-offer).
   - **Composer** (615–663): text `textarea` with the binding placeholder
     「把你正在想的、卡住的、好奇的，说给它听……」 + send button (wired); the attach/image/voice
     icons render per design but are **inert** (multimodal deferred). Binding footer
     disclosure **「AI 会陪你把想法想深，但不替你得出结论 · 你的对话只属于你」** (662).

All icons inline SVG (no lucide-react). Empty state (no threads / new thread) is honest —
no fabricated history.

---

## 8. Evidence & red lines

- Chat events (`prompt_sent`, card completion) enter the append-only `event` stream with
  `surface='chat'`, `project_id` NULL, at **supplementary** weight — readable by a future
  assessor aggregation, **not** wired to scoring this slice (chat→assessment aggregation
  was deferred in Slice 10). The existing `AppendEvent` resolves `user_id` by loading the
  project, so it cannot record a project-less chat event; this slice adds a small
  **user-keyed event insert** (`InsertUserEvent(user_id, surface, type, payload)` with
  `project_id` NULL). No `event.thread_id` column is added — thread attribution on events
  is deferred with aggregation.
- The on-record **disclosure is mandatory** (product §2.2) and is the binding dc.html copy
  above — present on every thread.
- **RL-1 / RL-4 hold in Chat.** There is no write path to any student deliverable (Chat has
  none). The coach reply is guiding, never ghost-writing; banned-phrasing enforcement runs
  on every reply. `reply` outputs are anchor-free by type but still typed and validated.

---

## 9. Non-goals (deferred; recorded as carry-forwards)

- **Multimodal input** — file/image attach, voice→text (composer buttons render inert).
- **chat → project intake seeding** — `chat_thread.seeded_project_id` + the intake copy
  of thread fragments into a new project graph (open product Q §607: which fragments,
  who curates).
- **Off-record thread control** — student marking a thread off-record (open product Q §606).
- **Semantic non-link card-moments** — opinion→steelman, comparison→weighing-matrix; these
  need a model-based classifier, not the keystone's URL heuristic.
- **Thread evidence nodes & graph_effects** — `graph_node`/`graph_edge` thread-scoping,
  the CRAAP card's evidence-minting on completion, and chat→assessment aggregation (chat
  events are recorded at supplementary weight but not yet aggregated).
- **Competence wiring** — `card_competence` is a dormant table with no update site anywhere
  in the platform today; wiring it (shared Studio + Chat + Course) is its own feature.
- **Coach explicit offer-suppression** — beyond the in-flight/decline suppression.

---

## 10. Testing & gates

- Go: `CGO_ENABLED=0 go test -p 1 ./...` on a quiet Docker daemon; FULL packages for the
  gate (agent, api, store, enforcement, contracts sync). `make sqlc` after query changes.
- Migration `0022` proven back-compat: existing project/task rows still satisfy the scope
  CHECKs; a thread-scoped material/card_instance (both task_id/project_id NULL, thread_id
  set) is insertable; a row with all three owners NULL is rejected; the down migration
  reverses.
- `RunChatStep`: pure classifier (URL→material→CRAAP@I3, suppression on
  proposed/active/completed/skipped) tested without a model; coach reply path tested with a
  stub provider (accept + banned-phrasing reject → silence, metered on reject).
- `reply` typed output: `ValidateOutput` accepts a well-formed reply, rejects empty body;
  Zod union round-trips.
- User-keyed event insert: a chat `prompt_sent` event persists with `project_id` NULL,
  `user_id` set, `surface='chat'`.
- Thin thread card submit: persists field_values + flips status to `completed`; writes NO
  graph_node/graph_edge and no competence row (assert absence).
- Endpoints: ownership 404 on all thread routes; entitlement gate on `/turn`;
  `RecordLLMCall` recorded on reject (llm_call count assertion).
- Contracts + web suites green; tsc clean; DTO parity test.
- Whole-branch review (Opus) mandatory. Never `git add` untracked user files; named paths
  only. Pre-existing `M package.json` + untracked user docs/pngs are NOT ours.
