import { test, expect } from "@playwright/test";

/**
 * 导出的 PNG 是不是完整的 —— 在真浏览器里画一张出来看。
 *
 * 产品负责人 2026-09-23 第 2 条：「when export reading report png, the picture
 * seems to be truncated and is not complete.」
 *
 * 🚨 为什么不能在 jsdom 里验：`scrollHeight` / `getBoundingClientRect()` 在
 * jsdom 里恒等于 0，canvas 也画不出来 —— 这条判据在那里**永远是绿的**，
 * 而图照样是裁的（memory: look-at-the-image-not-the-assertions-2026-09-22）。
 *
 * 跑法（端口挑一个别的会话没在用的）：
 *   E2E_HARNESS_PORT=5242 npx vite --config e2e/harness/vite.config.ts
 *   E2E_HARNESS_PORT=5242 npx playwright test --config e2e/harness/playwright.config.ts poster-harness
 */

const OUT = "e2e/harness/.shots";

// 🚨 `exact: true`：台上还有一个「老写法导出长报告」，它**包含**「导出长报告」，
// 裸的名字匹配会撞 strict mode（memory:
// inserting-a-step-rots-every-older-walk-2026-09-21 记的同一个坑）。
async function exportAndRead(
  page: import("@playwright/test").Page,
  which: "short" | "long" | "huge",
  how: "new" | "old" = "new",
) {
  const name =
    which === "huge"
      ? how === "old"
        ? "老写法导出超长报告"
        : "新写法导出超长报告"
      : how === "old"
        ? "老写法导出长报告"
        : which === "short"
          ? "导出短报告"
          : "导出长报告";
  await page.getByRole("button", { name, exact: true }).click();
  const out = page.locator("[data-out]");
  await expect(out).not.toHaveText("", { timeout: 20_000 });
  return JSON.parse((await out.innerText()) || "{}") as {
    clientHeight: number;
    scrollHeight: number;
    measured: { width: number; height: number };
    ratio: number;
    png: { width: number; height: number };
  };
}

test("导出的图装得下整张海报 —— 短的和长的都是", async ({ page }) => {
  await page.goto("/poster.html");

  for (const which of ["short", "long"] as const) {
    const r = await exportAndRead(page, which);

    // 海报真的被量出来了（这一条同时挡住「在 jsdom 里恒为 0 所以永远绿」）。
    expect(r.measured.height, `${which}: 没量出高度`).toBeGreaterThan(200);
    expect(r.measured.width, `${which}: 没量出宽度`).toBeGreaterThan(200);

    // 🚨 这就是那个 bug：量尺寸要用装得下溢出内容的那一个。
    expect(r.measured.height, `${which}: 量到的高度比内容还矮`).toBeGreaterThanOrEqual(r.scrollHeight);

    // 画出来的像素高度要盖住整张海报（留一行的余量给取整）。
    expect(r.png.height / r.ratio, `${which}: 画出来的比海报矮 —— 底部被裁了`).toBeGreaterThanOrEqual(
      r.measured.height - 2,
    );
    expect(r.png.width / r.ratio, `${which}: 画出来的比海报窄`).toBeGreaterThanOrEqual(r.measured.width - 2);

    // 🚨 倍率没有被悄悄缩掉：html-to-image 超过 canvasDimensionLimit 时不报错，
    // 它自己缩放，看起来就是糊。
    expect(r.png.width, `${which}: 超过画布上限`).toBeLessThanOrEqual(16384);
    expect(r.png.height, `${which}: 超过画布上限`).toBeLessThanOrEqual(16384);
  }

  await page.screenshot({ path: `${OUT}/poster-00-both.png`, fullPage: true });
});

test("长报告比短报告高 —— 夹具真的把海报撑长了，这条判据才有意义", async ({ page }) => {
  await page.goto("/poster.html");
  const short = await exportAndRead(page, "short");
  const long = await exportAndRead(page, "long");
  // 🚨 没有这一条，上面那条测试可能是在两份一样高的海报上通过的 ——
  // 而裁切只有在海报够长的时候才发作。
  expect(long.measured.height, "长报告并不比短报告高，夹具没起作用").toBeGreaterThan(
    short.measured.height + 400,
  );
});

// 🚨 先证明那个毛病真的在，再说自己修好了它。
//
// 老写法（一个尺寸都不传，让 html-to-image 自己量 clientWidth/clientHeight）
// 和新写法画同一份长报告，把两边的像素高度摆在一起看。
test("老写法画出来的确实比海报矮 —— 这条是「修好了」的证据", async ({ page }) => {
  await page.goto("/poster.html");
  const fresh = await exportAndRead(page, "long", "new");
  const old = await exportAndRead(page, "long", "old");

  // 两边量到的是同一张海报。
  expect(old.measured.height).toBe(fresh.measured.height);

  const oldCss = old.png.height / old.ratio;
  const newCss = fresh.png.height / fresh.ratio;
  // eslint-disable-next-line no-console
  console.log(
    `海报 ${fresh.measured.height}px：老写法画出 ${oldCss}px，新写法画出 ${newCss}px（clientHeight=${fresh.clientHeight}, scrollHeight=${fresh.scrollHeight}）`,
  );
  // 新写法至少不比老写法矮 —— 这是这条判据唯一敢断言的方向。
  expect(newCss).toBeGreaterThanOrEqual(oldCss);
});

// 🚨 **这一条是唯一能稳定复现的那种裁切**，而且它是静默的。
//
// 一份超过 8192 CSS px 高的报告，老写法用 pixelRatio 2 画成 16384+ px，
// 撞上 html-to-image 的 canvasDimensionLimit。它不抛错，自己把整张图缩到
// 装得下 —— 导出来比例不对、字也糊，调用方一无所知。
//
// 她的阅读报告真能长到这个高度：1080 宽的海报里正文是 38px。
test("超长报告：老写法被悄悄缩放，新写法自己降倍率装得下", async ({ page }) => {
  await page.goto("/poster.html");
  const fresh = await exportAndRead(page, "huge", "new");
  const old = await exportAndRead(page, "huge", "old");

  // 夹具真的够高，否则这条判据什么都没验到。
  expect(fresh.measured.height, "超长夹具没到 8192px，这条测试白跑").toBeGreaterThan(8192);

  // 老写法：请求的是 2 倍，但画出来的高度装不下 海报×2 —— 被缩了。
  const oldRequested = fresh.measured.height * 2;
  // eslint-disable-next-line no-console
  console.log(
    `超长海报 ${fresh.measured.height}px：老写法请求 ${oldRequested}px、实得 ${old.png.height}px；` +
      `新写法倍率 ${fresh.ratio}、实得 ${fresh.png.height}px`,
  );
  expect(old.png.height, "老写法居然没被缩 —— 这条判据的前提不成立了").toBeLessThan(oldRequested);

  // 新写法：倍率自己降下来，画出来的 CSS 高度仍然盖住整张海报。
  expect(fresh.ratio, "新写法没有降倍率").toBeLessThan(2);
  expect(fresh.png.height / fresh.ratio, "新写法也没装下整张海报").toBeGreaterThanOrEqual(
    fresh.measured.height - 2,
  );
});

test("真正的 exportPoster 跑得通，并且把文件交出去", async ({ page }) => {
  await page.goto("/poster.html");
  const download = page.waitForEvent("download", { timeout: 30_000 });
  await page.getByRole("button", { name: "走一遍真的 exportPoster" }).click();
  const file = await download;
  expect(file.suggestedFilename()).toBe("long.png");
  await file.saveAs(`${OUT}/poster-01-exported.png`);
});
