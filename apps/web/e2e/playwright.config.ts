import { defineConfig } from "@playwright/test";

// The bash harness (run-stack.sh) owns process lifecycle, so no `webServer`
// block here — Playwright only drives the browser against the already-running
// dev server. Serial + retries:1 because the suite shares one stack and one
// seeded database, and the live model is non-deterministic.
export default defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  retries: 1,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: "http://localhost:5173",
    trace: "retain-on-failure",
    actionTimeout: 15_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
