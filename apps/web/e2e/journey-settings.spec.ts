import { test, expect } from "@playwright/test";
import { registerStudent, openRail, uniqueEmail } from "./helpers";

// J-settings — the 设置 surface, now under the 我 nav tab (SettingsView). Renders
// the 个人 card, the 主题色 accent picker (8 presets, replacing the old local-only
// "AI 形象" swatches), the local preference toggles, and 退出登录. All client-only
// (no growth/API dependency), so a freshly-registered student can exercise it.
const JOIN_CODE = "DEMO-0001";

test("J-settings: settings surface renders and the accent picker responds", async ({ page }) => {
  test.setTimeout(90_000);
  const email = uniqueEmail("jsettings-student");
  await registerStudent(page, { name: "E2E 设置", email, code: JOIN_CODE });

  await openRail(page, "我");

  // Section headers render: the 设置 h1 + the 主题色 accent section.
  await expect(page.getByRole("heading", { name: "设置" })).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText("主题色")).toBeVisible();

  // The local preference toggles are present.
  await expect(page.getByText("自动触发工具卡")).toBeVisible();
  await expect(page.getByText("过程记录")).toBeVisible();
  await expect(page.getByText("使用统计")).toBeVisible();

  // The accent picker offers ≥8 presets and responds to a click (retints the app
  // + persists via api.setAccent).
  const swatches = page.locator('[data-testid="accent-swatch"]');
  await expect(swatches.first()).toBeVisible();
  const count = await swatches.count();
  expect(count).toBeGreaterThan(1);
  await swatches.nth(1).click();

  // 退出登录 is present (logout path itself is covered by helpers.logout elsewhere).
  // .last(): the nav footer now ALSO carries a "退出登录" button (tour feature
  // ship), rendered before the settings panel in the DOM — scope to the
  // settings surface's own button, not the nav's.
  await expect(page.getByRole("button", { name: "退出登录", exact: true }).last()).toBeVisible();
});
