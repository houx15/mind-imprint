import { Page, expect } from "@playwright/test";

export const ADMIN = { email: "admin@demo.mindimprint.local", password: "admin-dev-pass" };
export const PHOEBE = { email: "phoebe@demo.mindimprint.local", password: "phoebe-dev-pass" };

export function uniqueEmail(prefix: string): string {
  return `${prefix}+${Date.now()}@e2e.local`;
}

// Drives the login card. Auth inputs have no test hooks, so target by the
// password type + the first text input in the login card (see selector ref).
export async function login(page: Page, email: string, password: string): Promise<void> {
  await page.goto("/");
  const loginBtn = page.getByRole("button", { name: "登录" }).first();
  await expect(loginBtn).toBeVisible();
  await page.locator('input:not([type="password"])').first().fill(email);
  await page.locator('input[type="password"]').first().fill(password);
  await loginBtn.click();
  // Login resolves when the login submit button is gone (app rendered).
  await expect(page.getByRole("button", { name: "登录" })).toHaveCount(0, { timeout: 30_000 });
}

export async function logout(page: Page): Promise<void> {
  await page.getByRole("tab", { name: "设置" }).click();
  await page.getByText("退出登录").click();
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible({ timeout: 15_000 });
}

// Register → bind code → submit. Auto-signin lands in the app; resolves when
// the auth screen is gone. `code` is a teacher invite OR a class join code.
export async function registerWithCode(
  page: Page,
  opts: { name: string; email: string; password: string; code: string },
): Promise<void> {
  await page.goto("/");
  await page.getByText("注册").click();
  // Register step: name, email, password (three inputs; password is typed).
  await page.locator('input:not([type="password"])').nth(0).fill(opts.name);
  await page.locator('input:not([type="password"])').nth(1).fill(opts.email);
  await page.locator('input[type="password"]').first().fill(opts.password);
  await page.getByRole("button", { name: "下一步 · 绑定班级" }).click();
  // Bind step: class/invite code.
  await expect(page.getByText("班级邀请码")).toBeVisible();
  await page.locator("input").last().fill(opts.code);
  await page.getByRole("button", { name: "完成，进入思维印记" }).click();
  await expect(page.getByRole("button", { name: "完成，进入思维印记" })).toHaveCount(0, { timeout: 30_000 });
}

// ── Journey helpers (authored against the CURRENT refactored UI) ──────────────

export const E2E_PASS = "e2e-pass-12345";

// Register a fresh student with a class join code. Thin wrapper over
// registerWithCode with the shared password; lands on 工作室 (StudentApp default).
export async function registerStudent(
  page: Page,
  opts: { name: string; email: string; code: string },
): Promise<void> {
  await registerWithCode(page, { ...opts, password: E2E_PASS });
}

// Open a student rail surface by its label (聊天/课程/工作室/成长报告/设置).
export async function openRail(page: Page, label: string): Promise<void> {
  await page.getByRole("tab", { name: label }).click();
}

// Create a paper via the 写作工作室 funnel: 新建论文 → fill prompt → 开始.
// After create the workspace (StudioShell + CoachRail) opens directly.
// Resolves when the coach composer is present.
export async function createPaper(
  page: Page,
  opts: { title?: string; prompt: string },
): Promise<void> {
  await page.getByRole("button", { name: "新建论文" }).click();
  if (opts.title) {
    await page.getByPlaceholder("给这篇论文起个名字（可选）").fill(opts.title);
  }
  await page
    .getByPlaceholder("贴上任务要求；如果你已经有思路、资料或初稿，也一起贴进来——我会据此帮你规划环节。")
    .fill(opts.prompt);
  await page.getByRole("button", { name: "开始" }).click();
  // Workspace open ⇒ the coach composer is present.
  await expect(page.getByPlaceholder("把你的想法发给印记……")).toBeVisible({ timeout: 30_000 });
}

// Send one message to the AI 陪练 (coach rail composer) and wait for it to clear.
export async function coachSend(page: Page, text: string): Promise<void> {
  const composer = page.getByPlaceholder("把你的想法发给印记……");
  await composer.fill(text);
  await page.getByRole("button", { name: "发送" }).click();
}
