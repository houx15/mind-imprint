import { test, expect, type Page } from "@playwright/test";

/**
 * 在**真浏览器**里看那张思维导图 —— 不连后端、不要账号。
 *
 * 🚨 这条 spec 要验的三件事，单元测试一件都证明不了，因为它们全是几何：
 *
 *  1. **拖得出来**（同事 2026-09-20 的意见 1）：一条挂在深度 2 的节点，
 *     拖到空白画布上，升到最上层。这之前是空操作 —— 挂进去就再也出不来。
 *  2. **卡片上三分之一 = 放到旁边**：这条判据是像素，`moveOutlineNode`
 *     那十几条测试一个字都没碰它。
 *  3. **结尾就是结尾**（意见 3）：一个被挂到深度 1 的结尾，卡片上印的必须是
 *     「结尾」，不是「分论点 3」。
 *
 * 跑法（先起看图台）：
 *   npx vite --config e2e/harness/vite.config.ts
 *   npx playwright test e2e/harness/mindmap-harness.spec.ts --config e2e/harness/playwright.config.ts
 */

const OUT = "e2e/harness/.shots";

async function shape(page: Page): Promise<string> {
  return (await page.getByTestId("shape").innerText()).split("--- 拖动记录 ---")[0]!.trim();
}

const cardOf = (page: Page, text: string) =>
  page.locator("[data-outline-node]").filter({ hasText: text }).first();

/** 一次真的拖：分步移动，因为 DRAG_SLOP 和落点判定都挂在 pointermove 上。 */
async function dragTo(page: Page, from: string, to: { x: number; y: number }) {
  const box = await cardOf(page, from).boundingBox();
  expect(box, `找不到「${from}」那张卡`).toBeTruthy();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x, to.y, { steps: 14 });
}

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await expect(cardOf(page, "早上不用想穿什么")).toBeVisible({ timeout: 20_000 });
});

test("🚨 意见 3：挂在深度 1 的结尾，卡片上印的是「结尾」", async ({ page }) => {
  // 卡片上的小标题来自 kind 的那张表，不看深度。
  const closing = cardOf(page, "结尾回到");
  await expect(closing).toContainText("结尾");
  await expect(closing).not.toContainText("分论点");
  await page.screenshot({ path: `${OUT}/01-labels.png`, fullPage: true });
});

test("🚨 意见 1：把一条拖到空白画布上，它升到最上层", async ({ page }) => {
  expect(await shape(page)).toContain("    evidence: 我早上经常起晚");

  // 画布右下角的空白处。
  const canvas = (await page.locator(".mk-canvas").boundingBox())!;
  await dragTo(page, "我早上经常起晚", {
    x: canvas.x + canvas.width - 60,
    y: canvas.y + canvas.height - 60,
  });

  // 🚨 松手**之前**就要看得见会发生什么。
  await expect(page.getByText("放到空白处：移到最上层")).toBeVisible();
  await page.screenshot({ path: `${OUT}/02-dragging-to-blank.png`, fullPage: true });

  await page.mouse.up();

  const after = await shape(page);
  // 升到了最上层（没有缩进），而且 kind 跟着改了 —— 图上已经有中心论点，
  // 所以再上来一个最上层的块就是结尾。
  expect(after).toContain("closing: 我早上经常起晚");
  expect(after).not.toContain("    evidence: 我早上经常起晚");
  await page.screenshot({ path: `${OUT}/03-after-promote.png`, fullPage: true });
});

test("卡片上三分之一 = 放到它旁边，其余 = 挂到它底下", async ({ page }) => {
  const target = (await cardOf(page, "校服不好看").boundingBox())!;

  // 上三分之一：成为兄弟（深度 1），于是它从论据变成分论点。
  await dragTo(page, "我早上经常起晚", { x: target.x + target.width / 2, y: target.y + 4 });
  await expect(cardOf(page, "校服不好看")).toHaveClass(/mk-node-drop-sibling/);
  await page.screenshot({ path: `${OUT}/04-hover-sibling.png`, fullPage: true });
  await page.mouse.up();
  expect(await shape(page)).toContain("  point: 我早上经常起晚");
});

test("拖到卡片中间 = 挂到它底下，整张卡亮起来", async ({ page }) => {
  const target = (await cardOf(page, "校服不好看").boundingBox())!;
  await dragTo(page, "我早上经常起晚", {
    x: target.x + target.width / 2,
    y: target.y + target.height / 2,
  });
  await expect(cardOf(page, "校服不好看")).toHaveClass(/mk-node-drop(?!-sibling)/);
  await page.screenshot({ path: `${OUT}/05-hover-child.png`, fullPage: true });
  await page.mouse.up();
  // 还是一条论据（深度没变），只是换了个爹。
  expect(await shape(page)).toContain("    evidence: 我早上经常起晚");
});

// 同事 2026-09-22 的意见 3 的界面那一半：卡片上那个小标题点得动。
//
// 🚨 在这之前她**没有任何办法**改它。图上能拖，但拖动只改深度，
// 任何拖到深度 1 的东西一律变成「分论点」—— 「反方观点」这一种根本到不了。
test("🚨 卡片上的小标题点开是「这一条是什么」，改完当场变", async ({ page }) => {
  await page.goto("/");
  const label = page.getByRole("button", { name: "分论点", exact: true }).first();
  await expect(label).toBeVisible({ timeout: 20_000 });

  await label.click();
  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  // 闭表里议论文那一套都摆出来了，「反方观点」在里面。
  await expect(menu.getByRole("menuitem", { name: /反方观点/ })).toBeVisible();
  await page.screenshot({ path: "e2e/harness/.shots/r5-02-kind-picker.png", fullPage: true });

  await menu.getByRole("menuitem", { name: /反方观点/ }).click();
  await expect(menu).toBeHidden();
  // 改完当场看得见 —— 图上多了一张写着「反方观点」的卡。
  await expect(page.getByRole("button", { name: "反方观点", exact: true })).toBeVisible();
  await page.screenshot({ path: "e2e/harness/.shots/r5-03-kind-changed.png", fullPage: true });
});
