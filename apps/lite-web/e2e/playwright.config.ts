import { defineConfig } from "@playwright/test";

// Same shape as apps/web/e2e/playwright.config.ts: the bash harness
// (run-stack.sh) owns process lifecycle, so there is no `webServer` block —
// Playwright only drives the browser against an already-running stack.
//
// Two deliberate differences from the pro config:
//
//  - `globalSetup` puts the seeded school on the LITE edition and signs the
//    student in once. Every /api/v1/readings route is wrapped in
//    `requireEdition("lite")` (apps/api/internal/api/api.go), which 404s a pro
//    school — so without that UPDATE the whole walk would fail on a blank
//    landing page and look like a frontend bug.
//  - Port 5174, not 5173: the lite dev server must be able to run next to the
//    pro one without either harness silently driving the wrong app.
//
// Serial + retries:1 for the same reason as pro: one shared stack, one shared
// database, and a live model on the AI leg.
export default defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  retries: 1,
  // Generous: the lens leg spends two live model calls (summon → evaluate).
  timeout: 300_000,
  expect: { timeout: 15_000 },
  globalSetup: "./globalSetup.ts",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:5174",
    storageState: "e2e/.auth.json",
    trace: "retain-on-failure",
    actionTimeout: 15_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
