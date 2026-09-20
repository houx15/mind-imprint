import { defineConfig } from "@playwright/test";

/**
 * 看图台自己的 playwright 配置 —— 不碰 `e2e/playwright.config.ts`，
 * 因为那一份带着 globalSetup（注册账号、连后端），而这里一样都不需要。
 */
export default defineConfig({
  testDir: ".",
  timeout: 60_000,
  use: { baseURL: "http://localhost:5233", viewport: { width: 1280, height: 820 } },
  reporter: [["list"]],
});
