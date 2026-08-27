import { expect, test, type Page } from "@playwright/test";

/**
 * 带读 is the room's front door now, and this file walks it.
 *
 * The two rulings it protects (see the Amendment section of
 * docs/superpowers/specs/2026-08-27-lite-reading-room-scaffold-design.md):
 *
 *  - **印记 leads; she does not manage stages.** So the assertions here are
 *    the negative ones as much as the positive: no 做完了, no 跳过, and nothing
 *    in the progress list is clickable. An edit that "helpfully" gives her
 *    back a checkbox fails here.
 *  - **The paragraph tools are instruments, not a permanent shelf.** 详细带读
 *    is per paragraph, and the panel it opens carries the language-neutral
 *    想一想 / 仿写 alongside the explainers for the ARTICLE's own language.
 *
 * It lives beside reading-walk.spec.ts rather than inside it because that file
 * is one continuous journey (land → read → 完成 → 已完成) and this is a
 * different entrance to the same room.
 */

const RUN = Date.now().toString(36);
const titled = (name: string) => `${name} ${RUN}`;

// Chinese, four paragraphs — so the panel must offer 成语修辞 and never 翻译.
// Blank lines matter: the server splits blocks on them.
const ARTICLE_BODY = [
  "过去十年，全球太阳能装机容量增长了大约十倍。推动这件事的不是某一项突破性发明，而是制造规模、供应链和融资成本三件事同时变便宜。",
  "成本下降的幅度常被单独拎出来当作结论：组件价格在这十年里下降了八成以上。但价格只是发电成本的一部分，土地、并网、运维和资金成本在不同国家差别极大。",
  "真正的瓶颈已经从「发电贵不贵」转移到「电什么时候来」。太阳能的出力集中在正午前后，而用电高峰往往在傍晚，两者之间的错位要靠储能、需求响应或跨区输电来填。",
  "所以，如果储能和电网的问题不解决，继续增加装机带来的边际收益会递减：白天多出来的电卖不掉，甚至要被弃掉。",
].join("\n\n");

const BODY_PLACEHOLDER = "贴一个链接，或者把整篇正文粘进来——也可以上传 DOCX / PDF";
const TITLE_PLACEHOLDER = "给这次阅读起个名字（可留空）";
const READING_URL = /\/readings\/[0-9a-f-]{36}$/;

async function startReading(page: Page, title: string): Promise<void> {
  await page.goto("/readings");
  await page.getByPlaceholder(TITLE_PLACEHOLDER).fill(title);
  await page.getByPlaceholder(BODY_PLACEHOLDER).fill(ARTICLE_BODY);
  await page.getByRole("button", { name: "开始阅读" }).click();
  await expect(page).toHaveURL(READING_URL, { timeout: 30_000 });
  await expect(page.locator(".mk-reading-room")).toBeVisible({ timeout: 30_000 });
}

/**
 * One live model call (the plan turn), which is why this asserts on the SHAPE
 * of what came back — a numbered route and a first instruction — and never on
 * particular sentences. The model is free to phrase it its own way; it is not
 * free to hand her a checklist to work.
 */
test("带读: 印记 plans the route and leads, and she administrates none of it", async ({ page }) => {
  await startReading(page, titled("带读走查"));

  // The invitation, before anything has been spent.
  await expect(page.getByText("让我来带你详细读一遍这篇文章吧。")).toBeVisible();
  const start = page.getByRole("button", { name: "开始", exact: true });
  await expect(start).toBeVisible();

  await start.click();

  // The plan turn is a real model call on the chaperone tier.
  await expect(page.getByText("带读进度")).toBeVisible({ timeout: 120_000 });

  // A route of more than a couple of steps, and a first instruction to act on.
  // Scoped to the progress card, not a bare `ol` — an unscoped list selector
  // would stay green off any other list in the room if this one disappeared,
  // which is exactly the regression the next three assertions are for.
  const progress = page
    .locator("div")
    .filter({ has: page.getByText("带读进度") })
    .filter({ has: page.locator("ol") })
    .last();
  const steps = progress.locator("ol > li");
  expect(await steps.count()).toBeGreaterThan(2);

  // Ruling 1: progress, not controls. She has no way to mark her own step
  // done, and no step is clickable — 印记 moves her.
  await expect(page.getByRole("button", { name: "做完了" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "跳过" })).toHaveCount(0);
  await expect(steps.locator("button")).toHaveCount(0);

  // Skipping did not disappear, it moved into language: the composer is the
  // only control, and it invites her to answer rather than to administrate.
  await expect(page.getByPlaceholder(/读完这一步/)).toBeVisible();
});

/**
 * Deliberately no model call: this asserts the instruments are REACHABLE and
 * correctly scoped, which is the part a refactor breaks. Whether the coach
 * reaches for one is a model decision, and belongs in the unit tests where it
 * can be made deterministic (reading_coach_test.go).
 */
test("详细带读 opens one paragraph, carrying 想一想 and 仿写", async ({ page }) => {
  await startReading(page, titled("段落工具走查"));

  const opener = page.getByRole("button", { name: "详细带读" });
  // One per paragraph — not a single global button, and not a permanent shelf.
  expect(await opener.count()).toBe(4);

  await opener.nth(1).click();
  await expect(page.getByText("这一段").first()).toBeVisible();

  // Chinese article ⇒ the Chinese explainers, never the English ones. The
  // toolset is derived from the article's own characters server-side, so a
  // failure here means the language detection drifted.
  await expect(page.getByRole("button", { name: "成语修辞" })).toBeVisible();
  await expect(page.getByRole("button", { name: "翻译" })).toHaveCount(0);

  // Language-neutral, so present on every article whatever its language.
  await expect(page.getByRole("button", { name: "想一想" })).toBeVisible();
  await expect(page.getByRole("button", { name: "仿写" })).toBeVisible();
});
