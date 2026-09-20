import { test, expect } from "@playwright/test";
import { freshAccount } from "./freshAccount";

/**
 * 思维导图能不能拖 —— **在真浏览器里看**。
 *
 * 🚨 这条 spec 存在的理由，是 644 个单元测试**一个都证明不了这件事**。
 * 拖动靠的是 `setPointerCapture` 和 `document.elementFromPoint`，
 * jsdom 两样都没有真的实现；`moveOutlineNode` 那 10 条测试证明的是「算清单
 * 算得对」，不是「手指按下去之后会发生这件事」。
 * 这正是 [[test-logic-not-endless-frontend]] 记的那一课：344 个绿测试底下
 * 曾经躺着一张全白的导出图。
 *
 * 三件事都要看，而且都只有真浏览器能判：
 *
 *  1. 拖得动：把一条拖到另一条上面，它挂过去了。
 *  2. 拖完不会弹「改一下这一条」—— 一次拖动几乎总是从字上起手。
 *  3. 卡片捕获了指针之后，删除那颗按钮还按得动。
 *
 * 🚨 第 3 条是隔壁会话踩过的那一类：捕获之后**落点那张卡收不到任何 pointer
 * 事件**，挂在目标上的逻辑会整个失效。这里顺带验一下按钮没被捕获废掉。
 */

const API = (process.env.E2E_API_BASE ?? "https://mind-api.uni-robot.cn").replace(/\/+$/, "");

test("思维导图：把一条拖到另一条上面，它挂过去", async ({ browser }) => {
  test.setTimeout(180_000);

  const ctx = await freshAccount(browser, "mindmap-drag");
  const page = await ctx.newPage();

  // 🚨 拦 prompt 要在拖之前挂上。挂在拖之后的话，拖动真的弹了框也看不见 ——
  // 而「拖完弹出改字框」正是这条 spec 要抓的那一个。
  let prompted = false;
  page.on("dialog", (d) => {
    prompted = true;
    void d.dismiss();
  });

  // 直接用接口把一份图摆好 —— 这条 spec 要验的是拖，不是印记会不会长出节点。
  const created = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "食堂浪费", lang: "zh" },
  });
  expect(created.ok(), `创建写作失败：${created.status()} ${await created.text()}`).toBeTruthy();
  const writingId = (await created.json()).id as string;

  // 先把「设定」走完：setup_at 还是 null 的话，房间弹的是设定窗，图根本不在屏幕上。
  const setup = await ctx.request.put(`${API}/api/v1/writings/${writingId}/setup`, {
    data: { lang: "zh", targetWords: 800, note: "" },
  });
  expect(setup.ok(), `设定失败：${setup.status()} ${await setup.text()}`).toBeTruthy();

  const put = await ctx.request.put(`${API}/api/v1/writings/${writingId}/outline`, {
    data: {
      outline: [
        // 🚨 kind 要给，而且要给真的那一个（0182）：服务端按它算深度，
        // 一个只给 role 的用例测的不是真客户端会发的东西。
        { text: "中心论点", kind: "thesis", depth: 0 },
        { text: "理由A", kind: "point", depth: 1 },
        { text: "她的经历", kind: "evidence", depth: 2 },
        { text: "理由B", kind: "point", depth: 1 },
        { text: "另一个中心论点", kind: "thesis", depth: 0 },
      ],
    },
  });
  expect(put.ok(), `摆图失败：${put.status()} ${await put.text()}`).toBeTruthy();

  await page.goto(`/writings/${writingId}`);
  const cardOf = (text: string) => page.locator(`[data-outline-node]`).filter({ hasText: text }).first();
  await expect(cardOf("理由A")).toBeVisible({ timeout: 30_000 });
  await page.screenshot({ path: "e2e/.writewalk-online/mindmap-before.png", fullPage: true });

  // 一次真的拖：按住「理由A」，挪到「另一个中心论点」上面，松手。
  const from = await cardOf("理由A").boundingBox();
  const to = await cardOf("另一个中心论点").boundingBox();
  expect(from, "找不到「理由A」那张卡").toBeTruthy();
  expect(to, "找不到「另一个中心论点」那张卡").toBeTruthy();

  await page.mouse.move(from!.x + from!.width / 2, from!.y + from!.height / 2);
  await page.mouse.down();
  // 分几步移动：一步到位的话 pointermove 只发一次，
  // 而 DRAG_SLOP 的判定和落点高亮都挂在 move 上。
  await page.mouse.move(to!.x + to!.width / 2, to!.y + to!.height / 2, { steps: 12 });
  await page.screenshot({ path: "e2e/.writewalk-online/mindmap-dragging.png", fullPage: true });
  await page.mouse.up();

  // 落库之后再读接口 —— 屏幕上对了而库里没变，等于她刷新一下就白拖了。
  await expect
    .poll(
      async () => {
        const res = await ctx.request.get(`${API}/api/v1/writings/${writingId}/outline`);
        if (!res.ok()) return "读不到";
        const rows = (await res.json()).outline as { text: string; depth: number; position: number }[];
        return rows
          .slice()
          .sort((a, b) => a.position - b.position)
          .map((r) => `${"  ".repeat(r.depth)}${r.text}`)
          .join("\n");
      },
      { timeout: 20_000, message: "拖完之后库里的图没变成预期的样子" },
    )
    .toBe(["中心论点", "  理由B", "另一个中心论点", "  理由A", "    她的经历"].join("\n"));

  await page.screenshot({ path: "e2e/.writewalk-online/mindmap-after.png", fullPage: true });

  // 🚨 拖完不该弹出「改一下这一条」——一次拖动几乎总是从字上起手。
  await page.waitForTimeout(500);
  expect(prompted, "拖完之后弹了「改一下这一条」——一次拖动是从字上起手的").toBeFalsy();

  // 点一下（不移动）仍然要能改字：justDragged 只挡拖，不挡点。
  await cardOf("理由B").locator("span", { hasText: "理由B" }).first().click();
  await page.waitForTimeout(300);
  expect(prompted, "点一下没有打开「改一下这一条」——拖把点也挡掉了").toBeTruthy();

  await ctx.close();
});

/**
 * 🚨 同事 2026-09-20 的意见 1：「论点被拖入到子论点后就没法拖出来了」。
 *
 * 几何那一半由 `e2e/harness/mindmap-harness.spec.ts` 守着（不连后端，跑得快）；
 * 这一条守的是另一半：**拖出来之后库里真的变了**。屏幕上对了而库里没变，
 * 等于她刷新一下就白拖了。
 */
test("思维导图：挂进子层的节点，拖到空白处能升回最上层", async ({ browser }) => {
  test.setTimeout(180_000);

  const ctx = await freshAccount(browser, "mindmap-promote");
  const page = await ctx.newPage();
  page.on("dialog", (d) => void d.dismiss());

  const created = await ctx.request.post(`${API}/api/v1/writings`, {
    data: { idea: "校服", lang: "zh" },
  });
  expect(created.ok(), `创建写作失败：${created.status()}`).toBeTruthy();
  const writingId = (await created.json()).id as string;

  const setup = await ctx.request.put(`${API}/api/v1/writings/${writingId}/setup`, {
    data: { lang: "zh", targetWords: 800, note: "" },
  });
  expect(setup.ok(), `设定失败：${setup.status()}`).toBeTruthy();

  const put = await ctx.request.put(`${API}/api/v1/writings/${writingId}/outline`, {
    data: {
      outline: [
        { text: "校服省心", kind: "thesis", depth: 0 },
        { text: "早上不用挑", kind: "point", depth: 1 },
        // 她本来想说的是第二条理由，却被挂到了第一条底下 —— 要能拖出来。
        { text: "本来是条理由", kind: "evidence", depth: 2 },
      ],
    },
  });
  expect(put.ok(), `摆图失败：${put.status()}`).toBeTruthy();

  await page.goto(`/writings/${writingId}`);
  const card = page.locator("[data-outline-node]").filter({ hasText: "本来是条理由" }).first();
  await expect(card).toBeVisible({ timeout: 30_000 });

  const canvas = (await page.locator(".mk-canvas").boundingBox())!;
  const box = (await card.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(canvas.x + canvas.width - 60, canvas.y + canvas.height - 60, { steps: 14 });
  // 松手之前就该看见会发生什么。
  await expect(page.getByText("放到空白处：移到最上层")).toBeVisible();
  await page.mouse.up();

  await expect
    .poll(
      async () => {
        const res = await ctx.request.get(`${API}/api/v1/writings/${writingId}/outline`);
        if (!res.ok()) return "读不到";
        const rows = (await res.json()).outline as { text: string; depth: number; kind: string; position: number }[];
        const moved = rows.find((r) => r.text === "本来是条理由");
        return moved ? `${moved.depth}/${moved.kind}` : "不见了";
      },
      { timeout: 20_000, message: "拖到空白处之后，那个节点没有升到最上层" },
    )
    // 升到深度 0，而且 kind 跟着改了：图上已经有中心论点，所以它是结尾。
    .toBe("0/closing");

  await ctx.close();
});
