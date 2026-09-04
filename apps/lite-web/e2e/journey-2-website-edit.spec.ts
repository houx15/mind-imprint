import { expect, test, type Page } from "@playwright/test";
import { openSiteGate } from "./gate";

/**
 * 旅程二 · 她能改自己那一页。
 *
 * 产品负责人 2026-09-04：「2) he/she can edit the website」，以及
 * 「we can set that website to public viewable with sharable qrcode and link,
 *   and also can remain it private」。
 *
 * ## 「改」在这个产品里是哪两件事
 *
 * 1. **上面的字。** 她回对话里说给印记，印记把她的**原话**摆上去
 *    （`GroundSiteDraft` 逐字比对，对不上的静静丢掉）。这一屏没有输入框是设计，
 *    不是缺功能——归 `journey-website` 那条验。
 * 2. **它长什么样。** 配色、风格、头图，都在「视觉基调」里。
 *
 * 🚨 第 2 件在 2026-09-04 之前**没有入口**：材料清单只有「我的主页」一行，
 * 而「视觉基调」只有印记在第三关递过那一次，递完就回不去了。于是「发布不是
 * 终点，她随时能回来改」这句话只对文字成立，对样子不成立。这条 walk 钉的就是
 * 补上的那个入口，外加公开／私密这一对开关。
 *
 * 建这一页走的是接口（`openSiteGate`）——从零做出一页归 `journey-website`，
 * 这一条要看的是**做好之后还能不能动它**。
 */

test.describe.configure({ retries: 0 });

const API = (process.env.E2E_API_BASE ?? "").replace(/\/+$/, "");

interface SiteState {
  url: string;
  published: boolean;
  palette: { label: string; paper: string };
}

async function site(page: Page): Promise<SiteState> {
  return (await (await page.request.get(`${API}/api/v1/pbl/site`)).json()) as SiteState;
}

/** 从材料清单打开一件工具。发布之后她就是从这儿回去的。 */
async function openMaterial(page: Page, label: string, heading: string): Promise<void> {
  const close = page.getByRole("button", { name: "收起" });
  if (await close.count()) await close.first().click();
  await page.getByRole("button", { name: new RegExp(label) }).first().click();
  await expect(page.getByRole("heading", { name: heading })).toBeVisible({ timeout: 30_000 });
}

test("旅程二: 已经发布的主页 → 换一套配色 → 撤回 → 再上线", async ({ page }) => {
  page.on("pageerror", (e) => console.log("PAGEERROR:", e.message));

  /* 0 · 一页已经发布出去的主页。 */
  await page.goto("/projects");
  await openSiteGate(page);
  const made = await page.request.post(`${API}/api/v1/pbl/site/project`);
  expect([200, 201]).toContain(made.status());
  const id = (await made.json()).id as string;

  const before = await site(page);
  expect(before.published, "前置没做成：这一页应该已经在线上").toBe(true);

  await page.goto(`/projects/${id}`);
  await expect(page.getByPlaceholder("请输入")).toBeVisible();

  // 🚨 前置还差一样：**配色是从关一的关键词派生的**。
  //
  // 没有定下的读者，`POST /pbl/site/palettes` 会 400 `no_keywords`——那是对的
  // 行为（「挑一个你喜欢的颜色」是一道和这个项目无关的题），但它意味着这条
  // walk 必须先有一个选中的读者。关一怎么走归 `journey-website` 验，这里照样
  // 走接口把它摆好。
  const api = `${API}/api/v1/pbl/projects/${id}`;
  await expect
    .poll(
      async () => {
        const r = await page.request.get(`${api}/thread`);
        return r.ok() ? ((await r.json()) as unknown[]).length : 0;
      },
      { timeout: 180_000, message: "开场那一轮没落库" },
    )
    .toBeGreaterThanOrEqual(2);
  await page.getByPlaceholder("请输入").fill(
    "我高二，在读 IB，这两年一直在拆家里坏掉的电器，拆完写一篇为什么它修不好。",
  );
  await page.getByRole("button", { name: "发送" }).click();
  await expect
    .poll(
      async () => {
        const r = await page.request.get(`${api}/thread`);
        return r.ok() ? ((await r.json()) as unknown[]).length : 0;
      },
      { timeout: 180_000 },
    )
    .toBeGreaterThanOrEqual(4);

  const gen = await page.request.post(`${api}/personas/generate`);
  expect(gen.ok(), `生成读者失败：${gen.status()} ${await gen.text()}`).toBe(true);
  const people = (await gen.json()) as { id: string; keywords: string[] }[];
  expect(people.length, "一个候选读者都没生成出来").toBeGreaterThan(0);
  const pick = await page.request.post(`${api}/personas/${people[0].id}/choose`, {
    data: { keywords: people[0].keywords },
  });
  expect(pick.ok(), `定下读者失败：${pick.status()} ${await pick.text()}`).toBe(true);
  await page.reload();
  await expect(page.getByPlaceholder("请输入")).toBeVisible();

  /* 1 · 她从材料清单回到自己那一页。发布不是终点。 */
  await openMaterial(page, "我的主页", "上线");
  await expect(page.getByRole("button", { name: "撤回链接" })).toBeVisible();
  // 二维码和链接都在（她要发给家里人）。
  await expect(page.getByAltText("二维码")).toBeVisible();
  await expect(page.getByRole("button", { name: "复制链接" })).toBeVisible();
  await page.screenshot({ path: "e2e/.shots/j2-1-live.png", fullPage: true });

  /* 2 · 换一套配色 —— 这是「改这一页长什么样」唯一的地方。 */
  await openMaterial(page, "视觉基调", "视觉基调");
  await page.getByRole("button", { name: /看看配色|换一批/ }).click();
  const swatch = page.getByTestId("palette-option");
  await expect(swatch.first()).toBeVisible({ timeout: 180_000 });
  // 挑一个和现在这套不一样的。
  await swatch.nth((await swatch.count()) > 1 ? 1 : 0).click();
  await page.getByRole("button", { name: "确认选择" }).click();
  await expect
    .poll(async () => (await site(page)).palette.paper, { timeout: 30_000 })
    .not.toBe(before.palette.paper);
  await page.screenshot({ path: "e2e/.shots/j2-2-repalette.png", fullPage: true });

  /* 3 · 访客看到的就是新的那一套。改完不用重新发布。 */
  const after = await site(page);
  const visitor = await page.context().newPage();
  await visitor.goto(after.url);
  const paper = await visitor.evaluate(() => {
    const el = document.querySelector(".mk-site");
    return el ? getComputedStyle(el).getPropertyValue("--st-paper").trim() : "";
  });
  expect(paper.toLowerCase(), "她换了配色，访客看到的还是旧的").toBe(
    after.palette.paper.toLowerCase(),
  );
  await visitor.close();

  /* 4 · 收回来：撤回链接之后，拿着链接的人也打不开。 */
  await openMaterial(page, "我的主页", "上线");
  await page.getByRole("button", { name: "撤回链接" }).click();
  await expect(page.getByRole("button", { name: "上线", exact: true })).toBeVisible({
    timeout: 30_000,
  });
  expect((await site(page)).published, "按了撤回，服务端还说它在线上").toBe(false);

  // 🚨 查的是**接口**，不是那个地址。`/p/<token>` 是前端路由，dev server 对任何
  // 路径都回 index.html，所以 `GET url` 永远是 200——照那个断言写，撤回坏掉了也
  // 测不出来。真正决定访客看不看得到的是公开接口。
  const token = after.url.slice(after.url.lastIndexOf("/p/") + 3);
  const gone = await page.request.get(`${API}/api/v1/public/sites/${token}`);
  expect(gone.status(), "撤回之后，公开接口还把这一页给出去").toBe(404);

  // 而且拿着旧链接的人打开看到的不再是她那一页。
  const stranger = await page.context().newPage();
  await stranger.goto(after.url);
  await expect(stranger.getByText(before.palette.label).first()).toHaveCount(0);
  await stranger.screenshot({ path: "e2e/.shots/j2-3b-stranger-blocked.png", fullPage: true });
  await stranger.close();
  await page.screenshot({ path: "e2e/.shots/j2-3-private.png", fullPage: true });

  /* 5 · 再放出去。私密和公开是一对开关，不是一次性的决定。 */
  await page.getByRole("button", { name: "上线", exact: true }).click();
  await expect(page.getByRole("button", { name: "撤回链接" })).toBeVisible({ timeout: 30_000 });
  expect((await site(page)).published).toBe(true);
  await page.screenshot({ path: "e2e/.shots/j2-4-live-again.png", fullPage: true });
});
