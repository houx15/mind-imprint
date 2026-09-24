import { test, expect } from "@playwright/test";

/**
 * 「当前档位」—— 在真浏览器里量一遍。
 *
 * 跑法：
 *   E2E_HARNESS_PORT=5251 npx vite --config e2e/harness/vite.config.ts
 *   E2E_HARNESS_PORT=5251 npx playwright test --config e2e/harness/playwright.config.ts band-harness
 */

const OUT = "e2e/harness/.shots";

test("档位看得见，但那句话和那行小字没有被数字压掉", async ({ page }) => {
  await page.goto("/band.html");

  const bands = page.locator(".mk-comment-band");
  // 三条通篇的意见各有一块，单段那一条没有。
  await expect(bands).toHaveCount(3);

  // ① 数字确实比周围大 —— 一眼看得见是它存在的理由。
  const num = bands.first().locator(".mk-comment-band__num");
  const label = bands.first().locator(".mk-comment-band__label");
  const numSize = await num.evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
  const labelSize = await label.evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
  expect(numSize).toBeGreaterThan(labelSize);

  // ② 🚨 但旁边那句话和底下那行小字**都要真的看得见**。
  //
  // 一个孤零零的「第 3 档」会被读成考试分数；说明它量的是什么的那两处，
  // 是把它改读成「我现在卡在哪一层」的全部依据。
  await expect(label).toBeVisible();
  const note = bands.first().locator(".mk-comment-band__note");
  await expect(note).toBeVisible();
  await expect(note).toContainText("不是分数");
  const noteBox = (await note.boundingBox())!;
  expect(noteBox.height).toBeGreaterThan(0);

  // ③ 数字和说明在同一块里，没有被挤到两行之外或者溢出。
  const bandBox = (await bands.first().boundingBox())!;
  const numBox = (await num.boundingBox())!;
  expect(numBox.x).toBeGreaterThanOrEqual(bandBox.x);
  expect(numBox.x + numBox.width).toBeLessThanOrEqual(bandBox.x + bandBox.width + 1);

  // ④ 🚨 单段那一条**没有**档位：五档是给一整篇用的尺子。
  const blockCase = page.locator('[data-case*="单段"]');
  await expect(blockCase.locator(".mk-comment-band")).toHaveCount(0);
  // 但它仍然有四层等级那一排 —— 去掉的只有档位。
  //
  // 🚨 钉那一排本身，不钉「结构」两个字：那两个字在四层徽章上和意见正文里
  // 都出现，裸的 text= 会撞 strict mode（memory:
  // inserting-a-step-rots-every-older-walk-2026-09-21 记过同一个坑）。
  await expect(blockCase.locator("span", { hasText: /·(可优化|需修改|已达标|本轮未看)/ }).first()).toBeVisible();

  await page.screenshot({ path: `${OUT}/band-00-all.png`, fullPage: true });
});
