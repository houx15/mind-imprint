import { describe, expect, it } from "vitest";
import { moveOutlineNode, OUTLINE_MAX_DEPTH } from "./outlineMove";
import { buildMindMap } from "./MindMap";
import type { WritingOutlineItem } from "../api/writingRoom";

/** 一份扁平清单，写法照着屏幕上读到的样子：缩进 = 深度。 */
function outline(...rows: [string, number][]): WritingOutlineItem[] {
  return rows.map(([text, depth], i) => ({
    id: text,
    text,
    role: "",
    depth,
    position: i,
  }));
}

/** 把结果还原成「缩进 + 文字」，断言读起来就是图本身。 */
function shape(items: WritingOutlineItem[] | null): string[] {
  if (!items) return ["<null>"];
  return items
    .slice()
    .sort((a, b) => a.position - b.position)
    .map((r) => `${"  ".repeat(r.depth)}${r.text}`);
}

// 产品负责人那张截图上的图，照着摆：两个最上层的块、两条理由、一条她的经历。
const SCREENSHOT = outline(
  ["中心论点", 0],
  ["理由A", 1],
  ["她的经历", 2],
  ["理由B", 1],
  ["另一个中心论点", 0],
);

describe("moveOutlineNode", () => {
  it("把一条理由挪到另一个论点底下，它带着自己的材料一起走", () => {
    const got = moveOutlineNode(SCREENSHOT, "理由A", "另一个中心论点", "child");
    expect(shape(got)).toEqual([
      "中心论点",
      "  理由B",
      "另一个中心论点",
      "  理由A",
      "    她的经历",
    ]);
  });

  it("挪成兄弟时要跳过目标自己的子树，否则会变成它的孩子", () => {
    // 理由B 挪到 理由A 后面。理由A 底下挂着「她的经历」，落点必须在它之后。
    const got = moveOutlineNode(SCREENSHOT, "理由B", "理由A", "after");
    expect(shape(got)).toEqual([
      "中心论点",
      "  理由A",
      "    她的经历",
      "  理由B",
      "另一个中心论点",
    ]);
  });

  it("整段的深度是平移的，不是压平的", () => {
    // 把「中心论点」整棵挪到另一个论点底下：它自己 0→1，理由 1→2，
    // 而「她的经历」会到 3，超上限 ⇒ 整个动作拒绝。
    expect(moveOutlineNode(SCREENSHOT, "中心论点", "另一个中心论点", "child")).toBeNull();
  });

  it("不许拖进自己底下", () => {
    expect(moveOutlineNode(SCREENSHOT, "中心论点", "她的经历", "child")).toBeNull();
    expect(moveOutlineNode(SCREENSHOT, "理由A", "她的经历", "after")).toBeNull();
  });

  it("不许拖到自己身上", () => {
    expect(moveOutlineNode(SCREENSHOT, "理由A", "理由A", "child")).toBeNull();
  });

  it("认不出来的 id 就什么都不做", () => {
    expect(moveOutlineNode(SCREENSHOT, "不存在", "理由A", "child")).toBeNull();
    expect(moveOutlineNode(SCREENSHOT, "理由A", "不存在", "child")).toBeNull();
  });

  it("超过深度上限的动作一律拒绝，不悄悄压平", () => {
    const got = moveOutlineNode(SCREENSHOT, "理由A", "她的经历", "child");
    expect(got).toBeNull();
    // 上限确实是服务端那个数。
    expect(OUTLINE_MAX_DEPTH).toBe(2);
  });

  it("position 重排成连续的 0..n-1 —— 服务端和树都按它读", () => {
    const got = moveOutlineNode(SCREENSHOT, "理由A", "另一个中心论点", "child");
    expect(got!.map((r) => r.position)).toEqual([0, 1, 2, 3, 4]);
  });

  it("一个节点都不会掉", () => {
    const got = moveOutlineNode(SCREENSHOT, "理由A", "另一个中心论点", "child");
    expect(got!.map((r) => r.text).sort()).toEqual(SCREENSHOT.map((r) => r.text).sort());
  });

  /**
   * 🚨 这一条是真正要防的那个：算出来的清单要能被 buildMindMap 读成她期望的树。
   * 上面那些断言比的是扁平清单，而学生看的是树 —— 两者由「文档顺序 + 深度」
   * 这一条规则连起来，写错了深度而顺序恰好没变，扁平断言可能还是绿的。
   */
  it("挪完之后 buildMindMap 拼出来的树就是她看到的那棵", () => {
    const got = moveOutlineNode(SCREENSHOT, "理由A", "另一个中心论点", "child")!;
    const roots = buildMindMap(got);
    expect(roots.map((n) => n.item.text)).toEqual(["中心论点", "另一个中心论点"]);
    expect(roots[0]?.children.map((n) => n.item.text)).toEqual(["理由B"]);
    expect(roots[1]?.children.map((n) => n.item.text)).toEqual(["理由A"]);
    expect(roots[1]?.children[0]?.children.map((n) => n.item.text)).toEqual(["她的经历"]);
  });
});
