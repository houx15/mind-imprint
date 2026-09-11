import { defineConfig } from "@playwright/test";

/**
 * 这条 walk 自己的配置，理由和 `e2e/readwalk/playwright.config.ts` 一样：
 *
 *  - **不跑 globalSetup。** 那一段是给本地临时栈用的（起 Postgres、把学校改成
 *    lite、登一次）。这条 walk 默认打**线上**，账号在 spec 里自己注册。
 *  - **没有 storageState。** 同上。
 *
 * 跑法：
 *   cd apps/lite-web && npx playwright test --config e2e/writewalk/playwright.config.ts
 *
 * 本地栈上跑（先 `bash e2e/run-stack.sh --list` 把栈起起来是不行的，那个脚本
 * 跑完就拆；要么自己起 api+web，要么直接打线上）：
 *   E2E_BASE_URL=http://localhost:5174 E2E_API_BASE=http://localhost:8080 \
 *     npx playwright test --config e2e/writewalk/playwright.config.ts
 */
export default defineConfig({
  testDir: ".",
  // 两个学生各自注册、各自的浏览器上下文，互不相干。
  fullyParallel: true,
  workers: 2,
  retries: 0,
  // 一个学生走四十五步，每步一次学生模型 + 一次印记，都是真调用。
  timeout: 50 * 60_000,
  expect: { timeout: 20_000 },
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn",
    trace: "off",
    video: "off",
    actionTimeout: 20_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
