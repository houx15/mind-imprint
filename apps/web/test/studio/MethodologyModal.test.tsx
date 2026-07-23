import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MethodologyModal } from "@/studio/MethodologyModal";

describe("MethodologyModal (工具说明书)", () => {
  it("renders nothing when cardId is null", () => {
    const { container } = render(<MethodologyModal cardId={null} onClose={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the methodology sheet for a known card id and closes via the primary button", () => {
    const onClose = vi.fn();
    render(<MethodologyModal cardId="concession" onClose={onClose} />);
    expect(screen.getByText("工具说明书 · 我不懂为什么")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /明白了/ }));
    expect(onClose).toHaveBeenCalled();
  });

  it("falls back to sensible default copy for an unknown card id", () => {
    render(<MethodologyModal cardId="mystery-card-xyz" onClose={() => {}} />);
    expect(screen.getByText("工具说明书 · 我不懂为什么")).toBeInTheDocument();
  });
});
