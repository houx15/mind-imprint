import { test, expect } from "@playwright/test";

/**
 * 「依据」那个弹层，在真浏览器里按一次。
 *
 * 产品负责人点名要的就是它：「and we also need to tell teacher the rationale
 * or the real logic of our comment there (maybe a modal?)」。
 *
 * 🚨 为什么不能只靠 jsdom：memory `control-that-is-not-wired-2026-09-22` ——
 * 按钮长得像能用不等于它接线了；那一轮有一个控件的 click 被外层的
 * onPointerDown 吞掉，只有真浏览器抓得到。这里要证的正是同一件事：
 * 按下去有东西打开、里面有字、关得掉。
 *
 * 跑法（端口挑一个别的会话没在用的）：
 *   E2E_HARNESS_PORT=5241 npx vite --config e2e/harness/vite.config.ts
 *   E2E_HARNESS_PORT=5241 npx playwright test --config e2e/harness/playwright.config.ts grading-harness
 */

const OUT = "e2e/harness/.shots";

test("依据：有来源的那条打得开，三行齐；老师自己写的那条没有这个按钮", async ({ page }) => {
  await page.goto("/grading.html");

  // 三条意见都在。
  await expect(page.getByText("这是分析句的位置", { exact: false })).toBeVisible();

  // 🚨 按钮是**有条件**渲染的：第三条（老师自己写的，两样来源都空）不该有。
  const buttons = page.getByRole("button", { name: /依据$/ });
  await expect(buttons).toHaveCount(2);

  await page.screenshot({ path: `${OUT}/basis-00-card.png`, fullPage: true });

  // 第一条：维度 + 对应毛病 + 学生原句，三行都该在。
  await buttons.first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  for (const row of ["维度", "对应毛病", "学生原句"]) {
    await expect(dialog.getByText(row, { exact: true })).toBeVisible();
  }
  await expect(dialog.getByText("内容", { exact: true })).toBeVisible();
  await expect(dialog.getByText("举了例子，没有解释", { exact: true })).toBeVisible();

  // 🚨 弹层里不许出现占位符 —— pointHasBasis 的全部意义就是「没东西就不给
  // 按钮」，所以打开的这一个每一行都该是填满的。
  const text = (await dialog.innerText()) ?? "";
  for (const placeholder of ["待填写", "—", "undefined", "null"]) {
    expect(text, `弹层里出现了占位符「${placeholder}」`).not.toContain(placeholder);
  }
  await page.screenshot({ path: `${OUT}/basis-01-full.png`, fullPage: true });

  // 关得掉。
  await dialog.getByRole("button", { name: "关闭" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  // 第二条：毛病没匹配上（被 SanitizeProvenance 清空），只该有两行。
  await buttons.nth(1).click();
  const second = page.getByRole("dialog");
  await expect(second).toBeVisible();
  await expect(second.getByText("维度", { exact: true })).toBeVisible();
  await expect(second.getByText("对应毛病", { exact: true })).toHaveCount(0);
  await expect(second.getByText("学生原句", { exact: true })).toBeVisible();
  await page.screenshot({ path: `${OUT}/basis-02-partial.png`, fullPage: true });

  // Esc 也关得掉。
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("依据：弹层没有横向溢出，也没有原样印出来的 markdown 记号", async ({ page }) => {
  await page.goto("/grading.html");
  await page.getByRole("button", { name: /依据$/ }).first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  const overflow = await dialog.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflow, "弹层横向溢出了").toBeLessThanOrEqual(1);

  const text = (await dialog.innerText()) ?? "";
  expect(text, "说明里有 ** —— 这些字直接渲染成纯文本，星号会原样印出来").not.toContain("**");
});
