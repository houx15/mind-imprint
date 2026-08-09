import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { ProsePane } from "@/workspace/blocks/ProsePane";

// ProsePane is the proposal's plain prose surface. What matters here (Phase B):
// it loads AND saves against the SAME per-doc buffer the backend keys on
// `doc_kind` — so the proposal never collides with the essay.
vi.mock("@/workspace/api/workspace", () => ({ getDraft: vi.fn(async () => "已有的提案内容") }));
vi.mock("@/api/writing", () => ({ putBuffer: vi.fn(async () => {}) }));

import { getDraft } from "@/workspace/api/workspace";
import { putBuffer } from "@/api/writing";
const mockGetDraft = vi.mocked(getDraft);
const mockPut = vi.mocked(putBuffer);

beforeEach(() => {
  mockGetDraft.mockClear();
  mockPut.mockClear();
});

describe("ProsePane", () => {
  it("loads the buffer for its document kind into the textarea", async () => {
    render(<ProsePane projectId="p1" doc="proposal" />);
    const ta = (await screen.findByLabelText("研究提案正文")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value).toBe("已有的提案内容"));
    expect(mockGetDraft).toHaveBeenCalledWith("p1", "proposal");
  });

  it("autosaves edits to the same document's buffer (debounced)", async () => {
    render(<ProsePane projectId="p1" doc="proposal" />);
    const ta = (await screen.findByLabelText("研究提案正文")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value.length).toBeGreaterThan(0));
    fireEvent.change(ta, { target: { value: "我的新提案：探究印刷术与宗教改革。" } });
    await waitFor(() => expect(mockPut).toHaveBeenCalledWith("p1", "我的新提案：探究印刷术与宗教改革。", "proposal"), {
      timeout: 3000,
    });
  });

  it("is read-only when locked", async () => {
    render(<ProsePane projectId="p1" doc="proposal" locked />);
    const ta = (await screen.findByLabelText("研究提案正文")) as HTMLTextAreaElement;
    await waitFor(() => expect(ta.value.length).toBeGreaterThan(0));
    expect(ta).toHaveAttribute("readonly");
  });
});
