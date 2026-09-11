import type { Page } from "@playwright/test";
import { readScreen as readBaseScreen, type Affordances } from "../camp/screen";

/**
 * 写作房间这一屏，学生看得见什么、手上能做什么。
 *
 * 在营地那份 `screen.ts` 之上多两样，因为这个房间里有两件事**不是按钮**，
 * 而它们恰恰是这里最要紧的两个动作：
 *
 *   写一段   段落那一步的主角是一个 textarea。营地那份把输入框当成「一个能
 *            打字的地方」，够用；但写作这里要区分**哪个框是正文、哪个是
 *            跟印记说话**——往对话框里写作文，和往作文框里跟印记聊天，
 *            是两种完全不同的走查结果，混在一起就什么都测不出来。
 *   摆板     标注板的格子是 `<div data-board-bin>`，不是 button，
 *            `button:visible` 扫不到。摆不进去的话，这块板对模型等于不存在。
 *
 * 🚨 其余一律照抄那条纪律：**只给它看得见的字**。不给 id、不给接口、
 * 不告诉它这个房间有几步。它看不懂就说看不懂，那句「看不懂」就是结果。
 */

export type WriteAffordances = Affordances & {
  /**
   * 每个按钮是不是「已经选中」（`aria-pressed`）。按 `buttons` 的下标对齐。
   *
   * 🚨 没有这一条，走查看不见任何「我刚才那一下起作用了」的证据。
   * 英文那个学生因此把「English」连点了 29 下：产品**是**把选中状态标出来的
   * （`WritingSetupModal` 上就写着 `aria-pressed`），真人一眼就看见按钮亮了，
   * 而这只眼睛只读了 label 和 disabled。于是她按一下、屏幕「没变」、再按一下，
   * 三十步没出过那个弹窗，记录上看起来像设定弹窗卡死了。
   */
  pressed: boolean[];
  /** 正文框（段落/成稿）。`i` 是 fields 里的下标，直接给 fill 用。 */
  proseBoxes: { i: number; placeholder: string; value: string }[];
  /** 跟印记说话的那个框。 */
  chatBox: { i: number; placeholder: string } | null;
  /** 板上还没摆的卡片，和可以摆进去的格子。没有板的时候是 null。 */
  board: { chips: { i: number; text: string }[]; bins: { i: number; name: string }[] } | null;
  /**
   * 每个按钮 / 每个输入框属于哪一段（`data-write-block` 上的标题）。
   * 不在任何段落块里的是 null。按 `buttons` / `fields` 的下标对齐。
   *
   * 🚨 段落那一步屏幕上是**一段一张卡片**：标题、引导、正文框、这一段的几颗
   * 按钮，在一起。这只眼睛原来把它们全拉平成两串序号，于是 2026-09-11 的走查
   * 里十条卡壳有五条是同一件事：「有三个『标一下这一段』按钮，不知道是不是都要
   * 点」「『中心论点』那个引导到底管哪一段」「板子写着中心论点，我第一段已经写
   * 完了，它也不消失」。产品是分了组的，是读屏的人没读到。
   * 又一次 [[camp-simulated-students-2026-09-04]] 那四个坑里的第一个。
   */
  blockOfButton: (string | null)[];
  blockOfField: (string | null)[];
};

/** 哪些输入框是「正文」。按 placeholder 认，因为屏幕上只有这个是稳定的。 */
const PROSE_PLACEHOLDERS = ["写这一段", "请先写下你最想说的那句话"];

/**
 * 哪些输入框是「跟印记说话」。
 *
 * 🚨 **两个，不是一个。** 结构那一步是全屏的 `PlanningView`，它有**自己的**
 * Composer，placeholder 是「说说你的想法」；房间里那个（段落/成稿右栏）才是
 * 「想到什么，跟印记说说」。
 *
 * 第一版只认后者，于是在结构那一屏上 `chatBox` 是 null：`say` 报「这一屏没有
 * 跟印记说话的地方」，而 `write` 只填不发。学生往里打了二十步字，一句都没发
 * 出去，印记一轮都没回 —— 记录上看起来是这个产品卡死了。
 */
const CHAT_PLACEHOLDERS = ["想到什么，跟印记说说", "说说你的想法"];

export async function readScreen(page: Page): Promise<WriteAffordances> {
  const base = await readBaseScreen(page);

  const proseBoxes = base.fields
    .filter((f) => PROSE_PLACEHOLDERS.some((p) => f.placeholder.includes(p)))
    .map((f) => ({ i: f.i, placeholder: f.placeholder, value: f.value }));

  const chat = base.fields.find((f) => CHAT_PLACEHOLDERS.some((p) => f.placeholder.includes(p)));

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

  // 和 `camp/screen.ts` 里 `button:visible` 同一个选择器、同一个顺序，
  // 所以下标对得上。
  const pressed = await page
    .locator("button:visible")
    .evaluateAll((els) => els.map((e) => e.getAttribute("aria-pressed") === "true"))
    .catch(() => [] as boolean[]);

  // 🚨 必须用**同一个选择器**去问「你属于哪一段」，否则下标会和上面那两串
  // 错开一位，而错开之后的结果看起来仍然是合理的 —— 最难查的那一种。
  const blockOf = (sel: string) =>
    page
      .locator(sel)
      .evaluateAll((els) =>
        els.map((e) => e.closest("[data-write-block]")?.getAttribute("data-write-block") ?? null),
      )
      .catch(() => [] as (string | null)[]);
  const blockOfButton = await blockOf("button:visible");
  const blockOfField = await blockOf("input:visible, textarea:visible");

  return {
    ...base,
    pressed,
    proseBoxes,
    chatBox: chat ? { i: chat.i, placeholder: chat.placeholder } : null,
    board,
    blockOfButton,
    blockOfField,
  };
}

/** 给模型看的那一段。 */
export function renderWriteScreen(a: WriteAffordances): string {
  const bs = a.buttons.length
    ? a.buttons
        .map(
          (b) =>
            `  [${b.i}] ${b.label || "（没有文字的按钮）"}` +
            `${b.disabled ? "  ←按不动" : ""}${a.pressed[b.i] ? "  ←已经选中了" : ""}`,
        )
        .join("\n")
    : "  （一个按钮都没有）";

  // 🚨 **把这一屏上每一个能打字的地方都列出来**，然后再标注哪个是正文、
  // 哪个是对话框。
  //
  // 第一版只列了 proseBoxes（按 placeholder 认「写这一段」那种），于是落地页
  // 那个「说说你想写点什么」的框、以及「带一篇写好的进来」弹窗里的题目和正文，
  // 在学生眼里**一个都不存在**。她连着十步报「没有输入框」「按钮全是灰的」，
  // 记录看起来像产品根本没法用 —— 而产品是好的，是这只眼睛瞎了一块。
  //
  // 这正是 [[camp-simulated-students-2026-09-04]] 里那四个坑的第一个：
  // **判据字段写错，会把一个走通了的产品记成走不通。**
  const proseIdx = new Set(a.proseBoxes.map((f) => f.i));
  const chatIdx = a.chatBox?.i;
  const fields = a.fields.length
    ? a.fields
        .map((f) => {
          const kind =
            f.i === chatIdx ? "  ←跟印记说话的地方" : proseIdx.has(f.i) ? "  ←写文章正文的地方" : "";
          // 🚨 **截断要说出来。**
          //
          // 第一版这里写的是 `f.value.slice(0, 120)`，不声不响地切一刀。学生
          // 于是反复看到自己刚写的那段「停在半个字上」，一连五六步都在
          // 「补全被截掉的那一段」—— 而她的字一个都没丢，是这只眼睛自己切的。
          // 又一次：判据/渲染写错，会把一个好的产品记成坏的。
          const shown = f.value.length > 400 ? f.value.slice(0, 400) : f.value;
          const more = f.value.length > 400 ? `……（这一段一共 ${f.value.length} 字，这里只显示前 400 字，后面的没丢）` : "";
          const has = f.value ? `（里面已经有：${shown}${more}）` : "（空的）";
          // 🚨 框长什么样也要说 —— 真人一眼就分得出「一行的数字框」和
          // 「一大块写文章的框」，这只眼睛原来只报 placeholder，于是她把
          // 目标字数填进了「还想说点什么」那个大框里，连着三轮走查都卡在
          // 设定弹窗上。`kind` 在 camp/screen.ts 里本来就有，是这里没拿出来。
          const shape =
            f.kind === "number" ? "［一行·只能填数字］" : f.kind === "textarea" ? "［一大块·写长文字］" : "［一行］";
          return `  [${f.i}] ${shape} ${f.placeholder || "（没有提示文字）"} ${has}${kind}`;
        })
        .join("\n")
    : "  （这一屏没有能打字的地方）";

  const boardBlock = a.board
    ? [
        ``,
        `屏幕上有一块板。上面还没摆的卡片：`,
        a.board.chips.length
          ? a.board.chips.map((c) => `  [${c.i}] ${c.text}`).join("\n")
          : "  （都摆好了 —— 现在去点那颗「标好了」把结果交上去）",
        `能摆进去的格子：`,
        a.board.bins.map((b) => `  [${b.i}] ${b.name}`).join("\n"),
      ].join("\n")
    : "";

  // 🚨 **哪几样东西是一组，要说出来。** 段落那一步屏幕上是一段一张卡片；
  // 拉平之后她看到的是「三个一模一样的『标一下这一段』按钮」，只好猜。
  const blockNames: string[] = [];
  for (const n of [...a.blockOfButton, ...a.blockOfField]) {
    if (n !== null && !blockNames.includes(n)) blockNames.push(n);
  }
  const groups = blockNames.length
    ? [
        ``,
        `这一屏上的段落块（**一块 = 一个小标题 + 它自己的框 + 它自己的那几颗按钮**，`,
        `互不相干；一颗按钮只管它所在的那一块）：`,
        ...blockNames.map((name) => {
          const fs = a.fields.filter((f) => a.blockOfField[f.i] === name).map((f) => `[${f.i}]`);
          const btns = a.buttons
            .filter((b) => a.blockOfButton[b.i] === name)
            .map((b) => `[${b.i}]${b.label}`);
          return (
            `  ◆「${name}」这一块：` +
            `写它的框 ${fs.length ? fs.join(" ") : "（无）"}；` +
            `它的按钮 ${btns.length ? btns.join(" ") : "（无）"}`
          );
        }),
      ].join("\n")
    : "";

  return [
    `你现在这一屏上的字：`,
    `"""`,
    a.text,
    `"""`,
    groups,
    boardBlock,
    ``,
    `能按的按钮：`,
    bs,
    ``,
    `能打字的地方（序号就是 write / say 里要填的那个）：`,
    fields,
  ].join("\n");
}

/** 两屏一不一样。用来发现「按了没反应」。 */
export function screenKey(a: WriteAffordances): string {
  return [
    a.url,
    a.text.length,
    a.text.slice(-500),
    a.buttons.map((b, i) => `${b.label}${a.pressed[i] ? "*" : ""}`).join(","),
    a.board ? `board:${a.board.chips.length}` : "no-board",
  ].join("|");
}
