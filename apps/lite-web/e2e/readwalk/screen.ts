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
  /** 板上还没摆的卡片，和可以摆进去的格子。没有板的时候两个都是空的。 */
  board: { chips: { i: number; text: string }[]; bins: { i: number; name: string }[] } | null;
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

  const hasBoard = (await page.locator(".mk-board").count()) > 0;
  const board = hasBoard
    ? {
        chips: (
          await page
            .locator(".mk-board__loose .mk-board__chip")
            .evaluateAll((els) => els.map((e) => (e.textContent ?? "").trim()))
            .catch(() => [] as string[])
        ).map((text, i) => ({ i, text })),
        bins: (
          await page
            .locator(".mk-board__bin")
            .evaluateAll((els) => els.map((e) => e.getAttribute("data-board-bin") ?? ""))
            .catch(() => [] as string[])
        ).map((name, i) => ({ i, name })),
      }
    : null;

  return { ...base, paragraphs, board };
}

/** 给模型看的那一段。 */
export function renderReadScreen(a: ReadAffordances): string {
  const bs = a.buttons.length
    ? a.buttons
        .map((b) => `  [${b.i}] ${b.label || "（没有文字的按钮）"}${b.disabled ? "  ←按不动" : ""}`)
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
        `屏幕上有一块板。上面还没摆的卡片：`,
        a.board.chips.length
          ? a.board.chips.map((c) => `  [${c.i}] ${c.text}`).join("\n")
          : "  （都摆好了）",
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
