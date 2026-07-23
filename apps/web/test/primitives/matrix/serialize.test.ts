import { describe, it, expect } from "vitest";
import { matrixStateToAnchors, anchorsToMatrixState } from "@/primitives/matrix/serialize";
import type { Col } from "@/primitives/matrix/serialize";
import type { MatrixState, Anchor } from "@mind-imprint/contracts";

const cols: Col[] = [
  { id: "position", label: "立场主张", q: "这个视角主张什么？用一句话说清。" },
  { id: "grounds", label: "依据", q: "它凭什么这么主张？它最强的依据是什么？" },
  { id: "blind_spot", label: "盲区", q: "这个视角看不见什么？它绕开了哪个事实？" },
];

describe("matrixStateToAnchors", () => {
  it("emits one anchor per cell: quote = row label, dimension = column id, answer = cell text, author student", () => {
    const state: MatrixState = {
      rows: [
        {
          id: "r1",
          label: "地方政府",
          cells: { position: "发展优先于治理", grounds: "GDP 与就业指标", blind_spot: "长期环境成本" },
          author: "student",
        },
      ],
    };
    const anchors = matrixStateToAnchors(state);
    expect(anchors).toHaveLength(3);
    expect(anchors[0]).toMatchObject({
      quote: "地方政府",
      dimension: "position",
      answer: "发展优先于治理",
      author: "student",
      material_id: "",
      block_id: "",
      start: 0,
      end: 0,
      question: "",
    });
    expect(anchors[1]).toMatchObject({ quote: "地方政府", dimension: "grounds", answer: "GDP 与就业指标" });
    expect(anchors[2]).toMatchObject({ quote: "地方政府", dimension: "blind_spot", answer: "长期环境成本" });
  });

  it("drops rows with a blank label entirely", () => {
    const state: MatrixState = {
      rows: [
        { id: "r1", label: "", cells: { position: "some claim" }, author: "student" },
        { id: "r2", label: "   ", cells: { position: "some claim" }, author: "student" },
        { id: "r3", label: "环保组织", cells: { position: "治理优先" }, author: "student" },
      ],
    };
    const anchors = matrixStateToAnchors(state);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]!.quote).toBe("环保组织");
  });

  it("emits no anchor for a blank cell", () => {
    const state: MatrixState = {
      rows: [{ id: "r1", label: "受影响居民", cells: { position: "反对搬迁", grounds: "", blind_spot: "" }, author: "student" }],
    };
    const anchors = matrixStateToAnchors(state);
    expect(anchors).toHaveLength(1);
    expect(anchors[0]).toMatchObject({ dimension: "position", answer: "反对搬迁" });
  });
});

describe("anchorsToMatrixState", () => {
  it("groups by quote in first-seen order and rehydrates an incomplete row", () => {
    const anchors: Anchor[] = [
      { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "地方政府", dimension: "position", author: "student", question: "", answer: "发展优先" },
      { id: "a2", material_id: "", block_id: "", start: 0, end: 0, quote: "环保组织", dimension: "position", author: "student", question: "", answer: "治理优先" },
      { id: "a3", material_id: "", block_id: "", start: 0, end: 0, quote: "地方政府", dimension: "grounds", author: "student", question: "", answer: "GDP 指标" },
    ];
    const state = anchorsToMatrixState(anchors, cols);
    expect(state.rows).toHaveLength(2);
    expect(state.rows[0]).toMatchObject({ label: "地方政府", cells: { position: "发展优先", grounds: "GDP 指标" } });
    expect(state.rows[1]).toMatchObject({ label: "环保组织", cells: { position: "治理优先" } });
    // the second row is INCOMPLETE (no grounds/blind_spot cell) and must
    // still survive rehydration — a half-filled matrix must survive a reload.
    expect(state.rows[1]!.cells.grounds).toBeUndefined();
  });

  it("drops anchors whose dimension is not a declared column id", () => {
    const anchors: Anchor[] = [
      { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "地方政府", dimension: "position", author: "student", question: "", answer: "发展优先" },
      { id: "a2", material_id: "", block_id: "", start: 0, end: 0, quote: "地方政府", dimension: "不存在的列", author: "student", question: "", answer: "orphaned" },
    ];
    const state = anchorsToMatrixState(anchors, cols);
    expect(state.rows).toHaveLength(1);
    expect(state.rows[0]!.cells).toEqual({ position: "发展优先" });
  });
});

describe("round-trip", () => {
  it("is exact on label/cells (modulo the regenerated id) for a well-formed two-row state", () => {
    const state: MatrixState = {
      rows: [
        {
          id: "r1",
          label: "地方政府",
          cells: { position: "发展优先于治理", grounds: "GDP 与就业指标", blind_spot: "长期环境成本" },
          author: "student",
        },
        {
          id: "r2",
          label: "环保组织",
          cells: { position: "治理应优先于短期增长", grounds: "长期健康与生态数据", blind_spot: "短期就业冲击" },
          author: "student",
        },
      ],
    };
    const roundTripped = anchorsToMatrixState(matrixStateToAnchors(state), cols);
    // ids are freshly minted on the way back (anchors carry no persisted row
    // id — the row's persisted identity is `label`), so compare label/cells
    // only, never id.
    const strip = (s: MatrixState) => s.rows.map(({ label, cells }) => ({ label, cells }));
    expect(strip(roundTripped)).toEqual(strip(state));
  });
});
