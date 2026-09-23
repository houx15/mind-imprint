import { test, expect } from "@playwright/test";

/**
 * 「我的阅读」列表里把一篇收起来 —— 在真浏览器里按一次。
 *
 * 产品负责人 2026-09-23 第 3 条：「阅读列表里旧的也没办法删除。」
 *
 * 🚨 为什么不能只靠 jsdom：memory `control-that-is-not-wired-2026-09-22` ——
 * 整行原来是一个 `<button>`，收起来那颗要摆在它旁边。按钮套按钮既不合法、
 * 里面那一下也会被外面那一下吃掉，而**只有真浏览器抓得到**这件事。
 *
 * 跑法：
 *   E2E_HARNESS_PORT=5248 npx vite --config e2e/harness/vite.config.ts
 *   E2E_HARNESS_PORT=5248 npx playwright test --config e2e/harness/playwright.config.ts history-harness
 */

const OUT = "e2e/harness/.shots";

test("收起来要按两次，而且不会顺手把那一行点开", async ({ page }) => {
  await page.goto("/history.html");
  await expect(page.getByText("粘错了的那一篇（分段全乱）")).toBeVisible();
  await page.screenshot({ path: `${OUT}/history-00-list.png`, fullPage: true });

  const archive = page.getByRole("button", { name: /^收起《粘错了的那一篇/ });
  await expect(archive).toHaveCount(1);
  // 🚨 **不悬停也看得见。** 第一版写的是 opacity-0 + group-hover，在这台
  // 看图台上两条判据照样全绿 —— 而触摸屏上没有 hover，用 iPad 的学生
  // 永远看不见这颗按钮。逐屏看图才看出来的（memory:
  // look-at-the-image-not-the-assertions-2026-09-22）。
  const shown = await archive.evaluate((el) => Number(getComputedStyle(el).opacity));
  expect(shown, "收起来那颗按钮要悬停才看得见 —— 触摸屏上等于没有").toBeGreaterThan(0.2);
  await archive.click();

  // 第一下只是问一句，那一篇还在。
  await expect(page.getByText("粘错了的那一篇（分段全乱）")).toBeVisible();
  await expect(page.getByRole("button", { name: "确认收起" })).toBeVisible();
  // 🚨 这一条是整台看图台的理由：按收起来**不许**把那一行点开。
  await expect(page.locator("[data-opened]")).toHaveText("");
  await page.screenshot({ path: `${OUT}/history-01-confirming.png`, fullPage: true });

  // 取消就回去，什么都没发生。
  await page.getByRole("button", { name: "取消" }).click();
  await expect(page.getByText("粘错了的那一篇（分段全乱）")).toBeVisible();
  await expect(page.locator("[data-opened]")).toHaveText("");

  // 再来一次，这回确认。
  await archive.click();
  await page.getByRole("button", { name: "确认收起" }).click();
  await expect(page.getByText("粘错了的那一篇（分段全乱）")).toHaveCount(0);
  // 别的两行还在。
  await expect(page.getByText("一件小事")).toBeVisible();
  await expect(page.getByText("读完的那一篇")).toBeVisible();
  // 从头到尾没有点开任何一行。
  await expect(page.locator("[data-opened]")).toHaveText("");
  await page.screenshot({ path: `${OUT}/history-02-archived.png`, fullPage: true });
});

test("那一行本身照常点得开 —— 加一颗按钮没有把主功能弄坏", async ({ page }) => {
  await page.goto("/history.html");
  await page.getByText("一件小事").click();
  await expect(page.locator("[data-opened]")).toHaveText("r1");
});
