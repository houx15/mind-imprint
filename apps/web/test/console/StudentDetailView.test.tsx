import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StudentDetailView } from "@/console/StudentDetailView";
import type { StudentDetail } from "@/api";
import { ApiError } from "@/api";

const detail = (over: Partial<StudentDetail> = {}): StudentDetail => ({
  student: { id: "u1", displayName: "Phoebe", avatarColor: "#3E7CA8", dBadge: "L3–L4", aBadge: "4.2", unrated: false },
  usage: { activeDays: 5, turns: 42, reportCount: 2, courseCount: 3 },
  // Real wire shape: the server emits time.RFC3339 (e.g. "2026-07-20T14:22:31+08:00"),
  // never a bare date — a fixture of just "2026-07-20" would hide a
  // rendering bug that shows the raw timestamp to the teacher.
  records: [
    { surface: "project", scopeId: "p1", title: "中国是否让地球变得更可持续？", date: "2026-07-20T14:22:31+08:00", status: "能力报告已生成", hasReport: true },
    { surface: "chat", scopeId: "ch1", title: "关于气候变化的讨论", date: "2026-07-18T09:03:00Z", status: "进行中", hasReport: false },
    { surface: "course", scopeId: "co1", title: "信息素养入门", date: "2026-07-15T22:47:11-05:00", status: "已完成", hasReport: true },
  ],
  ...over,
});

function makeClient(d: StudentDetail = detail()) {
  return { getStudentDetail: vi.fn(async () => d) };
}

describe("StudentDetailView", () => {
  it("renders the header name, D/A badges, and four usage cards", async () => {
    const client = makeClient();
    render(<StudentDetailView client={client} classId="c1" userId="u1" onBack={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("Phoebe")).toBeInTheDocument();
    expect(screen.getByText("L3–L4")).toBeInTheDocument();
    expect(screen.getByText("4.2")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument(); // activeDays
    expect(screen.getByText("42")).toBeInTheDocument(); // turns
    expect(screen.getByText("2")).toBeInTheDocument(); // reportCount
    expect(screen.getByText("3")).toBeInTheDocument(); // courseCount
    expect(client.getStudentDetail).toHaveBeenCalledWith("c1", "u1");
  });

  it("filters records by tab: clicking 项目 shows only the project row", async () => {
    render(<StudentDetailView client={makeClient()} classId="c1" userId="u1" onBack={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("中国是否让地球变得更可持续？")).toBeInTheDocument();
    expect(screen.getByText("关于气候变化的讨论")).toBeInTheDocument();
    expect(screen.getByText("信息素养入门")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("tab", { name: "项目" }));
    expect(screen.getByText("中国是否让地球变得更可持续？")).toBeInTheDocument();
    expect(screen.queryByText("关于气候变化的讨论")).not.toBeInTheDocument();
    expect(screen.queryByText("信息素养入门")).not.toBeInTheDocument();
  });

  it("clicking 查看报告 on the project row calls onOpenReport with its surface/scopeId", async () => {
    const onOpenReport = vi.fn();
    render(<StudentDetailView client={makeClient()} classId="c1" userId="u1" onBack={() => {}} onOpenReport={onOpenReport} />);
    await screen.findByText("中国是否让地球变得更可持续？");
    const [firstLink] = screen.getAllByText("查看报告");
    // First 查看报告 belongs to the project row (most recent / first in list)
    if (!firstLink) throw new Error("expected at least one 查看报告 link");
    await userEvent.click(firstLink);
    expect(onOpenReport).toHaveBeenCalledWith("project", "p1", "Phoebe");
  });

  it("a hasReport:false row (the chat record) exposes no 查看报告 affordance", async () => {
    render(<StudentDetailView client={makeClient()} classId="c1" userId="u1" onBack={() => {}} onOpenReport={() => {}} />);
    await screen.findByText("关于气候变化的讨论");
    // Only 2 records have hasReport:true (project + course) out of 3
    expect(screen.getAllByText("查看报告")).toHaveLength(2);
  });

  it("shows the primary 查看完整能力报告 button when a project report is available, wired to onOpenReport", async () => {
    const onOpenReport = vi.fn();
    render(<StudentDetailView client={makeClient()} classId="c1" userId="u1" onBack={() => {}} onOpenReport={onOpenReport} />);
    const btn = await screen.findByText("查看完整能力报告");
    await userEvent.click(btn);
    expect(onOpenReport).toHaveBeenCalledWith("project", "p1", "Phoebe");
  });

  it("hides the primary button when no project record has a report", async () => {
    const d = detail({
      records: [
        { surface: "chat", scopeId: "ch1", title: "讨论", date: "2026-07-18T09:03:00Z", status: "进行中", hasReport: false },
        { surface: "project", scopeId: "p1", title: "进行中的项目", date: "2026-07-19T09:03:00Z", status: "进行中", hasReport: false },
      ],
    });
    render(<StudentDetailView client={makeClient(d)} classId="c1" userId="u1" onBack={() => {}} onOpenReport={() => {}} />);
    await screen.findByText("Phoebe");
    expect(screen.queryByText("查看完整能力报告")).not.toBeInTheDocument();
  });

  it("calls onBack when the 全部学生 link is clicked", async () => {
    const onBack = vi.fn();
    render(<StudentDetailView client={makeClient()} classId="c1" userId="u1" onBack={onBack} onOpenReport={() => {}} />);
    await userEvent.click(await screen.findByText("全部学生"));
    expect(onBack).toHaveBeenCalled();
  });

  it("renders only the date part of an RFC3339 record date, never the raw timestamp", async () => {
    render(<StudentDetailView client={makeClient()} classId="c1" userId="u1" onBack={() => {}} onOpenReport={() => {}} />);
    await screen.findByText("中国是否让地球变得更可持续？");
    expect(screen.getByText("2026-07-20 · 能力报告已生成")).toBeInTheDocument();
    expect(screen.queryByText(/T14:22:31/)).not.toBeInTheDocument();
    expect(screen.queryByText(/\+08:00/)).not.toBeInTheDocument();
  });

  it("shows an error state with 重试 when loading fails, and retries on click", async () => {
    const client = { getStudentDetail: vi.fn() };
    client.getStudentDetail
      .mockRejectedValueOnce(new ApiError("INTERNAL", "服务器错误", 500))
      .mockResolvedValueOnce(detail());
    render(<StudentDetailView client={client} classId="c1" userId="u1" onBack={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText(/服务器错误/)).toBeInTheDocument();
    await userEvent.click(screen.getByText("重试"));
    expect(await screen.findByText("Phoebe")).toBeInTheDocument();
  });
});
