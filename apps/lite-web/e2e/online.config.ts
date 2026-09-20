import { defineConfig } from "@playwright/test";

// 🚨 这份 config 就是「线上」的定义，而线上要三个值一起定，不是只定一个。
// 2026-09-19：只在这里写了 baseURL，于是一条号称打线上的走查**实际打的是本地
// dev server** —— `freshAccount` 读的是 env（默认 localhost:5174），它看不见
// 这份 config 的 `use.baseURL`。它红在第一屏「找不到开始兴趣测试」，读起来像
// 入口没做出来，其实是本地那台在跑旧代码。
// 在这里写进 env，是为了让「跑这份 config」和「打线上」成为同一件事。
process.env.E2E_BASE_URL ??= "https://mind-lite.uni-robot.cn";
process.env.E2E_API_BASE ??= "https://mind-api.uni-robot.cn";
// 线上的 Demo School 是 pro，DEMO-0001 注册出来的账号打轻量版的路一律 404。
process.env.E2E_JOIN_CODE ??= "G624-UXFE";

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
    baseURL: process.env.E2E_BASE_URL,
    storageState: "e2e/.auth.online.json",
    trace: "on",
    video: "on",
    screenshot: "on",
    actionTimeout: 30_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
