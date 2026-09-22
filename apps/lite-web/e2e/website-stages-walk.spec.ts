import { expect, test, type Page } from "@playwright/test";
import { freshAccount } from "./freshAccount";
import { skipWhilePaused } from "./pausedSurfaces";

// 项目 / 我的主页 还没做完，这两个面上的走查先停。见 pausedSurfaces.ts。
skipWhilePaused();

/**
 * 主页项目第一到第三关的工作面 —— 受众画像 / 站点采集 / 视觉基调。
 *
 * ## 为什么要单独有这一条
 *
 * `homepage-walk` 是**纯数据路径**那一条：它跳过前四关，直接把「印记摆好了她的
 * 话」这个状态塞进去，再验后面那半程。跳过的理由是对的（那四关每一关都要真的
 * 模型调用，每次 CI 都跑太贵），但代价是——**这三块工作面从来没有在浏览器里被
 * 打开过一次**。它们恰好是产品负责人 2026-09-03 那份要求里的第一到第三步：
 *
 *   1) 想清楚给谁看：一块受众画像板，AI 生成的画像 + 一组关键词；
 *   2) 去看真的个人网站：她自己找、贴进来，印记逐站回一张卡；
 *   3) 给网站定调子：材料、配色、风格、头图。
 *
 * 一条 walk 绿着、而这三屏没人看过，等于"全流程验过了"这句话是假的。
 *
 * ## 这一条只回答「她打开看见什么」
 *
 * 只断言每一屏的第一眼和它的主动作，外加截图。往下走（真生成三个人、真贴三个
 * 网址、真出三组配色）每一步都是一次真模型调用，一张画像实测 69 秒——那属于
 * `LIVE_LLM=1` 的实测，不属于每次都跑的这一条。
 *
 * 🚨 唯一的例外是第一关按下去那一下：她这一关的产出是后面每一关的输入，
 * 「按了生成之后到底出不出得来东西」是这条路上最值得每次都确认的一件事。
 * 所以那里等一个真结果，并且**允许它以一条报错收场**——模型不通的时候，
 * 屏幕上该出现的是一句错误，不是一个永远转下去的圈。
 */

test.describe.configure({ retries: 0 });

// 线上前后端是两个 host，接口要打到 mind-api 那台。见 gate.ts 顶部。
const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");

const TOOLS = [
  { tool: "persona", reason: "先把这一页给谁看想清楚" },
  { tool: "sites", reason: "去看几个真的个人网站是怎么做的" },
  { tool: "look", reason: "配色和风格决定别人第一眼看到什么" },
];

/**
 * 从对话里那张邀请卡进去。
 *
 * 🚨 走的是卡上的 `data-testid`，不是按名字找按钮。tools-walk 那个 helper 按
 * 「工具名」找按钮，靠的是那件工具已经在「进行中」那一列里有一个以它命名的
 * 按钮；一张**还没被接下**的邀请卡上没有这样的按钮——卡上唯一能点的那个叫
 * 「开始任务」，工具名只是卡里的一行字。这三关是她第一次见到这几件工具，卡
 * 一定是没接过的那种，所以只能从卡进去。
 */
async function openTool(page: Page, name: string, label: string) {
  // 先收起手上那件：工具是铺开的，占满整个房间，不收起来看不见下一张卡。
  const close = page.getByRole("button", { name: "收起" });
  if (await close.count()) await close.first().click();
  const start = page
    .getByTestId(`tool-invite-${name}`)
    .getByRole("button", { name: /开始任务|接受任务/ });
  // 🚨 印记那一轮还在跑的时候这颗按钮是 disabled 的（那是对的）。这条 walk 的卡
  // 是用接口递的，所以现在不容易撞上；journey-1 用真对话递，2026-09-05 就撞上了，
  // 报的是「element is not enabled」，看上去像这块工作面坏了。两边一起等。
  await expect(start).toBeEnabled({ timeout: 60_000 });
  await start.click();
  await expect(page.getByRole("heading", { name: label })).toBeVisible();
}

test("主页项目: 受众画像 / 站点采集 / 视觉基调 三块工作面各打开一次", async ({ browser }) => {
  // 🚨 这一条要三件**还没被接下**的工具，所以它自己注册一个账号。
  //
  // 共用那个种子账号不行：`journey-2` 会在同一个主页项目上打开「视觉基调」，
  // 那件工具就变成 accepted，而 accepted 的工具**不再渲染邀请卡**
  // （ToolInvite 只画没接过的）。下面 openTool 会等一张永远不出现的卡，
  // 15 秒超时，报出来的样子像是这块工作面坏了。见 freshAccount.ts。
  const ctx = await freshAccount(browser, "site-stages");
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log("PAGEERROR:", e.message));
  page.on("console", (m) => {
    if (m.type() === "error") console.log("CONSOLE ERROR:", m.text());
  });

  // 主页项目本身就是那道门的走法，不用先把门打开。
  await page.goto("/projects");
  const made = await page.request.post(`${API}/api/v1/pbl/site/project`);
  expect([200, 201], `开主页项目 = ${made.status()} ${await made.text()}`).toContain(made.status());
  const id = (await made.json()).id as string;
  const api = `${API}/api/v1/pbl/projects/${id}`;

  // 印记递工具走的就是这条路。重跑时不重复递。
  const already = new Set<string>(
    ((await (await page.request.get(`${api}/tools`)).json()) as { tool: string }[]).map(
      (t) => t.tool,
    ),
  );
  for (const t of TOOLS) {
    if (already.has(t.tool)) continue;
    const res = await page.request.post(`${api}/tools`, { data: t });
    expect(res.status(), `summon ${t.tool}`).toBe(201);
  }

  await page.goto(`/projects/${id}`);
  await expect(page.getByPlaceholder("请输入")).toBeVisible();

  // 🚨 等开场那一轮真的落库，再动第一关。
  //
  // 受众画像是从「她真做过的事」里推的，而**她在这个项目里说过的话也算材料**
  // （studentMaterialFor → studentOwnWords）。房间一打开会把驱动问题当作她的
  // 第一句发出去，可两条消息是模型答完之后同一个事务才写的——在那之前库里一条
  // 都没有。抢在它前面按「生成」，服务端看到的是一个什么都没说过的人，回的是
  // 「你还没有读过、写过或做过的东西可以用来想读者」。
  //
  // 那条报错本身是对的（它防的是印记凭空编一个读者）。假的是**测试**：它测出
  // 来的是一个真实学生走不到的时序。
  await expect
    .poll(
      async () => {
        const msgs = (await (await page.request.get(`${api}/thread`)).json()) as unknown[];
        return Array.isArray(msgs) ? msgs.length : 0;
      },
      { timeout: 120_000, message: "开场那一轮一直没落库" },
    )
    .toBeGreaterThanOrEqual(2);

  // 🚨 再让她真的说一段自己的事，然后才进第一关。
  //
  // 受众画像是从材料里推的，而一个**刚注册**的学生什么都没读过写过——她全部的
  // 材料就是那句驱动问题。prompt 里有一条硬规矩是「材料很少的时候就少给几个，
  // 不要靠编来凑满三个」，所以模型很老实地一个都不给，服务端回
  // `no usable persona came back`，她看到的是一句「生成失败」。
  //
  // 那个行为本身是对的（编三个模板读者比失败更坏），但它意味着**这一关的输入
  // 是她自己说的话**。一条声称在验第一关的 walk，如果连一句她自己的话都没有，
  // 验的就不是产品，是一个真实学生走不到的空账号。
  const said = await page.request.post(`${api}/turn`, {
    data: {
      text:
        "我高二，在读 IB。这两年我一直在拆家里坏掉的电器——台灯、电水壶、一个吹风机，" +
        "拆完写一篇为什么它修不好。已经写了三篇了。我想把这些放在一个地方给别人看。",
    },
  });
  expect(said.status(), `她说话 = ${said.status()} ${await said.text()}`).toBe(200);

  /* 1 · 受众画像。这一屏一个输入框都没有——她的动作全是判断。 */
  await openTool(page, "persona", "受众画像");
  await expect(page.getByText("请挑出一个真会点进你这一页的人")).toBeVisible();
  const start = page.getByRole("button", { name: /看看有谁|生成/ }).first();
  await expect(start).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/site-1-persona-empty.png", fullPage: true });

  // 按下去要真的出来东西。三个人是一次 compose 调用，画像随后一张一张来。
  //
  // 🚨 这里**不**写成「出人 或 出错都算过」。tools-walk 的复盘那条是那么写的，
  // 理由是模型不通不该看起来像界面坏了；可这一屏是第一次被验，那样写的结果是
  // 2026-09-04 第一次跑 941ms 就绿了——绿在一条「你还没有读过、写过或做过的
  // 东西」的报错上，而那条报错恰恰说明这一关没走通。
  await start.click();
  await expect(page.getByText("画像由 AI 生成").first()).toBeVisible({ timeout: 240_000 });
  // 材料是空的这条错必须不在：她已经说过话了，就不该被当成什么都没做过的人。
  await expect(page.getByText("你还没有读过、写过或做过", { exact: false })).toHaveCount(0);
  await page.screenshot({ path: "e2e/.shots/site-2-persona-people.png", fullPage: true });

  /* 2 · 站点采集。她自己去搜、把网址贴进来。 */
  await openTool(page, "sites", "站点采集");
  await page.screenshot({ path: "e2e/.shots/site-3-sites.png", fullPage: true });

  /* 3 · 视觉基调。配色、风格、头图。 */
  await openTool(page, "look", "视觉基调");
  await page.screenshot({ path: "e2e/.shots/site-4-look.png", fullPage: true });

  await ctx.close();
});
