import { describe, expect, it } from "vitest";
import { groupByCluster, noteKindMeta, type Note } from "./notes";

function note(id: string, cluster: string): Note {
  return {
    id,
    kind: "observation",
    body: id,
    author: "student",
    edited: false,
    cluster,
    x: 0,
    y: 0,
    createdAt: "2026-09-01T00:00:00Z",
  };
}

describe("groupByCluster", () => {
  // 还没归堆的那一列永远在最前面。归堆是这块板上唯一真正要动脑的操作，
  // 把待归的那一堆排到下面去，等于把要做的事藏起来。
  it("keeps the ungrouped pile first even when clusters sort before it", () => {
    const groups = groupByCluster([note("a", "阿姨说的"), note("b", ""), note("c", "打饭时间")]);
    expect(groups[0]?.cluster).toBe("");
    expect(groups[0]?.notes.map((n) => n.id)).toEqual(["b"]);
  });

  // 板上一张没归堆的也没有时，那一列仍然要在，否则拖回来就没有地方放。
  it("keeps an empty ungrouped pile as a drop target", () => {
    const groups = groupByCluster([note("a", "阿姨说的")]);
    expect(groups[0]?.cluster).toBe("");
    expect(groups[0]?.notes).toEqual([]);
  });

  it("groups the rest by name", () => {
    const groups = groupByCluster([
      note("a", "剩饭"),
      note("b", "剩饭"),
      note("c", "时间"),
    ]);
    const named = groups.slice(1);
    expect(named.map((g) => g.cluster)).toEqual(["剩饭", "时间"]);
    expect(named[0]?.notes).toHaveLength(2);
  });

  // 空白当成没归堆——「 」和「」对她是同一件事。
  it("treats whitespace-only clusters as ungrouped", () => {
    const groups = groupByCluster([note("a", "   ")]);
    expect(groups[0]?.notes.map((n) => n.id)).toEqual(["a"]);
    expect(groups).toHaveLength(1);
  });
});

describe("noteKindMeta", () => {
  it("has a plain-language label for every kind", () => {
    for (const kind of ["observation", "quote", "assumption", "question", "idea"] as const) {
      expect(noteKindMeta(kind).label).not.toBe("");
      expect(noteKindMeta(kind).kind).toBe(kind);
    }
  });
});
