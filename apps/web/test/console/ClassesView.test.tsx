import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClassesView } from "@/console/ClassesView";
import type { ClassSummary } from "@/api";

const cls = (over: Partial<ClassSummary> = {}): ClassSummary => ({
  id: "c1", name: "11 年级 A · TOK", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z", ...over,
});

describe("ClassesView", () => {
  it("teacher: shows 我的班级, lists classes, shows the join code and create button", async () => {
    const client = { listClasses: vi.fn(async () => [cls()]), createClass: vi.fn(), listTeachers: vi.fn(async () => []) };
    render(<ClassesView client={client} role="teacher" onOpenClass={() => {}} />);
    expect(await screen.findByText("我的班级")).toBeInTheDocument();
    expect(screen.getByText("11 年级 A · TOK")).toBeInTheDocument();
    expect(screen.getByText(/AB-CD/)).toBeInTheDocument();
    expect(screen.getByText("+ 新建班级")).toBeInTheDocument();
  });

  it("admin: shows 全校班级 and has create button", async () => {
    const client = { listClasses: vi.fn(async () => [cls()]), createClass: vi.fn(), listTeachers: vi.fn(async () => []) };
    render(<ClassesView client={client} role="admin" onOpenClass={() => {}} />);
    expect(await screen.findByText("全校班级")).toBeInTheDocument();
    expect(screen.getByText("+ 新建班级")).toBeInTheDocument();
  });

  it("teacher empty state prompts to create the first class", async () => {
    const client = { listClasses: vi.fn(async () => []), createClass: vi.fn(), listTeachers: vi.fn(async () => []) };
    render(<ClassesView client={client} role="teacher" onOpenClass={() => {}} />);
    expect(await screen.findByText(/还没有班级/)).toBeInTheDocument();
  });

  it("clicking a class card calls onOpenClass with its id", async () => {
    const client = { listClasses: vi.fn(async () => [cls()]), createClass: vi.fn(), listTeachers: vi.fn(async () => []) };
    const onOpenClass = vi.fn();
    render(<ClassesView client={client} role="teacher" onOpenClass={onOpenClass} />);
    await userEvent.click(await screen.findByText("11 年级 A · TOK"));
    expect(onOpenClass).toHaveBeenCalledWith("c1");
  });

  it("create flow calls createClass and surfaces the new join code", async () => {
    const created = cls({ id: "c2", name: "新班", join_code: "EF-GH" });
    const client = { listClasses: vi.fn(async () => []), createClass: vi.fn(async () => created), listTeachers: vi.fn(async () => []) };
    render(<ClassesView client={client} role="teacher" onOpenClass={() => {}} />);
    await userEvent.click(await screen.findByText("+ 新建班级"));
    await userEvent.type(screen.getByPlaceholderText(/班级名称/), "新班");
    await userEvent.click(screen.getByText("创建"));
    await waitFor(() => expect(client.createClass).toHaveBeenCalledWith({ name: "新班" }));
    expect(await screen.findByText(/EF-GH/)).toBeInTheDocument();
  });

  it("admin create flow uses a teacher picker and sends teacher_user_id", async () => {
    const created = cls({ id: "c9", name: "新建", join_code: "EF-GH" });
    const client = {
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(async () => created),
      listTeachers: vi.fn(async () => [{ id: "u1", display_name: "Ms Chen", email: "chen@x" }]),
    };
    render(<ClassesView client={client} role="admin" onOpenClass={() => {}} />);
    await screen.findByText("全校班级");
    await userEvent.click(screen.getByText("+ 新建班级"));
    await userEvent.type(screen.getByPlaceholderText(/班级名称/), "新建");
    await userEvent.selectOptions(await screen.findByTestId("teacher-picker"), "u1");
    await userEvent.click(screen.getByText("创建"));
    await waitFor(() => expect(client.createClass).toHaveBeenCalledWith({ name: "新建", teacher_user_id: "u1" }));
  });

  it("admin with no teachers cannot create (submit disabled)", async () => {
    const client = {
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(),
      listTeachers: vi.fn(async () => []),
    };
    render(<ClassesView client={client} role="admin" onOpenClass={() => {}} />);
    await userEvent.click(await screen.findByText("+ 新建班级"));
    expect(await screen.findByText(/请先在「教师」生成邀请码/)).toBeInTheDocument();
    expect(screen.getByText("创建")).toBeDisabled();
  });
});
