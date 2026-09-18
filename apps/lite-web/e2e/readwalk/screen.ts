import type { Page } from "@playwright/test";
import { readScreen as readBaseScreen, type Affordances } from "../camp/screen";

/**
 * 阅读室这一屏，学生看得见什么、手上能做什么。
 *
 * 在营地那份 `screen.ts` 之上多两样，因为阅读室里有两件事**不是按钮**，
 * 而它们恰恰是这个房间最要紧的两个动作：
 *
 *   划一句   带读里一半的步骤要她「在文章里点出一句」。真人是按住拖过去，
 *            模型没有手，所以这里把正文按段落编号摆给它，它说「第 3 段的
 *            这一句」，runner 去做那次真的划选。
 *   摆板     标注板/生词板的格子是 `<div data-board-bin>`，不是 button，
 *            `button:visible` 扫不到。摆不进去的话，这块板对模型就等于不存在。
 *
 * 🚨 其余一律照抄营地那条纪律：**只给它看得见的字**。不给段 id、不给接口、
 * 不告诉它下一步该是什么。它看不懂就说看不懂，那句「看不懂」就是结果。
 */

export type ReadAffordances = Affordances & {
  /** 正文段落，按屏幕上的顺序编号（1 起）。给它划句子用。 */
  paragraphs: { n: number; text: string }[];
  /** 板上的卡片（含已经摆进格子的那些，`in` 是它现在在哪一格）和所有格子。
   *  没有板的时候是 null。
   *  🚨 已经摆好的那些也要列出来：只列未摆的话，全部摆完之后 印记 让她「把某句
   *  挪到主张那一格」就没有任何东西可点了 —— 走查逐字报过这一条。 */
  board: {
    chips: { i: number; text: string; in: string }[];
    bins: { i: number; name: string }[];
  } | null;
};

export async function readScreen(page: Page): Promise<ReadAffordances> {
  const base = await readBaseScreen(page);

  const paragraphs = (
    await page
      .locator(".mk-reading-room__article-inner p[data-block-id]")
      .evaluateAll((els) => els.map((e) => (e.textContent ?? "").trim()))
      .catch(() => [] as string[])
  )
    .map((text, i) => ({ n: i + 1, text }))
    .filter((p) => p.text.length > 0);

  // 🚨 排序板（.mk-order，2026-09-18）不是「卡片 + 格子」：它是一列可以上下挪的
  // 卡片，没有格子。当成标注板来描述，模型看到的是几张卡片和零个格子，于是
  // 一遍遍地「摆」—— 线上第一次走记叙文就是这样（「上面已有条目，下面又有
  // 待摆放卡片，不知道操作对象是谁」）。那是走查瞎，不是产品坏。所以排序板
  // 单独描述（见下面 orderText），标注板这一侧只数不在排序板里的那些。
  // 🚨 交上去的板（.is-done，只读的那份回看）也不算：线上报道那一条走查里，
  // 上一块板的四张卡片被当成「还没摆的」列给模型，它于是连着二十步去挪一张
  // 挪不动的卡。
  const hasBoard = (await page.locator(".mk-board:not(.mk-order):not(.is-done)").count()) > 0;
  const board = hasBoard
    ? {
        chips: (
          await page
            .locator(".mk-board:not(.mk-order):not(.is-done) .mk-board__chip")
            .evaluateAll((els) =>
              els.map((e) => ({
                text: (e.textContent ?? "").trim(),
                in: e.closest("[data-board-bin]")?.getAttribute("data-board-bin") ?? "",
              })),
            )
            .catch(() => [] as { text: string; in: string }[])
        ).map((c, i) => ({ i, ...c })),
        bins: (
          await page
            .locator(".mk-board:not(.mk-order):not(.is-done) .mk-board__bin")
            .evaluateAll((els) => els.map((e) => e.getAttribute("data-board-bin") ?? ""))
            .catch(() => [] as string[])
        ).map((name, i) => ({ i, name })),
      }
    : null;

  // 🚨 她划出来的句子排在输入框上面，**还没发出去**。走查里她一遍遍地划、
  // 划完再划（82 次划、只换来 9 轮对话）—— 因为这一屏上没有任何东西告诉她
  // 「你已经划好了，现在该发出去」。真人看得见那几个引文小块和那颗按钮；
  // 模型只拿得到文字，所以这里明说。
  // 透镜开着的时候这一栏是锁住的，那颗「发出这 N 处」并不存在 —— 这时候劝她
  // 去点它，等于把她按在一个不存在的按钮上。
  const lensOpen = base.buttons.some((b) => b.label === "带我过去");
  const quoted = lensOpen ? undefined : (base.text.match(/已引用\s*(\d+)\s*处/) ?? [])[1];
  const text0 = quoted
    ? base.text +
      `\n\n（系统提示：你已经划好了 ${quoted} 处引文，它们还**没有**发出去。` +
      `要让印记看到，请点「发出这 ${quoted} 处」那颗按钮，或者在输入框里写一句话再发。` +
      `不要反复划新的句子。）`
    : base.text;

  // 🚨 板摆满了就该交上去。她摆完四张卡片之后停住了：「我摆完卡片了但屏幕
  // 没变化，不知道该点哪。」那颗按钮此刻刚变成可点的，但模型只拿得到文字。
  // 🚨 透镜有三段：看示范 → 选一句 → 写下发现并交上去。走查一直漏掉第三段。
  //
  // 选完之后屏幕上出现「重新选一句 / 记下这条发现」，而模型只拿得到文字，看不出
  // 「那一句已经收下了」。于是它以为自己还没选中，一遍遍地重选 —— 一条 170 步的
  // 走查里划了 123 次，只换来 3 句话。
  const lensDemo = base.buttons.some((b) => b.label === "看懂示范，开始选句");
  const lensPicked = base.buttons.some((b) => b.label === "记下这条发现");
  const allPlaced = board !== null && board.chips.length > 0 && board.chips.every((c) => c.in);
  let text = text0;
  if (allPlaced) {
    text += `\n\n（系统提示：板上的卡片都摆好了，现在请点「摆好了」那颗按钮交上去。）`;
  }
  if (lensDemo) {
    text +=
      `\n\n（系统提示：印记 正在示范这副透镜怎么用，上面那段就是示范。` +
      `读完它，再点「看懂示范，开始选句」—— 点之前在文章里选句子是不算数的。）`;
  }
  if (lensPicked) {
    text +=
      `\n\n（系统提示：你选的那一句已经收下了，不用再选。` +
      `现在请在输入框里写下你用这副透镜看出了什么，然后点「记下这条发现」。）`;
  }

  // 排序板：把现在的先后按行号说出来，并说清哪几颗按钮是挪它的。真人看得见
  // 每一行右边那对箭头挨着哪张卡；模型拿到的按钮名只有「↑」「↓」，看不出是
  // 哪一行的 —— 所以按钮名换成它们自己的 aria-label（「上移第2件」）。
  const orderRows = await page
    .locator(".mk-order:not(.is-done) .mk-order__row")
    .evaluateAll((els) => els.map((e) => (e.querySelector(".mk-order__chip")?.textContent ?? "").trim()))
    .catch(() => [] as string[]);
  let buttons = base.buttons;
  if (orderRows.length > 0) {
    const aria = await page
      .locator("button:visible")
      .evaluateAll((els) => els.map((e) => e.getAttribute("aria-label") ?? ""))
      .catch(() => [] as string[]);
    buttons = base.buttons.map((b) =>
      (b.label === "↑" || b.label === "↓") && aria[b.i] ? { ...b, label: aria[b.i]! } : b,
    );
    text +=
      `\n\n（系统提示：屏幕上有一块排序板，现在从上到下是：\n` +
      orderRows.map((t, i) => `  ${i + 1}. ${t}`).join("\n") +
      `\n用「上移第N件」「下移第N件」那几颗按钮调整先后，排好后点「排好了」。）`;
  }

  return { ...base, buttons, text, paragraphs, board };
}

/** 给模型看的那一段。 */
export function renderReadScreen(a: ReadAffordances): string {
  const bs = a.buttons.length
    ? a.buttons
        .map(
          (b) =>
            `  [${b.i}] ${b.label || "（没有文字的按钮）"}` +
            `${b.pressed ? "  ←已经打开了，别再点它" : ""}${b.disabled ? "  ←按不动" : ""}`,
        )
        .join("\n")
    : "  （一个按钮都没有）";
  const fs = a.fields.length
    ? a.fields.map((f) => `  [${f.i}] ${f.placeholder || "（没有提示文字）"}`).join("\n")
    : "  （没有能写字的地方）";

  const paras = a.paragraphs.length
    ? a.paragraphs.map((p) => `  第${p.n}段：${p.text}`).join("\n")
    : "  （屏幕上没有正文）";

  const boardBlock = a.board
    ? [
        ``,
        `屏幕上有一块板。板上的卡片：`,
        a.board.chips.length
          ? a.board.chips
              .map((c) => `  [${c.i}] ${c.text}${c.in ? `  ←现在在「${c.in}」格，可以挪` : "  ←还没摆"}`)
              .join("\n")
          : "  （板上没有卡片）",
        `能摆进去的格子：`,
        a.board.bins.map((b) => `  [${b.i}] ${b.name}`).join("\n"),
      ].join("\n")
    : "";

  return [
    `你现在这一屏上的字：`,
    `"""`,
    a.text,
    `"""`,
    ``,
    `左边这篇文章的正文（你可以从里面划出某一句）：`,
    paras,
    boardBlock,
    ``,
    `能按的按钮：`,
    bs,
    ``,
    `能写字的地方：`,
    fs,
  ].join("\n");
}

/** 两屏一不一样。用来发现「按了没反应」。 */
export function screenKey(a: ReadAffordances): string {
  return [
    a.url,
    a.text.length,
    a.text.slice(-500),
    a.buttons.map((b) => b.label).join(","),
    a.board ? `board:${a.board.chips.length}` : "no-board",
  ].join("|");
}
