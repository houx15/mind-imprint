# S1 · One continuous session + projection + compact + summary-on-return

> Build spec for **S1** of the multi-agent architecture (`2026-07-29-multi-agent-architecture-design.md` §5, §11).
> Goal: the four-room coach stops being amnesiac. There is **one agent, one continuous per-project session**; surfaces are views; the agent keeps context lean by **compaction** (fold-on-solidify). Plus **summary-on-return** when a project re-opens.

## 0. What exists (verified) — and the decision

- A **per-project chat thread already exists**: `chat_thread.seeded_project_id` + `chat_message(thread_id, role, content, modality, created_at)`, resolved by `getOrCreateThread` (`agentstore.go`). It is **orphaned** — the live four-room UI uses the **stateless `/coach`** endpoint (`coach.go`) which persists nothing, reloads nothing, and picks a system prompt by `scope`.
- Two mature coach runtimes exist:
  - **Intervention loop** (`RunAgentStep`, `postProjectTurn`) — silence-by-default, graph-nudge + card-summon. **Not** adopted by S1: a room chat must always reply. Left intact for **S4** (cross-phase card proposing).
  - **Conversational coach** (`ProposeChatReply` / `BuildChatContext`, standalone 聊天 tab) — **always replies**, has history + a compact context summary, persists both sides. **This is the S1 primitive**, lifted from per-user-thread to **per-project**.

**Decision:** S1 = *lift the always-reply conversational coach to the one per-project session*, feed it a **spine projection** (not a URL list), **surface-tag** every turn, and **fold** shaping turns on solidify. Card offers stay **off** (S4). No SSE rewrite — keep the JSON `{reply}` shape.

## 1. Data (migration 0038)

- `ALTER TABLE chat_message ADD COLUMN surface text` — the active surface a turn happened on (`plan|reading|writing|reflection|forming|…`). NULL = untagged/legacy. Display filters by it; **context never does** (continuity sees the whole thread).
- `ALTER TABLE chat_message ADD COLUMN folded_at timestamptz` — set when the turn has been folded into the spine (lever 1). NULL = live in the window. **Folded turns stay in the thread** (still shown on reload) but **leave the coach's context window**.
- `CREATE TABLE project_summary_prose (project_id uuid PK REFERENCES project ON DELETE CASCADE, prose text NOT NULL, created_at timestamptz DEFAULT now())` — first-open-wins, exactly like `project_mirror_prose` / `parent_report_prose`.

## 2. Backend

### 2a. Continuous coach (`coach.go` rewrite; S1.2)
`POST /projects/{id}/coach` keeps its `{scope, user_input}` in / `{reply}` out shape, but:
1. resolve the per-project thread (`getOrCreateThread`);
2. persist the student turn → `chat_message(role=user, surface=scope)`;
3. load the **active window** (non-folded, last N, both roles) via new `ListActiveChatMessagesByProject`;
4. build the **spine projection** (2c) + name the active surface;
5. `ProposeChatReply(history, projection, "" /*cardSummary*/, "" /*flag: card offers OFF*/)` — always replies;
6. persist the assistant turn → `chat_message(role=assistant, surface=scope)`;
7. meter (`RecordLLMCall`, Purpose="coach"), append the `coach_turn` event (unchanged), return `{reply}`.

Posture: reuse `chatCoachPosturePrompt` (already forbids writing the assessed deliverable). The old per-`scope` prompt specialization is replaced by **surface-in-projection** ("学生当前在『写作』房间") — one agent, tuned by the projection, not four prompts.

### 2b. History load (S1.2)
New query `ListActiveChatMessagesByProject` = `ListChatMessagesByProject` + `AND cm.folded_at IS NULL`, ordered, capped in Go to last N (12). A separate `ListChatMessagesByProjectAll` (folded included) backs the display GET (2e).

### 2c. Spine projection (`internal/studio` or a new `projectcoach` helper; S1.3)
A compact, always-on text projection from the read model's `ProjectData` (`projection.go`): proposal summary (4 dims, one line each) · plan status (counts by col + milestones once they exist) · reading-list index (title + one-line takeaway/decision) · outline skeleton (top nodes) · recent activity (last few `activity_log_entry`). Deep detail is deferred to an on-demand skill (later). This is D2 — generous always-on projection.

### 2d. Fold-on-solidify (S1.4)
New store method `FoldChatSurfaceBefore(projectID, surface, before time.Time)` → `UPDATE chat_message SET folded_at=now() WHERE thread=... AND surface=$surface AND folded_at IS NULL AND created_at < $before`. Called:
- on **proposal finalize** (`putProposal`, `workspace.go`) → fold `surface IN ('forming','proposal_review')` turns;
- on **plan generate** (`generatePlan`) → fold `surface='plan'`... (only the pre-plan shaping turns; keep it conservative — fold up to the solidify moment).
The shaping dialogue is now durable structure (the proposal/plan rows); its raw turns leave the window. Nothing is lost — they remain in the thread and the spine carries the result.

### 2e. Summary-on-return (S1.5)
Mirror the `project_mirror_prose` pattern exactly:
- `GET /projects/{id}/summary` → deterministic spine snapshot (live) + stored prose (nil until composed). No spend.
- `POST /projects/{id}/summary` → if a row exists, return it (no spend); else compose once via **flagship** (`EvalResolver`) from the projection (2c), record LLM cost even on failure, `INSERT ... ON CONFLICT DO NOTHING`, re-read the winner. A failed compose is not persisted (retries next open).

## 3. Frontend (S1.6)

- New api client: `getCoachHistory(projectId, surface)` (display slice) + `getProjectSummary` / `postProjectSummary`.
- Each room, on mount for a project, **loads its surface-slice** of the one thread (replaces the hard-coded greeting-only ephemeral `chat`), then appends turns as today (still JSON `{reply}`). One thread, surface-tagged display (D1).
- On entering a project (Directory → room shell in `WorkspaceContainer`), fetch summary-on-return; show the compact paragraph (compose-on-first-open via POST).
- Keep `ChatMsg` shape; no SSE.

## 4. Tests + deploy (S1.7)

- Go: new store methods + coach rewrite + fold + summary handlers (api package, testcontainers); projection builder unit tests.
- contracts: any new response types (summary, coach history).
- web: room history-load + summary-on-return render.
- Full suites (contracts, `go ./...`, web), then deploy (0038 applies) + smoke.

## 5. Explicitly deferred (not S1)

- Card offers / cross-phase card proposing → **S4**.
- Size-threshold `conversation digest` backstop → **S4** (S1 folds on solidify only).
- Reading room as a formal brief-in/takeaways-out sub-agent → **S2**.
- New-session triggers beyond "one project = one session" → later.
- Retiring `RunAgentStep`/intervention duplication → not now; it's S4's substrate.
