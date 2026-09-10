import { defineConfig } from "@playwright/test";

/**
 * 这条 walk 自己的配置，因为它和别的 e2e 有两处不一样：
 *
 *  - **不跑 globalSetup。** 那一段是给本地临时栈用的（起 Postgres、把学校改成
 *    lite、登一次）。这条 walk 打的是**线上**，账号在 spec 里自己注册。
 *  - **没有 storageState。** 同上：它自己注册、自己登录。
 *
 * 跑法（默认打线上）：
 *   cd apps/lite-web && npx playwright test --config e2e/readwalk/playwright.config.ts
 */
export default defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  // 一个学生走四十步，每步一次学生模型 + 一次印记，都是真调用。
  timeout: 45 * 60_000,
  expect: { timeout: 20_000 },
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "https://mind-lite.uni-robot.cn",
    trace: "off",
    video: "off",
    actionTimeout: 20_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
