import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { WritingView } from "./WritingView";
import { STUDIO_FIXTURE } from "../fixtures";

const draft = STUDIO_FIXTURE.views.writing.draft;

describe("WritingView (写作/S5 shell)", () => {
  it("renders the sub-tabs and the read-only draft textarea in edit mode", () => {
    render(<WritingView draft={draft} mode="edit" />);
    expect(screen.getByText("编辑 · 安静")).toBeInTheDocument();
    expect(screen.getByText("预览 · 批注")).toBeInTheDocument();
    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(textarea.value).toBe(draft);
    expect(textarea).toHaveAttribute("readonly");
    expect(screen.getByText(/想听意见，点「整稿体检」/)).toBeInTheDocument();
  });

  it("shows the rendered paragraphs and a disabled 整稿体检 button in preview mode", () => {
    render(<WritingView draft={draft} mode="preview" />);
    const button = screen.getByRole("button", { name: /整稿体检/ });
    expect(button).toBeInTheDocument();
    expect(button).toBeDisabled();
    const [firstPara] = draft.split("\n\n");
    expect(screen.getByText(firstPara!)).toBeInTheDocument();
  });
});
