import { describe, expect, it } from "vitest";
import { paragraphShapeOf } from "./paragraphShape";

describe("paragraphShape", () => {
  // 讲义（五）的主体段五句型。R4 之前这里是四步，少的那一句是**阐释句**。
  // 🚨 Go 侧 writing_sentence.go 有逐字一致的一份，
  // TestParagraphShapeMatchesFrontend 直接读这个文件比对。
  it("正文段是讲义的五句型，阐释句在观点句和材料句之间", () => {
    const steps = paragraphShapeOf("point").map((s) => s.label);
    expect(steps).toEqual(["观点句", "阐释句", "材料句", "分析句", "结论句"]);
  });

  it("分析句那一格点得出三种写法，不是只说「要分析」", () => {
    const hint = paragraphShapeOf("point").find((s) => s.label === "分析句")?.hint ?? "";
    expect(hint).toContain("分析原因");
    expect(hint).toContain("假设条件");
    expect(hint).toContain("共同点");
  });

  // 记叙文那四块（R4）。在这之前一篇记叙文进了这间屋子会被硬塞进
  // 中心论点／分论点／论据 —— 一副用不上的骨架。
  it("记叙文那四种块各有自己的段内结构", () => {
    for (const k of ["scene", "detail", "turn", "feeling"]) {
      expect(paragraphShapeOf(k).length, k).toBeGreaterThan(0);
    }
    // 🚨 两种文体的说法不许串台：记叙文那几块里不该出现「论点」。
    for (const k of ["scene", "detail", "turn", "feeling"]) {
      const all = paragraphShapeOf(k).map((s) => s.label + s.hint).join("");
      expect(all, k).not.toContain("论点");
    }
  });

  it("🚨 开头那一份明说不必举例 —— 例子在后面的段里（同事的意见 9）", () => {
    const hints = paragraphShapeOf("opening").map((s) => s.hint).join("");
    expect(hints).toContain("具体例子留在主体段展开");
  });

  it("结尾不要求引入新证据，只要求说得比开头更准", () => {
    const steps = paragraphShapeOf("closing").map((s) => s.label);
    expect(steps).toContain("总结论证结果");
    expect(steps.join("")).not.toContain("新的例子");
  });

  it("没有对应结构的那几种返回空 —— 摆一份不适用的步骤比不摆更糟", () => {
    for (const k of ["evidence", "reference", "reasoning", "gap", "free"]) {
      expect(paragraphShapeOf(k), k).toEqual([]);
    }
  });

  it("每一步都要有一句说明，不能只有一个标签", () => {
    for (const k of ["opening", "thesis", "point", "counter", "rebuttal", "closing",
                     "scene", "detail", "turn", "feeling"]) {
      for (const s of paragraphShapeOf(k)) {
        expect(s.label.trim(), `${k} 的标签`).not.toBe("");
        expect(s.hint.trim(), `${k} / ${s.label} 的说明`).not.toBe("");
      }
    }
  });
});
