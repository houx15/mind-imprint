# Full-Stack E2E Live Smoke — Design Spec

> **Authored:** 2026-06-27. **Status:** approved (scoping), ready for plan.
> **Goal:** a single browser-driven, full-stack smoke that runs the real web →
> real Go binary → real ephemeral Postgres → **real DeepSeek**, proving the whole
> platform works end-to-end across all three roles before new modules begin.
> **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.
> **Acceptance backbone:** the Phoebe / 「中国是否让地球变得更可持续？」 scenario from `AGENTS.md`.

## Goal

The platform's layers are each well-tested in isolation — Go has unit +
testcontainers integration incl. two scripted E2E verticals
(`TestE2EPhoebeVertical`, `TestE2EOrgProvisioning`); web has ~357 vitest tests
with injected fake clients. **No test exercises the real, fully-assembled stack.**
This spec adds that missing layer: one Playwright suite that drives a real browser
through a real running stack, with a real DeepSeek model, asserting the
cross-role golden path and a few focused edge paths.

## Scope decisions (locked during brainstorming)

1. **Real DeepSeek throughout.** The model is **not** stubbed. Both the chaperone
   turns and the flagship evaluation hit real DeepSeek via the server-side key in
   `apps/api/.env.local`. Consequence: this is a **live smoke**, not a deterministic
   CI gate — assertions target structural invariants, not model wording.
2. **Zero production-code change.** Because we use the real model and the real
   binary as-is, no `STUB_LLM` seam or any other server change is introduced. The
   suite is purely additive test/harness code.
3. **Local-only, not in CI.** Run on demand from a developer machine. CI has no
   DeepSeek key and the run is non-deterministic, so it is deliberately not wired
   into CI.
4. **Harness boots a throwaway Postgres.** The run script starts an ephemeral
   Postgres via `docker run`, applies migrations (which seed admin / Phoebe /
   Demo-Class), runs the suite, and tears everything down. Clean seeded state
   every run.
5. **Coverage = one stitched golden-path lifecycle + focused edge specs.** A single
   continuous journey proves the cross-role handoffs; small independent specs cover
   error/restraint paths.

## Non-goals

- ❌ No deterministic CI gate (real model ⇒ probabilistic). Not added to CI.
- ❌ No `STUB_LLM` / scripted-provider seam, no `main.go` change, no new product
  behavior — this only *tests* what exists.
- ❌ No `docker-compose` checked in (the run script brings the DB up itself).
- ❌ No visual-regression / pixel snapshots — assert behavior and structure.
- ❌ No email-flow testing (P2 signup auto-verifies; the verify handler is dormant).

---

## Environment & preconditions

The developer creates **`apps/api/.env.local`** (gitignored) before running:

```
DATABASE_URL=postgres://postgres:postgres@localhost:5432/mindimprint?sslmode=disable
CORS_ORIGINS=http://localhost:5173
COOKIE_SECURE=false
DEEPSEEK_API_KEY=<developer-supplied>
```

- The web side needs **no** env: Vite proxies `/api` → `:8080` (`vite.config.ts`).
- Models are fixed in `keyresolver.go` (`deepseek-chat` chaperone / `deepseek-reasoner`
  flagship eval, base `https://api.deepseek.com/v1`) — only the raw key is supplied.
- The run script overrides `DATABASE_URL` to point at the throwaway container and
  passes `STUB_LLM` nowhere (no such gate exists).

### Seeded accounts (from migrations `0002/0004/0006`)
- **Admin:** `admin@demo.mindimprint.local` / `admin-dev-pass` (Demo School).
- **Student Phoebe:** `phoebe@demo.mindimprint.local` / `phoebe-dev-pass`.
- **Demo Class join code:** `DEMO-0001`.

The golden path uses the seeded **admin** to bootstrap, then provisions a **fresh**
teacher + class + student to exercise real registration.

---

## Architecture

### Orchestration harness — `apps/web/e2e/run-stack.sh`

A single script owns the full lifecycle, with a `trap` ensuring teardown on any exit:

1. **Boot Postgres:** `docker run -d` a throwaway `postgres:16` on a free port,
   poll `pg_isready` until ready (condition-based wait, no fixed sleep).
2. **Migrate + seed:** `cd apps/api && DATABASE_URL=… go run ./cmd/api -migrate-up`
   (applies all migrations incl. the seed school/admin/Phoebe/class).
3. **Start API:** `DATABASE_URL=… COOKIE_SECURE=false CORS_ORIGINS=http://localhost:5173 go run ./cmd/api`
   in the background, loading `apps/api/.env.local` for `DEEPSEEK_API_KEY`; poll
   `GET /api/v1/healthz` until 200.
4. **Serve web:** `pnpm --filter web dev` on `:5173`; poll until the page serves.
   **Dev server is deliberate, not preview:** Vite's `server.proxy` (`/api` → `:8080`)
   is dev-only, so the web and API share the `:5173` origin and the session cookie
   stays first-party. `vite preview` has no proxy configured, and a cross-origin
   `VITE_API_BASE_URL` would make the cookie cross-site (`SameSite`/`Secure` trap) —
   both rejected. A production-bundle variant would require adding `preview.proxy`
   (a config change); kept out of scope to honor zero-production-change.
5. **Run Playwright:** `pnpm --filter web exec playwright test` against `:5173`.
6. **Teardown:** stop API + web processes, `docker rm -f` the Postgres container,
   on every exit path.

The script is the single entry point: `bash apps/web/e2e/run-stack.sh`.

### Playwright config — `apps/web/e2e/playwright.config.ts`
- `baseURL: http://localhost:5173`, single Chromium project.
- Generous timeouts for live-model latency: per-test ≥ 120s, and explicit
  `expect(...).toPass({ timeout })` / `waitFor` around model-dependent steps.
- `retries: 1` (a live-model run may need one retry); `workers: 1` (shared stack +
  shared seeded DB ⇒ serial).
- `webServer` is **not** used (the bash harness owns process lifecycle); Playwright
  only drives the browser.
- New devDependency on `apps/web`: `@playwright/test` (+ `playwright install chromium`).

### Test helpers — `apps/web/e2e/helpers.ts`
Role-scoped login + navigation helpers and stable selectors:
- `loginAs(page, email, password)` — drives the real `AuthScreen`.
- `signupTeacher(page, inviteCode, email, password)` / `signupStudent(page, joinCode, …)`.
- Selectors keyed to existing UI text/roles (confirmed against the live app at build
  time): console rail tabs (班级/教师/导入/概览), workspace card surface, process
  tree, evaluation 「你的思维印记」 panel.

---

## The golden-path lifecycle — `golden-path.spec.ts`

One continuous `test()` (serial), asserting at each cross-role seam. Model-dependent
steps use generous `waitFor`/`toPass`.

1. **Admin bootstrap.** `loginAs(admin)` → console lands on 概览 → navigate 教师 →
   mint a teacher-invite for a fresh email → **assert the invite code is shown in the
   UI** (capture it).
2. **Teacher registration.** Log out → `signupTeacher(inviteCode, …)` → assert
   auto-verified landing on the teacher console (班级).
3. **Class creation.** Teacher creates a class → **assert a join code is surfaced**
   (capture it).
4. **Student registration.** Log out → `signupStudent(joinCode, …)` → assert
   auto-verified landing on the student workspace.
5. **Phoebe task (real model).**
   - Create a task and paste the Phoebe source link/prompt that strongly matches
     `sift_craap`'s 「何时用」.
   - Run a turn; **wait for a `summon_card` proposal to appear** (see Determinism
     below — retried/generous wait; a clear "model declined" diagnostic on timeout).
   - **Open** the card (student confirm), fill the SIFT envelope, submit.
   - **Assert the process tree grows** (a new node for the completed card).
   - Run another turn so the completed card refeeds; assert the reply streams
     (proving refeed-aware history did not error).
   - Trigger evaluation; **wait for 「你的思维印记」** to render a rubric (real
     `deepseek-reasoner`, `MaxTokens: 8000`).
6. **Teacher sees signals.** Log out → `loginAs(teacher)` → open the class →
   **assert the new student appears on the roster with signal counts > 0**
   (task/eval/card), aggregate-only (no work contents).
7. **Admin overview.** `loginAs(admin)` → 概览 → **assert the new class/student are
   reflected** in the aggregates.

### Determinism & flakiness (the one real risk)
`tool_choice` is `"auto"` (`deepseek.go:84`) — the model decides whether to summon a
card. Mitigations, in order:
- The pasted task content is crafted to strongly match a card's trigger ("何时用"),
  the way the Phoebe scenario is designed to.
- The summon step uses a generous `waitFor`; if no proposal appears, the student
  sends **one** follow-up message that more explicitly describes needing to vet a
  source, then waits again.
- On final timeout the test fails with an explicit diagnostic — "model declined to
  summon under tool_choice=auto" — distinguishing a *model* non-summon from a
  *pipeline* break. `retries: 1` absorbs the occasional decline.

This is accepted: a live smoke proves the real system behaves, tolerating model
variance, rather than asserting exact outputs.

---

## Focused edge specs (small, independent)

Each is its own short `test`, using seeded accounts where possible (no live-model
calls ⇒ fast and stable):

- **`auth.spec.ts`** — wrong password → inline error; visiting a protected route
  while logged-out → auth screen (no crash).
- **`registration.spec.ts`** — signup with a malformed/invalid class join code →
  rejected with the backend's user-facing message; the org invariant (no
  class-less account) holds.
- **`restraint.spec.ts`** — the restraint iron law in the UI: a summoned card is
  **proposed, not auto-opened** (opening requires student confirm), and dismissing
  /skipping a card still leaves a recorded process-tree signal. (Uses the student
  workspace; may reuse the golden-path session state or seed directly.)

---

## Error handling (harness)

- Any stage failing (PG not ready, migrations error, API not healthy, web not
  serving) aborts with a clear message and runs full teardown (`trap`).
- Teardown is idempotent and runs on success, failure, and Ctrl-C.
- The script never prints secrets; `DEEPSEEK_API_KEY` is read by the API process
  from `.env.local`, never echoed.
- A missing `apps/api/.env.local` or empty `DEEPSEEK_API_KEY` fails fast with a
  setup hint (the live steps would otherwise return `errNoProvider`).
- Docker absent → fail fast with a hint (throwaway PG needs Docker).

## Testing the harness itself

This *is* the test layer, so "testing the tests" stays light:
- A `--smoke-stack` style dry path (or a tiny first spec) that only asserts the app
  loads at `:5173` and `GET /api/v1/healthz` is 200 — proves the harness wiring
  before the expensive live-model journey.
- Helpers (`loginAs`, selector lookups) are exercised by the specs themselves.

## File structure (all additive)

```
apps/web/e2e/
  run-stack.sh            # boot PG → migrate/seed → API → web → playwright → teardown
  playwright.config.ts    # chromium, serial, generous timeouts, retries:1
  helpers.ts              # role login/signup + stable selectors
  golden-path.spec.ts     # the stitched cross-role lifecycle (live model)
  auth.spec.ts            # wrong password / protected-route-logged-out
  registration.spec.ts    # bad join code rejected; org invariant
  restraint.spec.ts       # card proposed-not-opened; skip still recorded
  RUNBOOK.md              # how to run, env setup, expected outcomes, known model variance
apps/web/package.json     # +@playwright/test devDependency
```

## Carry-forward (recorded — to `docs/遗留项追踪_Carryforward.md`)

- **No deterministic CI gate for the full stack.** If a deterministic gate is later
  wanted, revisit the previously-considered `STUB_LLM` env-gated scripted provider
  (off by default) so CI can run the golden path without a live model.
- **Selector stability.** First plan task confirms live-app selectors; if the UI
  lacks stable hooks, adding `data-testid`s is a small follow-up (kept out of scope
  here to honor zero-production-change, but flagged).
