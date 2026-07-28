# Slice 1 · Foundation — SHARED SPEC

Pinned shapes so contracts (1a) / backend (1b) / frontend (1c) agree. See `../2026-07-29-writing-studio-redesign-build-breakdown.md` for the whole plan and `../2026-07-29-writing-studio-redesign-prd.md` + `apps/web/src/proto/` for the design.

## Canonical types (camelCase over the wire)

```ts
// proposal
type Proposal = { objective: string; reason: string; activities: string; resources: string };

// plan
type PlanTag = "read" | "write" | "review";
type PlanColumn = "todo" | "doing" | "done";
type PlanItem = { id: string; title: string; tag: PlanTag; column: PlanColumn; stage: string;
                  refMaterialId: string | null; start: number; days: number; position: number };

// library
type Collection = { id: string; name: string; parentId: string | null; position: number };
type UseDecision = "use" | "maybe" | "drop" | null;
type Credibility = "strong" | "mixed" | "weak";
type ReadingNote = { quote: string; finding: string };            // projected from reading outcomes
type Reference = { id: string; title: string; classification: string; author: string; credentials: string;
                   year: string; url: string; tags: string[]; collectionId: string | null;
                   credibility: Credibility | null; evaluation: string; decision: UseDecision;
                   pending: boolean; searchHints: string[]; materialId: string | null; notes: ReadingNote[] };

// write
type OutlineNode = { id: string; text: string; depth: number; position: number };

// activity log
type LogSource = "auto" | "me";
type LogEntry = { id: string; date: string; text: string; source: LogSource };   // date = "MM-DD" for display

// review
type ReflectionDoc = { answers: string[]; done: boolean };         // answers indexed to the 5 prompts
type MirrorSection = { title: string; body: string };
type Mirror = { sections: MirrorSection[]; carryForwards: string[] };

// the GET /projects/{id} projection (lean — rooms fetch their own data)
type WorkspaceProjection = { id: string; title: string; qualification: string; proposal: Proposal };
```

## 1a · Contracts (`packages/contracts/src/`)
New files, one concept each, `export const X = z.object(...)` + `export type X = z.infer<typeof X>`:
`proposal.ts`, `planItem.ts`, `collection.ts`, `reference.ts`, `outlineNode.ts`, `activityLog.ts`, `reflectionDoc.ts`, `mirror.ts`, `workspace.ts`.
Re-export each via `src/index.ts` barrel. Add a vitest per file under `test/` asserting a valid object parses and an invalid one fails (match neighbor style). Use `z.enum` for the unions; nullable via `.nullable()`. `credibility` nullable; `decision` nullable. Do NOT collide with existing exports (`Proposal`/`Reference` verbs live in `agentOutput.ts` under different names — these top-level `Proposal`/`Reference` are new domain objects; if a name clashes at the barrel, keep these names and alias the older one is NOT allowed — instead verify no clash: current barrel has no top-level `Proposal`/`Reference`/`Collection`/`OutlineNode`/`Mirror`; proceed). Run `pnpm --filter @mind-imprint/contracts test` green.

## 1b · Backend schema (`apps/api`)
Goose migrations in `internal/store/migrations/` (next number after 0035), one file `00NN_workspace_redesign.sql` with all new tables (Up + Down):
`project_proposal`, `plan_item`, `collection`, `reference`, `outline_node`, `activity_log_entry`, `project_reflection`, `project_mirror_prose` (columns per build-breakdown §Data model). FKs to `project(id)` ON DELETE CASCADE. CHECK constraints for the enums. `position int` default 0.
Then: extend ONLY the HTTP handler `getProject` (`internal/api/`) to return `WorkspaceProjection` JSON `{id,title,qualification,proposal}` — proposal read from `project_proposal` (or zero-value `{"","","",""}` if absent). Add sqlc query `GetProjectProposal`. **Do NOT touch `internal/studio` projection or the assessment path** (still used by finish/growth). Add stub sqlc queries only if needed to compile. Run `make sqlc` (CGO_ENABLED=0) + `go build ./...` green. Add a Go test that `GET /projects/{id}` returns the new shape for a seeded project.

## 1c · Frontend shell + retirement (`apps/web/src/`)
1. New module `workspace/`:
   - `Icon.tsx` (+ `BLOCK_META`) and the four blocks **ported from `proto/`** (`PlanBlock/ReadingBlock/WritingBlock/ReviewBlock`) running on **local component state** for now (real wiring lands in slices 2–5). Keep all interactions working locally (drag, edit, tabs).
   - `WorkspaceContainer.tsx`: left rail (project title + qualification tag + four rooms 01–04 + back-to-all-projects), lands in Project Management, instant room view-swap, plus a **reading-room swap** slot: hold `readingSource: MaterialSource | null`; when set, render the existing `<ReadingRoom projectId source api onBack onOpenLogged?/>` from `studio/reading/ReadingRoom`; nothing sets it yet (slice 3 wires it) but the mechanism must exist.
   - `Directory.tsx`: all-projects list + create (reuse existing `api.listProjects` / `api.createProject`; create needs only title + qualification — no prompt gate). Opening a project sets projectId and shows the rooms.
   - `api/workspace.ts`: `getWorkspace(id): Promise<WorkspaceProjection>` calling `GET /api/v1/projects/{id}` and parsing the contract. Container fetches it for title/qual/proposal.
2. Wire `shell/StudentApp.tsx` studio tab → `WorkspaceContainer` (replace `StudioContainer`).
3. **Retire** (delete): `studio/StudioContainer.tsx, StudioShell.tsx, StationRail.tsx, ViewFrame.tsx, Directory.tsx, CoachRail.tsx, EquipmentBar.tsx, MethodologyModal.tsx, WorkOrder.tsx, DispositionCard.tsx, SpotCheckPanel.tsx, state.ts, conversation.ts, fixtures.ts, index.ts`; `studio/views/*`; `studio/StudioAnnotateCard.tsx, StudioToulminCard.tsx, StudioMatrixCard.tsx, StudioSortCard.tsx, StudioScaleCard.tsx`; `dev/DevApp.tsx, dev/StudioPanel.tsx, dev/MaterialPanel.tsx`.
   - **KEEP**: `studio/reading/*`, `studio/material/*`, `studio/Bean.tsx`, `studio/StudioCardSheet.tsx`, all `cards/*`, `primitives/*`, `api/*`, `dev/Harness.tsx`. (chat/courses import Bean + StudioCardSheet directly — must keep compiling.)
   - Before deleting each `Studio*Card` host, grep that no surviving file imports it; if one does, keep it.
**Acceptance**: `pnpm --filter web build` passes; `pnpm --filter web test` passes (delete/adjust tests that referenced retired code; keep reading tests). App boots to the four-room workspace, rooms switch, directory create works, ReadingRoom still compiles/enters via the container mechanism.
