import { test, expect } from "@playwright/test";

/**
 * R4 的看图台 —— 讲义那套词在**真浏览器**里长什么样。
 *
 * jsdom 看不见布局：五句型比原来的四步多一行，引导框在 320px 的左栏里
 * 会不会挤、会不会溢出，只有真的渲染出来才知道
 * （[[test-logic-not-endless-frontend]]：UI 用眼睛看，不靠 jsdom 断言）。
 *
 * 跑法：
 *   npx vite --config e2e/harness/vite.config.ts     # 另开一个终端
 *   npx playwright test --config e2e/harness/playwright.config.ts teaching-harness
 */

test("段落页左栏：五句型五行都在，没有溢出", async ({ page }) => {
  await page.goto("/flow.html");
  await page.getByTestId("go-snippets").click();

  const box = page.locator("aside");
  await expect(box).toBeVisible();

  // 讲义（五）的五句，逐个看得见。
  for (const label of ["观点句", "阐释句", "材料句", "分析句", "结论句"]) {
    await expect(box.getByText(label, { exact: false }).first()).toBeVisible();
  }

  // 🚨 原来那四步的名字不该还在 —— 两套词同时出现，她不知道该听哪一套。
  for (const gone of ["分论点句", "回扣"]) {
    await expect(box.getByText(gone, { exact: true })).toHaveCount(0);
  }

  // 🚨 没有横向溢出：左栏是 320px，五句的说明都不短。
  const overflow = await box.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflow, "左栏横向溢出了").toBeLessThanOrEqual(1);

  // 🚨 markdown 记号不许原样印出来（2026-09-20 就是在看图台上发现的）。
  const text = (await box.innerText()) ?? "";
  expect(text, "说明里有 ** —— 这些字直接渲染成纯文本，星号会原样印出来").not.toContain("**");

  await page.screenshot({ path: ".shots/r4-guide-five-sentences.png", fullPage: true });
});
