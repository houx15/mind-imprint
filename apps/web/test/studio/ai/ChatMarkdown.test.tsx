import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";

/**
 * ChatMarkdown (Round 3, Part 1): the shared chat-bubble markdown renderer
 * replacing the four duplicated `renderRich` copies that only ever handled
 * `**bold**`. These tests exercise the renderer directly — the mappers that
 * wire it into each room (StudioCoachChat/PlanBlock/WritingBlock/ReviewBlock)
 * are covered by their own toChatMessages tests.
 */
describe("ChatMarkdown", () => {
  it("renders a bullet list as real <li> elements, not raw '- ' text", () => {
    render(<ChatMarkdown text={"- a\n- b"} />);
    expect(screen.getByText("a").tagName).toBe("LI");
    expect(screen.getByText("b").tagName).toBe("LI");
    expect(screen.queryByText(/^- a/)).toBeNull();
  });

  it("renders a markdown link as a real anchor, opening in a new tab safely", () => {
    render(<ChatMarkdown text={"[点这里](https://e.com)"} />);
    const link = screen.getByRole("link", { name: "点这里" });
    expect(link).toHaveAttribute("href", "https://e.com");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("renders **bold** as a real <strong>, not literal asterisks", () => {
    render(<ChatMarkdown text={"这是**重点**内容"} />);
    const strong = screen.getByText("重点");
    expect(strong.tagName).toBe("STRONG");
    expect(screen.queryByText(/\*\*/)).toBeNull();
  });
});
