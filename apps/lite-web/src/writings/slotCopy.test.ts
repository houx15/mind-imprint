import { describe, expect, it } from "vitest";
import { slotJob, slotHeadingLabel, slotClaimLabel } from "./slotCopy";
import { slotTitle, type Slot } from "./slots";
import { outlineKindLabel } from "./outlineKind";

/**
 * 产品负责人 2026-09-23 逐字报的断点：
 *
 *   「在「结构」这一步，它们标的是 场景、转折、感悟。在「行文」这一步，
 *     还是 场景、转折、感悟。到了「段落」这一步，全部变成
 *     分论点 1、分论点 2、分论点 3。开头那张卡的引导语也变成
 *     「提出这篇要证明的中心论点……」」
 *
 * 🚨 钉的是**三步印的是同一个词**这条不变量，不是某一句具体的文案 ——
 * 文案会改，而「段落这一步和图上说的不是一回事」是这个 bug 本身。
 */

function slot(over: Partial<Slot> = {}): Slot {
  return {
    number: 1,
    kind: "point",
    position: 0,
    outlineId: "o1",
    heading: "那天下午的雨",
    role: "",
    snippet: null,
    materials: [],
    claim: "",
    needsPoint: false,
    outlineKind: "scene",
    ...over,
  };
}

const ARGUMENT_WORDS = ["分论点", "中心论点", "论据", "论证"];

describe("段落这一步的卡片名字，和结构图印的是同一个词", () => {
  const NARRATIVE_KINDS = ["scene", "turn", "feeling", "detail"];

  it.each(NARRATIVE_KINDS)("记叙文的 %s 卡不印「分论点」", (kind) => {
    const s = slot({ outlineKind: kind });
    const title = slotTitle(s, 1, "zh", "narrative");
    expect(title).toContain(outlineKindLabel(kind, "zh"));
    for (const w of ARGUMENT_WORDS) {
      expect(title).not.toContain(w);
    }
  });

  it("议论文照旧印「分论点」—— 修好记叙文不该把议论文弄坏", () => {
    expect(slotTitle(slot({ outlineKind: "point" }), 2, "zh", "argument")).toBe("分论点 2");
    expect(slotTitle(slot({ outlineKind: "counter" }), 1, "zh", "argument")).toContain("反方观点");
  });

  it("🚨 英文那边不再退回中文术语", () => {
    expect(slotTitle(slot({ outlineKind: "point" }), 1, "en", "argument")).toBe("Topic sentence 1");
    expect(slotTitle(slot({ outlineKind: "scene" }), 1, "en", "narrative")).toBe("Scene 1");
    expect(slotTitle(slot({ kind: "opening" }), 0, "en", "argument")).toBe("Introduction");
    expect(slotTitle(slot({ kind: "closing" }), 0, "en", "argument")).toBe("Conclusion");
  });

  it("小标题走同一个函数，所以和标题、和图上必然一致", () => {
    for (const kind of [...NARRATIVE_KINDS, "point", "counter"]) {
      const s = slot({ outlineKind: kind });
      expect(slotHeadingLabel(s, "zh", "argument")).toBe(outlineKindLabel(kind, "zh"));
      expect(slotHeadingLabel(s, "en", "argument")).toBe(outlineKindLabel(kind, "en"));
    }
    // 🚨 主体卡拿到一个不是段落层的 kind（老数据里第二个深度 0 节点算
    // thesis，一条写过字的论据也会自己成一张卡）时，退回这一篇文体的默认
    // 段落名 —— 那张卡的活是「先写出这一段要说的那句话」，印成「中心论点」
    // 或者「论据 · 你见过的事」都不是这一段的名字。
    expect(slotHeadingLabel(slot({ outlineKind: "" }), "zh", "argument")).toBe("分论点");
    expect(slotHeadingLabel(slot({ outlineKind: "thesis" }), "zh", "argument")).toBe("分论点");
    expect(slotHeadingLabel(slot({ outlineKind: "closing" }), "zh", "argument")).toBe("分论点");
    // 🚨 别的 kind 按自己的名字印 —— 方向和 2026-09-20 那次修的一样
    //（那次是「结尾」被印成「分论点 3」）。
    expect(slotHeadingLabel(slot({ outlineKind: "evidence" }), "zh", "argument")).toBe("论据 · 你见过的事");
    expect(slotHeadingLabel(slot({ outlineKind: "thesis" }), "zh", "narrative")).toBe("场景");
  });

  it("中心论点那个小标题也按语言走", () => {
    expect(slotClaimLabel("zh")).toBe("中心论点");
    expect(slotClaimLabel("en")).toBe("Thesis statement");
  });
});

describe("这一张卡要做的事", () => {
  it("🚨 记叙文的开头不再是「提出这篇要证明的中心论点」", () => {
    const job = slotJob(slot({ kind: "opening" }), "zh", "narrative");
    for (const w of ARGUMENT_WORDS) {
      expect(job).not.toContain(w);
    }
    expect(job).toContain("在哪儿");
  });

  it("议论文的开头照旧提中心论点", () => {
    expect(slotJob(slot({ kind: "opening" }), "zh", "argument")).toContain("中心论点");
  });

  it("记叙文的结尾落在具体画面上，不落在中心论点上", () => {
    const job = slotJob(slot({ kind: "closing" }), "zh", "narrative");
    expect(job).not.toContain("中心论点");
    expect(job).toContain("画面");
  });

  it.each(["scene", "detail", "turn", "feeling"])("记叙文的 %s 卡有自己的一句活", (kind) => {
    const jobs = new Set(
      ["scene", "detail", "turn", "feeling"].map((k) => slotJob(slot({ outlineKind: k }), "zh", "narrative")),
    );
    // 四种各说各的，不是同一句话抄四遍。
    expect(jobs.size).toBe(4);
    const job = slotJob(slot({ outlineKind: kind }), "zh", "narrative");
    for (const w of ARGUMENT_WORDS) {
      expect(job).not.toContain(w);
    }
  });

  it("🚨 英文那边整句都是英文，不混中文", () => {
    const cases: Array<[Partial<Slot>, string]> = [
      [{ kind: "opening" }, "narrative"],
      [{ kind: "closing" }, "narrative"],
      [{ kind: "opening" }, "argument"],
      [{ outlineKind: "scene" }, "narrative"],
      [{ outlineKind: "turn" }, "narrative"],
      [{ outlineKind: "point" }, "argument"],
      [{ outlineKind: "counter" }, "argument"],
      [{ outlineKind: "", needsPoint: true }, "argument"],
      [{ kind: "free" }, "argument"],
    ];
    for (const [over, genre] of cases) {
      const job = slotJob(slot(over), "en", genre);
      expect(job, `${JSON.stringify(over)} / ${genre}`).not.toMatch(/[一-鿿]/);
    }
  });

  it("每一格都有话说 —— 没有一张卡拿到空字符串", () => {
    for (const lang of ["zh", "en"]) {
      for (const genre of ["argument", "narrative"]) {
        for (const kind of ["opening", "closing", "free", "point"] as const) {
          expect(slotJob(slot({ kind }), lang, genre).trim()).not.toBe("");
        }
        for (const ok of ["scene", "detail", "turn", "feeling", "point", "counter", "rebuttal", ""]) {
          expect(slotJob(slot({ outlineKind: ok }), lang, genre).trim()).not.toBe("");
        }
      }
    }
  });
});
