import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProposalAnnotationGroup } from "@/workspace/blocks/ReferencePanel";
import type { DraftAnnotation } from "@mind-imprint/contracts";

const items: DraftAnnotation[] = [
  { id: "1", level: "paper", nature: "good", quote: "", locator: "", note: "整体结构清晰" },
  { id: "2", level: "paragraph", nature: "suggest", quote: "", locator: "第2段", note: "可以先交代背景" },
  { id: "3", level: "sentence", nature: "problem", quote: "中国一定会成功", locator: "第3段", note: "这是断言，缺证据" },
];

describe("ProposalAnnotationGroup", () => {
  it("renders 总体/段落/句子 with the right colors and only the sentence underlined", () => {
    render(<ProposalAnnotationGroup items={items} />);

    // paper summary (green good)
    expect(screen.getByText("总体")).toBeTruthy();
    expect(screen.getByText("整体结构清晰")).toBeTruthy();

    // paragraph: locator + note, NO underline on the note
    expect(screen.getByText(/第2段/)).toBeTruthy();
    const paraNote = screen.getByText("可以先交代背景");
    expect(paraNote.className).not.toContain("underline");

    // sentence: quoted text underlined + colored red (problem)
    const quote = screen.getByText(/中国一定会成功/);
    expect(quote.className).toContain("underline");
    expect(quote.className).toContain("text-mk-danger");
    expect(screen.getByText("这是断言，缺证据")).toBeTruthy();
  });

  it("shows a calm line when there are no 批注", () => {
    render(<ProposalAnnotationGroup items={[]} />);
    expect(screen.getByText(/批注会在印记看过你的写作后出现/)).toBeTruthy();
  });
});
