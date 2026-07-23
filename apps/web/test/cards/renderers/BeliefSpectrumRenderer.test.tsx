import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { loadRegistry, type CardSpec } from "@mind-imprint/contracts";
import { BeliefSpectrumRenderer } from "@/cards/renderers/BeliefSpectrumRenderer";

const card = loadRegistry()["belief-spectrum"] as CardSpec;
const selfField = card.steps[0]!.fields.find((f) => f.key === "self") as { stops: string[] };

describe("BeliefSpectrumRenderer", () => {
  it("writes the self position as a stop index via onField", () => {
    const onField = vi.fn();
    render(<BeliefSpectrumRenderer card={card} values={{}} onField={onField} onExpandStep={vi.fn()} onNote={vi.fn()} />);
    // values={} → no stances yet, so the only axis row is 我 (self).
    fireEvent.click(screen.getByRole("radio", { name: selfField.stops[2] }));
    expect(onField).toHaveBeenCalledWith("self", 2);
  });

  it("renders the issue text field and the methodology panel trigger", () => {
    render(<BeliefSpectrumRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} onNote={vi.fn()} />);
    expect(screen.getByText("争议议题是什么？")).toBeTruthy();
    expect(screen.getByText("方法")).toBeTruthy();
  });

  it("fires onNote when the methodology panel is expanded", () => {
    const onNote = vi.fn();
    render(<BeliefSpectrumRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} onNote={onNote} />);
    fireEvent.click(screen.getByText("方法"));
    expect(onNote).toHaveBeenCalledWith("main");
  });

  it("adds a stance and writes the whole stances array back via onField", () => {
    const onField = vi.fn();
    render(<BeliefSpectrumRenderer card={card} values={{}} onField={onField} onExpandStep={vi.fn()} onNote={vi.fn()} />);
    fireEvent.click(screen.getByText("+ 添加一方"));
    expect(onField).toHaveBeenCalledWith("stances", expect.arrayContaining([expect.objectContaining({ position: expect.anything() })]));
  });
});
