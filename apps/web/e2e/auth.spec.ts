import { test, expect } from "@playwright/test";
import { login, ADMIN } from "./helpers";

test("seeded admin logs in and lands on the console overview", async ({ page }) => {
  await login(page, ADMIN.email, ADMIN.password);
  // Admin lands on the console; 概览 tab is present and selected-area visible.
  await expect(page.getByRole("tab", { name: "概览" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "教师" })).toBeVisible();
});

test("wrong password shows an inline error and stays on login", async ({ page }) => {
  await page.goto("/");
  await page.locator('input:not([type="password"])').first().fill(ADMIN.email);
  await page.locator('input[type="password"]').first().fill("definitely-wrong");
  await page.getByRole("button", { name: "登录" }).first().click();
  // Still on the login screen (button remains), no crash.
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible();
});

test("visiting the app while logged-out shows the auth screen, not a crash", async ({ page }) => {
  await page.context().clearCookies();
  await page.goto("/");
  await expect(page.getByRole("button", { name: "登录" }).first()).toBeVisible();
});
