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

/**
 * 保留线索 / 新的探索 / 现在总结。
 *
 * # 它守的是哪条反馈
 *
 * 2026-09-20：「兴趣探索过程有些长，如果学生想要中途退出直接总结，或者暂时
 * 放一放这条线索就不行。」
 *
 * 三件事在这一条里都真的做一遍：**保留**之后回来接得上、**新的探索**真的把
 * 上次那几段清掉（而不是嘴上说清了、对话还铺在那儿）、**现在总结**在八问
 * 没问完时也真的生成一份报告。
 *
 * # 它花模型调用，所以和上面那条分开
 *
 * 上面那条四步就到、一次调用都不发。这一条要两轮探询加一次生成（选词 +
 * 散文），大约一分钟。
 */
test("兴趣测试：保留线索、换一条重新问、中途总结", async ({ browser }) => {
  test.setTimeout(300_000);
  const ctx = await freshAccount(browser, "awakening-hold");
  const page = await ctx.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });

  await page.goto("/tree");
  await page.getByRole("button", { name: "开始兴趣测试" }).first().click();
  await expect(page.getByText("AWAKENING_PROTOCOL")).toBeVisible({ timeout: 30_000 });
  await page.getByRole("button", { name: "跳过开场剧情" }).click();
  await page.getByRole("button", { name: /加入觉醒者联盟/ }).click();

  // 能量卡牌四组，然后选助手 —— 这两屏是进探询的必经之路。
  for (let i = 0; i < 4; i++) {
    await expect(page.getByRole("heading", { name: "能量线索" })).toBeVisible();
    await page.locator(".awk-option").nth(0).click();
    await page.locator(".awk-option").nth(3).click();
    await page.getByRole("button", { name: /下一组|完成校准/ }).click();
  }
  await page.getByRole("button", { name: "选择你的印记" }).click();
  await page.getByRole("button", { name: /资深向导/ }).click();
  await page.getByRole("button", { name: "确认连接" }).click();

  /* ── 终端：一段都没答时总结不了 ───────────────────────────────────────── */

  await expect(page.getByText("INTEREST DIAGNOSTIC")).toBeVisible({ timeout: 30_000 });
  // 🚨 没有语料就没有报告。这个按钮这时必须按不动，而不是按下去生成一份
  // 里面一个字都不是她写的报告。
  await expect(page.getByRole("button", { name: "现在总结" })).toBeDisabled();
  await page.screenshot({ path: `${SHOTS}/re-03-terminal-exit.png`, fullPage: true });

  const answer = async (text: string) => {
    await page.locator("textarea").fill(text);
    await page.getByRole("button", { name: "发送" }).click();
    const busy = page.getByText("印记正在回复");
    await busy.waitFor({ state: "visible", timeout: 15_000 }).catch(() => undefined);
    await busy.waitFor({ state: "detached", timeout: 150_000 });
  };

  const first =
    "最近老是刷到潮汐发电的视频，一个海湾里的闸门一开一合就能发电，我看了四十分钟还在看";
  await answer(first);
  await expect(page.getByRole("button", { name: "现在总结" })).toBeEnabled();

  /* ── 暂时保留：出门，回来接得上 ───────────────────────────────────────── */

  await page.getByRole("button", { name: "暂时保留兴趣线索" }).click();
  await expect(page).toHaveURL(/\/tree$/);
  await page.getByRole("button", { name: "继续上次的兴趣测试" }).click();

  await expect(page.getByRole("heading", { name: "兴趣测试" })).toBeVisible({
    timeout: 30_000,
  });
  // 入口那一屏这时说的是「保留下来的线索」，并且旁边摆着换一条的路。
  await expect(page.getByRole("button", { name: /继续上次保留的兴趣线索/ })).toBeVisible();
  await expect(page.getByRole("button", { name: /新的探索/ })).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/re-04-hub-held.png`, fullPage: true });

  /* ── 新的探索：上次那一段真的不在了 ───────────────────────────────────── */

  await page.getByRole("button", { name: /新的探索/ }).click();
  // 问一次那一屏：它要把「会被清空」说清楚，所以它也要被人眼看过。
  await expect(page.getByRole("button", { name: "确认清空并重新开始" })).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/re-04b-fresh-confirm.png`, fullPage: true });
  await page.getByRole("button", { name: "确认清空并重新开始" }).click();
  await expect(page.getByText("INTEREST DIAGNOSTIC")).toBeVisible({ timeout: 30_000 });
  // 🚨 判据是她那句原话不在屏幕上，不是轮数 —— 清空要是只清了计数，
  // 对话还铺在那儿，那就是假清空。
  await expect(page.getByText(first)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "现在总结" })).toBeDisabled();
  await page.screenshot({ path: `${SHOTS}/re-05-fresh.png`, fullPage: true });

  /* ── 现在总结：八问没问完也交还给她一份报告 ───────────────────────────── */

  await answer(
    "我家在海边，小时候赶海要看潮汐表，我一直觉得那张表很神奇，现在发现它跟发电是同一件事",
  );
  await page.getByRole("button", { name: "现在总结" }).click();
  await expect(page.getByRole("button", { name: "确认总结" })).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/re-05b-summarize-confirm.png`, fullPage: true });
  await page.getByRole("button", { name: "确认总结" }).click();

  await expect(page.getByRole("heading", { name: "你的兴趣印记" })).toBeVisible({
    timeout: 180_000,
  });
  await page.screenshot({ path: `${SHOTS}/re-06-early-report.png`, fullPage: true });

  // 报告里任何一处 undefined / null 都说明某个字段在半路掉了形状 ——
  // 一份只走了两问的报告正好是那些字段最容易为空的时候。
  const dirty = await page.evaluate(() => {
    const root = document.querySelector("[data-awakening-report]");
    const text = root?.textContent ?? "";
    for (const bad of ["undefined", "null", "[object Object]", "NaN"]) {
      if (text.includes(bad)) return `报告里出现了 ${bad}`;
    }
    return "";
  });
  expect(dirty, dirty).toBe("");

  await page.getByRole("button", { name: "回到我的树" }).click();
  await expect(page).toHaveURL(/\/tree$/);
  await expect(page.getByRole("button", { name: "再做一次兴趣测试" })).toBeVisible({
    timeout: 30_000,
  });

  await ctx.close();
});
