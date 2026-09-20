import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * full-loop-walk —— 从**产品自己的两个入口**走完阅读和写作。
 *
 * # 和 reading-walk / writing-walk 的分工
 *
 * 那两条 walk 从落地页进：`/readings` 粘一篇文章，`/writings` 打一句话。它们
 * 守的是房间本身。**但学生不是从落地页进来的。** 导航第一格是探索，`/` 落在
 * 探索地图上；写作的由头来自她自己的树。落地页那个输入框是「我手头已经有一篇
 * 要读」时才用的门，不是这条链子的起点。
 *
 * 所以这一条走的是那条真的链子，两个入口各守一段：
 *
 *   探索地图 → 一颗星 → 现在读 → 阅读室（没有正文，粘进来）→ 带读 → 完成
 *     → 报告 → 「可以加进你的兴趣树」，她点「加入」
 *   → 兴趣树上多出那个词 → 点开它 → 继续深挖 → 去写
 *     → 写作间 → 设定 → 结构 → 段落 → 成稿 → 完成这篇
 *
 * # 为什么这两段必须连着跑，不能各跑各的
 *
 * 树上的词不是凭空有的：它由**完成一次阅读**采出来，而且 2026-09-07 之后还要
 * 她在报告上点头才上树（`reports/TreeProposals.tsx`）。也就是说「从兴趣树进写作
 * 间」这个入口，前提是她已经读完过一篇。第二段于是不是一条独立的 walk，它是
 * 第一段的下游 —— 中间那三步（采集入队 → 报告上提出 → 她点「加入」）是这两个
 * 入口之间唯一的路，断在哪一步，写作那个入口就是不存在的。
 *
 * 故 `mode: "serial"`，两条测试共用一个 context：第一条把词种上树，第二条才
 * 有门可进。
 *
 * # 一个刚注册的学生
 *
 * 用 `freshAccount` 而不是种子账号（[[e2e-preconditions-not-file-order]]）：
 * 树必须是空的，否则第二段点开的那个词可能是上一次跑留下的，而不是这一次读出来
 * 的 —— 那样这条 walk 就证明不了这条链子接得上。
 *
 * # 会真的花钱
 *
 * 星图生成、带读一轮、她的一句追问、阅读报告、采集、四颗种子、写作开场、一轮
 * 结构、段落引导、写作报告 —— 十次以上真实的模型调用。所以两条测试各自把
 * timeout 抬到自己的量级，不用套件默认的 300 秒。
 */

test.describe.configure({ mode: "serial" });

const RUN = Date.now().toString(36);

let ctx: BrowserContext;
let page: Page;

/** 第一段结束时她点「加入」的那个词。第二段靠它在树上找到入口。 */
let acceptedWord = "";

/**
 * 她自己粘进阅读室的正文。
 *
 * 🚨 从地图进来的那一篇**通常没有正文**：`readNow` 只替她试一次抓取，抓不到是
 * 正常结果（NewsSheet.tsx 里那句 `.catch(() => undefined)`），阅读室摆出粘贴框，
 * 而原文那一页刚刚已经开在旁边了。这段字就是她从那一页复制过来的东西。
 *
 * 空行是有意义的：服务端按空行切段（`reading_blocks.go`）。
 */
const PASTED_BODY = [
  "过去十年，全球太阳能装机容量增长了大约十倍。推动这件事的不是某一项突破性发明，而是制造规模、供应链和融资成本三件事同时变便宜。",
  "成本下降的幅度常被单独拎出来当作结论：组件价格在这十年里下降了八成以上。但价格只是发电成本的一部分，土地、并网、运维和资金成本在不同国家差别极大，同样的组件价格并不意味着同样的电价。",
  "真正的瓶颈已经从「发电贵不贵」转移到「电什么时候来」。太阳能的出力集中在正午前后，而用电高峰往往在傍晚，两者之间的错位要靠储能、需求响应或跨区输电来填。",
  "所以，如果储能和电网的问题不解决，继续增加装机带来的边际收益会递减：白天多出来的电卖不掉，甚至要被弃掉。十年的增长是真实的，但把它直接外推到下一个十年，是一种过于轻松的乐观。",
].join("\n\n");

/** 印记真的回了话的那一条气泡。带 `aria-label` 的是「正在打字」，不算。 */
function assistantReplies(p: Page) {
  return p.locator('[data-role="assistant"]:not([aria-label])');
}

test.beforeAll(async ({ browser }) => {
  ctx = await freshAccount(browser, "full-loop");
  page = await ctx.newPage();

  // 🚨 星球是**一直在飘的**（`.exp-drift-*`，26–35 秒一圈，`infinite`）。
  // Playwright 点之前要等元素停下来，而它永远不停 —— 第一次跑就是这么挂的：
  // 「element is not stable」重试到 15 秒超时，报出来的样子像是星球点不动。
  //
  // 不用 `force: true` 绕过去：那会连「这个按钮真的能点吗」一起跳过，而这正是
  // 这一段要证明的事。改成打开 `prefers-reduced-motion` —— explore.css 末尾
  // 那一节本来就为它把星球钉在终态上，所以这走的是产品自己的一条真路径
  // （一个对动效敏感的学生看到的就是这一屏），不是测试专用的后门。
  // 树那一屏（tree.css）和 index.css 里的动画同样认这个偏好。
  await page.emulateMedia({ reducedMotion: "reduce" });

  // 🚨 「现在读」会 `window.open` 原文那一页（NewsSheet.tsx，而且必须在 await
  // 之前开，否则被当弹窗拦掉）。这里把它关掉，免得后面的断言落到那一页上。
  ctx.on("page", (opened) => {
    if (opened !== page) void opened.close().catch(() => undefined);
  });
});

test.afterAll(async () => {
  await ctx?.close();
});

/* ── 入口一 · 探索地图 ─────────────────────────────────────────────────── */

test("入口一：探索地图上的一颗星 → 阅读室 → 完成 → 在报告上把词加进树", async () => {
  // 星图生成（十二个源 + 一次模型调用）、带读一轮、她的一句追问、阅读报告
  // （旗舰调用，几十秒）、采集。
  test.setTimeout(1_500_000);

  // ── `/` 就是探索地图，不是阅读室 ────────────────────────────────────────
  await page.goto("/");
  await expect(page.getByRole("tab", { name: "今日探索地图" })).toHaveAttribute(
    "aria-selected",
    "true",
    { timeout: 60_000 },
  );

  // ── 今天有星图 ─────────────────────────────────────────────────────────
  // 第一个打开的人触发抓取 + 一次模型调用，要几秒到几十秒。等不到的时候，把
  // 屏幕上那张卡的原话带出来 —— 「没有星球可点」和「今天生成失败了，原因是 X」
  // 是两件事，报错必须说得出是哪一件。
  const planets = page.locator(".exp-planet");
  try {
    await expect(planets.first()).toBeVisible({ timeout: 240_000 });
  } catch {
    const card = page.locator("text=/今天没有星图|星图读取失败|正在生成今天的星图/").first();
    const said = (await card.count()) ? (await card.locator("..").innerText()).trim() : "（屏幕上什么都没说）";
    throw new Error(`探索地图上一颗星都没有，阅读的入口不存在。屏幕上说：\n${said}`);
  }
  const count = await planets.count();
  expect(count, "星图应该是五颗").toBeGreaterThanOrEqual(1);

  // ── 点开一颗 ───────────────────────────────────────────────────────────
  // aria-label 是「标题 — 主枝名」（Planet.tsx），抽屉的 label 就是这个标题。
  const label = (await planets.first().getAttribute("aria-label")) ?? "";
  const starTitle = label.split(" — ")[0]?.trim() ?? "";
  expect(starTitle.length, `星球的 aria-label 读不出标题：${label}`).toBeGreaterThan(0);

  await planets.first().click();
  const sheet = page.getByRole("dialog", { name: starTitle });
  await expect(sheet).toBeVisible({ timeout: 30_000 });
  // 这一屏是「这条新闻 → 它想问你 → 出处 → 现在读 / 稍后读」，两个动作都在。
  await expect(sheet.getByRole("button", { name: /现在读/ })).toBeVisible();
  await expect(sheet.getByRole("button", { name: /稍后读/ })).toBeVisible();

  // ── 现在读 → 落进阅读室的那一篇 ────────────────────────────────────────
  await sheet.getByRole("button", { name: /现在读/ }).click();
  await expect(page).toHaveURL(/\/readings\/[0-9a-f-]{36}$/, { timeout: 120_000 });
  const readingId = new URL(page.url()).pathname.split("/").pop()!;

  // ── 没有正文就粘一份 ───────────────────────────────────────────────────
  //
  // 两条路都是对的：服务端替她抓到了正文（房间直接开），或者没抓到（粘贴框）。
  // 从地图进来绝大多数是后者，而这一屏是落地页那条 walk 从来走不到的。
  const pastePanel = page.getByRole("heading", { name: "这次阅读还没有正文" });
  const room = page.locator(".mk-reading-room");
  await expect(pastePanel.or(room).first()).toBeVisible({ timeout: 60_000 });

  if (await pastePanel.isVisible()) {
    await page.getByPlaceholder("把文章正文粘贴到这里…").fill(PASTED_BODY);
    await page.getByRole("button", { name: "开始阅读" }).click();
  }
  await expect(room).toBeVisible({ timeout: 60_000 });
  await expect(page.locator("p[data-block-id]").first()).toBeVisible({ timeout: 30_000 });

  // 这一篇的名字来自那颗星，不是她打的。
  await expect(page.getByRole("heading", { level: 2 })).toContainText(starTitle.slice(0, 8));

  // ── 带读走一轮：开始 + 她的一句追问 ────────────────────────────────────
  //
  // 采集读的是**她自己写下的话**，所以这一句不是走过场：没有她的话，报告上那
  // 一节提不出词，第二段的入口就不存在。
  const replies = assistantReplies(page);
  const thinking = page.locator('[aria-label="印记正在打字"]');
  await page.getByRole("button", { name: "开始", exact: true }).click();
  await expect(replies).toHaveCount(1, { timeout: 300_000 });
  await expect(thinking).toHaveCount(0, { timeout: 300_000 });
  // 🚨 AI 失败要说出来，绝不假装成一句陪练的话。banner 在就是这一轮塌了。
  await expect(page.getByRole("alert")).toHaveCount(0);

  await page
    .getByPlaceholder(/读完这一步|还想聊点什么/)
    .fill("我最在意的是储能：如果白天多出来的电存不下来，那装机再多是不是就没意义了？");
  await page.getByRole("button", { name: "发送" }).click();
  await expect(replies).toHaveCount(2, { timeout: 300_000 });
  await expect(thinking).toHaveCount(0, { timeout: 300_000 });
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── 完成这篇 → 报告 ────────────────────────────────────────────────────
  await page.getByRole("button", { name: "完成这篇" }).click();
  const finalize = page.getByRole("dialog", { name: "完成这篇" });
  await expect(finalize.getByRole("heading", { name: "完成这篇？" })).toBeVisible();
  await finalize.getByRole("button", { name: "完成，看报告" }).click();
  await expect(finalize).toBeHidden({ timeout: 60_000 });

  // 第一次打开这份报告就是在生成它 —— 一次旗舰调用，几十秒。
  const report = page.locator("article");
  await expect(report).toBeVisible({ timeout: 300_000 });
  await expect(report.getByRole("region", { name: "这次的数据" })).toBeVisible();

  // ── 报告最后那一节：她点头，词才上树 ───────────────────────────────────
  //
  // 采集在后台队列里跑（完成时入队 + 每两分钟扫尾），所以这一节先是「处理中」。
  // 组件自己问八次、每次隔四秒就停下，改说「这一篇的词还在整理」并给一个
  // 「再看一次」。队列慢过 32 秒是常事，所以这里替她按那个按钮，最多按十轮。
  const section = page.getByRole("heading", { name: "可以加进你的兴趣树" });
  const joinButtons = page.getByRole("button", { name: "加入" });
  for (let i = 0; i < 10; i++) {
    if (await joinButtons.first().isVisible().catch(() => false)) break;
    const again = page.getByRole("button", { name: "再看一次" });
    if (await again.isVisible().catch(() => false)) await again.click();
    await page.waitForTimeout(15_000);
  }
  await expect(
    section,
    "报告上没有「可以加进你的兴趣树」——采集没跑出词，写作那个入口就是空的",
  ).toBeVisible({ timeout: 60_000 });
  await expect(joinButtons.first()).toBeVisible({ timeout: 60_000 });

  // 每一条都带着她自己写的那句话。没有它，这就是一句「猜你喜欢」。
  const firstRow = joinButtons.first().locator("xpath=ancestor::li[1]");
  await expect(firstRow.getByText("你自己写的")).toBeVisible();
  acceptedWord = (await firstRow.locator("strong").first().innerText()).trim();
  expect(acceptedWord.length, "候选词是空的").toBeGreaterThan(0);

  // 🚨 点完之后那一行会重画，「加入」两个按钮换成「已加入」三个字 —— 于是
  // `firstRow` 这个从「加入」按钮往上找的定位器就再也解析不出东西了（Playwright
  // 的定位器是每次用的时候才求值的）。所以改用**那个词**去认这一行：它在这一步
  // 前后都不变。
  const rowByWord = page.getByRole("listitem").filter({ hasText: acceptedWord }).first();
  await joinButtons.first().click();
  await expect(rowByWord.getByText("已加入", { exact: true })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText("操作失败：", { exact: false })).toHaveCount(0);

  test.info().annotations.push({ type: "加进树的词", description: acceptedWord });
});

/* ── 入口二 · 兴趣树 ───────────────────────────────────────────────────── */

test("入口二：兴趣树上刚长出来的那个词 → 继续深挖 → 去写 → 写作间走完一篇", async () => {
  // 四颗种子、写作开场、一轮结构、段落引导、成稿的体检、写作报告。
  test.setTimeout(1_800_000);

  expect(acceptedWord, "上一条没有把词加进树，这一条无从进起").not.toBe("");

  // ── 树上有她刚认下的那个词 ─────────────────────────────────────────────
  await page.goto("/tree");
  await expect(page.getByRole("tab", { name: "我的兴趣树" })).toHaveAttribute(
    "aria-selected",
    "true",
    { timeout: 60_000 },
  );
  const leaf = page.getByRole("button", { name: new RegExp(escapeRe(acceptedWord)) }).first();
  await expect(
    leaf,
    `报告上加进去的「${acceptedWord}」没有出现在树上 —— 那一步点了头却没落地`,
  ).toBeVisible({ timeout: 120_000 });

  // ── 点开它：证据来自刚才那一篇阅读 ─────────────────────────────────────
  await leaf.click();
  const drawer = page.getByRole("dialog", { name: acceptedWord });
  await expect(drawer).toBeVisible({ timeout: 30_000 });
  await expect(drawer.getByRole("heading", { name: "相关活动" })).toBeVisible();
  // 这个词是从一次阅读里来的，抽屉里就必须看得见那一次。
  await expect(drawer.getByText("阅读", { exact: true }).first()).toBeVisible();

  // ── 继续深挖：四颗种子，其中一颗是「去写」 ─────────────────────────────
  await expect(drawer.getByRole("heading", { name: "继续深挖" })).toBeVisible();
  const goWrite = drawer.getByRole("button", { name: "在写作间打开" });
  await expect(
    goWrite,
    "「继续深挖」里没有「去写」那一颗 —— 从树进写作间的门不存在",
  ).toBeVisible({ timeout: 240_000 });

  // 那颗种子的正文会原样变成这一篇的由头（`createWriting({ idea: seed.text })`）。
  const seedCard = goWrite.locator("xpath=ancestor::div[1]");
  const seedText = (await seedCard.locator("p").first().innerText()).trim();
  expect(seedText.length, "种子是空的").toBeGreaterThan(0);

  await goWrite.click();
  await expect(page).toHaveURL(/\/writings\/[0-9a-f-]{36}$/, { timeout: 120_000 });
  const writingId = new URL(page.url()).pathname.split("/").pop()!;

  // ── 由头真的是树上那句话，一路带进了写作间 ─────────────────────────────
  const setup = page.getByRole("dialog", { name: "开始之前" });
  await expect(setup).toBeVisible({ timeout: 60_000 });
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/setup") && r.request().method() === "PUT"),
    setup.getByRole("button", { name: "开始", exact: true }).click(),
  ]);
  await expect(setup).toHaveCount(0, { timeout: 30_000 });

  await expect(page.getByRole("heading", { level: 1 })).toContainText(seedText.slice(0, 12));
  await expect(page.locator('[data-role="student"]', { hasText: seedText })).toBeVisible();

  // ── 结构：印记先开口，一轮之后思维导图上有她的话 ───────────────────────
  await expect(assistantReplies(page)).toHaveCount(1, { timeout: 300_000 });
  await expect(page.getByRole("alert")).toHaveCount(0);

  const [planResp] = await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/plan/turn") && r.request().method() === "POST",
      { timeout: 300_000 },
    ),
    (async () => {
      await page
        .getByPlaceholder("说说你的想法")
        .fill("我最想让读的人相信：这件事值得先想清楚代价，再决定要不要做。");
      await page.getByRole("button", { name: "发送", exact: true }).click();
    })(),
  ]);
  const planned = (await planResp.json()) as {
    outline: { id: string; text: string }[];
  };
  // 🚨 这里**不能**要求「这一轮必须往图上加一个点」。印记完全可能先回一个问题
  // 再动图 —— 第六次走查第一轮就交回了空的 outline，而那一轮它做的事是对的。
  // 图长没长成，看的是两轮谈完之后的样子，见下面。
  for (const node of planned.outline) expect(node.text.trim().length).toBeGreaterThan(0);
  await expect(page.getByRole("alert")).toHaveCount(0);

  // 🚨 第二轮。一轮之后图上只有一个「中心论点（待聚焦）」—— 印记自己在回话里说
  // 那还不是一个主张，并且追问她倾向哪一边。**这时候的图还不是一份能写的提纲**，
  // 段落那一屏于是排不出引导（第五次走查就停在那里，等一个永远不来的引导框）。
  // 结构本来就是一轮一轮谈出来的，一轮就走人是这条 walk 不像学生的地方，不是
  // 产品的毛病 —— writing-walk 也是谈两轮。
  const [secondResp] = await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/plan/turn") && r.request().method() === "POST",
      { timeout: 300_000 },
    ),
    (async () => {
      await page
        .getByPlaceholder("说说你的想法")
        .fill(
          "我更偏向电池是有价值的：第一个理由是工厂的生产时间没那么好改，很多工序要连着跑；" +
            "第二个理由是电池除了存电还能稳住电网的波动，这一点工厂调时间做不到。",
        );
      await page.getByRole("button", { name: "发送", exact: true }).click();
    })(),
  ]);
  const afterSecond = (await secondResp.json()) as {
    outline: { id: string; text: string }[];
  };
  // 图只会长，不会被印记改写或删掉 —— 图上的字是她的。
  for (const before of planned.outline) {
    const still = afterSecond.outline.find((n) => n.id === before.id);
    expect(still, `印记把她写的「${before.text}」从图上删掉了`).toBeTruthy();
    expect(still!.text, "印记改写了她写在图上的话").toBe(before.text);
  }
  expect(afterSecond.outline.length).toBeGreaterThanOrEqual(planned.outline.length);
  // 两轮谈完，图上总得有东西 —— 段落那一屏的每一格都是从这些点来的，图是空的，
  // 她就没有可写的地方。
  expect(
    afterSecond.outline.length,
    "谈了两轮，思维导图上一个点都没有 —— 段落那一屏会是空的",
  ).toBeGreaterThanOrEqual(1);
  for (const node of afterSecond.outline) expect(node.text.trim().length).toBeGreaterThan(0);
  await expect(page.getByRole("alert")).toHaveCount(0);

  // ── 段落：她自己写，印记只给引导 ───────────────────────────────────────
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page.getByRole("button", { name: /去写/ }).click(),
  ]);
  await expect(page.getByRole("heading", { name: "段落" })).toBeVisible({ timeout: 60_000 });

  // 2026-09-18 起一次只摊开一张纸，换一段是点上面那一叠卡片。
  const cards = page.locator("[data-write-card]");
  const boxes = page.getByPlaceholder("写这一段……");
  await expect(boxes.first()).toBeVisible({ timeout: 60_000 });
  // 引导在到达时就在，不用点「卡住了？」。
  await expect(page.getByText("写作引导").first()).toBeVisible({ timeout: 300_000 });
  await expect(page.getByRole("alert")).toHaveCount(0);

  const p1 = `我先说说我自己看到的一面：${seedText.slice(0, 18)}这件事，最容易被忽略的是它的代价落在谁身上。`;
  const p2 = "但只看代价也不公平——如果因为怕出错就什么都不做，那些真正能被改善的地方也一起被放掉了。";

  // 🚨 有几个格子，**必须在还站在「段落」这一屏的时候数**。走到「成稿」之后这些
  // 格子就不在页面上了，那时候再 `boxes.count()` 拿到的是 0 —— 于是下面那句
  // 「拼出来的应该等于她写的几段」会拿一段去比两段，报的样子像是成稿把她的字
  // 拼错了，其实是这条 walk 数错了。
  const slots = await cards.count();
  const written = [p1, p2].slice(0, Math.max(1, Math.min(slots, 2)));

  for (let i = 0; i < written.length; i++) {
    await cards.nth(i).click();
    await boxes.first().fill(written[i]!);
    await Promise.all([
      page.waitForResponse((r) => r.url().includes("/snippets") && r.request().method() === "PUT"),
      boxes.first().blur(),
    ]);
  }
  await expect(page.getByText("保存这一段失败，请重试。")).toHaveCount(0);

  // ── 成稿：到达即拼装，拼出来的就是她那几段，一个字不多 ─────────────────
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/stage") && r.request().method() === "POST"),
    page
      .getByRole("navigation", { name: "写作四步" })
      .getByRole("button", { name: "成稿" })
      .click(),
  ]);
  const draftBox = page.getByPlaceholder("请先写下你最想说的那句话，再围绕它展开。");
  // 拼装是**纯粹的字符串拼接**（writing_compose.go 只 trim 再用空行连起来），
  // 所以这里断言完全相等，不是「包含」—— 印记在这一步一个字都不许加。
  await expect(draftBox).toHaveValue(written.join("\n\n"), { timeout: 60_000 });

  // ── 完成这篇 → 起名字 → 已完成 ─────────────────────────────────────────
  const pieceName = `从树上那个词写起 ${RUN}`;
  await page.getByRole("button", { name: "完成这篇", exact: true }).click();
  const nameBox = page.getByPlaceholder("写一个你想让别人看到的名字");
  await expect(nameBox).toBeVisible({ timeout: 120_000 });
  await nameBox.fill(pieceName);
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/finish") && r.request().method() === "POST", {
      timeout: 120_000,
    }),
    page.getByRole("button", { name: "确认并完成" }).click(),
  ]);
  await expect(page.getByText("已完成", { exact: true })).toBeVisible({ timeout: 60_000 });
  await expect(page.getByRole("heading", { name: pieceName })).toBeVisible();
  // 完成之后先跑一次写作报告；她的字要等它落下来才显示。
  await expect(
    page.getByText("印记正在把这次写的东西整理成一份报告", { exact: false }),
  ).toHaveCount(0, { timeout: 600_000 });
  await expect(page.getByText(p1)).toBeVisible();
  expect(new URL(page.url()).pathname).toBe(`/writings/${writingId}`);
});

/** 关键词是模型给的，可能带正则元字符。 */
function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
