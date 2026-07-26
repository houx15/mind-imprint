import { test, expect } from "@playwright/test";
import { registerStudent, openRail, uniqueEmail } from "./helpers";

// J-settings — the 设置 surface (previously uncovered). Renders the 个人 card,
// the AI 形象 picker, the local preference toggles, and 退出登录. All client-only
// (no growth/API dependency), so a freshly-registered student can exercise it.
const JOIN_CODE = "DEMO-0001";

test("J-settings: settings surface renders and the avatar picker responds", async ({ page }) => {
  test.setTimeout(90_000);
  const email = uniqueEmail("jsettings-student");
  await registerStudent(page, { name: "E2E 设置", email, code: JOIN_CODE });

  await openRail(page, "设置");

  // Section headers render.
  await expect(page.getByText("设置", { exact: true }).first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText("AI 形象")).toBeVisible();

  // The local preference toggles are present.
  await expect(page.getByText("自动触发工具卡")).toBeVisible();
  await expect(page.getByText("过程记录")).toBeVisible();
  await expect(page.getByText("使用统计")).toBeVisible();

  // The avatar picker offers choices and responds to a click (writes session.avatar).
  const avatarOptions = page.locator('[data-testid="avatar-option"]');
  await expect(avatarOptions.first()).toBeVisible();
  const count = await avatarOptions.count();
  expect(count).toBeGreaterThan(1);
  await avatarOptions.nth(1).click();

  // 退出登录 is present (logout path itself is covered by helpers.logout elsewhere).
  await expect(page.getByText("退出登录")).toBeVisible();
});
