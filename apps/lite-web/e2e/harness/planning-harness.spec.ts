import { test, expect } from "@playwright/test";

/**
 * 构思那一屏 —— 同事 2026-09-22 的意见 1。
 *
 *	「选择题目进入写作后，无法看到完整的题目。想要看完整的题目还需要退出重新
 *	  搜索，可能不利于学生**边看题目边构思**。」
 *
 * 2026-09-21 加的那一栏题目只挂在段落和成稿两步上 —— 构思和行文各自在房间
 * 外壳之前 early-return，于是它们还挂着那行 line-clamp-2 的小字，全文只在
 * title 属性里。而「边看题目边构思」说的正是这一屏。
 *
 * 三栏挤不挤、折起来之后正文有没有变宽、那张图还剩多少地方 —— 这些是**几何**，
 * jsdom 一个字都证明不了（AGENTS.md：UI 用真浏览器看）。
 *
 * 跑法（先起看图台）：
 *   npx vite --config e2e/harness/vite.config.ts
 *   npx playwright test e2e/harness/planning-harness.spec.ts --config e2e/harness/playwright.config.ts
 */

const OUT = "e2e/harness/.shots";

test.beforeEach(async ({ page }) => {
  await page.goto("/planning.html");
});

test("🚨 构思的时候，整道题和对话、图同时在屏幕上", async ({ page }) => {
  const rail = page.getByRole("complementary", { name: "题目" });
  await expect(rail).toBeVisible({ timeout: 20_000 });

  // 整道题都在，不是两行小字加一个 title 属性。
  // 钉的是**题面的末尾**：截断的时候先没的就是它。
  await expect(rail).toContainText("请结合自身经历，写一篇文章");
  await expect(rail).toContainText("不少于800字");
  // 目标字数也在这一栏里。
  await expect(rail).toContainText("目标字数 800");

  // 对话和图同时在屏幕上 —— 三栏，不是题目挤掉其中一个。
  await expect(page.getByText("写作构思")).toBeVisible();
  await expect(page.getByText("你的思路")).toBeVisible();
  await page.screenshot({ path: `${OUT}/r5-04-planning.png`, fullPage: true });
});

test("题目那一栏折得起来，折完正文变宽", async ({ page }) => {
  const rail = page.getByRole("complementary", { name: "题目" });
  await expect(rail).toBeVisible({ timeout: 20_000 });
  const wide = (await rail.boundingBox())!.width;

  await page.getByTitle("折起题目").click();
  await expect(page.getByTitle("展开题目")).toBeVisible();
  const narrow = (await rail.boundingBox())!.width;
  expect(narrow, "折起来之后那一栏该窄下去").toBeLessThan(wide);
  // 折起来之后它还说得出自己是什么。
  await expect(rail).toContainText("题目");
  await page.screenshot({ path: `${OUT}/r5-05-planning-folded.png`, fullPage: true });

  await page.getByTitle("展开题目").click();
  await expect(page.getByTitle("折起题目")).toBeVisible();
});

test("🚨 图上那条被摆错的，她自己改得动（意见 3）", async ({ page }) => {
  // 「黑心商家哪怕赚很多钱，也是失败」现在摆在「论据 · 你找来的材料」里。
  const label = page.getByRole("button", { name: "论据 · 你找来的材料", exact: true }).last();
  await expect(label).toBeVisible({ timeout: 20_000 });
  await label.click();

  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  await menu.getByRole("menuitem", { name: /反方观点/ }).click();

  // 改完当场看得见。
  await expect(page.getByRole("button", { name: "反方观点", exact: true })).toBeVisible();
  await page.screenshot({ path: `${OUT}/r5-06-planning-rekind.png`, fullPage: true });
});
