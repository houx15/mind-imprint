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
