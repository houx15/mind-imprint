import { test, expect } from "@playwright/test";

/**
 * 「诗按诗的样子摆」—— 在真浏览器里量一遍。
 *
 * 跑法：
 *   E2E_HARNESS_PORT=5251 npx vite --config e2e/harness/vite.config.ts
 *   E2E_HARNESS_PORT=5251 npx playwright test --config e2e/harness/playwright.config.ts poem-harness
 *
 * 🚨 断言量的是 `getComputedStyle` 和真实盒子，不是类名在不在。
 * memory `read-the-effective-css-layer-2026-09-20`：设计稿分层覆盖，
 * 读第一条同名规则读到的常常是被盖掉的旧值。
 */

const OUT = "e2e/harness/.shots";

test("一首诗摆成一块纸，说明文一个像素都不变", async ({ page }) => {
  await page.goto("/poem.html");

  const sheets = page.locator(".mk-poem-sheet");
  await expect(sheets).toHaveCount(3);

  // ① 那块纸真的画出来了：有边框、有自己的底色、比正文窄。
  const sheet = sheets.first();
  const sheetBox = (await sheet.boundingBox())!;
  expect(sheetBox.width).toBeLessThanOrEqual(561);
  const sheetStyle = await sheet.evaluate((el) => {
    const s = getComputedStyle(el);
    return { border: s.borderTopWidth, bg: s.backgroundColor, radius: s.borderTopLeftRadius };
  });
  expect(parseFloat(sheetStyle.border)).toBeGreaterThan(0);
  expect(sheetStyle.bg).not.toBe("rgba(0, 0, 0, 0)");
  expect(parseFloat(sheetStyle.radius)).toBeGreaterThan(0);

  // ② 🚨 最要紧的一条：换行留住了。
  //
  // 同一段文字，改前是连成一行的二十个字，改后是四行。量的是**高度**，
  // 因为「有没有换行」在 DOM 里看不出来 —— 文字一个字都没变。
  const before = page.locator('.mk-reading-room__article-inner:not([data-genre]) p[data-block-id]').first();
  const after = sheet.locator("p[data-block-id]").first();
  const beforeH = (await before.boundingBox())!.height;
  const afterH = (await after.boundingBox())!.height;
  // 🚨 比的是 textContent 不是 innerText：innerText 读的是**画出来的**样子，
  // 改前是一行、改后是四行，正好相反地证明了这件事 —— 而这条断言要钉的是
  // 「文字一个字都没动」。
  expect(await before.evaluate((el) => el.textContent)).toBe(
    await after.evaluate((el) => el.textContent),
  );
  // 改前确实是折成空格的那一行（毛病本身），改后是四行。
  expect((await before.innerText()).split("\n").length).toBe(1);
  expect((await after.innerText()).split("\n").length).toBe(4);
  expect(afterH).toBeGreaterThan(beforeH * 2.5);
  expect(await after.evaluate((el) => getComputedStyle(el).whiteSpace)).toBe("pre-wrap");

  // ③ 诗行是衬线体、字距拉开、比正文大。
  const poemType = await after.evaluate((el) => {
    const s = getComputedStyle(el);
    return { family: s.fontFamily, size: parseFloat(s.fontSize), spacing: s.letterSpacing };
  });
  expect(poemType.family.toLowerCase()).toContain("serif");
  expect(poemType.size).toBeGreaterThan(15);
  expect(parseFloat(poemType.spacing)).toBeGreaterThan(1);

  // ④ 整列居中，不是逐行居中。
  //
  // 🚨 这两件事在一首五绝上长得一模一样（每行等长），分得开的只有现代诗。
  // 判据：诗行盒子在纸里居中（列居中），而盒子**里面**是左对齐（不逐行居中）。
  const modernSheet = sheets.nth(1);
  const modern = modernSheet.locator("p[data-block-id]").first();
  expect(await modern.evaluate((el) => getComputedStyle(el).textAlign)).toBe("left");
  const modernBox = (await modern.boundingBox())!;
  const modernSheetBox = (await modernSheet.boundingBox())!;
  const leftGap = modernBox.x - modernSheetBox.x;
  const rightGap = modernSheetBox.x + modernSheetBox.width - (modernBox.x + modernBox.width);
  expect(Math.abs(leftGap - rightGap)).toBeLessThan(6);

  // ⑤ 诗上收起段号 —— 一首四句的诗常常只有一段，那个「1」不告诉她任何事。
  const poemMarker = await after.evaluate(
    (el) => getComputedStyle(el, "::before").display,
  );
  expect(poemMarker).toBe("none");

  // ⑥ 🚨 对照组：说明文那一屏一个像素都不该变。
  //
  // 这一条是这份看图台存在的另一半理由 —— 加一条按体裁分叉的样式，最容易的
  // 翻车方式是它漏到别的体裁上去。
  const prose = page.locator('.mk-reading-room__article-inner:not([data-genre])').last()
    .locator("p[data-block-id]").first();
  const proseType = await prose.evaluate((el) => {
    const s = getComputedStyle(el);
    return { size: parseFloat(s.fontSize), spacing: s.letterSpacing, ws: s.whiteSpace, marker: getComputedStyle(el, "::before").display };
  });
  expect(proseType.size).toBe(15);
  expect(proseType.spacing).toBe("normal");
  expect(proseType.ws).toBe("normal");
  expect(proseType.marker).not.toBe("none");

  await page.screenshot({ path: `${OUT}/poem-00-all.png`, fullPage: true });
  await sheets.first().screenshot({ path: `${OUT}/poem-01-jiangxue.png` });
  await modernSheet.screenshot({ path: `${OUT}/poem-02-modern.png` });
  await sheets.nth(2).screenshot({ path: `${OUT}/poem-03-seven.png` });
});
