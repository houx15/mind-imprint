import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { OverviewView } from "@/console/OverviewView";
import type { Overview } from "@/api";

const overview = (over: Partial<Overview> = {}): Overview => ({
  counts: { student: 120, teacher: 8, class: 12, project: 340, evaluation: 95, active_student: 77 },
  usage_by_tier: [{ tier: "chaperone", prompt_tokens: 1000, completion_tokens: 2000, cost: "1.23" }],
  ...over,
});

describe("OverviewView", () => {
  it("renders the six counts and the usage row", async () => {
    const client = { getOverview: vi.fn(async () => overview()) };
    render(<OverviewView client={client} />);
    expect(await screen.findByText("概览")).toBeInTheDocument();
    expect(screen.getByText("120")).toBeInTheDocument();
    expect(screen.getByText("活跃学生")).toBeInTheDocument();
    expect(screen.getByText("chaperone")).toBeInTheDocument();
    expect(screen.getByText("1.23")).toBeInTheDocument();
  });

  it("shows the empty-usage note when usage_by_tier is empty", async () => {
    const client = { getOverview: vi.fn(async () => overview({ usage_by_tier: [] })) };
    render(<OverviewView client={client} />);
    expect(await screen.findByText("暂无用量。")).toBeInTheDocument();
  });
});
