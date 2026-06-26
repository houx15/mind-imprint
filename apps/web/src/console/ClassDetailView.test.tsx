import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClassDetailView } from "./ClassDetailView";
import type { ClassDetail } from "../api";
import { ApiError } from "../api";

const NOW = Date.parse("2026-06-26T12:00:00Z");

const detail = (over: Partial<ClassDetail> = {}): ClassDetail => ({
  class: { id: "c1", name: "11 年级 A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z" },
  roster: [
    { id: "u1", display_name: "Phoebe", email: "p@d", last_active_at: "2026-06-26T10:00:00Z", task_count: 3, evaluation_count: 1, card_count: 7 },
    { id: "u2", display_name: "Mia", email: "m@d", last_active_at: null, task_count: 0, evaluation_count: 0, card_count: 0 },
  ],
  ...over,
});

function makeClient(d: ClassDetail) {
  return {
    getClass: vi.fn(async () => d),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
  };
}

describe("ClassDetailView roster", () => {
  it("renders the class name, join code, and roster rows with signals", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    expect(await screen.findByText("11 年级 A")).toBeInTheDocument();
    expect(screen.getByText(/AB-CD/)).toBeInTheDocument();
    expect(screen.getByText("Phoebe")).toBeInTheDocument();
    expect(screen.getByText("2 小时前")).toBeInTheDocument();
    expect(screen.getByText("从未")).toBeInTheDocument();
  });

  it("shows the aggregate-only caption and column headers", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    expect(await screen.findByText("姓名")).toBeInTheDocument();
    expect(screen.getByText("任务")).toBeInTheDocument();
    expect(screen.getByText(/暂无学生作品详情/)).toBeInTheDocument();
  });

  it("renders an empty-roster note with the join code", async () => {
    const client = makeClient(detail({ roster: [] }));
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    expect(await screen.findByText(/还没有学生加入/)).toBeInTheDocument();
  });
});

describe("ClassDetailView mutations", () => {
  it("renames the class", async () => {
    const client = makeClient(detail());
    client.renameClass.mockResolvedValue({ ...detail().class, name: "新名字" });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
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
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    await userEvent.click(await screen.findByText("轮换"));
    expect(client.regenerateJoinCode).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText("确认轮换"));
    await waitFor(() => expect(client.regenerateJoinCode).toHaveBeenCalledWith("c1"));
    expect(await screen.findByText(/ZZ-ZZ/)).toBeInTheDocument();
  });

  it("removes a student only after confirming, then drops the row", async () => {
    const client = makeClient(detail());
    client.removeEnrollment.mockResolvedValue(undefined);
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    await userEvent.click(await screen.findByLabelText("移除 Phoebe"));
    expect(client.removeEnrollment).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText("确认移除"));
    await waitFor(() => expect(client.removeEnrollment).toHaveBeenCalledWith("c1", "u1"));
    await waitFor(() => expect(screen.queryByText("Phoebe")).not.toBeInTheDocument());
  });

  it("keeps the detail view visible and shows an inline error when rename fails", async () => {
    const client = makeClient(detail());
    client.renameClass.mockRejectedValue(new ApiError("INTERNAL", "服务器错误", 500));
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
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
