import { describe, expect, it } from "vitest";
import { indentTarget, outline, treeTodo, type TreeNode } from "./tree";

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

describe("indentTarget", () => {
  // 缩进 = 挂到上面那条同层的下面。
  it("is the previous sibling at the same depth", () => {
    const ordered = [node("a", null, 0, 0), node("b", null, 0, 1)];
    expect(indentTarget(ordered, "b")?.id).toBe("a");
  });

  // 第一条没有可以缩进去的地方。
  it("is null for the first node", () => {
    const ordered = [node("a", null, 0, 0)];
    expect(indentTarget(ordered, "a")).toBeNull();
  });

  // 上一条比自己浅（是自己的父节点），缩进就没有意义了。
  it("is null when the previous node is the parent", () => {
    const ordered = [node("a", null, 0, 0), node("a1", "a", 1, 0)];
    expect(indentTarget(ordered, "a1")).toBeNull();
  });

  // 跨过更深的子孙，找到上一条真正的同层。
  it("skips over deeper descendants to find the real previous sibling", () => {
    const ordered = [
      node("a", null, 0, 0),
      node("a1", "a", 1, 0),
      node("a1x", "a1", 2, 0),
      node("b", null, 0, 1),
    ];
    expect(indentTarget(ordered, "b")?.id).toBe("a");
  });
});

describe("treeTodo", () => {
  const three = [node("a", null, 0, 0), node("b", null, 0, 1), node("c", null, 0, 2)];

  it("asks for a few blocks first", () => {
    expect(treeTodo({ tree: "main", nodes: [node("a", null, 0, 0)], checks: [] })).toBe(
      "至少先分出三块",
    );
  });

  it("counts the questions still unanswered", () => {
    expect(
      treeTodo({ tree: "main", nodes: three, checks: [{ question: "covers", answer: "都在" }] }),
    ).toBe("还有 2 个问题没想");
  });

  // 空白不算答过。
  it("does not count a whitespace answer", () => {
    expect(
      treeTodo({ tree: "main", nodes: three, checks: [{ question: "covers", answer: "  " }] }),
    ).toBe("还有 3 个问题没想");
  });

  it("is empty once all three are answered", () => {
    expect(
      treeTodo({
        tree: "main",
        nodes: three,
        checks: [
          { question: "covers", answer: "都在" },
          { question: "coherent", answer: "顺" },
          { question: "better", answer: "想过了" },
        ],
      }),
    ).toBe("");
  });
});
