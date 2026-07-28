# Slice 5 · Review room + assessment rewire — SHARED SPEC

Room = `workspace/blocks/ReviewBlock.tsx`. Design: PRD §4.4 + `apps/web/src/proto/blocks/ReviewBlock.tsx`. Left = the student's own 5-dimension reflection (goal anchored to the kickoff objective). Right = 你的思维印记 mirror (AI narrative + two carry-forwards, "for reference, not a grade"). 完成回顾 archives + quietly generates the process assessment into the growth report.

## Endpoint contracts (`/api/v1/projects/{id}`)

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/reflection-doc` | — | `{answers: string[], done: boolean}` (answers length up to 5) |
| PUT | `/reflection-doc` | `{answers: string[], done?: boolean}` | `{answers, done}` (upsert) |
| GET | `/mirror` | — | `{sections:[{title,body}], carryForwards:string[]}` OR JSON `null` (no LLM call, ever) |
| POST | `/mirror` | — | `{sections, carryForwards}` — **first-open-wins**: returns stored if present (no spend), else composes once (flagship) + stores |
| POST | `/finish` | — | existing; gate rewired (see below) |

## Backend (`apps/api`)
Study: `internal/api/project_finish.go` (the gate to rewire), `internal/api/assessment.go` (`generateProjectReport`, `buildAssessmentInputFromProject`, `graphSummary`, `workSamplesFromProject`), `internal/api/parent_report.go` (`composeParentProse` = the first-open-wins + flagship-compose + `RecordLLMCall` pattern to mirror), `internal/agent/AssessReport` input.

1. **Reflection doc**: sqlc `GetProjectReflection`/`UpsertProjectReflection` over `project_reflection` (answers jsonb, done bool). Handlers `GET/PUT /reflection-doc` in a new `internal/api/workspace_review.go`. `PUT` upserts; on `done` flipping true, `appendAutoLog(...,"完成回顾")`.
2. **Mirror**: sqlc `GetProjectMirror`/`InsertProjectMirror` over `project_mirror_prose`. 
   - `GET /mirror`: return stored `{sections,carryForwards}` or JSON `null`. No model call.
   - `POST /mirror`: if a row exists → return it (no spend, first-open-wins). Else compose via **flagship `EvalResolver`**: build a process summary (proposal 4 dims + reflection answers + a digest of events/cards + the current draft from `GetEditBuffer` + outline) → a new `internal/agent` composer `ComposeMirror(ctx, provider, resolved, in) (Mirror, usage, err)` that prompts for STRICT JSON `{"sections":[{"title","body"}],"carryForwards":["",""]}` — restrained, second-person, "how your thesis grew / how reading fed writing / where you figured it out vs leaned on 印记 / which cards you summoned", + two forward-looking takeaways. `RecordLLMCall(surface="studio",purpose="mirror")` BEFORE any bail. On compose error, still record cost and return a graceful minimal mirror (never 500). Store via `InsertProjectMirror` (ON CONFLICT do nothing → re-read, concurrent-winner semantics). 
3. **Finish gate rewire** (`project_finish.go`): REMOVE the `draft_polish.whole_draft_review == "solid"` gate. Replace with: require `project_reflection.done == true` → else 422 `{code:"reflection_not_done", message:"先完成回顾再归档"}`. Keep the already-finished 409 and the flagship generation/cost-record unchanged.
4. **Assessment enrichment (A6)** — in `assessment.go`, feed the new student work into the assessor WITHOUT changing `agent.AssessReport`'s signature:
   - `workSamplesFromProject` (or the call site): also include the current `edit_buffer` draft (not only committed snapshots), so a draft that was never "committed" still counts.
   - Enrich the free-text context string passed as the `graph` summary (or extend `graphSummary`) to PREPEND: the proposal's four dimensions, the reflection answers, and the outline text — clearly labelled — so the flagship assessor sees the real kickoff + reflection + structure. This is the minimal honest rewire; do not fabricate.
   - Keep everything else (`studio.Load`/`studio.Project`, events, cards, dispositions, rounds) as-is.
5. Go tests: reflection PUT/GET round-trip + done flips; mirror POST composes once then GET returns it and a second POST does not re-spend (first-open-wins — assert only one `llm_call` with purpose=mirror); finish blocked when reflection not done (422) and succeeds after done=true (mock provider) producing an evaluation row that `GET /assessment` then returns; ownership 404.

## Frontend (`apps/web/src/workspace`)
- `api/workspace.ts` (extend): `getReflection(id)`, `putReflection(id,{answers,done?})`, `getMirror(id)` (nullable), `postMirror(id)`. For finish, reuse the existing project-finish client if present (grep `finishProject` in `apps/web/src/api/`), else add `finishProject(id)` calling `POST /finish`.
- `WorkspaceContainer.tsx`: pass `projectId`, `proposal`, and `onFinished` (the StudentApp handoff already threaded into the container) down to `ReviewBlock`.
- `blocks/ReviewBlock.tsx`:
  - `getReflection` on mount → 5 answers (pad/truncate to the 5 prompts). Edits debounce ~600ms → `putReflection({answers})`. The 目标与达成 prompt shows the kickoff objective from `proposal.objective` (the anchor).
  - Mirror pane: `getMirror` on mount; if null, `postMirror` once to generate, then render `sections` + 带走这两点 `carryForwards`. Label "供你参考——不是评分". Show a subtle loading state while composing.
  - 完成回顾 → `putReflection({answers, done:true})` → `finishProject(projectId)`; on success show "已归档 · 这次的过程评估已记入你的成长报告" + a "查看成长报告" that calls `onFinished()`. Handle 422 gracefully.
- Nothing is graded to the student here.

## Acceptance
- `go build ./...` + new Go tests green (mirror first-open-wins asserted). `pnpm --filter web build` + `pnpm --filter web test` green.
- Data-flow: write reflection answers, reload → persist. Open Review → mirror composes and shows. 完成回顾 → project finishes, `GET /assessment` returns a report, it appears in the growth report. Re-open Review → mirror is the same stored one (no re-spend).
