import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ChatReport } from "./ChatReport";
import { api } from "../../api";

vi.mock("../../api", () => ({ api: { getChatAssessment: vi.fn(), generateChatAssessment: vi.fn() } }));

const sample = {
  dimensions: [{ code: "D1", name: "问题意识", level: "L3", evidence: "你追问了三次" }],
  narrative: "你的问题越来越锋利。",
  generatedAt: "2026-07-18T00:00:00Z",
};

beforeEach(() => { vi.clearAllMocks(); });

describe("ChatReport", () => {
  it("shows the opt-in CTA and never auto-generates when no report exists", async () => {
    (api.getChatAssessment as any).mockResolvedValue(null);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    await screen.findByText("生成本次对话的思维印记");
    expect(api.generateChatAssessment).not.toHaveBeenCalled(); // 铁律 2: opt-in only
  });

  it("generates on click and renders the dimensions + narrative", async () => {
    (api.getChatAssessment as any).mockResolvedValue(null);
    (api.generateChatAssessment as any).mockResolvedValue(sample);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    fireEvent.click(await screen.findByText("生成本次对话的思维印记"));
    await waitFor(() => expect(screen.getByText("问题意识")).toBeInTheDocument());
    expect(screen.getByText("你的问题越来越锋利。")).toBeInTheDocument();
  });

  it("rehydrates a stored report on mount without generating", async () => {
    (api.getChatAssessment as any).mockResolvedValue(sample);
    render(<ChatReport threadId="t1" onClose={() => {}} />);
    await waitFor(() => expect(screen.getByText("问题意识")).toBeInTheDocument());
    expect(api.generateChatAssessment).not.toHaveBeenCalled();
  });
});
