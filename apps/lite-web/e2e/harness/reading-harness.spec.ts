import { test, expect } from "@playwright/test";

/**
 * 2026-09-22 那六条里**看得见**的四处，在真浏览器里看一眼 —— 不连后端、
 * 不要账号。
 *
 * 判据挑的都是 jsdom 量不出来的那种：盘在不在页签那一行里、浮层会不会
 * 掉出屏幕、工具条上多了两颗按钮之后折不折行、十五步那排圆点撑不撑得住。
 *
 * 跑法（先起看图台）：
 *   npx vite --config e2e/harness/vite.config.ts
 *   npx playwright test e2e/harness/reading-harness.spec.ts --config e2e/harness/playwright.config.ts
 */

const OUT = "e2e/harness/.shots";

test.beforeEach(async ({ page }) => {
  await page.goto("/reading.html");
  await expect(page.getByRole("button", { name: /带读进度/ })).toBeVisible({ timeout: 20_000 });
});

test("盘长在页签那一行里，而且不占印记说话的地方", async ({ page }) => {
  const disc = page.getByRole("button", { name: /带读进度/ });
  const tabs = page.locator(".mk-lite-coachtabs");

  // 🚨 几何判据：盘的竖直中心落在页签行里面。它原来浮在房间左下角，
  // 「搬到右栏」搬对了没有，只有这个量得出来。
  const d = (await disc.boundingBox())!;
  const t = (await tabs.boundingBox())!;
  expect(d.y).toBeGreaterThanOrEqual(t.y - 2);
  expect(d.y + d.height).toBeLessThanOrEqual(t.y + t.height + 2);
  // 靠右：盘的右边贴着页签行的右边，两个页签在它左边。
  expect(d.x + d.width).toBeGreaterThan(t.x + t.width - 8);

  // 🚨 常驻的那块步骤条没有了。产品负责人圈掉的就是它。
  await expect(page.locator(".mk-planwide")).toHaveCount(0);
  await expect(page.getByText("第 4 步 / 共 15 步")).toHaveCount(0);

  await page.screenshot({ path: `${OUT}/read-01-folded.png`, fullPage: true });
});

test("悬停给清单，而且不掉出屏幕", async ({ page }) => {
  await page.getByRole("button", { name: /带读进度/ }).hover();
  const panel = page.locator(".mk-plandial__panel");
  await expect(panel).toBeVisible();
  await expect(panel.getByText("第 4 步 / 共 15 步")).toBeVisible();

  // 🚨 浮层往左展开（盘在行的最右端）。往右展开就会被裁掉，而 jsdom 看不见。
  const p = (await panel.boundingBox())!;
  const vw = page.viewportSize()!.width;
  expect(p.x).toBeGreaterThanOrEqual(0);
  expect(p.x + p.width).toBeLessThanOrEqual(vw);

  await page.screenshot({ path: `${OUT}/read-02-hover.png`, fullPage: true });
});

test("点开是展开视图：这一步、它的说明、定位原文、十五个圆点", async ({ page }) => {
  await page.getByRole("button", { name: /带读进度/ }).click();

  const wide = page.locator(".mk-planwide");
  await expect(wide).toBeVisible();
  await expect(wide.getByText("第 4 步 / 共 15 步")).toBeVisible();
  await expect(wide.getByText("通读第10–11段·例外与转折")).toBeVisible();
  await expect(wide.getByText("读这两段，看作者在哪一句上承认了例外。")).toBeVisible();

  // 🚨 展开视图是盘的兄弟节点，所以它落在页签那一行里。它必须换到自己一行、
  // 且不许横着捅出这一栏 —— 第一版就是这么坏的（截图里它捅出了整栏，还把
  // 「阅读成果」压成了竖排），而当时这条测试是绿的。
  const col = (await page.getByTestId("coachcol").boundingBox())!;
  const wb = (await wide.boundingBox())!;
  expect(wb.x).toBeGreaterThanOrEqual(col.x - 1);
  expect(wb.x + wb.width).toBeLessThanOrEqual(col.x + col.width + 1);
  // 在页签行下面，不和它并排。
  const tabsBox = (await page.locator(".mk-lite-coachtabs > [role=tablist], .mk-lite-coachtabs").first().boundingBox())!;
  const discBox = (await page.getByRole("button", { name: /带读进度/ }).boundingBox())!;
  expect(wb.y).toBeGreaterThanOrEqual(discBox.y + discBox.height - 1);
  expect(tabsBox).toBeTruthy();

  // 十五个圆点都在，而且那一排是**横向滚**，不是把框撑破。
  //
  // 🚨 这条第一版写的是「最后一颗圆点在框里面」—— 那是在钉一件设计从来
  // 没有承诺过的事：十五颗圆点本来就摆不下，那一排一直是 `overflow-x: auto`
  // （产品负责人那张截图里它就停在 13）。判据要钉真失败（框被撑破），
  // 不是它的影子。
  const dots = wide.locator(".mk-planwide__path li");
  await expect(dots).toHaveCount(15);
  const path = wide.locator(".mk-planwide__path");
  const scrolls = await path.evaluate((el) => el.scrollWidth > el.clientWidth + 1);
  expect(scrolls, "那一排该是横向滚的").toBe(true);

  await wide.getByRole("button", { name: /定位原文/ }).click();
  await expect(page.getByTestId("log")).toContainText("定位原文 → b10");

  await page.screenshot({ path: `${OUT}/read-03-expanded.png`, fullPage: true });

  // 再点一下收回去 —— 收不回去它就是第二块常驻的步骤条。
  await page.getByRole("button", { name: /带读进度/ }).click();
  await expect(page.locator(".mk-planwide")).toHaveCount(0);
});

test("划选工具条：摘抄和放入对话框一直在，查词/语法看划的是词还是句", async ({ page }) => {
  const bars = page.locator(".mk-seltools");

  // 划的是一整句 → 语法在，查词不在。
  const sentence = bars.nth(0);
  await expect(sentence.getByRole("button", { name: "摘抄" })).toBeVisible();
  await expect(sentence.getByRole("button", { name: "放入对话框" })).toBeVisible();
  await expect(sentence.getByRole("button", { name: "语法" })).toBeVisible();
  await expect(sentence.getByRole("button", { name: "查词" })).toHaveCount(0);

  // 🚨 一条工具条不许折行。多了两颗按钮之后它还是一条 —— 折了就会盖住她
  // 刚划的那几个字，而那正是这条工具条唯一的指称对象。
  const barBox = (await sentence.boundingBox())!;
  for (const b of await sentence.locator("button").all()) {
    const bb = (await b.boundingBox())!;
    expect(bb.y).toBeGreaterThanOrEqual(barBox.y - 1);
    expect(bb.y + bb.height).toBeLessThanOrEqual(barBox.y + barBox.height + 1);
  }

  await sentence.getByRole("button", { name: "摘抄" }).click();
  await sentence.getByRole("button", { name: "放入对话框" }).click();
  await expect(page.getByTestId("log")).toContainText("摘抄");
  await expect(page.getByTestId("log")).toContainText("放入对话框");

  // 划的是一个词 → 查词在，语法不在；已经摘过的那一句按钮按不动。
  const word = bars.nth(1);
  await expect(word.getByRole("button", { name: "查词" })).toBeVisible();
  await expect(word.getByRole("button", { name: "语法" })).toHaveCount(0);
  await expect(word.getByRole("button", { name: "已摘抄" })).toBeDisabled();

  await page.screenshot({ path: `${OUT}/read-04-seltools.png`, fullPage: true });
});

test("阅读成果里的我摘抄的：回原文 + 和印记说", async ({ page }) => {
  // 钉那一节真正的标题（h3），不是裸 getByText —— 看图台的小标题里也有这三个字。
  await expect(page.getByRole("heading", { name: "我的摘抄", exact: true })).toBeVisible();
  await expect(page.getByText("Some professors fear that ChatGPT could lead to cheating.")).toBeVisible();

  await page.getByRole("button", { name: "回到第 3 段" }).click();
  await page.getByRole("button", { name: "讨论这句" }).first().click();

  const log = page.getByTestId("log");
  await expect(log).toContainText("回到原文 b3");
  await expect(log).toContainText("和印记说 b3");

  await page.screenshot({ path: `${OUT}/read-05-harvest.png`, fullPage: true });
});
