import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { MaterialSource } from "@mind-imprint/contracts";
import { ReadingRoom, type LiteReadingRoomApi } from "@lite/readings/ReadingRoom";
import { ReadingPlanRail } from "@lite/readings/ReadingPlanRail";
import { placeBar } from "@lite/readings/BlockToolbar";
import type { ReadingTask } from "@lite/api/readingRoom";

/**
 * 手机上能看，屏幕上只有一个进度。
 *
 * ## ⚠️ What this file can and cannot prove
 *
 * jsdom applies no stylesheet, evaluates no media query and lays nothing out,
 * so it can NEVER answer 「375px 下标签栏散架了吗」. Anything asserting on a
 * measured width here would be measuring zero. The three checks below are
 * therefore split by what each is actually capable of:
 *
 *  1. **structure** — that the room renders the controls at all, and renders
 *     the override hook the stylesheet needs;
 *  2. **the stylesheet as source text** — the layout contract lives in CSS,
 *     so it is read as CSS. This is what catches a future edit deleting the
 *     wrap rule or dropping the `.mk-lite-room` scope (which would silently
 *     lose to the shared file);
 *  3. **`placeBar`, as a pure function** — real arithmetic, real assertions,
 *     no layout engine needed.
 *
 * The judgement itself was made by looking: `.deploy-local/
 * card-eyeball-2026-08-29/r3-after-375.png`.
 */

const LITE_CSS = readFileSync(resolve(process.cwd(), "src/index.css"), "utf8");

/** The stylesheet with comments stripped — a rule quoted inside a `/* … *​/`
 *  block must never be mistaken for a rule that is in force. */
const CSS = LITE_CSS.replace(/\/\*[\s\S]*?\*\//g, "");

const SOURCE: MaterialSource = {
  id: "atom-1",
  title: "屋顶上的太阳能，真的能省钱吗",
  sourceUrl: "",
  kind: "article",
  origin: "",
  blocks: [
    { id: "b1", text: "屋顶光伏这几年便宜了很多，装一套大概要两三万元。" },
    { id: "b2", text: "普通家庭一年省下的电费大概是两三千元。" },
  ],
  locked: false,
  role: "",
  tier: "",
  takeaway: "",
  anchors: [],
  timeSpentS: 0,
  lateralRead: false,
  isLateralInstrument: false,
  siftSkipped: false,
  lateralRelation: "",
  lateralJudgment: "",
};

const API = {
  async getOpenCard() {
    return null;
  },
  async summonCard() {
    return null;
  },
  async pickEvidence() {
    return null;
  },
  async confirmCard() {
    return null;
  },
  async skipCard() {
    return null;
  },
  async getTakeawayDraft() {
    throw new Error("not used");
  },
  async postFinalizeReading() {
    return null;
  },
} as unknown as LiteReadingRoomApi;

function renderRoom() {
  return render(
    <ReadingRoom
      readingId="atom-1"
      source={SOURCE}
      api={API}
      onBack={() => {}}
      tasks={[]}
      onTasks={() => {}}
      coachMessages={[]}
      blockTools={[]}
      blockNotes={[]}
      onBlockNote={() => {}}
    />,
  );
}

afterEach(cleanup);

describe("窄屏下的工具条：够得到，不裂字", () => {
  it("透镜库和完成这篇都在 DOM 里，而且没有被藏起来", () => {
    renderRoom();
    // The 375px screenshot's actual failure: both were pushed OUT of a pane
    // whose `overflow` is `hidden`, i.e. present in the DOM and unreachable
    // with a finger. A presence assertion cannot see that — which is exactly
    // why the CSS contract below exists — but it does catch the other way to
    // break this, which is to "fix" the crowding by hiding one of them.
    const lens = screen.getByRole("button", { name: /^透镜库 · \d+$/ });
    const finish = screen.getByRole("button", { name: "完成这篇" });
    for (const el of [lens, finish]) {
      expect(el.hidden).toBe(false);
      expect(el.getAttribute("aria-hidden")).toBeNull();
      expect(el.style.display).not.toBe("none");
    }
  });

  it("房间带着 lite 自己的覆盖钩子 mk-lite-room", () => {
    const { container } = renderRoom();
    const room = container.querySelector(".mk-reading-room");
    // Without this class every rule in the block below is dead: the shared
    // stylesheet under apps/web is off limits, so the ONLY way lite restyles
    // the room is a second class it can raise specificity with.
    expect(room?.classList.contains("mk-lite-room")).toBe(true);
  });

  it("index.css 里所有针对房间的规则都挂在 .mk-lite-room 上", () => {
    // A rule written as a bare `.mk-reading-room__toolbar` ties with the
    // shared file (0,1,0) and then wins or loses on bundle order — which is
    // decided by a component import, not by anything in this repo. Every
    // override must carry the extra class.
    const selectors = [...CSS.matchAll(/([^{}]+)\{/g)]
      .map((m) => m[1]!.trim())
      .filter((s) => s.includes("mk-reading-room"));
    expect(selectors.length).toBeGreaterThan(0);
    for (const selector of selectors) {
      expect(selector.includes(".mk-lite-room"), `未加 .mk-lite-room 作用域：${selector}`).toBe(true);
    }
  });

  it("标签不允许被挤扁：flex 不收缩，文字不换行", () => {
    // 阅读成果 broke into 阅/读/成/果 because the tabs were shrinkable and the
    // label was wrappable. Both of these are unconditional (no media query),
    // so a narrower phone than the one we screenshotted cannot re-break it.
    const tabs = ruleBody(".mk-lite-room .mk-reading-room__view-tabs");
    expect(tabs).toMatch(/flex\s*:\s*none/);
    const button = ruleBody(".mk-lite-room .mk-reading-room__view-tabs button");
    expect(button).toMatch(/white-space\s*:\s*nowrap/);
    expect(button).toMatch(/flex\s*:\s*none/);
    // The `0` badge landed on top of 果 because it is an `inline-grid` with a
    // `min-width` bigger than the line box it was squeezed into.
    expect(ruleBody(".mk-lite-room .mk-reading-room__count")).toMatch(/flex\s*:\s*none/);
  });

  it("窄屏下工具条换行而不是把按钮推出屏幕", () => {
    const narrow = mediaBlock("max-width: 860px");
    expect(narrow).toContain(".mk-lite-room .mk-reading-room__toolbar");
    const toolbar = ruleBody(".mk-lite-room .mk-reading-room__toolbar", narrow);
    // Wrapping is what makes reachability structural rather than a lucky fit:
    // whatever does not fit drops to the next row instead of off the edge.
    expect(toolbar).toMatch(/flex-wrap\s*:\s*wrap/);
    // …and the fixed 52px row has to give way, or the second row is clipped.
    expect(toolbar).toMatch(/height\s*:\s*auto/);
    // The hint gets a row of its own instead of being sliced mid-word.
    expect(ruleBody(".mk-lite-room .mk-reading-room__hint", narrow)).toMatch(/flex\s*:\s*1\s+0\s+100%/);
  });

  /** The declarations of the first rule with exactly this selector. */
  function ruleBody(selector: string, scope: string = CSS): string {
    // Selectors inside a `@media` block are indented, so anchor on the line
    // rather than on the newline itself.
    const at = scope.search(new RegExp(`^\\s*${escapeRe(selector)} \\{`, "m"));
    expect(at, `找不到规则 ${selector}`).toBeGreaterThan(-1);
    const open = scope.indexOf("{", at);
    const close = scope.indexOf("}", open);
    return scope.slice(open + 1, close);
  }

  function escapeRe(s: string): string {
    return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  }

  /** Everything inside `@media (<query>) { … }`, brace-matched. */
  function mediaBlock(query: string): string {
    const at = CSS.indexOf(`@media (${query})`);
    expect(at, `找不到 @media (${query})`).toBeGreaterThan(-1);
    let depth = 0;
    for (let i = CSS.indexOf("{", at); i < CSS.length; i += 1) {
      if (CSS[i] === "{") depth += 1;
      else if (CSS[i] === "}") {
        depth -= 1;
        if (depth === 0) return CSS.slice(at, i);
      }
    }
    throw new Error(`@media (${query}) 没有闭合`);
  }
});

describe("屏幕上只有一处进度数字", () => {
  const task = (id: string, label: string, status: ReadingTask["status"], detail = ""): ReadingTask => ({
    id,
    position: 0,
    kind: "read",
    label,
    detail,
    blockId: "",
    status,
    completedAt: null,
  });
  const TASKS: ReadingTask[] = [
    task("t1", "先读一遍", "done"),
    task("t2", "换一个透镜再看", "pending", "在第四段找一句"),
    task("t3", "自己说一遍", "pending"),
  ];

  it("rail 头上不再有 N / M——那是她进房间第一眼看到的分数", () => {
    const { container } = render(<ReadingPlanRail tasks={TASKS} />);
    expect(screen.getByText("带读进度")).toBeTruthy();
    // Not a text query for one literal: any `数字 / 数字` anywhere in the rail
    // is the thing that reads as a score, whatever its wording.
    expect(container.textContent ?? "").not.toMatch(/\d\s*\/\s*\d/);
    // …and not through the accessibility tree either. The bar used to be a
    // `progressbar` with aria-valuenow/aria-valuemax, i.e. 「0 of 3」 read out
    // loud — the same number, only invisible.
    expect(container.querySelector("[role='progressbar']")).toBeNull();
    expect(container.querySelector("[aria-valuenow]")).toBeNull();
  });

  it("rail 仍然是标题 + 步骤清单", () => {
    // The ruling dropped the number, not the rail.
    const { container } = render(<ReadingPlanRail tasks={TASKS} />);
    expect(container.querySelectorAll("ol > li")).toHaveLength(3);
    expect(screen.getByText("换一个透镜再看")).toBeTruthy();
  });
});

describe("段落工具条永远不压在正文上", () => {
  const H = 34; // the real pill: py-1 around a 26px 豆豆
  const VIEWPORT = 800;
  const GUTTER = 28; // lite's own paragraph margin (index.css)

  /** The two blank bands around a paragraph laid out with GUTTER margins. */
  function gutters(rect: { top: number; bottom: number }) {
    return { above: rect.top - GUTTER, below: rect.bottom + GUTTER };
  }

  it("落在段间空白的正中，而不是贴着某一段的边", () => {
    // Centred is the whole point: a 34px bar in a 28px gutter has to poke 3px
    // into each neighbour, and 3px of an 8px leading is blank paper. Hugging
    // one edge would spend all 6px on ONE side, which is where the last line
    // of the paragraph above actually is.
    const rect = { top: 400, bottom: 480 };
    const top = placeBar(rect, gutters(rect), VIEWPORT, H);
    expect(top + H / 2).toBe(400 - GUTTER / 2);
  });

  it("上方的空白出屏时翻到段落下方的空白", () => {
    const rect = { top: 8, bottom: 120 };
    const top = placeBar(rect, gutters(rect), VIEWPORT, H);
    expect(top + H / 2).toBe(120 + GUTTER / 2);
  });

  /**
   * 🚨 The regression this whole function exists for.
   *
   * `_toolbar-overlap.png`: the old code took the below-branch and then
   * clamped it into the viewport — `Math.min(rect.bottom + GAP, innerHeight -
   * BAR_H - 12)` — which, once a long paragraph's bottom edge had scrolled
   * past the fold, dragged the bar back UP into the middle of that paragraph
   * and covered the words: 「光伏板的寿命通[bar]常有二十五年。」
   */
  /**
   * 🚨 The regression this whole function exists for, at the scroll position
   * that produced `_toolbar-overlap.png`: a paragraph whose bottom is far
   * below the fold. The old formula answered `min(1400 + 10, 800 - 44 - 12)`
   * = 744 — i.e. 444px INSIDE this paragraph, on its words.
   */
  it("段落底部在屏幕外时不会被拉回正文里", () => {
    const rect = { top: 300, bottom: 1400 };
    const top = placeBar(rect, gutters(rect), VIEWPORT, H);
    expect(top).not.toBe(744);
    // Centred on the gutter above, so it intrudes at most 3px into the
    // paragraph's leading — never as far as a glyph.
    expect(top + H - rect.top).toBeLessThanOrEqual((H - GUTTER) / 2);
    expect(top).toBeGreaterThanOrEqual(12);
  });

  it("段落顶部在屏幕外时落到段落下方，而不是压在字上", () => {
    const rect = { top: -600, bottom: 300 };
    const top = placeBar(rect, gutters(rect), VIEWPORT, H);
    expect(rect.bottom - top).toBeLessThanOrEqual((H - GUTTER) / 2);
    expect(top + H).toBeLessThanOrEqual(VIEWPORT - 12);
  });

  it("空白带出屏时放弃居中，但不放弃「不遮正文」", () => {
    // The gutter above straddles the bottom edge of the screen and the
    // paragraph runs off it, so neither band can be centred on. The bar goes
    // flush against the paragraph's top edge rather than onto it.
    const rect = { top: 790, bottom: 2000 };
    const top = placeBar(rect, gutters(rect), VIEWPORT, H);
    expect(top + H).toBeLessThanOrEqual(rect.top);
    expect(top).toBeGreaterThanOrEqual(12);
    expect(top + H).toBeLessThanOrEqual(VIEWPORT - 12);
  });

  it("段落上下都出屏时才允许重叠，并且贴到空间较大的一边", () => {
    // Nothing can avoid an overlap here — the paragraph IS the viewport — so
    // the contract is only that the bar stays on screen and reachable.
    const rect = { top: -100, bottom: 900 };
    const top = placeBar(rect, gutters(rect), VIEWPORT, H);
    expect(top).toBeGreaterThanOrEqual(12);
    expect(top + H).toBeLessThanOrEqual(VIEWPORT - 12);
  });
});
