import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClassDetailView } from "@/console/ClassDetailView";
import type { ClassDetail, ClassRoster } from "@/api";
import type { WeeklyReport } from "@/api/teacher";
import { ApiError } from "@/api";

const detail = (over: Partial<ClassDetail> = {}): ClassDetail => ({
  class: { id: "c1", name: "11 年级 A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z" },
  roster: [
    { id: "u1", display_name: "Phoebe", email: "p@d", last_active_at: "2026-06-26T10:00:00Z", project_count: 3, evaluation_count: 1, card_count: 7 },
    { id: "u2", display_name: "Mia", email: "m@d", last_active_at: null, project_count: 0, evaluation_count: 0, card_count: 0 },
  ],
  teachers: [{ id: "t1", display_name: "Ms Chen", email: "chen@x" }],
  ...over,
});

const classRoster = (): ClassRoster => ({
  roster: [
    { id: "u1", displayName: "Phoebe", avatarColor: "#3E7CA8", activeProjects: 2, reportCount: 1, coursesFinished: 3 },
    { id: "u2", displayName: "Mia", avatarColor: "#9198A8", activeProjects: 0, reportCount: 0, coursesFinished: 0 },
  ],
  header: { classSize: 2, activeStudents: 1, activeProjects: 2, turns: 42, reports: 1 },
});

// Minimal but shape-accurate WeeklyReport fixture (proseReady:true so
// ClassWeeklyView, mounted by default behind the 周报 tab, never calls
// generateClassWeeklyProse in these roster/mutation-focused tests).
const weeklyReport = (): WeeklyReport => ({
  weekLabel: "第 30 周（7.20–7.26）",
  weekStart: "2026-07-20T00:00:00Z",
  weekEnd: "2026-07-27T00:00:00Z",
  asOf: "2026-07-24T07:30:00Z",
  className: "11 年级 A",
  classSize: 2,
  stats: [
    { key: "active_students", label: "本周活跃学生", value: 2, unit: "/ 2 人", foot: "登录并有活动的学生", delta: "±0", deltaDir: "flat" },
    { key: "reports", label: "生成能力报告", value: 0, unit: "份", foot: "来自项目、对话与课程", delta: "±0", deltaDir: "flat" },
    { key: "turns", label: "AI 对话轮次", value: 42, unit: "轮", foot: "反映本周使用强度", delta: "±0", deltaDir: "flat" },
    { key: "course_steps", label: "完成课程节", value: 0, unit: "节", foot: "平台内自学课程", delta: "±0", deltaDir: "flat" },
  ],
  praise: [],
  watch: [],
  isLatestWeek: true,
  comment: "本周点评",
  proseReady: true,
});

function makeClient(d: ClassDetail, roster: ClassRoster = classRoster()) {
  return {
    getClass: vi.fn(async () => d),
    getClassRosterReport: vi.fn(async () => roster),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
    listTeachers: vi.fn(async () => [{ id: "t2", display_name: "Mr Li", email: "li@x" }]),
    assignTeacher: vi.fn(async () => ({ teachers: [{ id: "t1", display_name: "Ms Chen", email: "chen@x" }, { id: "t2", display_name: "Mr Li", email: "li@x" }] })),
    removeTeacher: vi.fn(async () => undefined),
    getClassWeeklyReport: vi.fn(async () => weeklyReport()),
    generateClassWeeklyProse: vi.fn(),
  };
}

const noop = () => {};

describe("ClassDetailView sub-tabs", () => {
  it("lands on the weekly tab and shows the roster only after switching", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    expect(await screen.findByText(/班级周报/)).toBeInTheDocument();
    expect(screen.queryByText("进行中项目")).not.toBeInTheDocument();
    await userEvent.click(screen.getByText("全部学生"));
    expect(await screen.findByText("对话轮次")).toBeInTheDocument();
  });
});

describe("ClassDetailView roster", () => {
  it("renders the class name, join code, live header, and roster rows with activity counts", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    expect(await screen.findByText("11 年级 A")).toBeInTheDocument();
    expect(screen.getByText(/AB-CD/)).toBeInTheDocument();
    await userEvent.click(screen.getByText("全部学生"));
    expect(await screen.findByText("Phoebe")).toBeInTheDocument();
    expect(screen.getByText("Mia")).toBeInTheDocument();
    // live header strip
    expect(screen.getByText("活跃学生")).toBeInTheDocument();
    expect(screen.getByText("1 / 2")).toBeInTheDocument();
    // roster activity counts (Phoebe's row)
    const row = screen.getByText("Phoebe").closest("tr")!;
    expect(row).toHaveTextContent("2"); // activeProjects
    expect(row).toHaveTextContent("1"); // reportCount
    expect(row).toHaveTextContent("3"); // coursesFinished
  });

  it("shows the enriched column headers", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("全部学生"));
    expect(await screen.findByText("学生")).toBeInTheDocument();
    // "进行中项目" and "能力报告" also appear in the live-header strip above the table.
    expect(screen.getAllByText("进行中项目").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("能力报告").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("完成课程")).toBeInTheDocument();
  });

  it("roster rows are clickable and open the student detail view", async () => {
    const client = makeClient(detail());
    const onOpenStudent = vi.fn();
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={onOpenStudent} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("全部学生"));
    const cell = await screen.findByText("Phoebe");
    await userEvent.click(cell);
    expect(onOpenStudent).toHaveBeenCalledWith("u1");
  });

  it("clicking the remove control does not also open the student (event does not bubble)", async () => {
    const client = makeClient(detail());
    const onOpenStudent = vi.fn();
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={onOpenStudent} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("全部学生"));
    await userEvent.click(await screen.findByLabelText("移除 Phoebe"));
    expect(onOpenStudent).not.toHaveBeenCalled();
  });

  it("renders an empty-roster note with the join code", async () => {
    const client = makeClient(detail({ roster: [] }), { roster: [], header: { classSize: 0, activeStudents: 0, activeProjects: 0, turns: 0, reports: 0 } });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("全部学生"));
    expect(await screen.findByText(/还没有学生加入/)).toBeInTheDocument();
  });

  it("a roster-report failure renders an error message with 重试, never the 'class is empty' copy (the class DOES have students)", async () => {
    const client = makeClient(detail()); // detail().roster has 2 students
    client.getClassRosterReport = vi.fn().mockRejectedValue(new ApiError("INTERNAL", "服务器错误", 500));
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);

    await screen.findByText("11 年级 A"); // class detail itself loaded fine
    await userEvent.click(screen.getByText("全部学生"));
    expect(await screen.findByText(/服务器错误/)).toBeInTheDocument();
    expect(screen.queryByText(/还没有学生加入/)).not.toBeInTheDocument();
    // No headed table silently rendered with zero rows.
    expect(screen.queryByText("学生")).not.toBeInTheDocument();
    expect(screen.queryByText("Phoebe")).not.toBeInTheDocument();

    client.getClassRosterReport = vi.fn().mockResolvedValue(classRoster());
    await userEvent.click(screen.getByText("重试"));
    expect(await screen.findByText("Phoebe")).toBeInTheDocument();
  });
});

describe("ClassDetailView mutations", () => {
  it("renames the class", async () => {
    const client = makeClient(detail());
    client.renameClass.mockResolvedValue({ ...detail().class, name: "新名字" });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("改名"));
    const input = screen.getByDisplayValue("11 年级 A");
    await userEvent.clear(input);
    await userEvent.type(input, "新名字");
    await userEvent.click(screen.getByText("保存"));
    await waitFor(() => expect(client.renameClass).toHaveBeenCalledWith("c1", "新名字"));
    expect(await screen.findByText("新名字")).toBeInTheDocument();
  });

  it("regenerates the join code only after confirming", async () => {
    const client = makeClient(detail());
    client.regenerateJoinCode.mockResolvedValue({ ...detail().class, join_code: "ZZ-ZZ" });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("轮换"));
    expect(client.regenerateJoinCode).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText("确认轮换"));
    await waitFor(() => expect(client.regenerateJoinCode).toHaveBeenCalledWith("c1"));
    expect(await screen.findByText(/ZZ-ZZ/)).toBeInTheDocument();
  });

  it("removes a student only after confirming, then drops the row", async () => {
    const client = makeClient(detail());
    client.removeEnrollment.mockResolvedValue(undefined);
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("全部学生"));
    await userEvent.click(await screen.findByLabelText("移除 Phoebe"));
    expect(client.removeEnrollment).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText("确认移除"));
    await waitFor(() => expect(client.removeEnrollment).toHaveBeenCalledWith("c1", "u1"));
    await waitFor(() => expect(screen.queryByText("Phoebe")).not.toBeInTheDocument());
  });

  it("keeps the detail view visible and shows an inline error when rename fails", async () => {
    const client = makeClient(detail());
    client.renameClass.mockRejectedValue(new ApiError("INTERNAL", "服务器错误", 500));
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.click(await screen.findByText("全部学生"));
    await userEvent.click(await screen.findByText("改名"));
    const input = screen.getByDisplayValue("11 年级 A");
    await userEvent.clear(input);
    await userEvent.type(input, "新名字");
    await userEvent.click(screen.getByText("保存"));
    await waitFor(() => expect(client.renameClass).toHaveBeenCalled());
    // Detail view (roster) is still visible — NOT replaced by the full-page load-error banner
    expect(screen.getByText("Phoebe")).toBeInTheDocument();
    // Inline error message is shown
    expect(await screen.findByText("服务器错误")).toBeInTheDocument();
    // The full-page "重试" link is NOT present
    expect(screen.queryByText("重试")).not.toBeInTheDocument();
  });
});

describe("ClassDetailView admin teacher section", () => {
  it("teacher role sees no teacher-management section", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} role="teacher" onOpenStudent={() => {}} onOpenReport={noop} />);
    await screen.findByText("11 年级 A");
    expect(screen.queryByText("任课教师")).not.toBeInTheDocument();
  });

  it("admin sees current teachers and can assign another", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} role="admin" onOpenStudent={() => {}} onOpenReport={noop} />);
    expect(await screen.findByText("任课教师")).toBeInTheDocument();
    expect(screen.getByText("Ms Chen")).toBeInTheDocument();
    await userEvent.selectOptions(await screen.findByTestId("assign-teacher-picker"), "t2");
    await userEvent.click(screen.getByText("添加"));
    await waitFor(() => expect(client.assignTeacher).toHaveBeenCalledWith("c1", "t2"));
    expect(await screen.findByText("Mr Li")).toBeInTheDocument();
  });

  it("admin can remove a teacher", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} role="admin" onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.click(await screen.findByLabelText("移除教师 Ms Chen"));
    await waitFor(() => expect(client.removeTeacher).toHaveBeenCalledWith("c1", "t1"));
  });
});
