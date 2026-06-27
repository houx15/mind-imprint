import { test, expect } from "@playwright/test";
import { uniqueEmail } from "./helpers";

// The org invariant: no account without a valid class/invite code. Signing up
// with a bogus code must be rejected and must NOT land the user in the app.
test("signup with an invalid join code is rejected and stays on the bind step", async ({ page }) => {
  await page.goto("/");
  await page.getByText("注册").click();
  await page.locator('input:not([type="password"])').nth(0).fill("Bad Code Student");
  await page.locator('input:not([type="password"])').nth(1).fill(uniqueEmail("badcode"));
  await page.locator('input[type="password"]').first().fill("e2e-pass-12345");
  await page.getByRole("button", { name: "下一步 · 绑定班级" }).click();
  await expect(page.getByText("班级邀请码")).toBeVisible();
  await page.locator("input").last().fill("NOPE-9999");
  await page.getByRole("button", { name: "完成，进入思维印记" }).click();
  // Rejected: the bind submit is still present (we never entered the app).
  await expect(page.getByRole("button", { name: "完成，进入思维印记" })).toBeVisible({ timeout: 15_000 });
});
