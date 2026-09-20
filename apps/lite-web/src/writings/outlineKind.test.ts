import { describe, expect, it } from "vitest";
import {
  OUTLINE_KIND_DEPTH,
  outlineKindFromRole,
  outlineKindIsMaterial,
  outlineKindLabel,
  outlineKindOf,
  rekindForDepth,
} from "./outlineKind";

/**
 * 🚨 这些用例是 `apps/api/internal/api/writing_kind_internal_test.go` 的
 * 对照组。两边测的是同一张表，改一边就要改另一边。
 */
describe("outlineKind", () => {
  it("每个 kind 的深度和 Go 侧一致", () => {
    expect(OUTLINE_KIND_DEPTH).toEqual({
      opening: 0,
      thesis: 0,
      closing: 0,
      point: 1,
      counter: 1,
      evidence: 2,
      reference: 2,
      reasoning: 2,
      rebuttal: 2,
      gap: 2,
      // 记叙文那四种（R4）：场景和转折是主干，细节挂在场景底下，
      // 感悟收在主干那一层。
      scene: 1,
      turn: 1,
      feeling: 1,
      detail: 2,
    });
  });

  it("记叙文那四种块也有标题 —— 没有的话她会看见一张没名字的卡", () => {
    expect(outlineKindLabel("scene")).toBe("场景");
    expect(outlineKindLabel("detail")).toBe("细节");
    expect(outlineKindLabel("turn")).toBe("转折");
    expect(outlineKindLabel("feeling")).toBe("感悟");
  });

  // 🚨 她把一块拖到主干那一层之后，记叙文里该变成「场景」，不是「分论点」。
  // 落错了她会看见一张写着「分论点」的卡 —— R1 修掉的那个毛病换了个文体
  // 又长出来一次。
  it("拖动之后的默认种类按文体分岔", () => {
    expect(rekindForDepth("detail", 1, true, "narrative")).toBe("scene");
    expect(rekindForDepth("scene", 2, true, "narrative")).toBe("detail");
    // 不传文体时照旧是议论文 —— 现有的调用点一个都不用改。
    expect(rekindForDepth("evidence", 1, true)).toBe("point");
  });

  it("标题用的是语文课上的正式词", () => {
    expect(outlineKindLabel("evidence")).toBe("论据 · 你见过的事");
    expect(outlineKindLabel("reference")).toBe("论据 · 你找来的材料");
    expect(outlineKindLabel("closing")).toBe("结尾");
    expect(outlineKindLabel("gap")).toBe("待补的材料");
    expect(outlineKindLabel("reasoning")).toBe("道理");
  });

  it("老行按 role 和深度兜底，一行都不丢", () => {
    expect(outlineKindOf({ role: "结尾", depth: 1 })).toBe("closing");
    expect(outlineKindOf({ role: "一条理由", depth: 1 })).toBe("point");
    expect(outlineKindOf({ role: "你经历过的事", depth: 2 })).toBe("evidence");
    expect(outlineKindOf({ role: "一份研究", depth: 2 })).toBe("reference");
    // 深度 2 上的「一条理由」是撑着分论点的道理，不是一条分论点。
    expect(outlineKindOf({ role: "一条理由", depth: 2 })).toBe("reasoning");
    // 🚨 role 说不出它是什么：算「她找来的」。少提醒一次比冤枉她强。
    expect(outlineKindFromRole("", 2)).toBe("reference");
  });

  it("存下来的 kind 胜过 role", () => {
    expect(outlineKindOf({ kind: "point", role: "结尾", depth: 0 })).toBe("point");
  });

  it("待补的材料不是材料 —— 它是一个洞", () => {
    expect(outlineKindIsMaterial("gap")).toBe(false);
    expect(outlineKindIsMaterial("reasoning")).toBe(false);
    expect(outlineKindIsMaterial("evidence")).toBe(true);
    expect(outlineKindIsMaterial("reference")).toBe(true);
  });

  describe("rekindForDepth —— 她拖动之后", () => {
    it("深度对得上就不动", () => {
      expect(rekindForDepth("evidence", 2, true)).toBe("evidence");
      expect(rekindForDepth("reference", 2, true)).toBe("reference");
    });

    it("一条论据被拖到深度 1，它就成了一条分论点", () => {
      expect(rekindForDepth("evidence", 1, true)).toBe("point");
    });

    it("图上已经有中心论点了，再拖一个到最上层就是结尾", () => {
      expect(rekindForDepth("point", 0, true)).toBe("closing");
      expect(rekindForDepth("point", 0, false)).toBe("thesis");
    });
  });
});
