import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ClassWeeklyView } from "@/console/ClassWeeklyView";
import type { WeeklyReport } from "@/api/teacher";

function report(over: Partial<WeeklyReport> = {}): WeeklyReport {
  return {
    weekLabel: "第 30 周（7.20–7.26）",
    weekStart: "2026-07-20T00:00:00.000Z",
    weekEnd: "2026-07-27T00:00:00.000Z",
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
    comment: null,
    proseReady: false,
    isLatestWeek: true,
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

  it("colors the delta pill by server deltaDir, not a fixed color", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "c" })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    // "turns" stat has deltaDir "down" in the fixture — down-style tokens, not the up-style green.
    const pill = await screen.findByText("-72");
    expect(pill).toHaveStyle({ color: "var(--mk-danger)", background: "var(--mk-danger-bg)" });
  });

  it("colors a praise card's avatar and tag chip with the fixed praise tokens, ignoring avatarColor", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({
        praise: [{
          userId: "u2", displayName: "林知远", avatarColor: "#3E7CA8", // a non-matcha server color
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
    // Fixed matcha tokens, NOT the server's avatarColor (#3E7CA8).
    expect(tag).toHaveStyle({ color: "var(--mk-matcha-fg)", background: "var(--mk-matcha-bg)" });
  });

  it("renders a distinct error with a retry when the fetch fails", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockRejectedValue(new Error("boom")),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("重试")).toBeInTheDocument();
  });

  it("navigates to the previous week and re-fetches with the shifted weekStart", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "c" })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    await screen.findByText("本周活跃学生");

    fireEvent.click(screen.getByLabelText("上一周"));

    await waitFor(() => expect(client.getClassWeeklyReport).toHaveBeenCalledTimes(2));
    expect(client.getClassWeeklyReport).toHaveBeenLastCalledWith("c1", "2026-07-13T00:00:00.000Z");
  });

  it("disables the next-week button once the server says this is the latest week", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "c", isLatestWeek: true })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    await screen.findByText("本周活跃学生");
    expect(screen.getByLabelText("下一周")).toBeDisabled();
  });

  it("enables the next-week button and shifts forward when not the latest week", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "c", isLatestWeek: false })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    await screen.findByText("本周活跃学生");
    const next = screen.getByLabelText("下一周");
    expect(next).not.toBeDisabled();

    fireEvent.click(next);

    await waitFor(() => expect(client.getClassWeeklyReport).toHaveBeenCalledTimes(2));
    expect(client.getClassWeeklyReport).toHaveBeenLastCalledWith("c1", "2026-07-27T00:00:00.000Z");
  });

  it("generates prose again for a different week after navigating", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ isLatestWeek: false })),
      generateClassWeeklyProse: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "c", isLatestWeek: false })),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    await waitFor(() => expect(client.generateClassWeeklyProse).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByLabelText("下一周"));

    await waitFor(() => expect(client.generateClassWeeklyProse).toHaveBeenCalledTimes(2));
  });
});
