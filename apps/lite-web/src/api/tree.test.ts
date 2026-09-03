import { describe, expect, it } from "vitest";
import { autoLayout, outline, treeTodo, type TreeNode } from "./tree";

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

// 自动排版按**量出来的**高度累加。
//
// 🚨 这个函数坏掉的样子是「看着都在，只是后一块把前一块压住了半句」——线上那棵
// 15 个节点的树就是这样：固定行距 74px，而块高是 minHeight:56，说明一换行就撑到
// 一百多。一件让她看结构有没有漏的工具，自己先把内容遮住了。
describe("autoLayout", () => {
  it("同一列里，下一块让开上一块的真实高度", () => {
    const nodes = [node("a", null, 0, 0), node("b", null, 0, 1), node("c", null, 0, 2)];
    const heights = new Map([["a", 120]]);
    const at = autoLayout(nodes, heights);
    const a = at.get("a")!;
    const b = at.get("b")!;
    const c = at.get("c")!;
    // a 有 120 高，b 必须落在它下边缘之外。
    expect(b.y).toBeGreaterThanOrEqual(a.y + 120);
    // b 是默认高度（56），c 只需让开 56，不该也让开 120。
    expect(c.y - b.y).toBeLessThan(120);
    expect(c.y).toBeGreaterThanOrEqual(b.y + 56);
  });

  it("量不到高度时按最矮的排，行距和以前一样", () => {
    const nodes = [node("a", null, 0, 0), node("b", null, 0, 1)];
    const at = autoLayout(nodes);
    expect(at.get("b")!.y - at.get("a")!.y).toBe(74);
  });

  it("她自己摆过的位置不动", () => {
    const moved = { ...node("a", null, 0, 0), x: 300, y: 400 };
    const at = autoLayout([moved], new Map([["a", 200]]));
    expect(at.get("a")).toEqual({ x: 300, y: 400 });
  });

  it("不同层各排各的列", () => {
    const nodes = [node("a", null, 0, 0), node("a1", "a", 1, 0)];
    const at = autoLayout(nodes);
    expect(at.get("a1")!.x).toBeGreaterThan(at.get("a")!.x);
    // 子节点在自己那一列里是第一块，不该被父节点的行号推下去。
    expect(at.get("a1")!.y).toBe(at.get("a")!.y);
  });
});
