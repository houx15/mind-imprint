# Deploying `apps/peraspera` to Vercel

This app is a static Astro site (all marketing pages are prerendered) plus **one
serverless function** (`/api/lead`, `src/pages/api/lead.ts`, `prerender = false`)
that writes leads to Supabase via `@supabase/supabase-js`.

`@supabase/supabase-js`'s realtime client requires a global `WebSocket`, which
exists in Node 22 but **not** Node 20. The function must run on **Node.js 22**.
This is pinned via `"engines": { "node": "22.x" }` in `apps/peraspera/package.json`
— Vercel derives both the build container's and the deployed function's Node.js
version from this field (confirmed by reading `@astrojs/vercel`'s `getRuntime()`,
which stamps `process.version` from the machine that ran `astro build` into
`.vercel/output/functions/*.func/.vc-config.json`). No `vercel.json` is needed
for this: there is no way to express a monorepo Root Directory in `vercel.json`,
and the Astro Vercel adapter fully owns the generated `.vercel/output/config.json`
— a hand-written `vercel.json` `functions` block would not affect routes the
Build Output API already covers.

## One-time project setup (Vercel dashboard, or `vercel link`)

1. Create/link a Vercel project for this repo.
2. **Root Directory**: `apps/peraspera` (Project Settings → General → Root Directory).
   Vercel will run the install + build from there; it auto-detects the `astro`
   framework preset and the pnpm workspace at the repo root.
3. **Build Command**: leave default (`pnpm build` / `astro build`, auto-detected).
   **Output**: none to set manually — the `@astrojs/vercel` adapter writes
   `.vercel/output` directly (Build Output API v3).
4. **Node.js Version**: leave on "Automatic" — it reads `engines.node` ("22.x")
   from `apps/peraspera/package.json`. If the dashboard ever shows a manual
   override, set it to **22.x** explicitly.
5. **Environment Variables** (Project Settings → Environment Variables). Add the
   following **names only** — get the values from the repo-root `.env.vercel`
   (gitignored, never commit it, never paste values into this file or into chat):
   - `NEXT_PUBLIC_SUPABASE_URL`
   - `SUPABASE_URL`
   - `SUPABASE_SERVICE_KEY`
   - `DATABASE_URL`

   Set these for all environments you deploy to (Production / Preview / Development
   as needed).

   **Do NOT** add `VERCEL_TOKEN` as a project environment variable — it is a
   personal/CI credential used only by the `vercel` CLI (`--token` flag or
   `VERCEL_TOKEN` in your local/CI shell env), not something the deployed app reads.

## Deploying

### Via CLI

```bash
# from the repo root, targeting the app's directory
vercel --cwd apps/peraspera          # preview deploy
vercel --cwd apps/peraspera --prod   # production deploy

# equivalent, explicit form
vercel deploy --cwd apps/peraspera
vercel deploy --cwd apps/peraspera --prod
```

`VERCEL_TOKEN` must be set in your shell (or passed as `--token <value>`) for
non-interactive/CI use; it is never read by the deployed site.

### Via dashboard

Push to the branch connected to the Vercel project (or open a PR) — Vercel builds
and deploys automatically using the Root Directory / env vars configured above.

## Verifying the deploy

- Static pages (`/`, `/about`, `/apply`, `/institute*`, `/programs`, and the
  `/en/*` mirrors) should serve as prerendered HTML.
- `POST /api/lead` should be backed by a serverless function running Node 22
  (check the function's logs/runtime in the Vercel dashboard, or
  `vercel inspect <deployment-url>`).
- Locally, `pnpm --filter peraspera build` followed by inspecting
  `apps/peraspera/.vercel/output/functions/*.func/.vc-config.json` shows the
  `runtime` field — it will read `nodejs22.x` when the build runs on Node 22
  (e.g. on Vercel, once the `engines` pin takes effect); it reads whatever
  Node major version ran the local build otherwise. `.vercel/` is gitignored
  and is never committed.
