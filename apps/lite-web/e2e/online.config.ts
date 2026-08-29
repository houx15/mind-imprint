import { defineConfig } from "@playwright/test";

// Runs the SAME committed specs as playwright.config.ts, but against the
// live production stack instead of a throwaway local one.
//
// Differences, each forced by "this is production":
//  - globalSetup signs in through the real UI (see online.globalSetup.ts).
//  - retries: 0. A retry re-walks a live model and re-creates real rows in
//    a real account; a flake is worth reporting, not worth paying twice.
//  - Longer timeout: prod's flagship calls (report generation ~43s,
//    /questions ~30s) are slower than a local stub could ever be.
//  - Artifacts always on: the point of this run is evidence someone can look
//    at afterwards.
const ARTIFACTS =
  process.env.E2E_ARTIFACTS ??
  "/Users/houyuxin/08Coding/mind-imprint/.deploy-local/online-e2e-2026-08-29/artifacts";

export default defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 600_000,
  expect: { timeout: 30_000 },
  globalSetup: "./online.globalSetup.ts",
  outputDir: ARTIFACTS,
  reporter: [["list"], ["html", { outputFolder: `${ARTIFACTS}-html`, open: "never" }]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn",
    storageState: "e2e/.auth.online.json",
    trace: "on",
    video: "on",
    screenshot: "on",
    actionTimeout: 30_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
