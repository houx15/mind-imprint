import { describe, it, expect } from "vitest";
import { resolveReferences } from "@/workspace/blocks/referenceResolve";
import type { Annotation, Reference, ReferenceRef, Snippet } from "@mind-imprint/contracts";

function makeReference(overrides: Partial<Reference> = {}): Reference {
  return {
    id: "ref-1",
    title: "Nature Sustainability: China's renewable build-out",
    classification: "peer-reviewed",
    author: "Zhang et al.",
    credentials: "Nature Sustainability",
    year: "2024",
    url: "https://example.com/nature-sustainability",
    tags: [],
    collectionId: null,
    credibility: "strong",
    evaluation: "",
    readingNote: "关注可再生能源投资规模。",
    decision: "use",
    pending: false,
    searchHints: [],
    materialId: "mat-1",
    notes: [{ quote: "中国 2023 年新增光伏装机全球第一。", finding: "支持论点：中国在可再生能源上投入巨大。" }],
    takeaway: {
      findings: ["中国可再生能源投资连续多年全球领先。"],
      credibility: { verdict: "strong", why: "同行评审期刊，数据可追溯。" },
      keyQuotes: [{ quote: "China accounted for 60% of new renewable capacity.", why: "关键数据点。" }],
      newLeads: [],
      proposalImpact: "可以用作论证中国可持续投入的直接证据。",
    },
    readingStatus: "done",
    ...overrides,
  };
}

describe("resolveReferences", () => {
  it("resolves a material ref present in the library, flattening notes/takeaway into fragments", () => {
    const lib = [makeReference()];
    const refs: ReferenceRef[] = [{ kind: "material", id: "ref-1", label: "Nature Sustainability 文章" }];

    const resolved = resolveReferences(refs, lib, [], [])[0];
    if (!resolved || resolved.missing || resolved.kind !== "material") throw new Error("expected resolved material");

    expect(resolved.title).toBe("Nature Sustainability: China's renewable build-out");
    expect(resolved.fragments).toContain("关注可再生能源投资规模。");
    expect(resolved.fragments).toContain("可以用作论证中国可持续投入的直接证据。");
    expect(resolved.fragments).toContain("中国可再生能源投资连续多年全球领先。");
    expect(resolved.fragments.some((f: string) => f.includes("China accounted for 60%"))).toBe(true);
    expect(resolved.fragments.some((f: string) => f.includes("中国 2023 年新增光伏装机全球第一"))).toBe(true);
  });

  it("resolves a note ref via a matching snippet id", () => {
    const snippets: Snippet[] = [{ id: "snip-1", text: "碳排放全球第一是常见的反例。", position: 0, section: "反例" }];
    const refs: ReferenceRef[] = [{ kind: "note", id: "snip-1", label: "反例片段" }];

    const resolved = resolveReferences(refs, [], snippets, [])[0];
    if (!resolved || resolved.missing || resolved.kind !== "note") throw new Error("expected resolved note");

    expect(resolved.quote).toBe("反例");
    expect(resolved.finding).toBe("碳排放全球第一是常见的反例。");
  });

  it("resolves an annotation ref present in the persisted 批注 list", () => {
    const annotations: Annotation[] = [
      { id: "anno-1", criterion: "分析与论证", band: "中段", text: "反例没接回主张 → 把反例接回你的核心主张" },
    ];
    const refs: ReferenceRef[] = [{ kind: "annotation", id: "anno-1", label: "批注" }];

    const resolved = resolveReferences(refs, [], [], annotations)[0];
    if (!resolved || resolved.missing || resolved.kind !== "annotation") throw new Error("expected resolved annotation");

    expect(resolved.text).toBe("反例没接回主张 → 把反例接回你的核心主张");
    expect(resolved.criterion).toBe("分析与论证");
    expect(resolved.band).toBe("中段");
  });

  it("marks an annotation ref as missing when the id has no matching persisted annotation", () => {
    const refs: ReferenceRef[] = [{ kind: "annotation", id: "anno-1", label: "批注" }];

    const resolved = resolveReferences(refs, [], [], [])[0];

    expect(resolved?.missing).toBe(true);
    expect(resolved?.kind).toBe("annotation");
  });

  it("marks a dangling id as missing regardless of kind", () => {
    const refs: ReferenceRef[] = [
      { kind: "material", id: "does-not-exist", label: "幽灵材料" },
      { kind: "note", id: "does-not-exist-either", label: "幽灵笔记" },
    ];

    const resolved = resolveReferences(
      refs,
      [makeReference()],
      [{ id: "snip-1", text: "x", position: 0, section: null }],
      [],
    );

    expect(resolved.every((r) => r.missing === true)).toBe(true);
    expect(resolved[0]).toEqual({ kind: "material", id: "does-not-exist", label: "幽灵材料", missing: true });
    expect(resolved[1]).toEqual({ kind: "note", id: "does-not-exist-either", label: "幽灵笔记", missing: true });
  });
});
