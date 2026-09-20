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
