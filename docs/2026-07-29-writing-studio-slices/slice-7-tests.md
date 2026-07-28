# Slice 7 · Test engineering (function + journey) — SPEC

Goal: the whole tree goes green, retired-flow test debt removed, and the redesign's mainline is covered by a real journey test. Then run everything.

## A. Remove retired-flow test debt (backend)
Slice 1b repurposed `GET /projects/{id}` from the six-station `StudioProjection` to the lean `WorkspaceProjection`, so any test that walks stations / asserts the old projection shape now fails. These test a **retired UX** and must be deleted (not "fixed"):
- Confirmed red: `internal/api/walkable_stations_test.go` (`stations=[]`), `internal/api/journey_walk_test.go`, and any of `walk_s0_s6_test.go` / `advance_test.go` / `journey_test.go` / `search_plan_seed_test.go` that assert stations/readiness via `GET /projects/{id}` or walk S0–S6.
- Rule: delete a test file (or the specific test) ONLY if it exercises the retired station-walk/readiness UX through the changed projection. Do NOT delete tests for handlers that still exist and still pass (onboarding/framing/perspectives submit handlers remain in the backend as dead-but-harmless code — their direct handler tests, if still green, stay).
- Run `go test ./internal/api/` after each removal; iterate until green. Note in the commit which files were removed and why.

(Backend station handlers themselves are left in place this round — removing them risks the assessment/skill machinery `studio.Project` still uses. Documented as a follow-up, not a blocker.)

## B. Function-based coverage (already substantial; fill gaps)
Per-slice handler tests already exist (proposal/plan/log/coach, library/enter-reading, outline/draft, reflection/mirror/finish). Add only what's missing:
- Frontend reducer/unit tests for the pure logic the rooms rely on if not already covered: outline indent/promote/add/delete + mind-map tree build, kanban move, gantt clamp (start/days bounds), annotated-bib row mapping, export fns (already in slice 6). Put under `apps/web/test/workspace/`.

## C. Journey-based E2E (the acceptance mainline)
Add ONE Go integration test `internal/api/workspace_journey_test.go` that walks the redesign mainline against a real DB (testcontainers), signed in as the seeded student, using the real HTTP handlers (mock provider for the LLM endpoints, as coach/mirror/finish tests already do):
1. create a project (or use the seeded one) → `GET /projects/{id}` returns the workspace projection.
2. `PUT /proposal` (4 dims) → `GET /projects/{id}` reflects it.
3. `POST /coach {forming}` → a reply; a `coach_turn` event exists.
4. `POST /plan/items` ×N, `PATCH` (move column + reschedule + resize), `GET /plan` reflects; `POST /log` + `GET /log`.
5. `POST /references` (with a URL, fake Fetcher) → `POST /references/{rid}/enter-reading` returns a MaterialSource with blocks; `GET /library` shows it.
6. `PUT /outline` (nested) → `GET /outline`; `PUT /buffer` → `GET /draft`.
7. `PUT /reflection-doc {answers, done:true}` → `POST /finish` succeeds → `GET /assessment` returns a report → it also appears via `GET /growth/history`.
8. `POST /mirror` returns sections+carryForwards; a second `POST /mirror` does not re-spend.
Assert the persisted data flow end-to-end. This is the single most important test.

## D. Run everything
- `cd apps/api && DOCKER_HOST=... go test ./...` → all green (state the command).
- `pnpm --filter @mind-imprint/contracts test` → green.
- `pnpm --filter web test` + `pnpm --filter web build` → green.
Report the final counts for each suite and the journey test result.
