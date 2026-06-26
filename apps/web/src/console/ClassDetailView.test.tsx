import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ClassDetailView } from "./ClassDetailView";
import type { ClassDetail } from "../api";

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
