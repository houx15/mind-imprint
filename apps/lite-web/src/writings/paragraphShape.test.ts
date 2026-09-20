import { describe, expect, it } from "vitest";
import { paragraphShapeOf } from "./paragraphShape";

describe("paragraphShape", () => {
  it("正文段的四步里必须有「分析」—— 学生最常跳过的就是这一步", () => {
    const steps = paragraphShapeOf("point").map((s) => s.label);
    expect(steps).toEqual(["分论点句", "论据", "分析", "回扣"]);
  });

  it("🚨 开头那一份明说不必举例 —— 例子在后面的段里（同事的意见 9）", () => {
    const hints = paragraphShapeOf("opening").map((s) => s.hint).join("");
    expect(hints).toContain("不必举例");
  });

  it("结尾不要求引入新证据，只要求说得比开头更准", () => {
    const steps = paragraphShapeOf("closing").map((s) => s.label);
    expect(steps).toContain("说得比开头更准");
    expect(steps.join("")).not.toContain("新的例子");
  });

  it("没有对应结构的那几种返回空 —— 摆一份不适用的步骤比不摆更糟", () => {
    for (const k of ["evidence", "reference", "reasoning", "gap", "free"]) {
      expect(paragraphShapeOf(k), k).toEqual([]);
    }
  });

  it("每一步都要有一句说明，不能只有一个标签", () => {
    for (const k of ["opening", "thesis", "point", "counter", "rebuttal", "closing"]) {
      for (const s of paragraphShapeOf(k)) {
        expect(s.label.trim(), `${k} 的标签`).not.toBe("");
        expect(s.hint.trim(), `${k} / ${s.label} 的说明`).not.toBe("");
      }
    }
  });
});
