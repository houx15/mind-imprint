import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// WA · the Write room's 整稿体检 (check-my-draft). Mock the thin api modules
// WritingBlock calls directly.
vi.mock("@/workspace/api/workspace", () => ({
  getOutline: vi.fn(async () => []),
  putOutline: vi.fn(async () => []),
  getDraft: vi.fn(async () => "我的草稿第一段。中国在可再生能源上的贡献是实质性的。"),
  coach: vi.fn(async () => ({ reply: "", proposal: null })),
  getCoachHistory: vi.fn(async () => []),
  persistProjectCard: vi.fn(async () => "ci1"),
  dismissProposal: vi.fn(async () => {}),
}));
vi.mock("@/api/writing", () => ({
  putBuffer: vi.fn(async () => {}),
  runDraftReview: vi.fn(),
}));
vi.mock("@/workspace/export", () => ({ exportDraftDocx: vi.fn(async () => new Blob()) }));

import { runDraftReview, putBuffer } from "@/api/writing";
import { exportDraftDocx } from "@/workspace/export";
import { WritingBlock } from "@/workspace/blocks/WritingBlock";

const mockReview = vi.mocked(runDraftReview);
const mockPutBuffer = vi.mocked(putBuffer);
const mockExport = vi.mocked(exportDraftDocx);
const PROPOSAL = { objective: "论证中国是否让地球更可持续", reason: "r", activities: "a", resources: "res" };

beforeEach(() => {
  vi.clearAllMocks();
  mockReview.mockResolvedValue({
    items: [
      { criterion_code: "AO2", criterion_name: "分析与论证", band: "中段", evidence: "给出了一个反例", missing: "反例没有接回主张", fix: "把反例接回你的核心主张" },
    ],
    wordCount: 120,
    inBand: true,
  });
});

async function openDraftTab() {
  render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} onOpenRoom={() => {}} />);
  await userEvent.click(screen.getByRole("button", { name: "写作" }));
  const ta = (await screen.findByPlaceholderText(/在这里写你的草稿/)) as HTMLTextAreaElement;
  // getDraft resolves async — wait for the persisted draft to populate.
  await waitFor(() => expect(ta.value.length).toBeGreaterThan(0));
  return ta;
}

describe("WritingBlock · 整稿体检 (WA)", () => {
  it("runs the review and renders advice read-only, never editing the draft", async () => {
    const ta = await openDraftTab();
    const before = ta.value;
    expect(before).toContain("我的草稿第一段");

    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));

    await waitFor(() => expect(mockReview).toHaveBeenCalledWith("p1", before, "board"));
    expect(await screen.findByText(/把反例接回你的核心主张/)).toBeInTheDocument();
    expect(screen.getByText(/分析与论证/)).toBeInTheDocument();
    // 克制: the review is advice, it must not rewrite the draft.
    expect(ta.value).toBe(before);
  });

  it("re-runs with the chosen voice", async () => {
    await openDraftTab();
    await userEvent.selectOptions(screen.getByLabelText("体检视角"), "sceptic");
    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));
    await waitFor(() => expect(mockReview).toHaveBeenCalledWith("p1", expect.any(String), "sceptic"));
  });

  it("exports the body via 导出成品 (WB)", async () => {
    const ta = await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: /导出成品/ }));
    await waitFor(() => expect(mockExport).toHaveBeenCalledWith(ta.value, expect.objectContaining({ title: "T" })));
  });

  it("goal strip routes to Review to finish (WB)", async () => {
    const onOpenRoom = vi.fn();
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} onOpenRoom={onOpenRoom} />);
    await userEvent.click(screen.getByRole("button", { name: /去完成/ }));
    expect(onOpenRoom).toHaveBeenCalledWith("reflection");
  });

  it("surfaces an error without crashing", async () => {
    mockReview.mockRejectedValueOnce(new Error("boom"));
    await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: "让印记体检整稿" }));
    expect(await screen.findByText(/体检没跑完/)).toBeInTheDocument();
    // autosave still fired before the review attempt
    expect(mockPutBuffer).toHaveBeenCalled();
  });
});
