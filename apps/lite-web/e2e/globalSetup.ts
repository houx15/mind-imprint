import { execFileSync } from "node:child_process";
import { request } from "@playwright/test";

// The seeded dev student (migrations 0002 + 0004). She belongs to Demo School,
// which ships as `edition='pro'` — see below.
export const STUDENT = {
  email: "phoebe@demo.mindimprint.local",
  password: "phoebe-dev-pass",
};

// Relative to the cwd Playwright is invoked from (apps/lite-web — see
// run-stack.sh and the package's `e2e` script), matching `use.storageState`
// in playwright.config.ts.
const STORAGE_STATE = "e2e/.auth.json";

const PG_CONTAINER = process.env.E2E_PG_CONTAINER ?? "mindimprint-lite-e2e-pg";
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:5174";

/**
 * globalSetup — two things the walk cannot do for itself.
 *
 * 1. **Put the school on the lite edition.** Edition belongs to the SCHOOL,
 *    not the account (migration 0093), and every `/api/v1/readings*` route is
 *    wrapped in `requireEdition("lite")`, which answers 404 — deliberately
 *    indistinguishable from "no such route" — for a pro school. The seed ships
 *    Demo School as pro, so without this UPDATE the walk would open a landing
 *    page whose history call 404s and whose 开始阅读 dies on POST /readings.
 *    Seeding it here (rather than by hand before a run) is what keeps the
 *    suite runnable from a cold `run-stack.sh`.
 *
 * 2. **Sign in once.** The lite shell has no login surface of its own in P1 —
 *    the session cookie is shared infrastructure. Signing in through the dev
 *    server's `/api` proxy means the cookie is stored against the same origin
 *    the browser will use.
 */
export default async function globalSetup(): Promise<void> {
  execFileSync(
    "docker",
    [
      "exec",
      PG_CONTAINER,
      "psql",
      "-U",
      "postgres",
      "-d",
      "mindimprint",
      "-v",
      "ON_ERROR_STOP=1",
      "-c",
      "UPDATE schools SET edition='lite'",
    ],
    { stdio: "pipe" },
  );

  const ctx = await request.newContext({ baseURL: BASE_URL });
  const res = await ctx.post("/api/v1/auth/signin", { data: STUDENT });
  if (!res.ok()) {
    throw new Error(`signin failed: HTTP ${res.status()} ${await res.text()}`);
  }
  await ctx.storageState({ path: STORAGE_STATE });
  await ctx.dispose();
}
