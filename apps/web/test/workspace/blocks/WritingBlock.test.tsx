import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// WA · the Write room's 整稿体检 (check-my-draft). Mock the thin api modules
// WritingBlock calls directly.
vi.mock("@/workspace/api/workspace", () => ({
  getOutline: vi.fn(async () => []),
  putOutline: vi.fn(async () => []),
  getDraft: vi.fn(async () => "我的草稿第一段。中国在可再生能源上的贡献是实质性的。"),
  coach: vi.fn(async () => ({ reply: "", proposal: null, linkOffer: null, dimSuggestion: null })),
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
import { coach } from "@/workspace/api/workspace";
import { exportDraftDocx } from "@/workspace/export";
import { WritingBlock, paragraphAtCaret } from "@/workspace/blocks/WritingBlock";

// WC · M1 — the caret-fallback paragraph math. Real separator offsets, no drift.
describe("paragraphAtCaret (WC · M1)", () => {
  const src = "P0 第一段。\n\nP1 第二段。\n\n\n\nP2 第三段。"; // gaps of 2 and 4 newlines
  it("picks the block the caret sits in (start of a paragraph)", () => {
    const p1Start = src.indexOf("P1");
    expect(paragraphAtCaret(src, p1Start)).toBe("P1 第二段。");
  });
  it("does not drift across a 4-newline gap", () => {
    const p2Start = src.indexOf("P2");
    expect(paragraphAtCaret(src, p2Start)).toBe("P2 第三段。");
    expect(paragraphAtCaret(src, src.length)).toBe("P2 第三段。");
  });
  it("empty text yields empty (button no-ops)", () => {
    expect(paragraphAtCaret("", 0)).toBe("");
  });
});

const mockReview = vi.mocked(runDraftReview);
const mockPutBuffer = vi.mocked(putBuffer);
const mockExport = vi.mocked(exportDraftDocx);
const mockCoach = vi.mocked(coach);
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
  await userEvent.click(screen.getByRole("button", { name: "正文" }));
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

  it("summons a writing card from the deck (WC · card-hang)", async () => {
    render(<WritingBlock projectId="p1" title="T" proposal={PROPOSAL} onOpenRoom={() => {}} />);
    // deck launcher lives in the always-present rail
    await userEvent.click(screen.getByTitle("写作卡"));
    const toulmin = await screen.findByRole("button", { name: /论证构建卡/ });
    await userEvent.click(toulmin);
    // StudioCardSheet mounts (its 工具卡 label + the card name)
    expect(await screen.findByText("工具卡")).toBeInTheDocument();
  });

  it("pins a draft part and scopes the coach turn to it (WC · part-by-part)", async () => {
    await openDraftTab();
    await userEvent.click(screen.getByRole("button", { name: "就这一段问印记" }));
    // pinned chip appears in the rail
    expect(await screen.findByText("就这一段")).toBeInTheDocument();
    const composer = screen.getByPlaceholderText("就这一段，你想问什么？");
    await userEvent.type(composer, "这段够有力吗{Enter}");
    await waitFor(() => {
      const [, scope, turn] = mockCoach.mock.calls.at(-1)!;
      expect(scope).toBe("writing");
      expect(turn).toContain("就这一段想");
      expect(turn).toContain("这段够有力吗");
    });
  });
});
