import { describe, expect, it } from "vitest";
import { ParentReport, ParentReportProse, ParentStageReport, ParentStageProse } from "../src/parentReport";

const wire = {
  cover: { name: "林知远", subject: "嵌入式广告与正常化", klass: "IBDP 一年级 · 研究组", typeLabel: "项目报告", dateStr: "2026年7月25日", warmLine: "把 AI 当审稿人。" },
  glance: "方法对齐、证据充分", dOverview: "多数维度稳定在熟练", aOverview: "六个方面都观察到主动信号",
  dRows: [
    { code: "D1", name: "任务理解与问题表述", badge: "优秀", reading: "能把宽泛话题收窄。" },
    { code: "D6", name: "反思与元认知", badge: "暂无", reading: "暂无可计入的证据" },
  ],
  aRows: [{ code: "A1", name: "方向自主", state: "观察到主动信号", reading: "自己决定方向。" }],
  opportunity: "机会多由孩子自己创造。",
  advice: [{ title: "请他讲给你听", text: "说清这份研究不能说明什么。" }],
  prose: "present" as const,
};

describe("ParentReport", () => {
  it("accepts a full wire object", () => {
    expect(ParentReport.parse(wire)).toBeTruthy();
  });
  it("accepts prose:null (deterministic-only)", () => {
    expect(ParentReport.parse({ ...wire, prose: null }).prose).toBeNull();
  });
  it("rejects an unknown prose sentinel", () => {
    expect(() => ParentReport.parse({ ...wire, prose: "maybe" })).toThrow();
  });
  it("validates the composer bundle", () => {
    const prose = {
      glance: "g", dOverview: "d", aOverview: "a", opportunity: "o", warmLine: "w",
      dReadings: { D1: "r" }, aReadings: { A1: "r", A2: "r", A3: "r", A4: "r", A5: "r", A6: "r" },
      advice: [{ title: "t", text: "x" }],
    };
    expect(ParentReportProse.parse(prose)).toBeTruthy();
  });
});

describe("ParentStageReport", () => {
  const base = {
    cover: { name: "林知远", subject: "第 30 周（7.20–7.26）", klass: "IBDP 一年级 · 研究组", typeLabel: "阶段报告", dateStr: "2026年7月25日", warmLine: "" },
    stats: [
      { value: "6 天", label: "本周活跃" },
      { value: "78", label: "对话轮次" },
      { value: "3 份", label: "生成报告" },
      { value: "5 节", label: "完成课程" },
    ],
    stageGrowth: "", stageHighlight: "", stageForward: "",
    advice: [],
    prose: null as null | "present",
  };

  it("accepts a deterministic (pre-prose) stage report", () => {
    expect(ParentStageReport.parse(base)).toBeTruthy();
  });

  it("accepts a composed stage report", () => {
    const composed = { ...base, stageGrowth: "有变化", stageHighlight: "亮点", stageForward: "往前看",
      advice: [{ title: "t", text: "x" }], prose: "present" as const };
    expect(ParentStageReport.parse(composed)).toBeTruthy();
  });

  it("rejects a non-sentinel prose value", () => {
    expect(() => ParentStageReport.parse({ ...base, prose: "yes" })).toThrow();
  });

  it("ParentStageProse round-trips the composer bundle", () => {
    expect(ParentStageProse.parse({
      warmLine: "w", stageGrowth: "g", stageHighlight: "", stageForward: "f",
      advice: [{ title: "t", text: "x" }, { title: "t2", text: "y" }, { title: "t3", text: "z" }],
    })).toBeTruthy();
  });
});
