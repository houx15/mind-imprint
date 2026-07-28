# Slice 4 · Write room — SHARED SPEC

Room = `workspace/blocks/WritingBlock.tsx`. Design: PRD §4.3 + `apps/web/src/proto/blocks/WritingBlock.tsx`. Goal strip (thesis + link back to kickoff) · Outline tab (bullets ⇄ mind-map over the SAME data) · Write tab (one panel: write-here=buffer ⇄ upload) · AI rail (coach, writing scope — never writes the body).

## Endpoint contracts (`/api/v1/projects/{id}`)

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/outline` | — | `{nodes: OutlineNode[]}` ordered by position |
| PUT | `/outline` | `{nodes: [{id?,text,depth}]}` | `{nodes: OutlineNode[]}` (server assigns/keeps ids, sets position = array index; replaces the whole set) |
| GET | `/draft` | — | `{content: string}` (current edit_buffer, "" if none) |
| PUT | `/buffer` | `{content}` | 204 — **already exists** (`putEditBuffer`), reuse for autosave |

Coach (writing scope) already exists (`POST /coach {scope:"writing"}`).

## Backend (`apps/api`)
- `edit_buffer` already has `GetEditBuffer`/`UpsertEditBuffer` (`writing.sql`) + the `PUT /buffer` handler (`writing.go`). Reuse.
- sqlc `workspace.sql` (extend): `ListOutlineNodes` (by project, order position), `DeleteOutlineNodes` (by project), `CreateOutlineNode(project_id,text,depth,position)`. Run `make sqlc`.
- Handlers in `internal/api/workspace_write.go`:
  - `GET /outline` → nodes.
  - `PUT /outline`: in ONE transaction, delete all outline_node for the project, re-insert the posted array in order (position=index; ignore any incoming id and mint fresh, OR preserve — minting fresh is simplest and fine since the client re-reads). Return the fresh nodes. `appendAutoLog` lightly on first non-empty outline save (optional).
  - `GET /draft` → `{content}` via `GetEditBuffer` (empty string on no row).
- Register routes in `api.go`.
- Go tests: outline PUT (with nested depths) → GET round-trips + replace semantics (second PUT with fewer nodes shrinks the set); draft GET reflects a prior `PUT /buffer`; ownership 404.

## Frontend (`apps/web/src/workspace`)
- `api/workspace.ts` (extend): `getOutline(id)`, `putOutline(id, nodes: {id?,text,depth}[])`, `getDraft(id)`. For autosave reuse the existing `putBuffer` from `apps/web/src/api/writing.ts` (import it) — do NOT duplicate.
- `WorkspaceContainer.tsx`: pass `projectId`, `proposal`, and `onOpenRoom` down to `WritingBlock` (mirror what PlanBlock already receives).
- `blocks/WritingBlock.tsx` — replace mock with real data (keep prototype structure/classes, incl. the mind-map SVG layout):
  - **Goal strip**: show `proposal.objective`; 看开题 → `onOpenRoom("plan")`.
  - **Outline tab**: `getOutline` on mount → nodes; all edits (inline text, ← promote / → indent, + add, × delete) mutate a single local `nodes` state and **debounce ~700ms → `putOutline`** then reconcile ids from the response. The **mind-map** view renders the SAME `nodes` state as a tree (title as root) and its inline node edits write back to the same state (two-way synced by construction). Keep the proto's flat-with-depth model + clamp depth 0..2.
  - **Write tab**: `getDraft` on mount → textarea content; on change, **debounce ~800ms → `putBuffer(projectId, content)`**; keep the live word count. 我在别处写了 (upload): read `.md`/`.txt`/`.markdown` files client-side as text → set content + `putBuffer`; for `.docx`/`.pdf` (binary) show "已上传，正文解析稍后支持" and store the filename note (do not crash) — real parsing is a later concern.
  - **AI rail**: `coach(projectId, "writing", text)` (client fn exists). Card-summon hook: leave a clearly-commented stub where a future summoned thinking-card would mount (no behavior).
- Empty outline → a single blank editable row + "新增一条". Empty draft → placeholder.

## Acceptance
- `go build ./...` + new Go tests green. `pnpm --filter web build` + `pnpm --filter web test` green.
- Data-flow: edit the outline (bullets or mind-map), reload → persists; type in the draft, reload → persists; upload a .md → becomes the draft. Goal strip shows the real objective and 看开题 jumps to Project Management.
