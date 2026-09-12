import { describe, expect, it } from "vitest";
import { parseMarkdownTable } from "@/primitives/annotate/markdownTable";

/**
 * 🚨 判据要紧：松一点的判据会把正文里偶然出现的一条竖线认成表格，而认错的代价
 * 是那一段整段从正文里消失（它会被画成一张空表）。
 */
describe("parseMarkdownTable", () => {
  it("认出一张普通的表", () => {
    const got = parseMarkdownTable(
      [
        "| 年份 | 县域高中 | 地级市高中 |",
        "| --- | --- | --- |",
        "| 2013 | 21.9% | 78.1% |",
      ].join("\n"),
    );
    expect(got).not.toBeNull();
    expect(got!.header).toEqual(["年份", "县域高中", "地级市高中"]);
    expect(got!.rows).toEqual([["2013", "21.9%", "78.1%"]]);
  });

  it("对齐记号也认（:--: 这种）", () => {
    const got = parseMarkdownTable(["| a | b |", "|:---|---:|", "| 1 | 2 |"].join("\n"));
    expect(got?.rows).toEqual([["1", "2"]]);
  });

  it("普通段落不是表", () => {
    expect(parseMarkdownTable("这是一段正文，里面没有任何竖线。")).toBeNull();
    // 正文里偶然出现一条竖线 —— 不能当成表。
    expect(parseMarkdownTable("他说「A | B」这种写法很常见，但这是一段话。")).toBeNull();
  });

  it("只有表头没有数据行，不算表", () => {
    expect(parseMarkdownTable(["| a | b |", "| --- | --- |"].join("\n"))).toBeNull();
  });

  it("少了分隔行就不算表", () => {
    expect(parseMarkdownTable(["| a | b |", "| 1 | 2 |", "| 3 | 4 |"].join("\n"))).toBeNull();
  });
});
