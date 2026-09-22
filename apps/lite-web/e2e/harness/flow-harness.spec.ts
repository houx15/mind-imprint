import { test, expect } from "@playwright/test";

/**
 * R3 那三块新屏，在真浏览器里看一眼 —— 不连后端、不要账号。
 *
 *  1. **行文**（同事的意见 4）：四张论证结构卡、每条分论点一个方法下拉、
 *     那张能拖的图。这一步一个字的正文都不写。
 *  2. **段落页左栏顶上那张缩略图**（意见 5 的 UI）：图在上、引导在下，
 *     点「放大」开一个浮层。
 *  3. **「这一段里的几步」**（意见 5 的语言）：段内结构排在方法名前面。
 *
 * 跑法（先起看图台）：
 *   npx vite --config e2e/harness/vite.config.ts
 *   npx playwright test e2e/harness/flow-harness.spec.ts --config e2e/harness/playwright.config.ts
 */

const OUT = "e2e/harness/.shots";

test.beforeEach(async ({ page }) => {
  await page.goto("/flow.html");
});

test("行文：四张论证结构卡，选一个再点一下取消", async ({ page }) => {
  for (const name of ["总分式", "并列式", "层进式", "对照式"]) {
    await expect(page.getByRole("button", { name: new RegExp(name) })).toBeVisible({ timeout: 20_000 });
  }
  // 每张卡都要带一句借来的示范 —— 光给定义，「层进式」和「并列式」
  // 在一个中学生眼里是同一句话。
  await expect(page.getByText(/比如：/).first()).toBeVisible();

  await page.getByRole("button", { name: /层进式/ }).click();
  await expect(page.getByTestId("structure")).toHaveText("struct_progressive");
  await page.screenshot({ path: `${OUT}/r3-01-flow.png`, fullPage: true });

  // 再点一下 = 取消。
  await page.getByRole("button", { name: /层进式/ }).click();
  await expect(page.getByTestId("structure")).toHaveText("(还没选)");
});

test("🚨 行文：顺序真的改得动，而且那个下拉没了（同事 2026-09-22 的意见 4、5）", async ({ page }) => {
  // 意见 5：「this is awkward this selection... they would hate this.」
  // 那个「每一条打算怎么证明」的下拉整块拿掉了 —— 它存下来的 method
  // 没有任何一条提示词读过。
  await expect(page.locator('button[aria-haspopup="listbox"]')).toHaveCount(0, { timeout: 20_000 });
  await expect(page.getByText("每一条打算怎么证明")).toHaveCount(0);

  // 意见 4：「这三个论点的顺序无法拖动改变」。原来那一行左边有个抓手图标，
  // 而那一行**没有任何一个拖动事件** —— 图标承诺了一件它做不到的事。
  const rows = page.locator('[aria-label^="把「"]');
  // 两条分论点 → 各一对上下按钮。
  await expect(rows).toHaveCount(4);

  // 第一条的「往前挪」该是禁用的，最后一条的「往后挪」也一样。
  await expect(page.getByLabel("把「放学能联系家长」往前挪")).toBeDisabled();
  await expect(page.getByLabel("把「学习上能查不会的题」往后挪")).toBeDisabled();

  await page.screenshot({ path: `${OUT}/r5-01-flow-order.png`, fullPage: true });

  // 往后挪一条：落库那一份清单里，它排到了另一条后面。
  await page.getByLabel("把「放学能联系家长」往后挪").click();
  const put = await page.evaluate(() => (window as unknown as { lastPut?: { outline?: { text: string }[] } }).lastPut);
  const texts = (put?.outline ?? []).map((n) => n.text);
  expect(texts.indexOf("学习上能查不会的题")).toBeLessThan(texts.indexOf("放学能联系家长"));
  // 🚨 挂在它底下的材料跟着走，没有掉在原地。
  expect(texts.indexOf("放学能联系家长")).toBeLessThan(texts.indexOf("上周三五点半那次"));
});

test("🚨 行文这一步一个字的正文都不写（同事：并非填充内容）", async ({ page }) => {
  await expect(page.getByRole("heading", { name: "行文", exact: true })).toBeVisible({ timeout: 20_000 });
  // 板上不该有任何一个可以输入正文的地方。
  await expect(page.locator("textarea")).toHaveCount(0);
  await expect(page.locator('input[type="text"]')).toHaveCount(0);
});

test("段落左栏：图在上、引导在下，点放大开浮层", async ({ page }) => {
  await page.getByTestId("go-snippets").click();

  const map = page.getByRole("heading", { name: "这一篇的结构" });
  const guide = page.getByText("写作引导");
  await expect(map).toBeVisible({ timeout: 20_000 });
  await expect(guide).toBeVisible();

  // 🚨 图必须在引导**上面** —— 同事截图里那个箭头指的就是这个顺序。
  const mapBox = (await map.boundingBox())!;
  const guideBox = (await guide.boundingBox())!;
  expect(mapBox.y, "图该排在引导上面").toBeLessThan(guideBox.y);
  await page.screenshot({ path: `${OUT}/r3-02-snippets-left.png`, fullPage: true });

  await page.getByRole("button", { name: "放大查看这一篇的结构" }).click();
  await expect(page.getByRole("button", { name: "关闭结构预览" })).toBeVisible();
  await page.screenshot({ path: `${OUT}/r3-03-map-zoom.png`, fullPage: true });
});

test("引导框：段内结构排在方法名前面，主体段的五句都在，而且顺序对", async ({ page }) => {
  await page.getByTestId("go-snippets").click();
  const steps = page.getByRole("heading", { name: "这一段里的几步" });
  await expect(steps).toBeVisible({ timeout: 20_000 });

  // 🚨 这里原来钉的是「分析」这两个字，和一句写死的提示语
  //（「这一步最常被跳过」）。R4 把主体段从四步换成讲义的五句型之后，
  // 那一格叫「分析句」，那句提示也挪到了「阐释句」上 —— 于是这条从 R4
  // 那天起就一直红着，而产品一点问题都没有。
  // 钉不变的那一截：**五格都在，而且顺序是讲义那个顺序**。
  // 提示语一个字都不钉 —— 它是会改的文案。
  const labels = ["观点句", "阐释句", "材料句", "分析句", "结论句"];
  const ys: number[] = [];
  for (const label of labels) {
    const cell = page.getByText(label, { exact: true });
    await expect(cell, `主体段少了「${label}」那一格`).toBeVisible();
    ys.push((await cell.boundingBox())!.y);
  }
  for (let i = 1; i < ys.length; i++) {
    expect(ys[i]!, `「${labels[i]}」该排在「${labels[i - 1]}」后面`).toBeGreaterThan(ys[i - 1]!);
  }

  // 「想一想」那一节在它下面 —— 先说这一段怎么搭，再问问题。
  const think = page.getByRole("heading", { name: "想一想" });
  if (await think.isVisible()) {
    expect((await steps.boundingBox())!.y).toBeLessThan((await think.boundingBox())!.y);
  }
});
