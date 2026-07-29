// @vitest-environment node
// The export fns need no DOM (saveBlob is guarded away when document is
// absent); node's Blob implements arrayBuffer(), which jsdom's does not.
import { describe, it, expect } from "vitest";
import type { LogEntry, PlanItem, Proposal, Reference } from "@mind-imprint/contracts";
import {
  exportAnnotatedBib,
  exportTimescale,
  exportProposalDocx,
  exportActivityLog,
  exportDraftDocx,
} from "../../src/workspace/export";

const XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
const DOCX_MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document";

// Both .xlsx and .docx are zip containers — every valid file starts with the
// ZIP local-file-header magic "PK" (0x50 0x4B).
async function startsWithPK(blob: Blob): Promise<boolean> {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  return bytes[0] === 0x50 && bytes[1] === 0x4b;
}

const refs: Reference[] = [
  {
    id: "r1",
    title: "Chen et al. (2019), Nature Sustainability",
    classification: "期刊论文",
    author: "Chen, C. et al.",
    credentials: "同行评议期刊 Nature Sustainability",
    year: "2019",
    url: "https://doi.org/10.1038/s41893-019-0220-7",
    tags: ["正方", "一手源"],
    collectionId: "c-for",
    credibility: "strong",
    evaluation: "能回答：卫星确证地球在变绿。",
    decision: "use",
    pending: false,
    searchHints: [],
    materialId: "m1",
    notes: [{ quote: "增量主要来自农业集约化与大规模植树", finding: "变绿≠生态系统整体改善。" }],
  },
  {
    id: "r2",
    title: "待补充来源",
    classification: "",
    author: "",
    credentials: "",
    year: "",
    url: "",
    tags: [],
    collectionId: null,
    credibility: null,
    evaluation: "",
    decision: null,
    pending: true, // skipped by the export
    searchHints: ["搜 China CO2 emissions"],
    materialId: null,
    notes: [],
  },
];

const plan: PlanItem[] = [
  { id: "p1", title: "读：NASA 卫星植被数据", tag: "read", column: "done", stage: "阶段一 · 研究与写作", refMaterialId: "m1", start: 0, days: 2, position: 0 },
  { id: "p2", title: "写：确定论点与结构", tag: "write", column: "doing", stage: "阶段一 · 研究与写作", refMaterialId: null, start: 4, days: 2, position: 1 },
  { id: "p3", title: "省：模拟答辩与复盘", tag: "review", column: "todo", stage: "阶段二 · 展示与答辩", refMaterialId: null, start: 16, days: 2, position: 2 },
];

const proposal: Proposal = {
  objective: "回答：经济增长与环境可持续之间的张力，该用什么尺度来判断？",
  reason: "新闻里中国既大规模植树造林，又是全球碳排放第一。",
  activities: "溯源关键数据 → 读正反两方文献 → 搭论证。",
  resources: "",
};

const log: LogEntry[] = [
  { id: "l1", date: "07-02", text: "确定题目方向。", source: "me" },
  { id: "l2", date: "07-03", text: "在阅读室溯源发现「NASA 数据」是转述。", source: "auto" },
];

const project = { title: "中国的发展让地球更可持续了吗？" };

describe("workspace exports", () => {
  it("exportAnnotatedBib → non-empty .xlsx (zip)", async () => {
    const blob = await exportAnnotatedBib(refs, project);
    expect(blob).toBeInstanceOf(Blob);
    expect(blob.size).toBeGreaterThan(0);
    expect(blob.type).toBe(XLSX_MIME);
    expect(await startsWithPK(blob)).toBe(true);
  });

  it("exportTimescale → non-empty .xlsx (zip)", async () => {
    const blob = await exportTimescale(plan, project, 18);
    expect(blob.size).toBeGreaterThan(0);
    expect(blob.type).toBe(XLSX_MIME);
    expect(await startsWithPK(blob)).toBe(true);
  });

  it("exportProposalDocx → non-empty .docx (zip)", async () => {
    const blob = await exportProposalDocx(proposal, { ...project, qualification: "TOK · 拓展论文风格" });
    expect(blob.size).toBeGreaterThan(0);
    expect(blob.type).toBe(DOCX_MIME);
    expect(await startsWithPK(blob)).toBe(true);
  });

  it("exportActivityLog → non-empty .docx (zip)", async () => {
    const blob = await exportActivityLog(log, project);
    expect(blob.size).toBeGreaterThan(0);
    expect(blob.type).toBe(DOCX_MIME);
    expect(await startsWithPK(blob)).toBe(true);
  });

  it("exportAnnotatedBib handles an empty library without throwing", async () => {
    const blob = await exportAnnotatedBib([], project);
    expect(blob.size).toBeGreaterThan(0);
  });

  it("exportDraftDocx builds a .docx from the student's body (WB)", async () => {
    const blob = await exportDraftDocx("# 引言\n第一段。\n\n第二段的论证。", { title: "中国是否让地球更可持续", qualification: "EPQ" });
    expect(blob.size).toBeGreaterThan(0);
    expect(blob.type).toBe(DOCX_MIME);
    expect(await startsWithPK(blob)).toBe(true);
  });

  it("exportDraftDocx handles an empty body without throwing", async () => {
    const blob = await exportDraftDocx("", { title: "空项目" });
    expect(blob.size).toBeGreaterThan(0);
    expect(await startsWithPK(blob)).toBe(true);
  });
});
