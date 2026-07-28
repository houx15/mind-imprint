# Slice 2 · Project Management room — SHARED SPEC

Room = `workspace/blocks/PlanBlock.tsx`. Design: PRD §4.1 + `apps/web/src/proto/blocks/PlanBlock.tsx`. Two phases share one home: **forming** (coach chat + 4 editable proposal dims + generate/write-proposal) and **working** (kanban / gantt / activity-log, all editable & persisted).

## Endpoint contracts (all under `/api/v1/projects/{id}`, session + ownership required, 404 hides non-owned)

| Method | Path | Body | Response |
|---|---|---|---|
| PUT | `/proposal` | `{objective,reason,activities,resources}` | `{proposal:{...}}` (upsert) |
| GET | `/plan` | — | `{items: PlanItem[]}` ordered by (stage, position, start) |
| POST | `/plan/items` | `{title,tag,column,stage,refMaterialId?,start,days}` | `{item: PlanItem}` |
| PATCH | `/plan/items/{iid}` | partial any of `{title,tag,column,stage,start,days,position,refMaterialId}` | `{item: PlanItem}` |
| DELETE | `/plan/items/{iid}` | — | 204 |
| GET | `/log` | — | `{entries: LogEntry[]}` newest-last, `date`="MM-DD" |
| POST | `/log` | `{text}` | `{entry: LogEntry}` (source="me", entry_date=today) |
| POST | `/coach` | `{scope:"forming"\|"find_sources"\|"writing", user_input}` | `{reply: string}` — JSON, NOT SSE |

`PlanItem` wire shape = the contract (`{id,title,tag,column,stage,refMaterialId,start,days,position}`). Note DB column is `col`; map to JSON `column`.

## Backend (`apps/api`)
- Routes: register the above in `internal/api/api.go` (protected block). Spend endpoints (`/coach`) call `HasEntitlement` + `RecordLLMCall`; the rest are cheap CRUD (no LLM).
- sqlc `internal/store/queries/workspace.sql` (extend): `ListPlanItems`, `CreatePlanItem`, `GetPlanItem`, `UpdatePlanItem` (sets ALL columns by id+project_id — handler merges partial over the current row), `DeletePlanItem`; `ListActivityLog`, `CreateActivityLogEntry(project_id, entry_date, text, source)`. `UpsertProjectProposal` already exists.
- Handlers in a new `internal/api/workspace_plan.go` (proposal PUT, plan CRUD, log GET/POST) and `internal/api/coach.go`.
- **`appendAutoLog(ctx, q, projectID, text)`** helper (source='auto', entry_date=today) — export it for later slices; call it here on: proposal first-save and plan item created (keep light — 1 line each).
- **`/coach`** (`coach.go`): load owned project → `HasEntitlement` → resolve mid-tier via `ChatResolver` → build a restrained system prompt by scope (see below) → single provider completion (NON-streaming; reuse however the agent makes a one-shot model call — look at how `internal/agent` calls `deps.Provider`; a minimal `Complete`-style call with system+user messages) → `RecordLLMCall(surface="studio", purpose="coach")` BEFORE any bail → append `event{type:"coach_turn", payload:{scope,student:..,ai:..}}` → respond `{reply}`. On empty/error model output, still record cost, return a graceful short fallback reply (never 500 the student).
  - Scope prompts (Chinese, embody the four 铁律 — AI 克制, one question at a time, never gives answers, never writes for the student):
    - `forming`: 你是「印记」，陪学生想清楚一个研究项目的开题。一次只问一个问题，帮他把「目标/缘由/活动与时间/资源」四件事聊清楚；绝不替他定题、不给现成答案。回应简短。
    - `find_sources`: 你是「印记」，陪学生找资料。只给方向、关键词、可信度判断；绝不替他检索或提供来源链接。一次一个建议，简短。
    - `writing`: 你是「印记」，陪学生写作。聊提纲、挑逻辑、撞反例；绝不替他写正文、不给成段文字。一次一个问题，简短。
- Go tests (`internal/api/*_test.go`): proposal PUT round-trips; plan create→list→patch(move column, reschedule, resize)→delete; log post→list; ownership 404; coach returns a reply (can stub/mocked provider like existing turn tests do — see how studioturn_test injects a fake provider).

## Frontend (`apps/web/src/workspace`)
- Extend `api/workspace.ts` with typed client fns for every endpoint above (import types from `@mind-imprint/contracts`). Use the shared `apiFetch` from `apps/web/src/api/client.ts`.
- `WorkspaceContainer.tsx`: after `getWorkspace`, pass `projectId`, `proposal`, and a `refreshWorkspace()` down to `PlanBlock`. The rail title/qualification already come from the projection.
- `blocks/PlanBlock.tsx` — replace local mock state with real data:
  - **Proposal dims**: seed from `proposal` prop; edits debounced (~600ms) → `PUT /proposal`; coverage `x/4` computed from non-empty dims. `生成项目计划` → switch to working phase (persist nothing destructive; if proposal never saved, save first). `写开题报告（可选）` opens the writer (same 4 dims, §1–§4 labels) editing the SAME proposal, persisted; export button → structured `.md`/`.csv` for now (real `.docx` in slice 6).
  - **Forming chat**: send → `POST /coach {scope:"forming"}`; append student msg + returned reply. Keep the 中/EN toggle (English scope prompt variant is fine to send the same scope; the model replies in kind — or append a language hint to user_input).
  - **Working phase**: `GET /plan` on enter; kanban drag between columns → `PATCH {column}` (optimistic + reconcile); gantt drag-move → `PATCH {start}`, resize → `PATCH {days}` (keep the clamp logic from proto); `+ 添加任务` → `POST /plan/items` then refetch; add a small hover **delete** affordance on a card → `DELETE`. Cards click through to the matching room via the container's room switch (doorway — read→reading, write→writing, review→reflection).
  - **Activity log**: `GET /log`; `记一笔` → `POST /log`. Export → structured `.md`/`.csv` for now.
- Keep every interaction identical in feel to the prototype; the difference is persistence (survives reload) + real coach replies.

## Acceptance
- `cd apps/api && go build ./...` + new Go tests green (Docker up).
- `pnpm --filter web build` + `pnpm --filter web test` green.
- Manual data-flow: create/move/resize a task, edit a proposal dim, add a log note → reload → all persist. Coach chat returns a real (or mocked-in-test) reply.
