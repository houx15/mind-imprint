import { test, expect } from "@playwright/test";
import { login, ADMIN } from "./helpers";

// J-admin — the admin console: 概览 (real org stats) → 教师 (mint an invite) →
// 导入 (CSV batch import creates a class + join codes). Deterministic (no model).
test("J-admin: overview → mint teacher invite → CSV import", async ({ page }) => {
  test.setTimeout(120_000);
  await login(page, ADMIN.email, ADMIN.password);

  // 1. 概览 (default) — real org stats render (seeded org has students/classes).
  await expect(page.getByRole("tab", { name: "概览" })).toBeVisible();
  await expect(page.getByText("活跃学生")).toBeVisible({ timeout: 15_000 });
  // A non-zero student count proves real data was fetched (not a static label).
  await expect(page.locator("div").filter({ hasText: /^学生[1-9]\d*$/ }).first()).toBeVisible({ timeout: 15_000 });

  // 2. 教师 → mint a teacher invite code (T-XXXXXX).
  await page.getByRole("tab", { name: "教师" }).click();
  await expect(page.getByText("生成教师邀请码")).toBeVisible();
  await page.getByRole("button", { name: "生成邀请码" }).click();
  await expect(page.getByText(/新邀请码\s+T-/)).toBeVisible({ timeout: 15_000 });

  // 3. 导入 → upload a CSV → preview → import → classes + join codes appear.
  await page.getByRole("tab", { name: "导入" }).click();
  const className = `E2E导入班${Date.now()}`;
  const csv = `class,teacher_email,student_email\n${className},wu.teacher@demo.mindimprint.local,e2e-import-${Date.now()}@demo.local\n`;
  await page.locator('[data-testid="csv-input"]').setInputFiles({
    name: "roster.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(csv, "utf-8"),
  });
  await page.getByRole("button", { name: "导入" }).click();
  // Success: the result panel lists the created class + its join code.
  await expect(page.getByText("班级与邀请码")).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText(className)).toBeVisible();
});
