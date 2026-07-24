import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ClassWeeklyView } from "@/console/ClassWeeklyView";
import type { WeeklyReport } from "@/api/teacher";

function report(over: Partial<WeeklyReport> = {}): WeeklyReport {
  return {
    weekLabel: "第 30 周（7.20–7.26）",
    weekStart: "2026-07-20T00:00:00Z",
    weekEnd: "2026-07-27T00:00:00Z",
    asOf: "2026-07-24T07:30:00Z",
    className: "IBDP 一年级 · 研究组",
    classSize: 9,
    stats: [
      { key: "active_students", label: "本周活跃学生", value: 8, unit: "/ 9 人", foot: "登录并有活动的学生", delta: "+2", deltaDir: "up" },
      { key: "reports", label: "生成能力报告", value: 14, unit: "份", foot: "来自项目、对话与课程", delta: "±0", deltaDir: "flat" },
      { key: "turns", label: "AI 对话轮次", value: 386, unit: "轮", foot: "反映本周使用强度", delta: "-72", deltaDir: "down" },
      { key: "course_steps", label: "完成课程节", value: 23, unit: "节", foot: "平台内自学课程", delta: "+4", deltaDir: "up" },
    ],
    praise: [],
    watch: [{
      userId: "u1", displayName: "周子墨", avatarColor: "#C4574D",
      tagCode: "outsourced_judgment", tagLabel: "判断在外包", kind: "watch",
      evidence: "A 轴 0.5/5。提示词多为「帮我写一段」。",
      lead: "", action: "", hasReport: true, reportSurface: "project", reportScopeId: "p1",
    }],
    depth: { buckets: [
      { code: "L1", label: "起步 L1", count: 2 }, { code: "L2", label: "发展 L2", count: 3 },
      { code: "L3", label: "熟练 L3", count: 2 }, { code: "L4", label: "优秀 L4", count: 1 },
    ], ratedCount: 8, note: "" },
    autonomy: { mean: "2.6", delta: "+0.4", deltaDir: "up", ratedCount: 8, note: "" },
    comment: null,
    proseReady: false,
    ...over,
  };
}

describe("ClassWeeklyView", () => {
  it("renders the four stat cards with their deltas", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "本周点评" })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("本周活跃学生")).toBeInTheDocument();
    expect(screen.getByText("386")).toBeInTheDocument();
    expect(screen.getByText("±0")).toBeInTheDocument();
    expect(screen.getByText("-72")).toBeInTheDocument();
    expect(client.generateClassWeeklyProse).not.toHaveBeenCalled();
  });

  it("requests prose exactly once when it is missing", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report()),
      generateClassWeeklyProse: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "生成后的点评" })),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("生成后的点评")).toBeInTheDocument();
    await waitFor(() => expect(client.generateClassWeeklyProse).toHaveBeenCalledTimes(1));
  });

  it("renders a card's evidence even with no wording", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report()),
      generateClassWeeklyProse: vi.fn().mockResolvedValue(report()),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("周子墨")).toBeInTheDocument();
    expect(screen.getByText("判断在外包")).toBeInTheDocument();
    expect(screen.getByText(/A 轴 0.5\/5/)).toBeInTheDocument();
    expect(screen.getByText("本周点评暂未生成")).toBeInTheDocument();
  });

  it("shows the empty state when nobody needs attention", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ watch: [], praise: [], proseReady: true, comment: "c" })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("本周没有需要特别关注的学生")).toBeInTheDocument();
  });

  it("renders the unrated distribution empty state", async () => {
    // The server ALWAYS sends all four buckets — an unrated class sends them
    // with count 0, never an empty array. Mocking [] here would test a shape
    // the backend cannot produce.
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({
        depth: { buckets: [
          { code: "L1", label: "起步 L1", count: 0 }, { code: "L2", label: "发展 L2", count: 0 },
          { code: "L3", label: "熟练 L3", count: 0 }, { code: "L4", label: "优秀 L4", count: 0 },
        ], ratedCount: 0, note: "" },
        autonomy: { mean: "—", delta: "—", deltaDir: "flat", ratedCount: 0, note: "" },
        proseReady: true, comment: "c",
      })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("暂无可计入的证据")).toBeInTheDocument();
  });

  it("colors the A-axis delta pill by server deltaDir, never green when the class declines", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({
        autonomy: { mean: "2.1", delta: "-0.4", deltaDir: "down", ratedCount: 8, note: "" },
        proseReady: true, comment: "c",
      })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    const pill = await screen.findByText("-0.4");
    // Down-style (red), not the fixed green the design's own prototype hardcodes.
    expect(pill).toHaveStyle({ color: "#C4574D", background: "#F7E6E4" });
  });

  it("colors a praise card's avatar and tag chip with the design's fixed green, ignoring avatarColor", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({
        praise: [{
          userId: "u2", displayName: "林知远", avatarColor: "#3E7CA8", // a non-green server color
          tagCode: "depth_up", tagLabel: "深度升档", kind: "praise",
          evidence: "L2 → L3。", lead: "", action: "", hasReport: true, reportSurface: "project", reportScopeId: "p2",
        }],
        watch: [],
        proseReady: true, comment: "c",
      })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    const tag = await screen.findByText("深度升档");
    // Fixed design green (dc.html:134-135), NOT the server's avatarColor (#3E7CA8).
    expect(tag).toHaveStyle({ color: "#3E8A6E", background: "#E4F0EA" });
  });

  it("renders a distinct error with a retry when the fetch fails", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockRejectedValue(new Error("boom")),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("重试")).toBeInTheDocument();
  });
});
