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
 * 线索库：保留一条、起名、另起一条、中途总结、回头接着问。
 *
 * # 它守的是哪条反馈
 *
 * 2026-09-21：「如果学生只是暂时对上次的线索没有进一步的想法，想先放一放，
 * 清空了就没有记录了。所以我想能不能有一个线索库，保存学生曾提出的所有线索，
 * 学生可以随时选择暂停or开启新的线索，也可以选择中途总结。」
 *
 * 🚨 这一条最要紧的断言是**第一条线索的那句原话还在**：前一版里「换一条」
 * 是靠清空实现的，而这一版的全部意义就是那些字不再被删掉。
 *
 * # 它花模型调用，所以和上面那条分开
 *
 * 两轮探询 + 一次起名 + 一次报告生成，大约一分半。
 */
test("兴趣测试：线索库留住每一条她提出过的线索", async ({ browser }) => {
  test.setTimeout(300_000);
  const ctx = await freshAccount(browser, "awakening-library");
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
  // 三个助手各有一个颜色，选中之前就看得见 —— 进了终端整屏用的就是它。
  await page.screenshot({ path: `${SHOTS}/re-02b-guides.png`, fullPage: true });
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

  /* ── 暂时保留 → 给这条线索起名 ────────────────────────────────────────── */

  await page.getByRole("button", { name: "暂时保留兴趣线索" }).click();
  await expect(page.getByRole("heading", { name: "给这条线索起个名字" })).toBeVisible({
    timeout: 60_000,
  });
  // 🚨 候选里最后一个永远是从她原话裁出来的 —— 模型那次没回上来时它是唯一的
  // 一个，所以这一屏在任何情况下都有东西可挑。
  const choices = page.locator(".awk-option");
  await expect(choices.first()).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/re-04-naming.png`, fullPage: true });
  // 🚨 取的是卡上的**标题**，不是整张卡的文字 —— 卡面第一行是编号（「04」），
  // 拿它去当名字会让后面那几条断言在找一个到处都是的两位数。
  const chosen = (await choices.last().locator(".awk-option-copy strong").innerText()).trim();
  await choices.last().click();
  // 2026-09-22 文案改版：「就用这个名字」→「确认名称」（界面文案规则 2：
  // 按钮写「做什么」，用书面词）。
  await page.getByRole("button", { name: "确认名称" }).click();
  await expect(page).toHaveURL(/\/tree$/);

  /* ── 回来：线索库里那条还在，名字也在 ─────────────────────────────────── */

  await page.getByRole("button", { name: "继续上次的兴趣测试" }).click();
  await expect(page.getByRole("heading", { name: "兴趣测试" })).toBeVisible({
    timeout: 30_000,
  });
  await page.getByRole("button", { name: /兴趣线索库/ }).click();
  await expect(page.getByRole("heading", { name: "兴趣线索库" })).toBeVisible();
  await expect(page.getByText(chosen, { exact: false }).first()).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/re-05-library.png`, fullPage: true });

  /* ── 开启新线索：旧的那条一个字都不动 ─────────────────────────────────── */

  await page.getByRole("button", { name: /开启新线索/ }).click();
  await expect(page.getByText("INTEREST DIAGNOSTIC")).toBeVisible({ timeout: 30_000 });
  // 新线索是空的：上一条的对话不该跟过来。
  await expect(page.getByText(first)).toHaveCount(0);
  await expect(page.getByRole("button", { name: "现在总结" })).toBeDisabled();

  const second = "我们班那个总在改规则的桌游，每次玩法都不一样，我一直在想规则到底归谁定";
  await answer(second);

  /* ── 现在总结：八问没问完也交还给她一份报告 ───────────────────────────── */

  await page.getByRole("button", { name: "现在总结" }).click();
  await expect(page.getByRole("button", { name: "确认总结" })).toBeVisible();
  await page.getByRole("button", { name: "确认总结" }).click();
  await expect(page.getByRole("heading", { name: "你的兴趣印记" })).toBeVisible({
    timeout: 180_000,
  });
  await page.screenshot({ path: `${SHOTS}/re-06-early-report.png`, fullPage: true });

  // 报告里任何一处 undefined / null 都说明某个字段在半路掉了形状 ——
  // 一份只走了一问的报告正好是那些字段最容易为空的时候。
  const dirty = await page.evaluate(() => {
    const root = document.querySelector("[data-awakening-report]");
    const text = root?.textContent ?? "";
    for (const bad of ["undefined", "null", "[object Object]", "NaN"]) {
      if (text.includes(bad)) return `报告里出现了 ${bad}`;
    }
    return "";
  });
  expect(dirty, dirty).toBe("");

  /* ── 🚨 两条线索都在库里，第一条那句原话一个字都没少 ──────────────────── */

  await page.getByRole("button", { name: "回到我的树" }).click();
  await expect(page).toHaveURL(/\/tree$/);
  await page.getByRole("button", { name: /兴趣测试/ }).first().click();
  await expect(page.getByRole("heading", { name: "兴趣测试" })).toBeVisible({
    timeout: 30_000,
  });
  await page.getByRole("button", { name: /兴趣线索库/ }).click();
  await expect(page.getByRole("heading", { name: "兴趣线索库" })).toBeVisible();
  // 已经总结的那条标成「已总结」，并且仍然点得进去接着问。
  await expect(page.getByText("已总结").first()).toBeVisible();
  // 🚨 「查看兴趣印记」那条路必须真的看得见 —— 第一版它被卡自己的装饰层盖住，
  // 在 DOM 里存在、在屏幕上没有。toBeVisible 会做命中判定，挡住就红。
  await expect(page.getByRole("button", { name: "查看兴趣印记" }).first()).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/re-07-library-two.png`, fullPage: true });

  // 这是整条 spec 的靶心：换了一条线索之后，第一条里她写的那句话还在。
  await page.getByText(chosen, { exact: false }).first().click();
  await expect(page.getByText("INTEREST DIAGNOSTIC")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(first)).toBeVisible();
  // 🚨 接着问的是她停在的那一问，不是开场那一问。原来这一格恒给开场白，
  // 等于把一个已经答到第 2 问的学生往回推了一问。
  await expect(page.getByText("第 2 / 8 步")).toBeVisible();
  await expect(page.getByText("上次的兴趣测试在你的树上")).toHaveCount(0);
  await page.screenshot({ path: `${SHOTS}/re-08-back-on-first.png`, fullPage: true });

  /* ── 在这条线索上也总结一次，于是她有两份印记 ─────────────────────────── */

  await page.getByRole("button", { name: "现在总结" }).click();
  await page.getByRole("button", { name: "确认总结" }).click();
  await expect(page.getByRole("heading", { name: "你的兴趣印记" })).toBeVisible({
    timeout: 180_000,
  });
  await page.getByRole("button", { name: "回到我的树" }).click();
  await expect(page).toHaveURL(/\/tree$/);

  /* ── 🚨 「查看兴趣印记」要回得到之前那一份 ────────────────────────────── */

  // 2026-09-21 的反馈：「查看兴趣印记点进去后，只能看到上一次兴趣测试的印记，
  // 无法回顾之前的。」原来这条入口只带着最近那一份的 id。
  await page.getByRole("button", { name: "查看兴趣印记" }).click();
  await expect(page.getByRole("heading", { name: "兴趣印记" })).toBeVisible({
    timeout: 30_000,
  });
  const imprints = page.locator(".awk-option");
  await expect(imprints).toHaveCount(2);
  await page.screenshot({ path: `${SHOTS}/re-09-imprints.png`, fullPage: true });

  // 挑**旧的那一份**（表是新的在前，所以最后一行是先做的那条线索）。
  await imprints.last().click();
  await expect(page.getByRole("heading", { name: "你的兴趣印记" })).toBeVisible({
    timeout: 30_000,
  });
  // 它确实是另一份：这一份属于先做的那条线索（桌游那条）。
  await expect(page.getByText("桌游", { exact: false }).first()).toBeVisible();
  await page.screenshot({ path: `${SHOTS}/re-10-older-imprint.png`, fullPage: true });

  // 退一步回表，再挑另一份 —— 两份之间来回走得通。
  await page.getByRole("button", { name: "返回印记列表" }).click();
  await expect(page.getByRole("heading", { name: "兴趣印记" })).toBeVisible();
  await imprints.first().click();
  await expect(page.getByRole("heading", { name: "你的兴趣印记" })).toBeVisible({
    timeout: 30_000,
  });

  await ctx.close();
});
