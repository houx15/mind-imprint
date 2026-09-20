import { expect, test } from "@playwright/test";

import { freshAccount } from "./freshAccount";

/**
 * 做到一半走掉，再回来。
 *
 * # 它守的是哪条反馈
 *
 * 2026-09-20：「学生反馈兴趣探索做到一半不想做了，然后点击离开就是直接到
 * 兴趣树的页面了，没有办法到能量测试。」
 *
 * 从前回来只能接着停下的那一屏往下走，这条线是单向的 —— 她越过能量测试之后
 * 就再也回不去。现在复访落在入口那一屏，四件事摆开。
 *
 * # 为什么单独一条 spec，而不是塞进 awakening-eyeball
 *
 * 那一条走的是**走完一趟**（十四屏，一次真实的模型调用一次报告生成，一分半）。
 * 这一条走的是**没走完**，四步就到，一次模型调用都不发。两个前提不一样，
 * 混在一起只会让其中一个变成另一个的尾巴。
 *
 * 跑（三个值由 online.config.ts 自己定）：
 *
 *     pnpm --filter @mind-imprint/lite-web exec playwright test \
 *       -c e2e/online.config.ts awakening-reentry
 */

const SHOTS = process.env.E2E_SHOTS ?? "e2e/.shots/awakening";

test("兴趣测试：做到一半离开，回来能到能量测试", async ({ browser }) => {
  const ctx = await freshAccount(browser, "awakening-reentry");
  const page = await ctx.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });

  await page.goto("/tree");
  await page.getByRole("button", { name: "开始兴趣测试" }).first().click();
  await expect(page).toHaveURL(/\/tree\/awakening$/);

  // 走到能量线索那一屏，然后中途离开。
  await expect(page.getByText("AWAKENING_PROTOCOL")).toBeVisible({ timeout: 30_000 });
  await page.getByRole("button", { name: "跳过开场剧情" }).click();
  await page.getByRole("button", { name: /加入觉醒者联盟/ }).click();
  await expect(page.getByRole("heading", { name: "能量线索" })).toBeVisible();
  await page.getByRole("button", { name: "离开" }).click();
  await expect(page).toHaveURL(/\/tree$/);

  // 树上那条入口改口了 —— 一趟没走完不算「做过了」，所以它说的是「继续」。
  const door = page.getByRole("button", { name: "继续上次的兴趣测试" });
  await expect(door).toBeVisible({ timeout: 30_000 });
  await door.click();

  /* ── 回来落在入口那一屏，不是停下的那一屏 ─────────────────────────────── */

  await expect(page.getByRole("heading", { name: "兴趣测试" })).toBeVisible({
    timeout: 30_000,
  });
  // 开场那部片子不该再放一遍。
  await expect(page.getByText("AWAKENING_PROTOCOL")).toHaveCount(0);
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${SHOTS}/re-01-hub.png`, fullPage: true });

  // 🚨 这一条就是那条反馈本身：她到得了能量测试。
  await page.getByRole("button", { name: /能量测试/ }).click();
  await expect(page.getByRole("heading", { name: "能量线索" })).toBeVisible({
    timeout: 30_000,
  });
  await page.screenshot({ path: `${SHOTS}/re-02-energy.png`, fullPage: true });

  await ctx.close();
});
