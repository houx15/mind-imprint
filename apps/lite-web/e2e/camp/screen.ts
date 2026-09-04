import type { Page } from "@playwright/test";

/**
 * screen.ts —— 把「这个学生此刻看得见什么」变成一段文字。
 *
 * 这是整个营地模拟里最要紧的一条纪律：扮演学生的那个模型**只能拿到这里返回的
 * 东西**。不给它项目 id，不给它接口，不给它 `data-testid`，不告诉它下一步该是
 * 什么。它和一个真的高中生一样，只有一屏字和几个能按的按钮。
 *
 * 所以这里读的是 `innerText` 而不是 DOM 结构：`innerText` 只返回**渲染出来、
 * 看得见**的字，`display:none` 的、`hidden` 的、被折叠起来的都不在里面。这正是
 * 我们想问的那个问题——她看得见的那些字，够不够她知道下一步做什么。
 */

export type Affordances = {
  /** 屏幕上所有看得见的字（innerText，按渲染顺序）。 */
  text: string;
  /** 能按的按钮，序号就是 `click` 动作里的 index。 */
  buttons: { i: number; label: string; disabled: boolean }[];
  /** 能写字的地方。 */
  fields: { i: number; placeholder: string; value: string; kind: string }[];
  url: string;
};

const MAX_TEXT = 7000;

export async function readScreen(page: Page): Promise<Affordances> {
  // 🚨 别在这里 waitForLoadState("networkidle")：房间里有轮询，networkidle 永远
  // 不到，等出来的是超时而不是页面。
  const raw = await page
    .locator("body")
    .innerText()
    .catch(() => "");

  // 长对话会把这一屏顶爆。截**后面**：她看的是最新的几轮，不是开场白。
  const text =
    raw.length > MAX_TEXT ? "…（上面还有，这里只留最近的）\n" + raw.slice(-MAX_TEXT) : raw;

  const btn = page.locator("button:visible");
  const buttons = (
    await btn
      .evaluateAll((els) =>
        els.map((e) => ({
          label: (e.innerText || e.getAttribute("aria-label") || "").replace(/\s+/g, " ").trim(),
          disabled: (e as HTMLButtonElement).disabled,
        })),
      )
      .catch(() => [] as { label: string; disabled: boolean }[])
  ).map((b, i) => ({ i, ...b }));

  const fld = page.locator("input:visible, textarea:visible");
  const fields = (
    await fld
      .evaluateAll((els) =>
        els.map((e) => ({
          placeholder: e.getAttribute("placeholder") ?? "",
          value: (e as HTMLInputElement).value ?? "",
          kind: e.tagName.toLowerCase() === "textarea" ? "textarea" : (e.getAttribute("type") ?? "text"),
        })),
      )
      .catch(() => [] as { placeholder: string; value: string; kind: string }[])
  ).map((f, i) => ({ i, ...f }));

  return { text, buttons, fields, url: page.url() };
}

/** 给模型看的那一段。按钮带序号，因为它要按序号来选。 */
export function renderScreen(a: Affordances): string {
  const bs = a.buttons.length
    ? a.buttons
        .map((b) => `  [${b.i}] ${b.label || "（没有文字的按钮）"}${b.disabled ? "  ←按不动" : ""}`)
        .join("\n")
    : "  （一个按钮都没有）";
  const fs = a.fields.length
    ? a.fields
        .map(
          (f) =>
            `  [${f.i}] ${f.placeholder || "（没有提示文字）"}${f.value ? `  现在写着：${f.value.slice(0, 80)}` : ""}`,
        )
        .join("\n")
    : "  （没有能写字的地方）";
  return `屏幕上的字：\n"""\n${a.text}\n"""\n\n能按的按钮：\n${bs}\n\n能写字的地方：\n${fs}`;
}

/** 两屏一不一样。用来发现「按了没反应」和「转圈转住了」。 */
export function screenKey(a: Affordances): string {
  return `${a.url}|${a.text.length}|${a.text.slice(-400)}|${a.buttons.map((b) => b.label).join(",")}`;
}
