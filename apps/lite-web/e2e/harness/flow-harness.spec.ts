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

test("行文：只有分论点能标论证方法，论据和结尾不能", async ({ page }) => {
  // 🚨 那个下拉**不是**原生 <select>（2026-09-17 的裁定：不用原生表单控件），
  // 所以它的 role 是 button + aria-haspopup="listbox"，不是 combobox。
  // 第一版按 combobox 找，找到 0 个 —— 用例的眼睛错了，不是产品少了东西。
  const pickers = page.locator('button[aria-haspopup="listbox"]');
  // 两条分论点 → 两个下拉。论据、结尾、中心论点都没有。
  await expect(pickers).toHaveCount(2, { timeout: 20_000 });
  await expect(page.getByLabel("「放学能联系家长」用哪个论证方法")).toBeVisible();
  // 已经标过的那一条显示它的正式名称。
  await expect(pickers.first()).toContainText("举例论证");
});

test("🚨 行文这一步一个字的正文都不写（同事：并非填充内容）", async ({ page }) => {
  await expect(page.getByText(/这一步不写正文/)).toBeVisible({ timeout: 20_000 });
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

  await page.getByRole("button", { name: "放大看这张图" }).click();
  await expect(page.getByRole("button", { name: "关闭" })).toBeVisible();
  await page.screenshot({ path: `${OUT}/r3-03-map-zoom.png`, fullPage: true });
});

test("引导框：段内结构排在方法名前面，而且有「分析」那一步", async ({ page }) => {
  await page.getByTestId("go-snippets").click();
  const steps = page.getByRole("heading", { name: "这一段里的几步" });
  await expect(steps).toBeVisible({ timeout: 20_000 });

  // 学生最常跳过的那一步必须在。
  await expect(page.getByText("分析", { exact: true })).toBeVisible();
  await expect(page.getByText(/这一步最常被跳过/)).toBeVisible();

  // 「想一想」那一节在它下面 —— 先说这一段怎么搭，再问问题。
  const think = page.getByRole("heading", { name: "想一想" });
  if (await think.isVisible()) {
    expect((await steps.boundingBox())!.y).toBeLessThan((await think.boundingBox())!.y);
  }
});
