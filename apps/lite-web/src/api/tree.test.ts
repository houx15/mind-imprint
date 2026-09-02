import { describe, expect, it } from "vitest";
import { outline, treeTodo, type TreeNode } from "./tree";

function node(id: string, parentId: string | null, depth: number, ordinal: number): TreeNode {
  return {
    id,
    tree: "main",
    parentId,
    depth,
    ordinal,
    title: id,
    body: "",
    author: "student",
    edited: false,
    x: 0,
    y: 0,
  };
}

describe("outline", () => {
  // 服务端按 depth 一层一层返回；屏幕上要的是"这一条下面紧跟它自己的小块"。
  // 两者不是一个顺序，转换只能在一个地方做完。
  it("puts each child directly under its own parent", () => {
    const flat = [
      node("a", null, 0, 0),
      node("b", null, 0, 1),
      node("a1", "a", 1, 0),
      node("b1", "b", 1, 0),
    ];
    expect(outline(flat).map((n) => n.id)).toEqual(["a", "a1", "b", "b1"]);
  });

  it("orders siblings by ordinal, not by arrival", () => {
    const flat = [node("second", null, 0, 1), node("first", null, 0, 0)];
    expect(outline(flat).map((n) => n.id)).toEqual(["first", "second"]);
  });

  it("handles three levels", () => {
    const flat = [
      node("a", null, 0, 0),
      node("a1", "a", 1, 0),
      node("a1x", "a1", 2, 0),
      node("a2", "a", 1, 1),
    ];
    expect(outline(flat).map((n) => n.id)).toEqual(["a", "a1", "a1x", "a2"]);
  });

  // 🚨 数据坏了（父节点指向自己）也不该让页面转不出来。
  it("does not hang on a node that points at itself", () => {
    const flat = [node("a", null, 0, 0), node("loop", "loop", 1, 0)];
    expect(() => outline(flat)).not.toThrow();
    expect(outline(flat).map((n) => n.id)).toEqual(["a"]);
  });

  it("is empty for an empty tree", () => {
    expect(outline([])).toEqual([]);
  });
});


describe("treeTodo", () => {
  it("says when there is nothing to review yet", () => {
    expect(treeTodo({ tree: "main", nodes: [], checks: [] })).toBe("暂时没有需要审查的结构");
  });

  // 🚨 三个问题是思考框架，不是必答题。做成门槛，一次审视就变成一份问卷
  // （产品负责人 2026-09-02）。
  it("never blocks her on the three questions", () => {
    expect(
      treeTodo({ tree: "main", nodes: [node("a", null, 0, 0)], checks: [] }),
    ).toBe("");
  });
});
