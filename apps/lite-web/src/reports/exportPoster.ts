import { toPng } from "html-to-image";

/**
 * exportPoster — rasterizes `node` to a PNG and triggers a browser download.
 *
 * `node` is expected to be a `ReportPoster`'s own root element, passed
 * directly — never a wrapper div added just to hold a ref. There is nothing
 * for an extra wrapper layer to do here (the poster's own root already
 * carries its offscreen positioning and fixed size — see ReportPoster.tsx),
 * so passing one would only ask html-to-image to walk one more, pointless
 * node.
 *
 * `cacheBust: true` appends a cache-busting query string to any image
 * requests html-to-image issues while walking the node, so a re-export
 * later in the same tab can't quietly serve a stale cached asset.
 *
 * Never throws into the caller: a failed export is a picture she doesn't
 * get, not a broken room around her. It resolves to null on success and to
 * the failure's message otherwise, so a caller that shows failures (the
 * parent report editor, `导出失败：{message}`) can; `ReportPanel` ignores it.
 *
 * # 🚨 为什么这里要自己量尺寸（2026-09-23）
 *
 * 产品负责人第 2 条：「when export reading report png, the picture seems to be
 * truncated and is not complete.」
 *
 * 原来这里一个尺寸都不传。html-to-image 于是自己去量**活的那个节点**：
 * `clientWidth` / `clientHeight`，然后把克隆塞进一个正好这么大的
 * `<foreignObject viewBox="0 0 w h">`。
 *
 * ## 量出来的那一条：**画布上限**，而且它是静默的
 *
 * 看图台上复现到了（e2e/harness/poster-harness.spec.ts 那条「超长报告」）：
 * 一份 10388 CSS px 高的报告，`pixelRatio: 2` 要的是 20776px，
 * 而 html-to-image 的 canvasDimensionLimit 是 16384 —— 它**不报错**，
 * 自己把整张图缩到装得下为止，实得 16384px。导出来比例不对、字也糊，
 * 调用方一无所知。她的阅读报告真能长到这个高度：1080 宽的海报里正文 38px。
 * `fittingPixelRatio` 改成我们自己降倍率，而不是让它替我们缩。
 *
 * ## 没有复现到的那两条：照样兜住，但不要把它们当成已知病因
 *
 * 普通长度下，`clientHeight` 和 `scrollHeight` 量出来是同一个数
 * （看图台上 3492 == 3492 == 3492），老写法画出来一个像素都没少。
 * 所以下面这两条是**加固**，不是「找到的病因」——
 * 说成病因就是把没量过的东西写成事实：
 *
 *   - `clientHeight` 是内边距盒，装不下溢出的内容，而海报根节点上写着
 *     `overflow: hidden`；它还会**取整**，2643.6px 量成 2643。
 *     所以改用 scrollHeight / rect / offsetHeight 三者取最大再向上取整，
 *     并在克隆上放开 overflow。
 *   - 图片要先 decode 完再量。字体这一头**不是**这张海报的问题：
 *     它用的是系统字体栈（FONT_STACK，测试里钉着 PingFang SC），
 *     没有 webfont 要等。`fontsReady` 留着是因为它几乎不花时间、
 *     而且以后有人往海报里加一个 webfont 时不必再想起这件事。
 *
 * ## 「不完整」的另一半，在 ReportPoster.tsx
 *
 * 屏幕上那份报告有十来节，而这张海报原来只画四样。摘抄、笔记、透镜、
 * 收获一节都不在图里 —— 见那边新加的 PosterList。
 */

/** html-to-image 的画布上限。超过它会被悄悄缩放，而不是报错。 */
const CANVAS_LIMIT = 16384;

/** 一张给家长看的图，2 倍够清楚；太高的海报会自动降到装得下为止。 */
const PREFERRED_PIXEL_RATIO = 2;

export async function exportPoster(node: HTMLElement | null, filename: string): Promise<string | null> {
  if (!node) return "图片未生成";
  try {
    // 图片和字体都要先到齐，再量尺寸 —— 量在前面就会量到回退字体的行高。
    await Promise.all(Array.from(node.querySelectorAll("img"), (image) => image.decode()));
    await fontsReady();

    const { width, height } = posterSize(node);
    const pixelRatio = fittingPixelRatio(width, height);

    // 🚨 量不出来就**不传尺寸**，退回 html-to-image 自己去量。
    //
    // `scrollHeight` / `getBoundingClientRect()` 在 jsdom 里恒等于 0，在一个
    // display:none 的节点上也是 0。这时候硬传 `width: 0` 会画出一张空图 ——
    // 比原来的毛病糟得多。传 0 不如不传。
    const sized = width > 0 && height > 0;
    const dataUrl = await toPng(node, {
      pixelRatio,
      cacheBust: true,
      ...(sized
        ? {
            width,
            height,
            // 克隆上把 overflow 放开：尺寸已经按真实内容算过了，
            // 再裁一次只会把刚刚算进来的那部分又切掉。
            style: { overflow: "visible", width: `${width}px`, height: `${height}px` },
          }
        : {}),
    });
    const a = document.createElement("a");
    a.href = dataUrl;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    return null;
  } catch (e) {
    // Exporting a picture should never break the room around it.
    if (e instanceof Error && e.message) return e.message;
    return typeof e === "string" && e ? e : "没有更多信息";
  }
}

/**
 * posterSize —— 海报真正有多大。
 *
 * 三个来源取最大：
 *   - `scrollWidth/Height`：装得下溢出的内容（`clientHeight` 装不下）；
 *   - `getBoundingClientRect()`：亚像素，向上取整，不像 clientHeight 那样
 *     直接截掉小数；
 *   - `offsetWidth/Height`：带边框。
 *
 * 导出的是一张图，宁可多一像素的白边，不可少一像素的字。
 */
export function posterSize(node: HTMLElement): { width: number; height: number } {
  const rect = typeof node.getBoundingClientRect === "function" ? node.getBoundingClientRect() : null;
  const width = Math.ceil(Math.max(node.scrollWidth || 0, node.offsetWidth || 0, rect?.width ?? 0));
  const height = Math.ceil(Math.max(node.scrollHeight || 0, node.offsetHeight || 0, rect?.height ?? 0));
  return { width, height };
}

/**
 * fittingPixelRatio —— 在画布上限之内能用的最大倍率。
 *
 * 🚨 超限时 html-to-image **不报错**，它把整张图缩放到装得下为止，
 * 于是一份很长的阅读报告导出来是糊的，而调用方什么都不知道。
 * 与其让它替我们缩，不如我们自己降倍率：2 倍装不下就 1 倍，
 * 1 倍还装不下（8192 宽以上，实际到不了）就按比例算一个。
 */
export function fittingPixelRatio(width: number, height: number): number {
  const longest = Math.max(width, height, 1);
  if (longest * PREFERRED_PIXEL_RATIO <= CANVAS_LIMIT) return PREFERRED_PIXEL_RATIO;
  const fitted = CANVAS_LIMIT / longest;
  return fitted > 1 ? Math.floor(fitted * 100) / 100 : Math.max(fitted, 0.1);
}

/**
 * fontsReady —— 等字体加载完，且**永远不会挂住导出**。
 *
 * `document.fonts` 在 jsdom 和一些旧浏览器里没有；`ready` 在极端情况下也可能
 * 一直不 resolve。导出是一件她按了按钮在等的事，所以这里有一个上限，
 * 到点就继续 —— 字体没到齐顶多是行高按回退字体算，比卡住不给图好。
 */
async function fontsReady(timeoutMs = 3000): Promise<void> {
  const fonts = (document as Document & { fonts?: { ready?: Promise<unknown> } }).fonts;
  if (!fonts?.ready) return;
  await Promise.race([fonts.ready, new Promise((resolve) => setTimeout(resolve, timeoutMs))]);
}
