import { defineConfig } from "@playwright/test";

/**
 * 看图台自己的 playwright 配置 —— 不碰 `e2e/playwright.config.ts`，
 * 因为那一份带着 globalSetup（注册账号、连后端），而这里一样都不需要。
 */
export default defineConfig({
  testDir: ".",
  timeout: 60_000,
  // 端口跟着 vite.config.ts 那一个走（见那里 🚨 那段：两个 worktree 同时
  // 开看图台时，端口写死会让第二个会话**测到第一个会话的代码**）。
  use: {
    baseURL: `http://localhost:${process.env.E2E_HARNESS_PORT ?? 5234}`,
    viewport: { width: 1280, height: 820 },
  },
  reporter: [["list"]],
});
