import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

vi.mock("@/workspace/api/workspace", () => ({
  getLog: vi.fn(async () => [{ id: "l1", date: "08-01", text: "确认了研究问题", source: "auto" }]),
  getDraft: vi.fn(async (_id: string, doc: string) => (doc === "essay" ? "# 成品\n正文内容" : "# 提案\n提案内容")),
}));

import { ReviewArtifacts } from "@/workspace/blocks/ReviewArtifacts";
import type { Proposal } from "@mind-imprint/contracts";

const PROPOSAL: Proposal = {
  objective: "中国是否让地球更可持续？", reason: "个人兴趣", activities: "阅读+写作", resources: "NASA 数据", counterpoints: "碳排放第一",
};

describe("ReviewArtifacts", () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it("renders the four tabs; log is default; switching shows framework + finished paper", async () => {
    render(<ReviewArtifacts projectId="p1" proposal={PROPOSAL} />);
    // four tabs
    expect(screen.getByText("活动日志")).toBeTruthy();
    expect(screen.getByText("研究框架")).toBeTruthy();
    expect(screen.getByText("提案")).toBeTruthy();
    expect(screen.getByText("成品")).toBeTruthy();
    // log default
    expect(await screen.findByText("确认了研究问题")).toBeTruthy();
    // framework tab shows the objective
    fireEvent.click(screen.getByText("研究框架"));
    expect(await screen.findByText("中国是否让地球更可持续？")).toBeTruthy();
    // 成品 tab shows the finished paper
    fireEvent.click(screen.getByText("成品"));
    expect(await screen.findByText(/正文内容/)).toBeTruthy();
  });
});
