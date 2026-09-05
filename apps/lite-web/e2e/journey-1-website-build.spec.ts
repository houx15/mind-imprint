import { expect, test, type Page } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 旅程一 · 一个新学生做出他自己的主页 —— **不走捷径**。
 *
 * ## 为什么现有的 walk 不算数
 *
 * `homepage-walk` 用接口把「印记摆好了她的话」这个状态塞进去，再验后半程；
 * `website-stages-walk` 用接口把工具递给她。两条都绿，但两条都跳过了这件事
 * 真正会卡住的地方：**印记到底有没有在该递的时候把工具递出来**，以及**她说的
 * 话到底有没有变成页面上的字**。产品负责人 2026-09-04：
 *
 *   「we should fix and retest until we finished three things:
 *     1) new user make a personal website which is attractive
 *     2) he/she can edit the website
 *     3) he/she can do a project and finish it.」
 *
 * 所以这一条从一个**空账号**出发，只做学生做得到的动作：点门、审计划、说话、
 * 在工具里做判断、按完成。接口只用来**看**（轮询状态），不用来改。
 *
 * 🚨 文件名里的 `1` 是顺序，读的时候按这个序读。但**跑起来不再依赖这个序**：
 * 这一条要的是一个还没有主页的账号，所以它自己注册一个（`freshAccount`）。
 *
 * 原来这个前提是靠字母序守的，文件头上写着「它必须第一个跑」。那守不住——
 * 2026-09-05 `courses-walk.spec.ts` 一进来（c 排在 j 前面），它开了门，这一条
 * 立刻停在「先做你自己的主页。」找不到，看上去像门坏了，其实门是好的，只是
 * 已经开了。一条只能靠人记住的规矩，迟早会被下一个新 spec 破掉。
 *
 * ## 它会因为模型不听话而红，这是故意的
 *
 * 递工具是印记那一轮的判断（`websiteRoutine` 写着第一关配 persona）。这里不
 * 替它递——递不出来就是一个真的缺陷：一个学生坐在那儿，计划上写着第一步，而
 * 那一步的工具永远不出现。宁可红在这里，也不要用一次 POST 把它盖过去。
 */

test.describe.configure({ retries: 0 });

const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");

/** 她说一句话，等这一轮真的落库（两条消息是模型答完之后一个事务写的）。 */
async function say(page: Page, api: string, text: string): Promise<void> {
  const before = await threadLen(page, api);
  // 🚨 先等输入框能用。一件工具按完「完成」之后，房间会立刻把结果回灌给印记
  // （那正是闭环），这一轮里输入框是 disabled 的。不等就 fill，报的是
  // 「locator.fill 超时」——看起来像输入框不见了，其实是印记正在读她刚做完的
  // 那件东西。
  const box = page.getByPlaceholder("请输入");
  await expect(box).toBeEnabled({ timeout: 180_000 });
  await box.fill(text);
  await page.getByRole("button", { name: "发送" }).click();
  await expect
    .poll(() => threadLen(page, api), { timeout: 180_000, message: `这一轮没落库：${text}` })
    .toBeGreaterThan(before);
}

async function threadLen(page: Page, api: string): Promise<number> {
  const r = await page.request.get(`${api}/thread`);
  if (!r.ok()) return -1;
  const msgs = (await r.json()) as unknown[];
  return Array.isArray(msgs) ? msgs.length : -1;
}

/** 印记递过来的那几件。只读，不递。 */
async function toolsHanded(page: Page, api: string): Promise<string[]> {
  const r = await page.request.get(`${api}/tools`);
  if (!r.ok()) return [];
  const list = (await r.json()) as { tool: string }[];
  return Array.isArray(list) ? list.map((t) => t.tool) : [];
}

/**
 * 一直聊到印记把 `want` 这件工具递出来。
 *
 * 🚨 不替它递。`nudges` 是学生自己会说的话——她卡住的时候本来就会追问一句。
 * 全说完还没递出来，就红在这里，并且把印记这一路说了什么打出来，好判断是
 * prompt 的问题还是路线的问题。
 */
async function waitForTool(
  page: Page,
  api: string,
  want: string,
  lines: string[],
  maxTurns = 6,
): Promise<void> {
  for (let i = 0; i < maxTurns; i++) {
    if ((await toolsHanded(page, api)).includes(want)) return;
    // 说完一轮就再看一眼。递工具是印记那一轮里的判断，所以每一轮都是一次机会。
    await say(page, api, lines[i % lines.length]);
  }
  if ((await toolsHanded(page, api)).includes(want)) return;
  const msgs = await (await page.request.get(`${api}/thread`)).json();
  const handed = await toolsHanded(page, api);
  throw new Error(
    `聊了 ${maxTurns} 轮，印记始终没有递出「${want}」。\n` +
      `这一路它递过的是：${handed.join("、") || "（一件都没有）"}。\n` +
      `路线见 internal/pbl/website.go · websiteRoutine。\n` +
      `对话是：\n${JSON.stringify(msgs, null, 2).slice(-4000)}`,
  );
}

/**
 * 试着把 `want` 要出来，要不到就报 false —— 不抛。
 *
 * 🚨 印记递工具的**顺序**是浮动的。同一条路线，有的轮次「站点采集 → 结构审查
 * → 视觉基调」，有的轮次跳过结构直接给视觉基调。硬按一个顺序等，等不到的那次
 * 报出来像是"结构审查坏了"，其实只是它把这一步放到后面去了。
 *
 * 所以顺序不由这条 walk 规定：哪一关先递就先做哪一关，**但整趟走完必须都做过**
 * ——最后那一条断言才是真正要守的东西。
 */
async function tryTool(
  page: Page,
  api: string,
  want: string,
  lines: string[],
  maxTurns = 3,
): Promise<boolean> {
  for (let i = 0; i < maxTurns; i++) {
    if ((await toolsHanded(page, api)).includes(want)) return true;
    await say(page, api, lines[i % lines.length]);
  }
  return (await toolsHanded(page, api)).includes(want);
}

/** 她收回来几个站。 */
async function siteRefCount(page: Page, api: string): Promise<number> {
  const r = await page.request.get(`${api}/sites`);
  if (!r.ok()) return 0;
  const list = (await r.json()) as unknown[];
  return Array.isArray(list) ? list.length : 0;
}

/**
 * 一直说到这一页齐了（`SiteMissing` 空）。
 *
 * 🚨 这里等的是**服务端说齐了**，不是界面上看起来齐了。页面上的每一句都要能在
 * 她说过的话里逐字找到（`GroundSiteDraft`），对不上的那一句会被**静静丢掉**——
 * 不报错、不提示。所以「印记回了一句好的」完全不能说明那句话上了页面。
 */
async function settleSite(page: Page, api: string, nudges: string[]): Promise<void> {
  const missing = async (): Promise<string[]> => {
    const r = await page.request.get(`${API}/api/v1/pbl/site`);
    if (!r.ok()) return ["(取不到)"];
    return ((await r.json()) as { missing?: string[] }).missing ?? [];
  };
  for (const n of nudges) {
    if ((await missing()).length === 0) return;
    await say(page, api, n);
  }
  const left = await missing();
  if (left.length === 0) return;
  throw new Error(
    `说了 ${nudges.length} 轮，这一页还差：${left.join("、")}。\n` +
      `页面上的字必须逐字出自她说过的话（pbl.GroundSiteDraft），对不上的会被静静丢掉——` +
      `所以这里红，通常是印记润色了她的话，而不是它没答。`,
  );
}

async function openHandedTool(page: Page, name: string, label: string): Promise<void> {
  const close = page.getByRole("button", { name: "收起" });
  if (await close.count()) await close.first().click();
  await page
    .getByTestId(`tool-invite-${name}`)
    .getByRole("button", { name: /开始任务|接受任务/ })
    .click();
  await expect(page.getByRole("heading", { name: label })).toBeVisible();
}

/**
 * 按那颗「做完了」的按钮。todo 没清空时它是灰的，所以这一下同时也在断言
 * 「这一关真做完了」。
 *
 * 🚨 名字不都叫「完成」。ToolFrame 收 `finishLabel`，结构审查那颗叫「没有问题」
 * （审查的结论不是"做完了"，是"我看过，没问题"）。写死「完成」的话，那一关会
 * 以「找不到按钮」失败，看起来像界面坏了。
 */
async function finishTool(page: Page, label = "完成"): Promise<void> {
  const done = page.getByRole("button", { name: label, exact: true });
  await expect(done).toBeEnabled({ timeout: 15_000 });
  await done.click();
}

test("旅程一: 空账号 → 五关走完 → 一页发布出去的主页", async ({ browser }) => {
  // 🚨 「空账号」是这条旅程的题目，所以它自己注册一个。
  //
  // 以前这个前提靠文件名的字母序守着（文件头上写过「它必须第一个跑」），而那是
  // 一条只能靠人记住的规矩：2026-09-05 `courses-walk.spec.ts` 一进来（c 排在 j
  // 前面）就把门开了，这条立刻红在第一句断言上。见 freshAccount.ts。
  const ctx = await freshAccount(browser, "journey1");
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log("PAGEERROR:", e.message));
  page.on("console", (m) => {
    if (m.type() === "error") console.log("CONSOLE ERROR:", m.text());
  });

  /* 1 · 门。一个还没有主页的学生只有这一条路。 */
  await page.goto("/projects");
  await expect(page.getByRole("heading", { name: "先做你自己的主页。" })).toBeVisible();
  await page.getByRole("button", { name: /做我的主页/ }).click();
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/);
  const id = page.url().split("/").pop()!;
  const api = `${API}/api/v1/pbl/projects/${id}`;

  /* 2 · 房间。驱动问题是定好的，计划是预置的五步。 */
  await expect(page.getByText("我想让谁，看见我的什么？").first()).toBeVisible();
  await expect
    .poll(() => threadLen(page, api), { timeout: 180_000, message: "开场那一轮没落库" })
    .toBeGreaterThanOrEqual(2);
  // .first()：这句话在计划里和印记的话里各出现一次。
  await expect(page.getByText("想清楚给谁看").first()).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/j1-1-room.png", fullPage: true });

  /* 3 · 她审计划。这一步是她的，不是印记的。 */
  await page.getByRole("button", { name: "审核完成，开始！" }).click();
  await expect(page.getByRole("button", { name: "审核完成，开始！" })).toHaveCount(0);

  /* 4 · 她讲自己的事。受众画像是从这些话里推的。 */
  await say(
    page,
    api,
    "我高二，在读 IB。这两年我一直在拆家里坏掉的电器——台灯、电水壶、一个吹风机，" +
      "拆完写一篇为什么它修不好。已经写了三篇了。我想把这些放在一个地方给别人看。",
  );

  /* 5 · 等印记把第一关那件工具递出来。 */
  await waitForTool(page, api, "persona", [
    "那我先从哪一步开始？",
    "最近拆的是一个吹风机，拆到风扇那一层才发现电机是压进去的，取不出来。",
    "我不太确定这一页是给谁看的，你能帮我想想吗？",
    "我想不出来具体是谁会来看，你先给我几个人选吧。",
    "帮我做几个可能的读者出来。",
  ]);
  await page.screenshot({ path: "e2e/.shots/j1-2-persona-handed.png", fullPage: true });

  /* 6 · 关一：生成 → 挑一个人 → 留关键词 → 完成。 */
  await openHandedTool(page, "persona", "受众画像");
  await page.getByRole("button", { name: /看看有谁|生成/ }).first().click();
  await expect(page.getByText("画像由 AI 生成").first()).toBeVisible({ timeout: 240_000 });
  await page.screenshot({ path: "e2e/.shots/j1-3-persona-people.png", fullPage: true });

  // 挑第一个人。整张卡就是一个按钮。
  await page.getByText("画像由 AI 生成").first().click();
  await expect(page.getByText("关键词")).toBeVisible();
  await page.getByRole("button", { name: "确认选择" }).click();
  await expect(page.getByText("已确定").first()).toBeVisible({ timeout: 30_000 });
  await page.screenshot({ path: "e2e/.shots/j1-4-persona-settled.png", fullPage: true });

  await finishTool(page);

  // 🚨 闭环：她定下的读者必须回到印记那儿，否则第三关它会重新问一遍「给谁看」。
  const personas = (await (await page.request.get(`${api}/personas`)).json()) as {
    chosen: boolean;
    keywords: string[];
  }[];
  const chosen = personas.find((p) => p.chosen);
  expect(chosen, "没有一个读者被标成 chosen —— 关一没有落库").toBeTruthy();
  expect(chosen!.keywords.length, "读者定了，但一个关键词都没留下").toBeGreaterThan(0);

  /* 7 · 关二：去看真的个人网站，收三个回来。 */
  await waitForTool(page, api, "sites", [
    "读者定下来了，下一步做什么？",
    "我想看看别人的个人网站是怎么做的。",
    "有没有几个真的个人网站可以给我看看？",
    "我想先看看别人怎么排版，再决定我自己的结构。",
  ]);
  await openHandedTool(page, "sites", "站点采集");

  // 🚨 这一步真的会去外面读那几个站（服务端 FetchReadable）。收满三个就停；
  // 六个都试过还不够，说明是外网这一段断了，报清楚，别让它看起来像界面坏了。
  // 🚨 读不回来的要跳过，不能停。2026-09-04 实测那六个起点站只有三个干净可达：
  // 一个 DNS 不通、一个 301、一个 429。一个学生点下去就是「读取失败」，她会去点
  // 下一个——所以这条 walk 也必须这么走。收进来成功的会从列表里消失，失败的还留
  // 在原位，所以失败一次就把游标往后挪一格。
  const collect = page.getByRole("button", { name: "收进来" });
  const waitFor = async (target: number, ms: number): Promise<boolean> => {
    const until = Date.now() + ms;
    while (Date.now() < until) {
      if ((await siteRefCount(page, api)) >= target) return true;
      await page.waitForTimeout(1000);
    }
    return false;
  };
  let cursor = 0;
  for (let attempt = 0; attempt < 8; attempt++) {
    const have = await siteRefCount(page, api);
    if (have >= 3) break;
    const n = await collect.count();
    if (cursor >= n) break;
    await collect.nth(cursor).click().catch(() => undefined);
    if (!(await waitFor(have + 1, 45_000))) cursor++;
  }
  // 起点站不够就自己贴——这本来就是这件工具的另一半（「也可以自己去搜，把网址
  // 贴到下面」）。2026-09-04 实测那六个里只有三个干净可达（一个 DNS 不通、
  // 一个 301、一个 429），碰上其中一个又抽风就凑不齐，而那不是产品的问题。
  const paste = ["https://lilianweng.github.io", "https://d-d.design", "https://terrifyzhao.github.io"];
  for (const url of paste) {
    const have = await siteRefCount(page, api);
    if (have >= 3) break;
    const box = page.getByPlaceholder("粘一个网址");
    if (!(await box.count())) break;
    await box.fill(url);
    await page.getByRole("button", { name: /加入|读取中/ }).click().catch(() => undefined);
    await waitFor(have + 1, 45_000);
  }
  expect(
    await siteRefCount(page, api),
    "六个起点站加上自己贴的都凑不出三个 —— 这一步依赖外网，先确认这台机器出得去",
  ).toBeGreaterThanOrEqual(3);
  await page.screenshot({ path: "e2e/.shots/j1-5-sites.png", fullPage: true });
  await finishTool(page);

  /* 7b · 还在关二：把结构做成一张导图。
   *
   * 这一步是产品负责人要求里的那句「compose the structure in a mindmap」，
   * 路线里也写着（`websiteRoutine`：站点采集之后 structure）。但印记有时候会
   * 先给视觉基调、把结构放到后面——所以这里**不规定顺序**：现在递了就现在做，
   * 没递就先往下走，第三关之后再要一次。整趟走完必须做过（见文末断言）。 */
  const structureLines = [
    "Simon Willison 那个最像我想要的，首屏就把写的东西全摊开。",
    "那我的结构该怎么排？",
    "帮我把结构理成一张图吧。",
  ];
  let didStructure = false;
  if (await tryTool(page, api, "structure", structureLines)) {
    await openHandedTool(page, "structure", "结构审查");
    await page.screenshot({ path: "e2e/.shots/j1-5b-structure.png", fullPage: true });
    await finishTool(page, "没有问题");
    didStructure = true;
  }

  /* 8 · 关三：定调子。配色是从她关一留下的关键词派生的。 */
  await waitForTool(page, api, "look", [
    "三个站看完了，接下来呢？",
    "我想给这一页定个配色和风格。",
    "配色和风格我该怎么选？",
    "我想现在就把颜色和排版定下来。",
    "帮我定视觉基调吧。",
  ]);
  await openHandedTool(page, "look", "视觉基调");
  await page.getByRole("button", { name: /看看配色|换一批/ }).click();
  // 三组配色是一次 compose 调用。
  const swatch = page.getByTestId("palette-option");
  await expect(swatch.first()).toBeVisible({ timeout: 180_000 });
  await swatch.first().click();
  await page.getByRole("button", { name: "确认选择" }).click();
  await page.screenshot({ path: "e2e/.shots/j1-6-look.png", fullPage: true });
  await finishTool(page);

  // 结构如果刚才没轮到，现在补上。
  if (!didStructure) {
    expect(
      await tryTool(page, api, "structure", structureLines),
      "整趟走完，印记始终没有递出「结构审查」—— 而「把结构做成一张导图」是这一关" +
        "写在路线里的一步（websiteRoutine），也是产品负责人要求里的那一句。",
    ).toBe(true);
    await openHandedTool(page, "structure", "结构审查");
    await page.screenshot({ path: "e2e/.shots/j1-5b-structure.png", fullPage: true });
    await finishTool(page, "没有问题");
  }

  /* 9 · 她把要放到页面上的话说出来，印记摆上去。 */
  //
  // 🚨 页面上的每一句都必须能在她说过的话里逐字找到（GroundSiteDraft）。所以
  // 这里她说的就是她想印在页面上的那几句——一个真的学生也是这么说的。
  await say(page, api, "我是一个在读 IB 的高二学生，平时在拆家里修不好的东西。");
  await say(
    page,
    api,
    "首屏我想放这一句：一件还能修的东西，是谁决定它该被扔的？" +
      "关于我自己就写：我在拆家里所有还能拆的东西，然后写为什么它们修不好。",
  );

  // 印记把它们摆上去。摆不齐就再说一轮——她本来也会追问。
  await settleSite(page, api, [
    "把这几句放到我的主页上吧。",
    "首屏那句话就用：一件还能修的东西，是谁决定它该被扔的？",
    "名字底下那行写：我是一个在读 IB 的高二学生。",
    "关于那一段就写：我在拆家里所有还能拆的东西，然后写为什么它们修不好。",
    "这三处请照我说的原话放上去，不要改字。",
    "现在把这一版页面生成出来给我看。",
    "首屏、名字底下那行、关于，这三处都还空着，请补上。",
    "就用我上面说过的那几句，一个字都不用改。",
  ]);

  /* 10 · 上线。她从材料清单回到自己那一页。 */
  // 手上没开着工具的时候没有「收起」——按条件收。
  const close = page.getByRole("button", { name: "收起" });
  if (await close.count()) await close.first().click();
  await page.getByRole("button", { name: /我的主页/ }).first().click();
  await expect(page.getByRole("heading", { name: "上线" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/j1-7-ship-ready.png", fullPage: true });
  await page.getByRole("button", { name: "上线", exact: true }).click();
  await expect(page.getByRole("button", { name: "撤回链接" })).toBeVisible({ timeout: 30_000 });
  await page.screenshot({ path: "e2e/.shots/j1-8-published.png", fullPage: true });

  /* 11 · 访客看到的那一页：她的话、她的配色。 */
  const site = (await (await page.request.get(`${API}/api/v1/pbl/site`)).json()) as {
    url: string;
    published: boolean;
    palette: { paper: string; accent: string };
    content: { headline: string; role: string; about: string[] };
  };
  expect(site.published, "按了上线，但服务端说它没发布").toBe(true);
  expect(site.content.headline, "首屏那句话是空的").not.toBe("");
  expect(site.content.about.length, "「关于」一段都没有").toBeGreaterThan(0);

  const visitor = await page.context().newPage();
  await visitor.goto(site.url);
  await expect(visitor.getByText(site.content.headline).first()).toBeVisible({ timeout: 30_000 });
  // 🚨 配色真的跟到了访客那一页。肉眼在缩略图上分不清两个红，所以量算出来的值。
  const paper = await visitor.evaluate(() => {
    const el = document.querySelector(".mk-site");
    return el ? getComputedStyle(el).getPropertyValue("--st-paper").trim() : "";
  });
  expect(paper.toLowerCase(), "访客看到的是版式默认色，她挑的配色没跟过去").toBe(
    site.palette.paper.toLowerCase(),
  );
  await visitor.screenshot({ path: "e2e/.shots/j1-9-visitor.png", fullPage: true });
  await visitor.close();
  await ctx.close();
});
