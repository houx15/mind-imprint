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
