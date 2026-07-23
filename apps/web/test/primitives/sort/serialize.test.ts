import { describe, it, expect } from "vitest";
import { sortStateToAnchors, anchorsToSortState } from "@/primitives/sort/serialize";
import type { Bucket } from "@/primitives/sort/serialize";
import type { SortState, Anchor } from "@mind-imprint/contracts";

const buckets: Bucket[] = [
  { id: "事实", label: "可查证的事实", hint: "能被独立核查的陈述" },
  { id: "观点", label: "需论证的观点", hint: "成立需要给出理由" },
  { id: "价值判断", label: "藏价值的判断", hint: "带「应该/好坏」，藏着价值排序" },
];

describe("sortStateToAnchors", () => {
  it("maps each item's text/bucket/reason onto quote/dimension/answer, author student", () => {
    const state: SortState = {
      items: [{ id: "i1", text: "中国是碳排放第一大国", bucket: "事实", reason: "可用官方数据核查", author: "student" }],
    };
    const anchors = sortStateToAnchors(state);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]).toMatchObject({
      quote: "中国是碳排放第一大国",
      dimension: "事实",
      answer: "可用官方数据核查",
      author: "student",
      material_id: "",
      block_id: "",
      start: 0,
      end: 0,
      question: "",
    });
  });

  it("drops items with blank text — an empty row the student never filled is not data", () => {
    const state: SortState = {
      items: [
        { id: "i1", text: "", bucket: "事实", reason: "some reason", author: "student" },
        { id: "i2", text: "   ", bucket: "观点", reason: "some reason", author: "student" },
        { id: "i3", text: "中国应该做得更多", bucket: "价值判断", reason: "藏着应该", author: "student" },
      ],
    };
    const anchors = sortStateToAnchors(state);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]!.quote).toBe("中国应该做得更多");
  });
});

describe("anchorsToSortState", () => {
  it("drops anchors whose dimension is not a declared bucket id", () => {
    const anchors: Anchor[] = [
      { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "text one", dimension: "事实", author: "student", question: "", answer: "reason one" },
      { id: "a2", material_id: "", block_id: "", start: 0, end: 0, quote: "orphaned", dimension: "不存在的维度", author: "student", question: "", answer: "" },
    ];
    const state = anchorsToSortState(anchors, buckets);
    expect(state.items).toHaveLength(1);
    expect(state.items[0]).toMatchObject({ text: "text one", bucket: "事实", reason: "reason one", author: "student" });
  });
});

describe("round-trip", () => {
  it("is exact (field-for-field) for a well-formed two-item state", () => {
    const state: SortState = {
      items: [
        { id: "i1", text: "中国是碳排放第一大国", bucket: "事实", reason: "可用官方数据核查", author: "student" },
        { id: "i2", text: "中国应该为此负更多责任", bucket: "价值判断", reason: "藏着「应该」的价值排序", author: "student" },
      ],
    };
    const roundTripped = anchorsToSortState(sortStateToAnchors(state), buckets);
    // ids are freshly minted on the way back (anchors carry no persisted item
    // id), so compare everything except id — the same convention the round
    // trip has no other honest way to satisfy.
    const strip = (s: SortState) => s.items.map(({ id: _id, ...rest }) => rest);
    expect(strip(roundTripped)).toEqual(strip(state));
  });
});
