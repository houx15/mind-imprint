import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { loadRegistry } from "@mind-imprint/contracts";
import { CompletedCard } from "./CompletedCard";

const reg = loadRegistry();

describe("CompletedCard", () => {
  it("shows the card name, a completed marker, and the filled count", () => {
    render(<CompletedCard card={reg.concession!} filledCount={3} />);
    expect(screen.getByText("让步段 · 以退为进")).toBeInTheDocument();
    expect(screen.getByText(/已完成/)).toBeInTheDocument();
    expect(screen.getByText(reg.concession!.category)).toBeInTheDocument();
    expect(screen.getByText(/已填 3 项/)).toBeInTheDocument();
  });
});
