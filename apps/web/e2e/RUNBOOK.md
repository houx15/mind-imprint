# Full-Stack E2E Live Smoke — Runbook

A browser-driven smoke over the real stack (web → Go API → throwaway Postgres → **real DeepSeek**). Local-only; not a CI gate.

## One-time setup
1. Docker running (for the throwaway Postgres).
2. `pnpm install` at the repo root; `pnpm --filter web exec playwright install chromium`.
3. Create `apps/api/.env.local`:
   ```
   DATABASE_URL=postgres://postgres:postgres@localhost:5432/mindimprint?sslmode=disable
   CORS_ORIGINS=http://localhost:5173
   COOKIE_SECURE=false
   DEEPSEEK_API_KEY=<your key>
   ```
   (The harness overrides `DATABASE_URL` to the throwaway container; the key is what matters here.)

## Run
- Everything: `bash apps/web/e2e/run-stack.sh`
- One spec: `bash apps/web/e2e/run-stack.sh golden-path.spec.ts`
- Deterministic specs only (no key needed): `bash apps/web/e2e/run-stack.sh auth.spec.ts registration.spec.ts smoke.spec.ts`

## What each spec proves
- `smoke` — the stack is wired (app serves, `/healthz` is 200).
- `auth` — seeded admin login lands on 概览; wrong password stays on login; logged-out shows auth screen.
- `registration` — an invalid join code is rejected (org invariant).
- `golden-path` — the full cross-role lifecycle: admin invite → teacher signup → class → student join → Phoebe task (card summon → SIFT envelope → process tree → refeed → evaluation 你的思维印记) → teacher sees roster signals → admin overview.

## Expected model variance
`tool_choice` is `auto`, so the AI decides whether to summon a card. The golden path nudges once and waits generously; an occasional "model declined to summon" failure is live-model variance, not a platform break — re-run (`retries:1` already absorbs one). The evaluation uses the flagship `deepseek-reasoner` (`MaxTokens: 8000`) and can take up to ~90s.

## Pass criteria (live)
A real `summon_card` proposal appears, the SIFT envelope pins a process-tree node, the refeed turn streams a reply without error, and the evaluation renders a real 你的思维印记 rubric. Teacher roster shows the student with non-zero signals; admin overview reflects the new class.
