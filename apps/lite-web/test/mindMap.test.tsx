import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MindMap, buildMindMap } from "@lite/writings/MindMap";
import type { WritingOutlineItem } from "@lite/api/writingRoom";

/**
 * MindMap — pinned for MULTIPLE ROOTS.
 *
 * Task 3 made openings and conclusions top-level siblings of 中心论点: three
 * rows at `depth: 0`, ordered by `position`. Every earlier version of this
 * panel had only ever been fed one, so "the root" was an assumption nothing
 * tested. If it ever collapses back to one, the student loses her opening and
 * her landing off the map — silently, since the rows are still in the outline
 * and still reach 段落.
 */

afterEach(cleanup);

const node = (id: string, text: string, role: string, depth: number, position: number): WritingOutlineItem => ({
  id,
  text,
  role,
  depth,
  position,
});

const THREE_ROOTS: WritingOutlineItem[] = [
  node("a", "夏天路上晒得受不了", "开头", 0, 0),
  node("b", "该种，但要先定谁长期养", "中心论点", 0, 1),
  node("c", "谁来养这件事得先定", "结尾", 0, 2),
];

describe("MindMap renders every depth-0 node", () => {
  it("shows all three roots", () => {
    render(<MindMap items={THREE_ROOTS} justAdded={[]} />);
    expect(screen.getByText("夏天路上晒得受不了")).toBeTruthy();
    expect(screen.getByText("该种，但要先定谁长期养")).toBeTruthy();
    expect(screen.getByText("谁来养这件事得先定")).toBeTruthy();
  });

  it("keeps them in position order, not the order they arrived in", () => {
    // Deliberately shuffled: the map must read top-to-bottom as the piece
    // reads, so a conclusion appended to the array before an opening must
    // still land below it.
    const shuffled = [THREE_ROOTS[2]!, THREE_ROOTS[0]!, THREE_ROOTS[1]!];
    render(<MindMap items={shuffled} justAdded={[]} />);
    const texts = screen.getAllByText(/夏天路上晒得受不了|该种，但要先定谁长期养|谁来养这件事得先定/).map((el) => el.textContent);
    expect(texts).toEqual(["夏天路上晒得受不了", "该种，但要先定谁长期养", "谁来养这件事得先定"]);
  });

  it("keeps each root's own children under it rather than reparenting them", () => {
    // The flattened-tree convention: a row attaches to the nearest preceding
    // row shallower than itself. A second root appearing AFTER someone else's
    // children is where a single-root assumption breaks first.
    const withChildren: WritingOutlineItem[] = [
      node("b", "该种，但要先定谁长期养", "中心论点", 0, 0),
      node("b1", "夏天太热", "理由", 1, 1),
      node("c", "谁来养这件事得先定", "结尾", 0, 2),
      node("c1", "先定人再种树", "落点", 1, 3),
    ];
    const roots = buildMindMap(withChildren);
    expect(roots.map((r) => r.item.id)).toEqual(["b", "c"]);
    expect(roots[0]!.children.map((c) => c.item.id)).toEqual(["b1"]);
    expect(roots[1]!.children.map((c) => c.item.id)).toEqual(["c1"]);

    render(<MindMap items={withChildren} justAdded={[]} />);
    expect(screen.getByText("谁来养这件事得先定")).toBeTruthy();
    expect(screen.getByText("先定人再种树")).toBeTruthy();
  });
});

describe("the spine still reads as the spine", () => {
  /** The node box is the nearest ancestor carrying an inline background. */
  const boxOf = (text: string) => screen.getByText(text).closest("[style*='background']") as HTMLElement;

  it("fills the root that carries the 分论点, and leaves the bookends quieter", () => {
    // Three identical accent boxes in a column is what a bare multi-root
    // render produces, and it loses the one sentence the piece hangs off.
    // Told apart structurally (has children), never by matching `role` text,
    // which is free-form model prose.
    const piece: WritingOutlineItem[] = [
      node("a", "夏天路上晒得受不了", "开头", 0, 0),
      node("b", "该种，但要先定谁长期养", "中心论点", 0, 1),
      node("b1", "夏天太热", "理由", 1, 2),
      node("c", "谁来养这件事得先定", "结尾", 0, 3),
    ];
    render(<MindMap items={piece} justAdded={[]} />);

    expect(boxOf("该种，但要先定谁长期养").style.background).toContain("--mk-accent-50");
    expect(boxOf("夏天路上晒得受不了").style.background).toContain("--mk-surface");
    expect(boxOf("谁来养这件事得先定").style.background).toContain("--mk-surface");
    // A bookend is still top-level: an accent hairline, not the body border.
    expect(boxOf("夏天路上晒得受不了").style.borderColor).toContain("--mk-accent-300");
  });

  /**
   * 🚨 2026-09-20 改了判据，这条用例跟着改。
   *
   * 「哪一块是主心骨」原来靠结构推：最上层、而且底下挂着东西。那个推法有一个
   * 它自己注释里承认的「诚实的失败」—— 她刚说出主张、还没挂任何理由的那一刻，
   * 屏幕上没有任何一块是重点。
   *
   * 现在节点自己带着 kind，中心论点第一眼就认得出来，那个失败没有了。
   */
  it("中心论点一落图就是重点，不必等底下挂上东西", () => {
    render(<MindMap items={[THREE_ROOTS[1]!]} justAdded={[]} />);
    expect(boxOf("该种，但要先定谁长期养").style.background).toContain("--mk-accent-50");
  });

  it("开篇和结尾不抢中心论点的重点", () => {
    render(<MindMap items={THREE_ROOTS} justAdded={[]} />);
    expect(boxOf("夏天路上晒得受不了").style.background).toContain("--mk-surface");
    expect(boxOf("谁来养这件事得先定").style.background).toContain("--mk-surface");
  });
});
