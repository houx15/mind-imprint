import { expect, test } from "@playwright/test";

import { freshAccount } from "./freshAccount";

/**
 * 觉醒协议 · 用真浏览器看一遍。
 *
 * # 它和 awakening-walk.mjs 的分工
 *
 * 那一条打接口，证明链子是通的（节点推进、词写回树、第二趟知道她有什么）。
 * 这一条**只管看得见的那一半**：每一屏真的渲染了、按钮真的能点、房间里没有
 * lite 的导航轨、出门落在暖色的报告上。
 *
 * jsdom 看不见布局、看不见一张没加载出来的图、看不见一块盖住按钮的浮层。
 * 2026-08-30 的教训是 344 个测试全绿而导出的 PNG 是全白的 —— 所以这里每一屏
 * 都截一张图，留给人看。
 *
 * 跑：
 *
 *     E2E_API_BASE=https://mind-api.uni-robot.cn E2E_JOIN_CODE=G624-UXFE \
 *     pnpm --filter @mind-imprint/lite-web exec playwright test \
 *       -c e2e/online.config.ts awakening-eyeball
 */

const SHOTS = process.env.E2E_SHOTS ?? "e2e/.shots/awakening";

test("觉醒协议：十四屏走一遍，每一屏留一张图", async ({ browser }) => {
  const ctx = await freshAccount(browser, "awakening-eyeball");
  const page = await ctx.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });

  const shot = async (name: string) => {
    await page.waitForTimeout(400); // 让入场动画落定，否则截到一半透明
    await page.screenshot({ path: `${SHOTS}/${name}.png`, fullPage: true });
  };

  /* ── 门槛：空树上那条邀请 ─────────────────────────────────────────────── */

  await page.goto("/tree");
  // 一棵空树上必须**明确**出现那条邀请 —— 状态未知时不显示，所以这里的
  // 出现本身也证明 GET /api/v1/awakening 通了。
  const invite = page.getByRole("button", { name: "开始觉醒协议" });
  await expect(invite.first()).toBeVisible({ timeout: 30_000 });
  await shot("00-tree-empty-invite");

  await invite.first().click();
  await expect(page).toHaveURL(/\/tree\/awakening$/);

  /* ── 房间里不该有产品的壳 ─────────────────────────────────────────────── */

  // 导航轨是 lite 每一屏都有的东西。房间盖上来之后它必须被盖住 ——
  // 用命中测试，不看 bounding box：一个还在 DOM 里、被浮层盖住的元素
  // 的 box 仍然是对的（memory: walk-the-loop-and-show-the-result）。
  const railHidden = await page.evaluate(() => {
    const rail = document.querySelector("nav, [data-lite-rail]");
    if (!rail) return true;
    const r = rail.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return true;
    const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return !rail.contains(top);
  });
  expect(railHidden, "房间盖上来之后，导航轨不该还能被点到").toBe(true);

  /* ── 1 开场 ───────────────────────────────────────────────────────────── */

  await expect(page.getByText("AWAKENING_PROTOCOL")).toBeVisible();
  await shot("01-boot");
  await page.getByRole("button", { name: "跳过" }).click();

  /* ── 2 序章 ───────────────────────────────────────────────────────────── */

  await expect(page.getByText("你的判断仍然属于你。")).toBeVisible();
  await shot("02-world");
  // 走长的那条：先看清楚 AI。
  await page.getByRole("button", { name: /先看清楚 AI/ }).click();

  /* ── 3 提醒 ───────────────────────────────────────────────────────────── */

  await expect(page.getByText("先看清 AI，再决定要不要交给它。")).toBeVisible();
  await shot("03-warning");
  await page.getByRole("button", { name: "打开历史档案" }).click();

  /* ── 4 历史档案 ───────────────────────────────────────────────────────── */

  await expect(page.getByText("历史档案 01：认知让步")).toBeVisible();
  // 那张警示图必须真的加载出来 —— 一个碎图标 jsdom 永远看不见。
  const img = page.locator("figure img").first();
  await expect(img).toBeVisible();
  const loaded = await img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0);
  expect(loaded, "档案那张图没有加载出来（CDN 或路径不对）").toBe(true);

  for (const t of ["被丢掉的主动思考", "AI 接管了三个动作", "瘫坐的孩子"]) {
    await page.getByRole("button", { name: new RegExp(t) }).click();
  }
  await expect(page.getByText("档案确认题")).toBeVisible();
  await shot("04-archive");
  // 先选错一次 —— 错误次数是过程数据，不减分。
  await page.getByRole("button", { name: "A. 大脑本身" }).click();
  await expect(page.getByText(/大脑还在/)).toBeVisible();
  await page.getByRole("button", { name: "C. 判断的责任" }).click();
  await page.getByRole("button", { name: "进入三个实验" }).click();

  /* ── 5 AI 底牌 ────────────────────────────────────────────────────────── */

  for (let i = 0; i < 3; i++) {
    await expect(page.getByText("生成式 AI 的底牌")).toBeVisible();
    if (i === 0) await shot("05-deck");
    // 每张牌选第一个选项，两个选项都能推进。
    await page.locator(".awk-card").first().click();
    await page.getByRole("button", { name: /翻开下一张|三张底牌已经翻开/ }).click();
  }
  await expect(page.getByText("三张底牌已经翻开")).toBeVisible();
  await shot("06-deck-done");
  await page.getByRole("button", { name: "重新面对选择" }).click();

  /* ── 6 重新决定 ───────────────────────────────────────────────────────── */

  await shot("07-rejoin");
  await page.getByRole("button", { name: "加入并校准能量线索" }).click();

  /* ── 7 能量卡牌 ───────────────────────────────────────────────────────── */

  for (let i = 0; i < 4; i++) {
    await expect(page.getByText("能量线索")).toBeVisible();
    if (i === 0) await shot("08-energy");
    // 每组选两张。
    await page.locator(".awk-card").nth(0).click();
    await page.locator(".awk-card").nth(3).click();
    await page.getByRole("button", { name: /下一组|完成校准/ }).click();
  }
  await expect(page.getByText("你的能量方向")).toBeVisible();
  await shot("09-energy-result");
  await page.getByRole("button", { name: "选择你的印记" }).click();

  /* ── 8 印记助手 ───────────────────────────────────────────────────────── */

  await expect(page.getByText("选择你的印记")).toBeVisible();
  await shot("10-navigator");
  await page.getByRole("button", { name: /资深向导/ }).click();
  await page.getByRole("button", { name: "确认连接" }).click();

  /* ── 9 终端 ───────────────────────────────────────────────────────────── */

  await expect(page.getByText("兴趣探询")).toBeVisible();
  await shot("11-terminal-empty");

  const answers = [
    "最近老是刷到潮汐发电的视频，一个海湾里的闸门一开一合就能发电，我看了四十分钟还在看",
    "最吸引我的是那个闸门的节奏，它不是一直转，而是要等潮水到某个高度才动一次",
    "我家在海边，小时候赶海要看潮汐表，我一直觉得那张表很神奇，现在发现它跟发电是同一件事",
    "我想不通的是，既然潮汐这么规律，为什么全世界用潮汐发电的地方这么少",
    "为什么潮汐发电在少数海岸能建起来，在大多数海岸却建不起来？",
    "我需要先读懂潮差和地形的基础概念，再看一两个真的建成了的案例",
    "我猜是因为要有很大的潮差和很窄的海湾，但如果看到平缓海岸也有成功的例子，我会改想法",
    "我想做一个给同学看的图解，让他们一眼看出为什么我家那片海滩建不了",
  ];
  for (const [i, a] of answers.entries()) {
    const box = page.locator("textarea");
    await box.fill(a);
    await page.getByRole("button", { name: "发送" }).click();
    // 印记回完这一轮之前不往下走。
    //
    // 判据是「正在回复」那个指示消失，不是气泡的条数、也不是输入框可用：
    //   · 第一轮之前那个开场问题也是一个气泡，发完第一句它就不再渲染，
    //     所以条数不是单调加一的；
    //   · **最后一轮之后输入框整个不存在了**（换成了「完成探询」），
    //     按可用等会在那里挂到超时。
    const busy = page.getByText("印记正在回复");
    await busy.waitFor({ state: "visible", timeout: 15_000 }).catch(() => undefined);
    await busy.waitFor({ state: "detached", timeout: 150_000 });
    if (i === 1) await shot("12-terminal-mid");
  }
  await expect(page.getByText("八个问题已经问完")).toBeVisible({ timeout: 90_000 });
  await shot("13-terminal-done");
  await page.getByRole("button", { name: "完成探询" }).click();

  /* ── 10 三层 + 11 下一步 ──────────────────────────────────────────────── */

  await expect(page.getByText("你刚才走过的三层")).toBeVisible();
  await shot("14-lens");
  await page.getByRole("button", { name: /我好奇什么/ }).click();
  await page.getByRole("button", { name: "确认选择" }).click();

  await expect(page.getByText("为这条线索留下下一步")).toBeVisible();
  await shot("15-challenge");
  await page.getByRole("button", { name: /查资料，做比较/ }).click();
  await page.getByRole("button", { name: "确认下一步" }).click();

  /* ── 12 天赋卡牌 ──────────────────────────────────────────────────────── */

  await expect(page.getByText("能力卡牌")).toBeVisible();
  await shot("16-talent-pick");
  for (let i = 0; i < 5; i++) await page.locator(".awk-card").nth(i).click();
  await page.getByRole("button", { name: "进入三堆整理" }).click();

  await expect(page.getByText("三堆整理")).toBeVisible();
  await shot("17-talent-sort");
  // 五张各归一堆。每张卡下面是三个堆的按钮。
  for (let i = 0; i < 5; i++) {
    const lane = ["有能量", "会做但消耗", "想发展"][i % 3]!;
    await page.getByRole("button", { name: lane }).first().click();
  }
  await page.getByRole("button", { name: "生成报告" }).click();

  /* ── 13 报告 ──────────────────────────────────────────────────────────── */

  await expect(page.getByText("你的兴趣印记")).toBeVisible({ timeout: 300_000 });
  await shot("18-report");

  // 🚨 报告里每个词都必须真的有字。2026-09-19 那次接口走查抓到的就是这一层：
  // 结构少了 json 标签，卡片全是空的，而树上的词是对的。
  const cards = page.locator("section").filter({ hasText: "你在追什么" });
  await expect(cards).toBeVisible();
  const emptyCard = await page.evaluate(() => {
    const sec = [...document.querySelectorAll("section")].find((s) =>
      s.querySelector("h2")?.textContent?.includes("你在追什么"),
    );
    if (!sec) return "找不到「你在追什么」那一块";
    const text = sec.textContent ?? "";
    if (text.includes("undefined")) return "卡片里出现了 undefined";
    return "";
  });
  expect(emptyCard, emptyCard).toBe("");

  /* ── 出门：回到树，词在上面 ───────────────────────────────────────────── */

  await page.getByRole("button", { name: "回到我的树" }).click();
  await expect(page).toHaveURL(/\/tree$/);
  // 房间关上之后，导航轨回来了。
  await expect(page.locator("nav").first()).toBeVisible({ timeout: 30_000 });
  await shot("19-back-on-tree");

  await ctx.close();
});
